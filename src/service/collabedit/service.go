package collabedit

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sync"
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

type outboundMessage struct {
	Type         string              `json:"type"`
	DocumentID   string              `json:"documentId,omitempty"`
	User         *User               `json:"user,omitempty"`
	Users        []User              `json:"users,omitempty"`
	Page         *int64              `json:"page,omitempty"`
	Annotation   *annotationMessage  `json:"annotation,omitempty"`
	Annotations  []annotationMessage `json:"annotations,omitempty"`
	AnnotationID *int                `json:"annotationId,omitempty"`
	Error        string              `json:"error,omitempty"`
}

type inboundMessage struct {
	Type         string             `json:"type"`
	Page         *int64             `json:"page,omitempty"`
	Annotation   *annotationMessage `json:"annotation,omitempty"`
	AnnotationID *int               `json:"annotationId,omitempty"`
}

type singleUseToken struct {
	DocumentID string
	User       User
	ExpiresAt  time.Time
}

type client struct {
	conn *websocket.Conn
	room *room
	user User
	send chan []byte
}

type room struct {
	documentID string
	clients    map[*client]struct{}
}

type Service struct {
	mu          sync.RWMutex
	tokenTTL    time.Duration
	tokens      map[string]singleUseToken
	rooms       map[string]*room
	annotations *AnnotationStore
}

func NewService() *Service {
	return &Service{
		tokenTTL:    2 * time.Minute,
		tokens:      make(map[string]singleUseToken),
		rooms:       make(map[string]*room),
		annotations: NewAnnotationStore(),
	}
}

var PDFCollab = NewService()

func (s *Service) CreateSingleUseToken(documentID string, userID int) (*TokenResult, error) {
	user, err := s.authorizeOwner(documentID, userID)
	if err != nil {
		return nil, err
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.tokenTTL)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredTokensLocked(time.Now())
	s.tokens[token] = singleUseToken{
		DocumentID: documentID,
		User: User{
			UserID:   user.ID,
			Username: user.Username,
		},
		ExpiresAt: expiresAt,
	}

	return &TokenResult{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) ValidateConnection(documentID, token string) error {
	if token == "" {
		return ErrTokenRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tokens[token]
	if !ok {
		return ErrTokenInvalid
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(s.tokens, token)
		return ErrTokenExpired
	}

	if entry.DocumentID != documentID {
		return ErrTokenInvalid
	}

	return nil
}

func (s *Service) HandleConnection(documentID, token string, ws *websocket.Conn) error {
	user, err := s.consumeToken(documentID, token)
	if err != nil {
		s.sendError(ws, err.Error())
		return err
	}

	if err := s.annotations.EnsureDocumentLoaded(documentID); err != nil {
		s.sendError(ws, err.Error())
		return err
	}
	s.annotations.MarkRoomActive(documentID)

	client, users := s.joinRoom(documentID, ws, user)
	defer s.leaveRoom(client)

	go client.writePump()

	client.queue(outboundMessage{
		Type:       "room_state",
		DocumentID: documentID,
		Users:      users,
	})

	s.broadcast(documentID, outboundMessage{
		Type:       "user_joined",
		DocumentID: documentID,
		User:       &user,
	}, client)

	for {
		var payload []byte
		if err := websocket.Message.Receive(ws, &payload); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		if !json.Valid(payload) {
			client.queue(outboundMessage{
				Type:  "error",
				Error: "payload must be valid json",
			})
			continue
		}

		var message inboundMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			client.queue(outboundMessage{
				Type:  "error",
				Error: "failed to parse message",
			})
			continue
		}

		if err := s.handleClientMessage(documentID, client, message); err != nil {
			client.queue(outboundMessage{
				Type:  "error",
				Error: err.Error(),
			})
		}
	}
}

func (s *Service) handleClientMessage(documentID string, client *client, message inboundMessage) error {
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

		s.broadcast(documentID, outboundMessage{
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

		annotation, err := s.annotations.UpdateAnnotation(documentID, *message.Annotation)
		if err != nil {
			return err
		}

		s.broadcast(documentID, outboundMessage{
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

		annotation, err := s.annotations.MoveAnnotation(documentID, *message.Annotation)
		if err != nil {
			return err
		}

		s.broadcast(documentID, outboundMessage{
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

		if err := s.annotations.DeleteAnnotation(documentID, *message.AnnotationID); err != nil {
			return err
		}

		s.broadcast(documentID, outboundMessage{
			Type:         "annotation:deleted",
			DocumentID:   documentID,
			User:         &client.user,
			AnnotationID: message.AnnotationID,
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

func (s *Service) consumeToken(documentID, token string) (User, error) {
	if token == "" {
		return User{}, ErrTokenRequired
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tokens[token]
	if !ok {
		return User{}, ErrTokenInvalid
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(s.tokens, token)
		return User{}, ErrTokenExpired
	}

	if entry.DocumentID != documentID {
		return User{}, ErrTokenInvalid
	}

	delete(s.tokens, token)
	return entry.User, nil
}

func (s *Service) joinRoom(documentID string, ws *websocket.Conn, user User) (*client, []User) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRoom, ok := s.rooms[documentID]
	if !ok {
		currentRoom = &room{
			documentID: documentID,
			clients:    make(map[*client]struct{}),
		}
		s.rooms[documentID] = currentRoom
	}

	currentClient := &client{
		conn: ws,
		room: currentRoom,
		user: user,
		send: make(chan []byte, 32),
	}
	currentRoom.clients[currentClient] = struct{}{}

	users := make([]User, 0, len(currentRoom.clients))
	for member := range currentRoom.clients {
		users = append(users, member.user)
	}

	return currentClient, users
}

func (s *Service) leaveRoom(currentClient *client) {
	var flushDocumentID string

	s.mu.Lock()
	currentRoom := currentClient.room
	delete(currentRoom.clients, currentClient)

	if len(currentRoom.clients) == 0 {
		delete(s.rooms, currentRoom.documentID)
		flushDocumentID = currentRoom.documentID
		close(currentClient.send)
		s.mu.Unlock()
		if flushDocumentID != "" {
			s.annotations.MarkRoomInactive(flushDocumentID)
		}
		return
	}

	payload := mustMarshal(outboundMessage{
		Type:       "user_left",
		DocumentID: currentRoom.documentID,
		User:       &currentClient.user,
	})

	for member := range currentRoom.clients {
		member.queueBytes(payload)
	}
	close(currentClient.send)
	s.mu.Unlock()
}

func (s *Service) broadcast(documentID string, message outboundMessage, exclude *client) {
	payload := mustMarshal(message)

	s.mu.RLock()
	currentRoom := s.rooms[documentID]
	if currentRoom == nil {
		s.mu.RUnlock()
		return
	}
	for member := range currentRoom.clients {
		if member == exclude {
			continue
		}
		member.queueBytes(payload)
	}
	s.mu.RUnlock()
}

func (s *Service) cleanupExpiredTokensLocked(now time.Time) {
	for token, entry := range s.tokens {
		if now.After(entry.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
}

func (s *Service) sendError(ws *websocket.Conn, message string) {
	_ = websocket.Message.Send(ws, string(mustMarshal(outboundMessage{
		Type:  "error",
		Error: message,
	})))
}

func (c *client) writePump() {
	for payload := range c.send {
		if err := websocket.Message.Send(c.conn, string(payload)); err != nil {
			return
		}
	}
}

func (c *client) queue(message outboundMessage) {
	c.queueBytes(mustMarshal(message))
}

func (c *client) queueBytes(payload []byte) {
	select {
	case c.send <- payload:
	default:
		log.Warnf("dropping websocket client for user %d due to backpressure", c.user.UserID)
		_ = c.conn.Close()
	}
}

func mustMarshal(message outboundMessage) []byte {
	payload, err := json.Marshal(message)
	if err != nil {
		return []byte(`{"type":"error","error":"failed to marshal message"}`)
	}
	return payload
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
