package auth

import (
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

const sessionTTL = 5 * time.Minute

// SessionStore is a simple in-memory store for short-lived WebAuthn challenge data.
// NOTE: This does not survive instance restarts or scale across multiple GAE instances.
// For production multi-instance deployments, move challenge storage to Firestore.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*webauthn.SessionData
	expiry   map[string]time.Time
}

// NewSessionStore creates a SessionStore and starts a background cleanup goroutine.
func NewSessionStore() *SessionStore {
	s := &SessionStore{
		sessions: make(map[string]*webauthn.SessionData),
		expiry:   make(map[string]time.Time),
	}
	go s.cleanup()
	return s
}

// Set stores session data under key for sessionTTL.
func (s *SessionStore) Set(key string, data *webauthn.SessionData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[key] = data
	s.expiry[key] = time.Now().Add(sessionTTL)
}

// Get retrieves session data by key. Returns false if not found or expired.
func (s *SessionStore) Get(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.expiry[key]
	if !ok || time.Now().After(exp) {
		delete(s.sessions, key)
		delete(s.expiry, key)
		return nil, false
	}
	return s.sessions[key], true
}

// Delete removes session data by key.
func (s *SessionStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, key)
	delete(s.expiry, key)
}

// cleanup runs periodically to evict expired sessions.
func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(sessionTTL)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, exp := range s.expiry {
			if now.After(exp) {
				delete(s.sessions, k)
				delete(s.expiry, k)
			}
		}
		s.mu.Unlock()
	}
}
