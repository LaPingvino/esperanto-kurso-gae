package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

const attemptKind = "Attempt"

type attemptEntity struct {
	UserID        string    `datastore:"user_id"`
	ContentItemID string    `datastore:"content_item_id"`
	Correct       bool      `datastore:"correct"`
	Answer        string    `datastore:"answer,noindex"`
	TimeMS        int64     `datastore:"time_taken_ms"`
	Timestamp     time.Time `datastore:"timestamp"`
}

type AttemptStore struct {
	db *datastore.Client
}

func NewAttemptStore(db *datastore.Client) *AttemptStore {
	return &AttemptStore{db: db}
}

func (s *AttemptStore) Create(ctx context.Context, a *model.Attempt) error {
	e := &attemptEntity{
		UserID:        a.UserID,
		ContentItemID: a.ContentItemID,
		Correct:       a.Correct,
		Answer:        a.Answer,
		TimeMS:        a.TimeMS,
		Timestamp:     a.Timestamp,
	}
	key := datastore.IncompleteKey(attemptKind, nil)
	key, err := s.db.Put(ctx, key, e)
	if err != nil {
		return fmt.Errorf("attempt_store: Create: %w", err)
	}
	a.ID = strconv.FormatInt(key.ID, 10)
	return nil
}

func (s *AttemptStore) ListByUser(ctx context.Context, userID string, limit int) ([]*model.Attempt, error) {
	q := datastore.NewQuery(attemptKind).
		FilterField("user_id", "=", userID).
		Order("-timestamp").
		Limit(limit)
	return s.runQuery(ctx, q)
}

func (s *AttemptStore) ListByContent(ctx context.Context, contentID string, limit int) ([]*model.Attempt, error) {
	q := datastore.NewQuery(attemptKind).
		FilterField("content_item_id", "=", contentID).
		Order("-timestamp").
		Limit(limit)
	return s.runQuery(ctx, q)
}

func (s *AttemptStore) runQuery(ctx context.Context, q *datastore.Query) ([]*model.Attempt, error) {
	var entities []attemptEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("attempt_store: query: %w", err)
	}
	out := make([]*model.Attempt, len(entities))
	for i, e := range entities {
		out[i] = &model.Attempt{
			ID:            strconv.FormatInt(keys[i].ID, 10),
			UserID:        e.UserID,
			ContentItemID: e.ContentItemID,
			Correct:       e.Correct,
			Answer:        e.Answer,
			TimeMS:        e.TimeMS,
			Timestamp:     e.Timestamp,
		}
	}
	return out, nil
}
