package store

import (
	"context"
	"fmt"
	"strconv"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
)

const translationKind = "Translation"

type translationEntity struct {
	TargetType string `datastore:"target_type"`
	TargetID   string `datastore:"target_id"`
	Language   string `datastore:"language"`
	Text       string `datastore:"text,noindex"`
	AuthorID   string `datastore:"author_id"`
	VoteScore  int    `datastore:"vote_score"`
}

type TranslationStore struct {
	db *datastore.Client
}

func NewTranslationStore(db *datastore.Client) *TranslationStore {
	return &TranslationStore{db: db}
}

func (s *TranslationStore) Create(ctx context.Context, t *model.Translation) error {
	e := &translationEntity{
		TargetType: t.TargetType,
		TargetID:   t.TargetID,
		Language:   t.Language,
		Text:       t.Text,
		AuthorID:   t.AuthorID,
		VoteScore:  t.VoteScore,
	}
	key := datastore.IncompleteKey(translationKind, nil)
	key, err := s.db.Put(ctx, key, e)
	if err != nil {
		return fmt.Errorf("translation_store: Create: %w", err)
	}
	t.ID = strconv.FormatInt(key.ID, 10)
	return nil
}

func (s *TranslationStore) ListByTarget(ctx context.Context, targetType, targetID string) ([]*model.Translation, error) {
	q := datastore.NewQuery(translationKind).
		FilterField("target_type", "=", targetType).
		FilterField("target_id", "=", targetID)
	var entities []translationEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("translation_store: query: %w", err)
	}
	out := make([]*model.Translation, len(entities))
	for i, e := range entities {
		out[i] = &model.Translation{
			ID:         strconv.FormatInt(keys[i].ID, 10),
			TargetType: e.TargetType,
			TargetID:   e.TargetID,
			Language:   e.Language,
			Text:       e.Text,
			AuthorID:   e.AuthorID,
			VoteScore:  e.VoteScore,
		}
	}
	return out, nil
}
