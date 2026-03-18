package collabedit

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"paperlink/db"
	"paperlink/db/entity"
	"paperlink/db/repo"

	"gorm.io/gorm"
)

var (
	ErrAnnotationNotFound = errors.New("annotation not found")
	ErrInvalidAnnotation  = errors.New("invalid annotation")
)

type annotationMessage struct {
	ID        int                   `json:"id"`
	Type      entity.AnnotationType `json:"type"`
	Data      string                `json:"data"`
	Page      int64                 `json:"page"`
	CreatedAt int64                 `json:"createdAt"`
	UpdatedAt int64                 `json:"updatedAt"`
	PositionX float64               `json:"positionX"`
	PositionY float64               `json:"positionY"`
}

type annotationActionPayload struct {
	Previous *annotationMessage `json:"previous,omitempty"`
	Current  *annotationMessage `json:"current,omitempty"`
}

type documentAnnotationState struct {
	DocumentID     int
	DocumentUUID   string
	Annotations    map[int]*entity.Annotation
	DeletedIDs     map[int]struct{}
	PendingActions []entity.AnnotationAction
	Dirty          bool
	RoomActive     bool
	LastTouchedAt  time.Time
	LastRoomLeftAt time.Time
}

type AnnotationStore struct {
	mu            sync.Mutex
	flushInterval time.Duration
	idleTTL       time.Duration
	documents     map[string]*documentAnnotationState
	nextID        int
	nextIDLoaded  bool
}

func NewAnnotationStore() *AnnotationStore {
	store := &AnnotationStore{
		flushInterval: 15 * time.Second,
		idleTTL:       2 * time.Minute,
		documents:     make(map[string]*documentAnnotationState),
	}

	go store.flushLoop()
	return store
}

func (s *AnnotationStore) EnsureDocumentLoaded(documentUUID string) error {
	s.mu.Lock()
	if state, ok := s.documents[documentUUID]; ok {
		state.LastTouchedAt = time.Now()
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	doc := repo.Document.GetByUUIDWithFile(documentUUID)
	if doc == nil {
		return ErrDocumentNotFound
	}

	annotations, err := repo.Document.GetAnnotationsById(doc.ID)
	if err != nil {
		return err
	}

	state := &documentAnnotationState{
		DocumentID:     doc.ID,
		DocumentUUID:   documentUUID,
		Annotations:    make(map[int]*entity.Annotation, len(annotations)),
		DeletedIDs:     make(map[int]struct{}),
		PendingActions: make([]entity.AnnotationAction, 0),
		Dirty:          false,
		LastTouchedAt:  time.Now(),
	}

	for _, annotation := range annotations {
		annotationCopy := annotation
		state.Annotations[annotation.ID] = &annotationCopy
	}

	s.mu.Lock()
	if existing, ok := s.documents[documentUUID]; ok {
		existing.LastTouchedAt = time.Now()
		s.mu.Unlock()
		return nil
	}
	s.documents[documentUUID] = state
	s.mu.Unlock()

	return nil
}

func (s *AnnotationStore) MarkRoomActive(documentUUID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return
	}

	state.RoomActive = true
	state.LastTouchedAt = time.Now()
	state.LastRoomLeftAt = time.Time{}
}

func (s *AnnotationStore) MarkRoomInactive(documentUUID string) {
	s.mu.Lock()
	state := s.documents[documentUUID]
	if state == nil {
		s.mu.Unlock()
		return
	}

	state.RoomActive = false
	state.LastTouchedAt = time.Now()
	state.LastRoomLeftAt = time.Now()
	s.mu.Unlock()

	if err := s.FlushDocument(documentUUID); err != nil {
		log.Warnf("failed to flush annotations for %s on room shutdown: %v", documentUUID, err)
	}
}

func (s *AnnotationStore) GetPageAnnotations(documentUUID string, page int64) ([]annotationMessage, error) {
	if page <= 0 {
		return nil, ErrInvalidAnnotation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return nil, ErrDocumentNotFound
	}

	state.LastTouchedAt = time.Now()

	result := make([]annotationMessage, 0)
	for _, annotation := range state.Annotations {
		if annotation.Page != page {
			continue
		}
		result = append(result, toAnnotationMessage(annotation))
	}

	return result, nil
}

func (s *AnnotationStore) CreateAnnotation(documentUUID string, input annotationMessage) (*annotationMessage, error) {
	if err := validateAnnotationInput(input, true); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNextIDLocked(); err != nil {
		return nil, err
	}

	state := s.documents[documentUUID]
	if state == nil {
		return nil, ErrDocumentNotFound
	}

	now := time.Now().Unix()
	annotation := &entity.Annotation{
		ID:         s.nextID,
		Type:       input.Type,
		Data:       input.Data,
		Page:       input.Page,
		CreatedAt:  now,
		UpdatedAt:  now,
		PositionX:  input.PositionX,
		PositionY:  input.PositionY,
		DocumentID: state.DocumentID,
	}
	s.nextID++

	state.Annotations[annotation.ID] = annotation
	delete(state.DeletedIDs, annotation.ID)
	state.Dirty = true
	state.LastTouchedAt = time.Now()
	s.recordActionLocked(state, entity.Create, nil, annotation)

	result := toAnnotationMessage(annotation)
	return &result, nil
}

func (s *AnnotationStore) UpdateAnnotation(documentUUID string, input annotationMessage) (*annotationMessage, error) {
	if input.ID == 0 {
		return nil, ErrInvalidAnnotation
	}
	if err := validateAnnotationInput(input, false); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return nil, ErrDocumentNotFound
	}

	annotation := state.Annotations[input.ID]
	if annotation == nil {
		return nil, ErrAnnotationNotFound
	}

	previous := cloneAnnotation(annotation)
	annotation.Type = input.Type
	annotation.Data = input.Data
	annotation.Page = input.Page
	annotation.PositionX = input.PositionX
	annotation.PositionY = input.PositionY
	annotation.UpdatedAt = time.Now().Unix()

	state.Dirty = true
	state.LastTouchedAt = time.Now()
	s.recordActionLocked(state, entity.Update, previous, annotation)

	result := toAnnotationMessage(annotation)
	return &result, nil
}

func (s *AnnotationStore) MoveAnnotation(documentUUID string, input annotationMessage) (*annotationMessage, error) {
	if input.ID == 0 || input.Page <= 0 {
		return nil, ErrInvalidAnnotation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return nil, ErrDocumentNotFound
	}

	annotation := state.Annotations[input.ID]
	if annotation == nil {
		return nil, ErrAnnotationNotFound
	}

	previous := cloneAnnotation(annotation)
	annotation.Page = input.Page
	annotation.PositionX = input.PositionX
	annotation.PositionY = input.PositionY
	annotation.UpdatedAt = time.Now().Unix()

	state.Dirty = true
	state.LastTouchedAt = time.Now()
	s.recordActionLocked(state, entity.Move, previous, annotation)

	result := toAnnotationMessage(annotation)
	return &result, nil
}

func (s *AnnotationStore) DeleteAnnotation(documentUUID string, annotationID int) error {
	if annotationID == 0 {
		return ErrInvalidAnnotation
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return ErrDocumentNotFound
	}

	annotation := state.Annotations[annotationID]
	if annotation == nil {
		return ErrAnnotationNotFound
	}

	previous := cloneAnnotation(annotation)
	delete(state.Annotations, annotationID)
	state.DeletedIDs[annotationID] = struct{}{}
	state.Dirty = true
	state.LastTouchedAt = time.Now()
	s.recordActionLocked(state, entity.Delete, previous, nil)

	return nil
}

func (s *AnnotationStore) FlushDocument(documentUUID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.documents[documentUUID]
	if state == nil {
		return nil
	}

	return s.flushDocumentLocked(state)
}

func (s *AnnotationStore) flushLoop() {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		for _, state := range s.documents {
			if state.Dirty {
				if err := s.flushDocumentLocked(state); err != nil {
					log.Warnf("failed to flush annotations for %s: %v", state.DocumentUUID, err)
				}
			}
		}

		now := time.Now()
		for documentUUID, state := range s.documents {
			if state.RoomActive {
				continue
			}
			if state.Dirty {
				continue
			}
			if state.LastRoomLeftAt.IsZero() {
				continue
			}
			if now.Sub(state.LastRoomLeftAt) < s.idleTTL {
				continue
			}
			delete(s.documents, documentUUID)
		}
		s.mu.Unlock()
	}
}

func (s *AnnotationStore) flushDocumentLocked(state *documentAnnotationState) error {
	if !state.Dirty {
		return nil
	}

	return db.DB().Transaction(func(tx *gorm.DB) error {
		for annotationID := range state.DeletedIDs {
			if err := tx.Delete(&entity.Annotation{}, "id = ? AND document_id = ?", annotationID, state.DocumentID).Error; err != nil {
				return err
			}
		}

		for _, annotation := range state.Annotations {
			annotation.DocumentID = state.DocumentID
			if err := tx.Save(annotation).Error; err != nil {
				return err
			}
		}

		if len(state.PendingActions) > 0 {
			actions := make([]entity.AnnotationAction, len(state.PendingActions))
			copy(actions, state.PendingActions)
			if err := tx.Create(&actions).Error; err != nil {
				return err
			}
		}

		state.DeletedIDs = make(map[int]struct{})
		state.PendingActions = make([]entity.AnnotationAction, 0)
		state.Dirty = false
		state.LastTouchedAt = time.Now()

		return nil
	})
}

func (s *AnnotationStore) recordActionLocked(state *documentAnnotationState, action entity.Action, previous, current *entity.Annotation) {
	payload, err := json.Marshal(annotationActionPayload{
		Previous: toAnnotationMessagePtr(previous),
		Current:  toAnnotationMessagePtr(current),
	})
	if err != nil {
		log.Warnf("failed to marshal annotation action for %s: %v", state.DocumentUUID, err)
		return
	}

	var annotationID *int
	switch {
	case current != nil:
		id := current.ID
		annotationID = &id
	case previous != nil:
		id := previous.ID
		annotationID = &id
	}

	state.PendingActions = append(state.PendingActions, entity.AnnotationAction{
		Action:       action,
		Data:         string(payload),
		CreatedAt:    time.Now().Unix(),
		AnnotationID: annotationID,
	})
}

func validateAnnotationInput(input annotationMessage, isCreate bool) error {
	if input.Page <= 0 {
		return fmt.Errorf("%w: page must be positive", ErrInvalidAnnotation)
	}

	switch input.Type {
	case entity.Textbox, entity.Note, entity.Canvas:
	default:
		return fmt.Errorf("%w: unknown annotation type", ErrInvalidAnnotation)
	}

	if !isCreate && input.ID == 0 {
		return fmt.Errorf("%w: annotation id required", ErrInvalidAnnotation)
	}

	return nil
}

func toAnnotationMessagePtr(annotation *entity.Annotation) *annotationMessage {
	if annotation == nil {
		return nil
	}
	message := toAnnotationMessage(annotation)
	return &message
}

func toAnnotationMessage(annotation *entity.Annotation) annotationMessage {
	return annotationMessage{
		ID:        annotation.ID,
		Type:      annotation.Type,
		Data:      annotation.Data,
		Page:      annotation.Page,
		CreatedAt: annotation.CreatedAt,
		UpdatedAt: annotation.UpdatedAt,
		PositionX: annotation.PositionX,
		PositionY: annotation.PositionY,
	}
}

func cloneAnnotation(annotation *entity.Annotation) *entity.Annotation {
	if annotation == nil {
		return nil
	}
	copy := *annotation
	return &copy
}

func (s *AnnotationStore) ensureNextIDLocked() error {
	if s.nextIDLoaded {
		return nil
	}

	var maxID int
	if err := db.DB().Model(&entity.Annotation{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
		return err
	}

	s.nextID = maxID + 1
	s.nextIDLoaded = true
	return nil
}
