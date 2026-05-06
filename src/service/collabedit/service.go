package collabedit

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"paperlink/db/repo"
	"paperlink/util"

	"golang.org/x/net/websocket"
)

var log = util.GroupLog("COLLAB")

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrForbidden        = errors.New("forbidden")
	ErrTokenRequired    = errors.New("token required")
	ErrTokenInvalid     = errors.New("token invalid")
	ErrTokenExpired     = errors.New("token expired")
	ErrUserNotFound     = errors.New("user not found")
)

type User struct {
	UserID   int    `json:"userId"`
	Username string `json:"username"`
}

type TokenResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type annotationLockMessage struct {
	AnnotationID  int    `json:"annotationId"`
	User          User   `json:"user"`
	OwnerClientID string `json:"ownerClientId"`
	LockedAt      int64  `json:"lockedAt"`
}

type outboundMessage struct {
	Type            string                  `json:"type"`
	DocumentID      string                  `json:"documentId,omitempty"`
	ClientID        string                  `json:"clientId,omitempty"`
	User            *User                   `json:"user,omitempty"`
	Users           []User                  `json:"users,omitempty"`
	Page            *int64                  `json:"page,omitempty"`
	Annotation      *annotationMessage      `json:"annotation,omitempty"`
	Annotations     []annotationMessage     `json:"annotations,omitempty"`
	AnnotationID    *int                    `json:"annotationId,omitempty"`
	AnnotationLock  *annotationLockMessage  `json:"annotationLock,omitempty"`
	AnnotationLocks []annotationLockMessage `json:"annotationLocks,omitempty"`
	Error           string                  `json:"error,omitempty"`
}

type inboundMessage struct {
	Type         string             `json:"type"`
	Page         *int64             `json:"page,omitempty"`
	Annotation   *annotationMessage `json:"annotation,omitempty"`
	AnnotationID *int               `json:"annotationId,omitempty"`
}

type Service struct {
	mu           sync.RWMutex
	rooms        map[string]*room
	tokens       *tokenStore
	annotations  *AnnotationStore
	nextClientID uint64
}

func NewService() *Service {
	return &Service{
		rooms:       make(map[string]*room),
		tokens:      newTokenStore(2 * time.Minute),
		annotations: NewAnnotationStore(),
	}
}

var PDFCollab = NewService()

func (s *Service) CreateSingleUseToken(documentID string, userID int) (*TokenResult, error) {
	user, err := s.authorizeOwner(documentID, userID)
	if err != nil {
		return nil, err
	}

	return s.tokens.create(documentID, User{
		UserID:   user.ID,
		Username: user.Username,
	})
}

func (s *Service) ValidateConnection(documentID, token string) error {
	return s.tokens.validate(documentID, token)
}

func (s *Service) HandleConnection(documentID, token string, ws *websocket.Conn) error {
	user, err := s.tokens.consume(documentID, token)
	if err != nil {
		s.sendError(ws, err.Error())
		return err
	}

	if err := s.annotations.EnsureDocumentLoaded(documentID); err != nil {
		s.sendError(ws, err.Error())
		return err
	}
	s.annotations.MarkRoomActive(documentID)

	currentRoom := s.getOrCreateRoom(documentID)
	return currentRoom.handleConnection(s, ws, s.allocateClientID(), user)
}

func (s *Service) handleIncomingPayload(currentRoom *room, client *client, payload []byte) error {
	if !json.Valid(payload) {
		return errors.New("payload must be valid json")
	}

	var message inboundMessage
	if err := json.Unmarshal(payload, &message); err != nil {
		return errors.New("failed to parse message")
	}

	return s.handleClientMessage(currentRoom, client, message)
}

func (s *Service) handleClientMessage(currentRoom *room, client *client, message inboundMessage) error {
	documentID := currentRoom.documentID

	switch message.Type {
	case "annotations:get":
		if message.Page == nil {
			return ErrInvalidAnnotation
		}

		annotations, err := s.annotations.GetPageAnnotations(documentID, *message.Page)
		if err != nil {
			return err
		}

		client.queue(outboundMessage{
			Type:        "annotations:page",
			DocumentID:  documentID,
			Page:        message.Page,
			Annotations: annotations,
		})
		return nil

	case "annotation:create":
		if message.Annotation == nil {
			return ErrInvalidAnnotation
		}

		annotation, err := s.annotations.CreateAnnotation(documentID, *message.Annotation)
		if err != nil {
			return err
		}

		repo.Document.TouchUpdatedAt(documentID)

		currentRoom.broadcast(outboundMessage{
			Type:       "annotation:created",
			DocumentID: documentID,
			User:       &client.user,
			Annotation: annotation,
		}, nil)
		return nil

	case "annotation:update":
		if message.Annotation == nil {
			return ErrInvalidAnnotation
		}

		annotation, err := s.annotations.UpdateAnnotation(documentID, client.id, *message.Annotation)
		if err != nil {
			return err
		}

		repo.Document.TouchUpdatedAt(documentID)

		currentRoom.broadcast(outboundMessage{
			Type:       "annotation:updated",
			DocumentID: documentID,
			User:       &client.user,
			Annotation: annotation,
		}, nil)
		return nil

	case "annotation:move":
		if message.Annotation == nil {
			return ErrInvalidAnnotation
		}

		annotation, err := s.annotations.MoveAnnotation(documentID, client.id, *message.Annotation)
		if err != nil {
			return err
		}

		repo.Document.TouchUpdatedAt(documentID)

		currentRoom.broadcast(outboundMessage{
			Type:       "annotation:moved",
			DocumentID: documentID,
			User:       &client.user,
			Annotation: annotation,
		}, nil)
		return nil

	case "annotation:delete":
		if message.AnnotationID == nil {
			return ErrInvalidAnnotation
		}

		if err := s.annotations.DeleteAnnotation(documentID, client.id, *message.AnnotationID); err != nil {
			return err
		}

		repo.Document.TouchUpdatedAt(documentID)

		currentRoom.broadcast(outboundMessage{
			Type:         "annotation:deleted",
			DocumentID:   documentID,
			User:         &client.user,
			AnnotationID: message.AnnotationID,
		}, nil)
		return nil

	case "annotation:lock":
		if message.AnnotationID == nil {
			return ErrInvalidAnnotation
		}

		lock, err := s.annotations.AcquireAnnotationLock(documentID, *message.AnnotationID, client.id, client.user)
		if err != nil {
			return err
		}

		currentRoom.broadcast(outboundMessage{
			Type:           "annotation:locked",
			DocumentID:     documentID,
			AnnotationLock: lock,
		}, nil)
		return nil

	case "annotation:unlock":
		if message.AnnotationID == nil {
			return ErrInvalidAnnotation
		}

		lock, err := s.annotations.ReleaseAnnotationLock(documentID, *message.AnnotationID, client.id)
		if err != nil {
			return err
		}
		if lock == nil {
			return nil
		}

		currentRoom.broadcast(outboundMessage{
			Type:           "annotation:unlocked",
			DocumentID:     documentID,
			AnnotationLock: lock,
		}, nil)
		return nil

	default:
		return errors.New("unknown message type")
	}
}

func (s *Service) authorizeOwner(documentID string, userID int) (*repoUser, error) {
	doc := repo.Document.GetByUUIDWithFile(documentID)
	if doc == nil {
		return nil, ErrDocumentNotFound
	}

	if doc.UserID != userID {
		return nil, ErrForbidden
	}

	user, err := repo.User.Get(userID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	return &repoUser{
		ID:       user.ID,
		Username: user.Username,
	}, nil
}

type repoUser struct {
	ID       int
	Username string
}

func (s *Service) getOrCreateRoom(documentID string) *room {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRoom, ok := s.rooms[documentID]
	if !ok {
		currentRoom = newRoom(documentID)
		s.rooms[documentID] = currentRoom
	}

	return currentRoom
}

func (s *Service) removeRoomIfEmpty(currentRoom *room) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rooms[currentRoom.documentID] != currentRoom {
		return false
	}
	if !currentRoom.isEmpty() {
		return false
	}

	delete(s.rooms, currentRoom.documentID)
	return true
}

func (s *Service) allocateClientID() string {
	id := atomic.AddUint64(&s.nextClientID, 1)
	return strconv.FormatUint(id, 10)
}

func (s *Service) sendError(ws *websocket.Conn, message string) {
	_ = websocket.Message.Send(ws, string(mustMarshal(outboundMessage{
		Type:  "error",
		Error: message,
	})))
}
