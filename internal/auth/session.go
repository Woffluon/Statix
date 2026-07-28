package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Session struct {
	ID        string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionStore{
		sessions: make(map[string]Session),
		ttl:      ttl,
	}
}

func (s *SessionStore) Create(username string) (Session, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Session{}, fmt.Errorf("auth: failed to generate session token: %w", err)
	}

	id := hex.EncodeToString(tokenBytes)
	now := time.Now()
	sess := Session{
		ID:        id,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()

	return sess, nil
}

func (s *SessionStore) Get(id string) (Session, bool) {
	if id == "" {
		return Session{}, false
	}

	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()

	if !ok {
		return Session{}, false
	}

	if time.Now().After(sess.ExpiresAt) {
		s.Delete(id)
		return Session{}, false
	}

	return sess, true
}

func (s *SessionStore) Delete(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

func (s *SessionStore) Purge() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}
