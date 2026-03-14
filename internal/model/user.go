package model

import (
	"encoding/json"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// User represents a learner on the platform.
type User struct {
	ID         string               `firestore:"-"`
	Token      string               `firestore:"token"`
	Rating     float64              `firestore:"rating"`
	RD         float64              `firestore:"rd"`
	Volatility float64              `firestore:"volatility"`
	Role       string               `firestore:"role"` // "user"|"mod"|"admin"
	// Passkeys are serialized as JSON bytes in Firestore.
	PasskeysJSON []byte             `firestore:"passkeys_json"`
	Passkeys     []webauthn.Credential `firestore:"-"`
	Progress     map[string]bool    `firestore:"progress"`
	CreatedAt    time.Time          `firestore:"created_at"`
	LastSeenAt   time.Time          `firestore:"last_seen_at"`
}

// NewUser creates a User with Glicko-2 defaults.
func NewUser(id, token string) *User {
	return &User{
		ID:         id,
		Token:      token,
		Rating:     1500,
		RD:         350,
		Volatility: 0.06,
		Role:       "user",
		Progress:   make(map[string]bool),
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}
}

// MarshalPasskeys serialises Passkeys into PasskeysJSON for Firestore storage.
func (u *User) MarshalPasskeys() error {
	if len(u.Passkeys) == 0 {
		u.PasskeysJSON = nil
		return nil
	}
	b, err := json.Marshal(u.Passkeys)
	if err != nil {
		return err
	}
	u.PasskeysJSON = b
	return nil
}

// UnmarshalPasskeys deserialises PasskeysJSON into Passkeys.
func (u *User) UnmarshalPasskeys() error {
	if len(u.PasskeysJSON) == 0 {
		u.Passkeys = nil
		return nil
	}
	return json.Unmarshal(u.PasskeysJSON, &u.Passkeys)
}

// --- webauthn.User interface ---

func (u *User) WebAuthnID() []byte {
	return []byte(u.ID)
}

func (u *User) WebAuthnName() string {
	if len(u.ID) >= 8 {
		return u.ID[:8]
	}
	return u.ID
}

func (u *User) WebAuthnDisplayName() string {
	return "Uzanto " + u.WebAuthnName()
}

func (u *User) WebAuthnCredentials() []webauthn.Credential {
	return u.Passkeys
}

func (u *User) WebAuthnIcon() string {
	return ""
}
