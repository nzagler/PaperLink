package collabedit

import (
	"encoding/json"
	"errors"
	"io"
	"sync"

	"golang.org/x/net/websocket"
)

type client struct {
	id   string
	conn *websocket.Conn
	room *room
	user User
	send chan []byte
}

type room struct {
	documentID string
	mu         sync.RWMutex
	clients    map[*client]struct{}
}

func newRoom(documentID string) *room {
	return &room{
		documentID: documentID,
		clients:    make(map[*client]struct{}),
	}
}

func (r *room) handleConnection(s *Service, ws *websocket.Conn, clientID string, user User) error {
	client, users := r.join(ws, clientID, user)
	defer r.disconnect(s, client)

	go client.writePump()

	locks, err := s.annotations.GetDocumentLocks(r.documentID)
	if err != nil {
		return err
	}

	client.queue(outboundMessage{
		Type:            "room_state",
		DocumentID:      r.documentID,
		ClientID:        client.id,
		User:            &user,
		Users:           users,
		AnnotationLocks: locks,
	})

	r.broadcast(outboundMessage{
		Type:       "user_joined",
		DocumentID: r.documentID,
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

		if err := s.handleIncomingPayload(r, client, payload); err != nil {
			client.queue(outboundMessage{
				Type:  "error",
				Error: err.Error(),
			})
		}
	}
}

func (r *room) join(ws *websocket.Conn, clientID string, user User) (*client, []User) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentClient := &client{
		id:   clientID,
		conn: ws,
		room: r,
		user: user,
		send: make(chan []byte, 32),
	}
	r.clients[currentClient] = struct{}{}

	users := make([]User, 0, len(r.clients))
	for member := range r.clients {
		users = append(users, member.user)
	}

	return currentClient, users
}

func (r *room) disconnect(s *Service, currentClient *client) {
	releasedLocks := s.annotations.ReleaseLocksByOwner(r.documentID, currentClient.id)
	for index := range releasedLocks {
		lock := releasedLocks[index]
		r.broadcast(outboundMessage{
			Type:           "annotation:unlocked",
			DocumentID:     r.documentID,
			AnnotationLock: &lock,
		}, currentClient)
	}

	if !r.leave(currentClient) {
		return
	}
	if s.removeRoomIfEmpty(r) {
		s.annotations.MarkRoomInactive(r.documentID)
	}
}

func (r *room) leave(currentClient *client) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.clients, currentClient)
	if len(r.clients) == 0 {
		close(currentClient.send)
		return true
	}

	payload := mustMarshal(outboundMessage{
		Type:       "user_left",
		DocumentID: r.documentID,
		User:       &currentClient.user,
	})

	for member := range r.clients {
		member.queueBytes(payload)
	}
	close(currentClient.send)
	return false
}

func (r *room) broadcast(message outboundMessage, exclude *client) {
	payload := mustMarshal(message)

	r.mu.RLock()
	defer r.mu.RUnlock()

	for member := range r.clients {
		if member == exclude {
			continue
		}
		member.queueBytes(payload)
	}
}

func (r *room) isEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients) == 0
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
