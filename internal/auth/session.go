package auth

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	sessionTTL  = 5 * time.Minute
	sessionKind = "WebAuthnSession"
)

type sessionEntity struct {
	Data      []byte    `datastore:"data,noindex"`
	ExpiresAt time.Time `datastore:"expires_at"`
}

// SessionStore stores short-lived WebAuthn challenge sessions in Datastore so
// that they survive across GAE instance boundaries.
type SessionStore struct {
	db *datastore.Client
}

// NewSessionStore creates a SessionStore backed by Datastore.
func NewSessionStore(db *datastore.Client) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) key(k string) *datastore.Key {
	return datastore.NameKey(sessionKind, k, nil)
}

// Set stores session data under key for sessionTTL.
func (s *SessionStore) Set(key string, data *webauthn.SessionData) {
	raw, err := json.Marshal(data)
	if err != nil {
		return
	}
	e := &sessionEntity{Data: raw, ExpiresAt: time.Now().Add(sessionTTL)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = s.db.Put(ctx, s.key(key), e)
}

// Get retrieves session data by key. Returns false if not found or expired.
func (s *SessionStore) Get(key string) (*webauthn.SessionData, bool) {
	var e sessionEntity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.db.Get(ctx, s.key(key), &e); err != nil {
		return nil, false
	}
	if time.Now().After(e.ExpiresAt) {
		_ = s.db.Delete(ctx, s.key(key))
		return nil, false
	}
	var sd webauthn.SessionData
	if err := json.Unmarshal(e.Data, &sd); err != nil {
		return nil, false
	}
	return &sd, true
}

// Delete removes session data by key.
func (s *SessionStore) Delete(key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.db.Delete(ctx, s.key(key))
}
