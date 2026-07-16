package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

const translationKind = "Translation"
const translationVoteKind = "TranslationVote"

type translationVoteEntity struct {
	UserID        string `datastore:"user_id"`
	TranslationID string `datastore:"translation_id"`
	Value         int    `datastore:"value"`
}

type TranslationStore struct {
	db *datastore.Client
}

func NewTranslationStore(db *datastore.Client) *TranslationStore {
	return &TranslationStore{db: db}
}

func translationKey(id string) (*datastore.Key, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("translation_store: bad id %q: %w", id, err)
	}
	return datastore.IDKey(translationKind, n, nil), nil
}

func translationVoteKey(userID, translationID string) *datastore.Key {
	return datastore.NameKey(translationVoteKind, userID+"_"+translationID, nil)
}

func (s *TranslationStore) Create(ctx context.Context, t *model.Translation) error {
	t.CreatedAt = time.Now()
	key, err := s.db.Put(ctx, datastore.IncompleteKey(translationKind, nil), t)
	if err != nil {
		return fmt.Errorf("translation_store: Create: %w", err)
	}
	t.ID = strconv.FormatInt(key.ID, 10)
	return nil
}

// ListByTarget returns all translations for a content item slug, ordered by vote score desc.
func (s *TranslationStore) ListByTarget(ctx context.Context, targetID string) ([]*model.Translation, error) {
	q := datastore.NewQuery(translationKind).
		FilterField("target_id", "=", targetID).
		Order("-vote_score")
	return s.runQuery(ctx, q)
}

// GetVote returns the current user vote value for a translation (0 if none).
func (s *TranslationStore) GetVote(ctx context.Context, userID, translationID string) (int, error) {
	var e translationVoteEntity
	if err := s.db.Get(ctx, translationVoteKey(userID, translationID), &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return 0, nil
		}
		return 0, err
	}
	return e.Value, nil
}

// GetVotes returns the user's vote values for the given translations in one
// batched lookup, keyed by translation ID. Translations without a vote are
// absent from the map.
func (s *TranslationStore) GetVotes(ctx context.Context, userID string, translations []*model.Translation) (map[string]int, error) {
	votes := map[string]int{}
	if len(translations) == 0 {
		return votes, nil
	}
	keys := make([]*datastore.Key, len(translations))
	for i, t := range translations {
		keys[i] = translationVoteKey(userID, t.ID)
	}
	entities := make([]translationVoteEntity, len(keys))
	err := s.db.GetMulti(ctx, keys, entities)
	if err != nil {
		me, ok := err.(datastore.MultiError)
		if !ok {
			return nil, fmt.Errorf("translation_store: GetVotes: %w", err)
		}
		for i, e := range me {
			if e == nil && entities[i].Value != 0 {
				votes[translations[i].ID] = entities[i].Value
			} else if e != nil && e != datastore.ErrNoSuchEntity {
				return nil, fmt.Errorf("translation_store: GetVotes: %w", e)
			}
		}
		return votes, nil
	}
	for i, e := range entities {
		if e.Value != 0 {
			votes[translations[i].ID] = e.Value
		}
	}
	return votes, nil
}

// Vote records or toggles a vote (+1/-1) on a translation. The vote entity and
// the translation's score are updated in a single transaction so rapid repeat
// clicks cannot drift the score. Returns the effective vote value after the action.
func (s *TranslationStore) Vote(ctx context.Context, userID, translationID string, newValue int) (int, error) {
	tKey, err := translationKey(translationID)
	if err != nil {
		return 0, err
	}
	voteKey := translationVoteKey(userID, translationID)
	var effectiveValue int
	_, err = s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		existing := 0
		var ve translationVoteEntity
		switch err := tx.Get(voteKey, &ve); err {
		case nil:
			existing = ve.Value
		case datastore.ErrNoSuchEntity:
		default:
			return err
		}

		var delta int
		if existing == newValue {
			// Toggle off: remove existing vote.
			effectiveValue, delta = 0, -existing
		} else {
			effectiveValue, delta = newValue, newValue-existing
		}

		if effectiveValue == 0 {
			if err := tx.Delete(voteKey); err != nil {
				return err
			}
		} else {
			ve := &translationVoteEntity{UserID: userID, TranslationID: translationID, Value: effectiveValue}
			if _, err := tx.Put(voteKey, ve); err != nil {
				return err
			}
		}

		if delta != 0 {
			var t model.Translation
			if err := tx.Get(tKey, &t); err != nil {
				return err
			}
			t.VoteScore += delta
			if _, err := tx.Put(tKey, &t); err != nil {
				return err
			}
		}
		return nil
	})
	return effectiveValue, err
}

// GetByID returns a single translation by its numeric string ID.
func (s *TranslationStore) GetByID(ctx context.Context, id string) (*model.Translation, error) {
	key, err := translationKey(id)
	if err != nil {
		return nil, err
	}
	var t model.Translation
	if err := s.db.Get(ctx, key, &t); err != nil {
		return nil, err
	}
	t.ID = id
	return &t, nil
}

// UpdateText updates the text of an existing translation.
func (s *TranslationStore) UpdateText(ctx context.Context, id, newText string) error {
	key, err := translationKey(id)
	if err != nil {
		return err
	}
	_, err = s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var t model.Translation
		if err := tx.Get(key, &t); err != nil {
			return err
		}
		t.Text = newText
		_, err := tx.Put(key, &t)
		return err
	})
	return err
}

// Delete removes a translation by ID.
func (s *TranslationStore) Delete(ctx context.Context, id string) error {
	key, err := translationKey(id)
	if err != nil {
		return err
	}
	return s.db.Delete(ctx, key)
}

// ListAll returns the most-recently-added translations up to limit.
func (s *TranslationStore) ListAll(ctx context.Context, limit int) ([]*model.Translation, error) {
	q := datastore.NewQuery(translationKind).
		Order("-created_at").
		Limit(limit)
	return s.runQuery(ctx, q)
}

func (s *TranslationStore) runQuery(ctx context.Context, q *datastore.Query) ([]*model.Translation, error) {
	var out []*model.Translation
	keys, err := s.db.GetAll(ctx, q, &out)
	if err != nil {
		return nil, fmt.Errorf("translation_store: query: %w", err)
	}
	for i, k := range keys {
		out[i].ID = strconv.FormatInt(k.ID, 10)
	}
	return out, nil
}
