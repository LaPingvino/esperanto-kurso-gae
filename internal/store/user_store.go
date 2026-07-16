package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/go-webauthn/webauthn/webauthn"
)

const userKind = "User"

type userEntity struct {
	Token        string    `datastore:"token"`
	Username     string    `datastore:"username"`
	Rating       float64   `datastore:"rating"`
	RD           float64   `datastore:"rd"`
	Volatility   float64   `datastore:"volatility"`
	Role         string    `datastore:"role"`
	Lang         string    `datastore:"lang"`
	UILang       string    `datastore:"ui_lang"`
	PasskeysJSON  []byte    `datastore:"passkeys_json,noindex"`
	ProgressJSON  []byte    `datastore:"progress_json,noindex"`
	FavoritesJSON []byte    `datastore:"favorites_json,noindex"`
	KeepDataDays  int       `datastore:"keep_data_days"`
	StreakDays    int       `datastore:"streak_days"`
	StreakStartAt time.Time `datastore:"streak_start_at"`
	CreatedAt    time.Time `datastore:"created_at"`
	LastSeenAt   time.Time `datastore:"last_seen_at"`
	LastPracticeAt time.Time `datastore:"last_practice_at"`
}

func userToEntity(u *model.User) (*userEntity, error) {
	pkJSON, err := json.Marshal(u.Passkeys)
	if err != nil {
		return nil, err
	}
	prJSON, err := json.Marshal(u.Progress)
	if err != nil {
		return nil, err
	}
	favJSON, err := json.Marshal(u.Favorites)
	if err != nil {
		return nil, err
	}
	lang := u.Lang
	if lang == "" {
		lang = "en"
	}
	uiLang := u.UILang
	if uiLang == "" {
		uiLang = "eo"
	}
	return &userEntity{
		Token:        u.Token,
		Username:     u.Username,
		Rating:       u.Rating,
		RD:           u.RD,
		Volatility:   u.Volatility,
		Role:         u.Role,
		Lang:         lang,
		UILang:       uiLang,
		PasskeysJSON:  pkJSON,
		ProgressJSON:  prJSON,
		FavoritesJSON: favJSON,
		KeepDataDays:  u.KeepDataDays,
		StreakDays:    u.StreakDays,
		StreakStartAt: u.StreakStartAt,
		CreatedAt:    u.CreatedAt,
		LastSeenAt:   u.LastSeenAt,
		LastPracticeAt: u.LastPracticeAt,
	}, nil
}

func entityToUser(id string, e *userEntity) (*model.User, error) {
	lang := e.Lang
	if lang == "" {
		lang = "en"
	}
	uiLang := e.UILang
	if uiLang == "" {
		uiLang = "eo"
	}
	// Default Glicko-2 starting values for accounts created before ratings were tracked.
	rating, rd, volatility := e.Rating, e.RD, e.Volatility
	if rating == 0 {
		rating = 1500
	}
	if rd == 0 {
		rd = 350
	}
	if volatility == 0 {
		volatility = 0.06
	}
	u := &model.User{
		ID:         id,
		Token:      e.Token,
		Username:   e.Username,
		Rating:     rating,
		RD:         rd,
		Volatility: volatility,
		Role:       e.Role,
		Lang:       lang,
		UILang:     uiLang,
		KeepDataDays:  e.KeepDataDays,
		StreakDays:    e.StreakDays,
		StreakStartAt: e.StreakStartAt,
		CreatedAt:    e.CreatedAt,
		LastSeenAt:   e.LastSeenAt,
		LastPracticeAt: e.LastPracticeAt,
		Progress:   make(map[string]bool),
	}
	if len(e.PasskeysJSON) > 0 {
		if err := json.Unmarshal(e.PasskeysJSON, &u.Passkeys); err != nil {
			return nil, err
		}
	}
	if len(e.ProgressJSON) > 0 {
		if err := json.Unmarshal(e.ProgressJSON, &u.Progress); err != nil {
			return nil, err
		}
	}
	if len(e.FavoritesJSON) > 0 {
		if err := json.Unmarshal(e.FavoritesJSON, &u.Favorites); err != nil {
			return nil, err
		}
	}
	return u, nil
}

type UserStore struct {
	db *datastore.Client
}

func NewUserStore(db *datastore.Client) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) userKey(id string) *datastore.Key {
	return datastore.NameKey(userKind, id, nil)
}

func (s *UserStore) Create(ctx context.Context, u *model.User) error {
	e, err := userToEntity(u)
	if err != nil {
		return fmt.Errorf("user_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.userKey(u.ID), e)
	return err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var e userEntity
	if err := s.db.Get(ctx, s.userKey(id), &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, fmt.Errorf("user_store: GetByID %s: %w", id, err)
	}
	return entityToUser(id, &e)
}

// lookupNames returns a userID → current username map for the given IDs,
// looking each distinct ID up once. Missing users map to "".
func (s *UserStore) lookupNames(ctx context.Context, ids []string) map[string]string {
	names := map[string]string{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := names[id]; ok {
			continue
		}
		names[id] = ""
		if u, err := s.GetByID(ctx, id); err == nil && u != nil {
			names[id] = u.Username
		}
	}
	return names
}

// ResolveUsernames populates the Username field of each comment by looking up
// user IDs.
func (s *UserStore) ResolveUsernames(ctx context.Context, comments []*model.Comment) {
	ids := make([]string, len(comments))
	for i, c := range comments {
		ids[i] = c.UserID
	}
	names := s.lookupNames(ctx, ids)
	for _, c := range comments {
		c.Username = names[c.UserID]
	}
}

// ResolveTranslationAuthors fills AuthorUsername with each author's current
// username. The stored snapshot is kept when the author no longer exists or
// has not picked a username.
func (s *UserStore) ResolveTranslationAuthors(ctx context.Context, translations []*model.Translation) {
	ids := make([]string, len(translations))
	for i, t := range translations {
		ids[i] = t.AuthorID
	}
	names := s.lookupNames(ctx, ids)
	for _, t := range translations {
		if name := names[t.AuthorID]; name != "" {
			t.AuthorUsername = name
		}
	}
}

func (s *UserStore) GetByToken(ctx context.Context, token string) (*model.User, error) {
	q := datastore.NewQuery(userKind).FilterField("token", "=", token).Limit(1)
	var entities []userEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("user_store: GetByToken: %w", err)
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return entityToUser(keys[0].Name, &entities[0])
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	q := datastore.NewQuery(userKind).FilterField("username", "=", username).Limit(1)
	var entities []userEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("user_store: GetByUsername: %w", err)
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return entityToUser(keys[0].Name, &entities[0])
}

// SetUsername atomically sets a unique username. Returns error if taken.
func (s *UserStore) SetUsername(ctx context.Context, userID, username string) error {
	// Check uniqueness first.
	existing, err := s.GetByUsername(ctx, username)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != userID {
		return fmt.Errorf("uzantnomo jam uzata")
	}
	key := s.userKey(userID)
	_, err = s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Username = username
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

// ClearUsername removes the username from a user account.
func (s *UserStore) ClearUsername(ctx context.Context, userID string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Username = ""
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

// ResolveUserRef looks up a user by ID first, then by username.
// Returns the user and its ID, or an error if not found.
func (s *UserStore) ResolveUserRef(ctx context.Context, ref string) (*model.User, error) {
	if u, err := s.GetByID(ctx, ref); err == nil && u != nil {
		return u, nil
	}
	if u, err := s.GetByUsername(ctx, ref); err == nil && u != nil {
		return u, nil
	}
	return nil, fmt.Errorf("uzanto ne trovita: %s", ref)
}

// userAliasKind stores old-ID → new-ID mappings so that passkeys registered
// under a merged (deleted) user can still resolve to the surviving account.
const userAliasKind = "UserAlias"

type userAliasEntity struct {
	TargetID string `datastore:"target_id"`
}

// ResolveAlias follows a UserAlias chain, returning the final live user ID.
// Returns the input id unchanged if no alias exists.
func (s *UserStore) ResolveAlias(ctx context.Context, id string) string {
	for i := 0; i < 5; i++ { // cap chain length
		var e userAliasEntity
		key := datastore.NameKey(userAliasKind, id, nil)
		if err := s.db.Get(ctx, key, &e); err != nil || e.TargetID == "" {
			return id
		}
		id = e.TargetID
	}
	return id
}

// MergeUsers merges srcID into dstID: copies progress and passkeys,
// keeps dst ratings, stores an alias so old passkeys still resolve, deletes src.
func (s *UserStore) MergeUsers(ctx context.Context, dstID, srcID string) error {
	dst, err := s.GetByID(ctx, dstID)
	if err != nil || dst == nil {
		return fmt.Errorf("cela uzanto ne trovita: %s", dstID)
	}
	src, err := s.GetByID(ctx, srcID)
	if err != nil || src == nil {
		return fmt.Errorf("fonta uzanto ne trovita: %s", srcID)
	}
	// Merge progress maps.
	for k, v := range src.Progress {
		if v {
			dst.Progress[k] = true
		}
	}
	// Keep the better live streak. StreakDays is derived from StreakStartAt
	// and LastPracticeAt, so the whole tuple must move together — an expired
	// streak (CurrentStreakDays 0) never beats a live one.
	if src.CurrentStreakDays() > dst.CurrentStreakDays() {
		dst.StreakDays = src.StreakDays
		dst.StreakStartAt = src.StreakStartAt
		dst.LastPracticeAt = src.LastPracticeAt
	}
	// Keep username from src if dst has none.
	if dst.Username == "" && src.Username != "" {
		dst.Username = src.Username
	}
	// Copy passkeys so that devices registered under src still work.
	for _, pk := range src.Passkeys {
		dst.Passkeys = append(dst.Passkeys, pk)
	}
	if err := s.Update(ctx, dst); err != nil {
		return err
	}
	// Store an alias so that FinishPasskeyLogin can resolve the old ID.
	aliasKey := datastore.NameKey(userAliasKind, srcID, nil)
	if _, err := s.db.Put(ctx, aliasKey, &userAliasEntity{TargetID: dstID}); err != nil {
		return err
	}
	return s.db.Delete(ctx, s.userKey(srcID))
}

func (s *UserStore) Update(ctx context.Context, u *model.User) error {
	e, err := userToEntity(u)
	if err != nil {
		return fmt.Errorf("user_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.userKey(u.ID), e)
	return err
}

func (s *UserStore) UpdateRating(ctx context.Context, userID string, rating, rd, volatility float64) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Rating = rating
		e.RD = rd
		e.Volatility = volatility
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

// advanceStreak computes the streak state for a practice happening at now,
// given the previous anchor and the time of the previous practice. It returns
// the (possibly reset) anchor day and the derived 1-indexed day count.
func advanceStreak(startAt, lastPractice, now time.Time) (time.Time, int) {
	today := now.UTC().Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	lastDay := lastPractice.UTC().Truncate(24 * time.Hour)

	switch {
	case lastDay.Equal(today):
		// Already practiced today — keep the anchor as-is.
		if startAt.IsZero() {
			startAt = today
		}
	case lastDay.Equal(yesterday):
		// Practiced yesterday — streak continues into today.
		if startAt.IsZero() {
			startAt = yesterday
		}
	default:
		// Gap of more than one day (or first practice ever) — reset.
		startAt = today
	}

	startDay := startAt.UTC().Truncate(24 * time.Hour)
	if startDay.After(today) {
		startDay = today
	}
	return startDay, int(today.Sub(startDay).Hours()/24) + 1
}

// UpdateStreakAndSeen records a practice: updates last_practice_at and
// last_seen_at and recalculates the streak. StreakStartAt is the canonical
// anchor; StreakDays is always derived from it. The streak is keyed off
// LastPracticeAt, not LastSeenAt — the latter is bumped by the middleware on
// every page view, so merely browsing must not keep a streak alive.
// Call after every exercise attempt.
func (s *UserStore) UpdateStreakAndSeen(ctx context.Context, userID string) (int, error) {
	key := s.userKey(userID)
	var newStreak int
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		now := time.Now().UTC()
		lastPractice := e.LastPracticeAt
		if lastPractice.IsZero() {
			// Pre-migration account: LastSeenAt is the only proxy we have.
			lastPractice = e.LastSeenAt
		}
		e.StreakStartAt, e.StreakDays = advanceStreak(e.StreakStartAt, lastPractice, now)
		newStreak = e.StreakDays
		e.LastPracticeAt = now
		e.LastSeenAt = now
		_, err := tx.Put(key, &e)
		return err
	})
	return newStreak, err
}

func (s *UserStore) AddPasskey(ctx context.Context, userID string, cred webauthn.Credential) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		var passkeys []webauthn.Credential
		if len(e.PasskeysJSON) > 0 {
			if err := json.Unmarshal(e.PasskeysJSON, &passkeys); err != nil {
				return err
			}
		}
		passkeys = append(passkeys, cred)
		b, err := json.Marshal(passkeys)
		if err != nil {
			return err
		}
		e.PasskeysJSON = b
		_, err = tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) UpdateLang(ctx context.Context, userID, lang string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Lang = lang
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) UpdateUILang(ctx context.Context, userID, lang string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.UILang = lang
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.LastSeenAt = time.Now()
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

// ToggleFavorite adds slug to the user's favorites if absent, removes it if
// present. Returns true if the slug is now a favorite (was added).
func (s *UserStore) ToggleFavorite(ctx context.Context, userID, slug string) (bool, error) {
	key := s.userKey(userID)
	var added bool
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		var favs []string
		if len(e.FavoritesJSON) > 0 {
			_ = json.Unmarshal(e.FavoritesJSON, &favs)
		}
		found := false
		var next []string
		for _, s := range favs {
			if s == slug {
				found = true
			} else {
				next = append(next, s)
			}
		}
		if found {
			added = false
			favs = next
		} else {
			added = true
			favs = append([]string{slug}, favs...) // prepend so newest first
		}
		b, err := json.Marshal(favs)
		if err != nil {
			return err
		}
		e.FavoritesJSON = b
		_, err = tx.Put(key, &e)
		return err
	})
	return added, err
}

// UpdateKeepDataDays sets how many idle days before the account is auto-deleted.
// -1 = never, 0 = use default (7 for anon, 365 for named), >0 = explicit days.
func (s *UserStore) UpdateKeepDataDays(ctx context.Context, userID string, days int) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.KeepDataDays = days
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

// DeleteUser removes the user entity from Datastore.
func (s *UserStore) DeleteUser(ctx context.Context, userID string) error {
	return s.db.Delete(ctx, s.userKey(userID))
}

// ListDueForDeletion returns users whose inactivity exceeds their retention
// threshold. Anonymous users (no username) are deleted after 7 days; named
// users after their KeepDataDays (defaults to 365 if 0, skip if -1).
func (s *UserStore) ListDueForDeletion(ctx context.Context) ([]*model.User, error) {
	now := time.Now()

	// Fetch users not seen in the past 7 days — the minimum threshold.
	cutoff := now.Add(-7 * 24 * time.Hour)
	q := datastore.NewQuery(userKind).
		FilterField("last_seen_at", "<", cutoff).
		Limit(500)
	var entities []userEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("user_store: ListDueForDeletion: %w", err)
	}

	var due []*model.User
	for i, k := range keys {
		u, err := entityToUser(k.Name, &entities[i])
		if err != nil {
			continue
		}
		idle := now.Sub(u.LastSeenAt)
		var threshold time.Duration
		if u.Username == "" {
			// Anonymous: always 7 days
			threshold = 7 * 24 * time.Hour
		} else {
			days := u.KeepDataDays
			if days == -1 {
				continue // never delete
			}
			if days == 0 {
				days = 365 // default for named accounts
			}
			threshold = time.Duration(days) * 24 * time.Hour
		}
		if idle >= threshold {
			due = append(due, u)
		}
	}
	return due, nil
}

// ListTopUsers returns up to limit users who have set a username, ordered by
// rating descending. Used for the hall of fame page.
func (s *UserStore) ListTopUsers(ctx context.Context, limit int) ([]*model.User, error) {
	// Fetch named users sorted by username (username index exists); sort by
	// rating in Go to avoid needing a composite index.
	q := datastore.NewQuery(userKind).
		FilterField("username", ">", "").
		Order("username").
		Limit(500)
	var entities []userEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("user_store: ListTopUsers: %w", err)
	}
	users := make([]*model.User, 0, len(keys))
	for i, k := range keys {
		u, err := entityToUser(k.Name, &entities[i])
		if err != nil {
			continue
		}
		users = append(users, u)
	}
	// Sort by rating descending.
	sort.Slice(users, func(i, j int) bool {
		return users[i].Rating > users[j].Rating
	})
	if len(users) > limit {
		users = users[:limit]
	}
	return users, nil
}

// RenameFavorite scans all users and replaces oldFav with newFav in their Favorites.
// Returns the number of users updated. Used when renaming series/tag slugs.
func (s *UserStore) RenameFavorite(ctx context.Context, oldFav, newFav string) (int, error) {
	// Keys-only scan to find all users.
	q := datastore.NewQuery(userKind).KeysOnly()
	keys, err := s.db.GetAll(ctx, q, nil)
	if err != nil {
		return 0, fmt.Errorf("user_store: RenameFavorite scan: %w", err)
	}

	updated := 0
	// Process in batches of 100.
	for i := 0; i < len(keys); i += 100 {
		end := i + 100
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]
		entities := make([]userEntity, len(batch))
		if err := s.db.GetMulti(ctx, batch, entities); err != nil {
			// Partial errors are ok — skip missing entities.
			if me, ok := err.(datastore.MultiError); ok {
				for j, e := range me {
					if e != nil && e != datastore.ErrNoSuchEntity {
						_ = e
						_ = j
					}
				}
			}
		}
		for j, key := range batch {
			var favs []string
			if len(entities[j].FavoritesJSON) > 0 {
				_ = json.Unmarshal(entities[j].FavoritesJSON, &favs)
			}
			changed := false
			for k, f := range favs {
				if f == oldFav {
					favs[k] = newFav
					changed = true
				}
			}
			if !changed {
				continue
			}
			b, _ := json.Marshal(favs)
			entities[j].FavoritesJSON = b
			if _, err := s.db.Put(ctx, key, &entities[j]); err == nil {
				updated++
			}
		}
	}
	return updated, nil
}
