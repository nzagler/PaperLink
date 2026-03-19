package collabedit

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type singleUseToken struct {
	DocumentID string
	User       User
	ExpiresAt  time.Time
}

type tokenStore struct {
	mu     sync.Mutex
	ttl    time.Duration
	tokens map[string]singleUseToken
}

func newTokenStore(ttl time.Duration) *tokenStore {
	return &tokenStore{
		ttl:    ttl,
		tokens: make(map[string]singleUseToken),
	}
}

func (s *tokenStore) create(documentID string, user User) (*TokenResult, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.ttl)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupExpiredLocked(time.Now())
	s.tokens[token] = singleUseToken{
		DocumentID: documentID,
		User:       user,
		ExpiresAt:  expiresAt,
	}

	return &TokenResult{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *tokenStore) validate(documentID, token string) error {
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

func (s *tokenStore) consume(documentID, token string) (User, error) {
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

func (s *tokenStore) cleanupExpiredLocked(now time.Time) {
	for token, entry := range s.tokens {
		if now.After(entry.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
