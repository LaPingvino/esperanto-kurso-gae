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
	Username   string               `firestore:"username"`  // optional, unique display name
	Role       string               `firestore:"role"` // "user"|"mod"|"admin"
	Lang       string               `firestore:"lang"`    // preferred language for definitions, e.g. "en"
	UILang     string               `datastore:"ui_lang"` // preferred interface language, e.g. "en", "nl", "eo"
	// Passkeys are serialized as JSON bytes in Firestore.
	PasskeysJSON []byte             `firestore:"passkeys_json"`
	Passkeys     []webauthn.Credential `firestore:"-"`
	Progress     map[string]bool    `firestore:"progress"`
	Favorites      []string           `firestore:"favorites"`
	KeepDataDays   int                `firestore:"keep_data_days"` // -1=never delete, 0=default, >0=days of inactivity
	StreakDays      int                `firestore:"streak_days"`
	StreakStartAt   time.Time          `firestore:"streak_start_at"`
	CreatedAt    time.Time          `firestore:"created_at"`
	LastSeenAt   time.Time          `firestore:"last_seen_at"`
}

// IsFavorite returns true if the given slug is in the user's favorites.
func (u *User) IsFavorite(slug string) bool {
	for _, s := range u.Favorites {
		if s == slug {
			return true
		}
	}
	return false
}

// IsFavoriteSeries returns true if the given series slug is in the user's favorites.
func (u *User) IsFavoriteSeries(seriesSlug string) bool {
	return u.IsFavorite("series:" + seriesSlug)
}

// IsFavoriteTag returns true if the given tag is in the user's favorites.
func (u *User) IsFavoriteTag(tag string) bool {
	return u.IsFavorite("tag:" + tag)
}

// UILangOrDefault returns UILang if non-empty, else "eo".
func (u *User) UILangOrDefault() string {
	if u.UILang == "" {
		return "eo"
	}
	return u.UILang
}

// StreakDeadline returns the UTC time by which the user must practice to preserve
// their current streak (midnight at the end of the day after their last practice day).
// Returns zero time if no streak is established.
func (u *User) StreakDeadline() time.Time {
	if u.StreakDays == 0 || u.LastSeenAt.IsZero() {
		return time.Time{}
	}
	lastDay := u.LastSeenAt.UTC().Truncate(24 * time.Hour)
	return lastDay.Add(48 * time.Hour)
}

// StreakExpiresInHours returns how many whole hours remain before the streak expires.
// Returns -1 if no streak, 0 if already expired, positive if still active.
func (u *User) StreakExpiresInHours() int {
	deadline := u.StreakDeadline()
	if deadline.IsZero() {
		return -1
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Hours())
}

// DisplayName returns the username if set, otherwise a short anonymous ID.
func (u *User) DisplayName() string {
	if u.Username != "" {
		return u.Username
	}
	if len(u.ID) >= 8 {
		return "uzanto-" + u.ID[:6]
	}
	return "uzanto"
}

// CEFRLevel returns the CEFR level label for the user's Elo rating.
func (u *User) CEFRLevel() string {
	return RatingToCEFR(u.Rating)
}

// RatingToCEFR converts an Elo rating to a CEFR level label.
func RatingToCEFR(rating float64) string {
	switch {
	case rating < 1000:
		return "A0"
	case rating < 1200:
		return "A1"
	case rating < 1400:
		return "A2"
	case rating < 1600:
		return "B1"
	case rating < 1800:
		return "B2"
	case rating < 2000:
		return "C1"
	default:
		return "C2"
	}
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
		Lang:       "en",
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
