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
	// LastPracticeAt is when the user last completed an exercise. Unlike
	// LastSeenAt (bumped on every page view), only this drives the streak.
	LastPracticeAt time.Time `firestore:"last_practice_at"`
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

// lastPractice returns LastPracticeAt, falling back to LastSeenAt for
// accounts from before practice time was tracked separately.
func (u *User) lastPractice() time.Time {
	if !u.LastPracticeAt.IsZero() {
		return u.LastPracticeAt
	}
	return u.LastSeenAt
}

// StreakDeadline returns the UTC time by which the user must practice to preserve
// their current streak (midnight at the end of the day after their last practice day).
// Returns zero time if no streak is established.
func (u *User) StreakDeadline() time.Time {
	last := u.lastPractice()
	if u.StreakDays == 0 || last.IsZero() {
		return time.Time{}
	}
	lastDay := last.UTC().Truncate(24 * time.Hour)
	return lastDay.Add(48 * time.Hour)
}

// CurrentStreakDays returns the streak to display: the stored count while the
// streak is still alive, 0 once its deadline has passed. The stored StreakDays
// is only recalculated on practice, so without this an expired streak would
// keep showing its old count.
func (u *User) CurrentStreakDays() int {
	deadline := u.StreakDeadline()
	if deadline.IsZero() || time.Now().After(deadline) {
		return 0
	}
	return u.StreakDays
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

// PracticedToday reports whether the user completed an exercise today (UTC).
func (u *User) PracticedToday() bool {
	last := u.lastPractice()
	if last.IsZero() {
		return false
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	return last.UTC().Truncate(24 * time.Hour).Equal(today)
}

// NextCEFRLevel returns the label of the next CEFR level, or "" at C2.
func (u *User) NextCEFRLevel() string {
	switch RatingToCEFR(u.Rating) {
	case "A0":
		return "A1"
	case "A1":
		return "A2"
	case "A2":
		return "B1"
	case "B1":
		return "B2"
	case "B2":
		return "C1"
	case "C1":
		return "C2"
	}
	return ""
}

// CEFRProgressPercent returns how far (0–100) the rating has advanced through
// its current CEFR band. The open-ended A0 band is anchored at 800; C2 is 100.
func (u *User) CEFRProgressPercent() int {
	r := u.Rating
	if r >= 2000 {
		return 100
	}
	lo := 800.0
	for _, hi := range []float64{1000, 1200, 1400, 1600, 1800, 2000} {
		if r < hi {
			if r < lo {
				return 0
			}
			return int((r - lo) / (hi - lo) * 100)
		}
		lo = hi
	}
	return 100
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
