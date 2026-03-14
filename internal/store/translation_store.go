package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
)

const translationKind = "Translation"
const translationVoteKind = "TranslationVote"

type translationEntity struct {
	TargetID  string    `datastore:"target_id"`
	Language  string    `datastore:"language"`
	Text      string    `datastore:"text,noindex"`
	AuthorID  string    `datastore:"author_id"`
	VoteScore int       `datastore:"vote_score"`
	CreatedAt time.Time `datastore:"created_at"`
}

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

func (s *TranslationStore) Create(ctx context.Context, t *model.Translation) error {
	t.CreatedAt = time.Now()
	e := &translationEntity{
		TargetID:  t.TargetID,
		Language:  t.Language,
		Text:      t.Text,
		AuthorID:  t.AuthorID,
		VoteScore: 0,
		CreatedAt: t.CreatedAt,
	}
	key := datastore.IncompleteKey(translationKind, nil)
	key, err := s.db.Put(ctx, key, e)
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
	key := datastore.NameKey(translationVoteKind, userID+"_"+translationID, nil)
	var e translationVoteEntity
	if err := s.db.Get(ctx, key, &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return 0, nil
		}
		return 0, err
	}
	return e.Value, nil
}

// Vote records or toggles a vote (+1/-1) on a translation and updates its score.
// Returns the effective vote value after the action.
func (s *TranslationStore) Vote(ctx context.Context, userID, translationID string, newValue int) (int, error) {
	existing, err := s.GetVote(ctx, userID, translationID)
	if err != nil {
		return 0, err
	}

	var effectiveValue, delta int
	if existing == newValue {
		effectiveValue = 0
		delta = -existing
	} else {
		effectiveValue = newValue
		delta = newValue - existing
	}

	voteKey := datastore.NameKey(translationVoteKind, userID+"_"+translationID, nil)
	if effectiveValue == 0 {
		_ = s.db.Delete(ctx, voteKey)
	} else {
		ve := &translationVoteEntity{
			UserID:        userID,
			TranslationID: translationID,
			Value:         effectiveValue,
		}
		if _, err := s.db.Put(ctx, voteKey, ve); err != nil {
			return 0, err
		}
	}

	if delta != 0 {
		n, parseErr := strconv.ParseInt(translationID, 10, 64)
		if parseErr != nil {
			return effectiveValue, fmt.Errorf("translation_store: bad id %s: %w", translationID, parseErr)
		}
		tKey := datastore.IDKey(translationKind, n, nil)
		_, txErr := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
			var e translationEntity
			if err := tx.Get(tKey, &e); err != nil {
				return err
			}
			e.VoteScore += delta
			_, err := tx.Put(tKey, &e)
			return err
		})
		if txErr != nil {
			return effectiveValue, txErr
		}
	}

	return effectiveValue, nil
}

func (s *TranslationStore) runQuery(ctx context.Context, q *datastore.Query) ([]*model.Translation, error) {
	var entities []translationEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("translation_store: query: %w", err)
	}
	out := make([]*model.Translation, len(entities))
	for i, e := range entities {
		out[i] = &model.Translation{
			ID:        strconv.FormatInt(keys[i].ID, 10),
			TargetID:  e.TargetID,
			Language:  e.Language,
			Text:      e.Text,
			AuthorID:  e.AuthorID,
			VoteScore: e.VoteScore,
			CreatedAt: e.CreatedAt,
		}
	}
	return out, nil
}
