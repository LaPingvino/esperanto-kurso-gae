package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
)

const contentKind = "ContentItem"

type contentEntity struct {
	Type        string    `datastore:"type"`
	ContentJSON []byte    `datastore:"content_json,noindex"`
	Tags        []string  `datastore:"tags"`
	Source      string    `datastore:"source"`
	AuthorID    string    `datastore:"author_id"`
	Status      string    `datastore:"status"`
	Rating      float64   `datastore:"rating"`
	RD          float64   `datastore:"rd"`
	Volatility  float64   `datastore:"volatility"`
	VoteScore   int       `datastore:"vote_score"`
	Version     int       `datastore:"version"`
	ImageURL    string    `datastore:"image_url,noindex"`
	SeriesSlug  string    `datastore:"series_slug"`
	SeriesOrder int       `datastore:"series_order"`
	CreatedAt   time.Time `datastore:"created_at"`
	UpdatedAt   time.Time `datastore:"updated_at"`
}

func contentToEntity(item *model.ContentItem) (*contentEntity, error) {
	b, err := json.Marshal(item.Content)
	if err != nil {
		return nil, err
	}
	return &contentEntity{
		Type:        item.Type,
		ContentJSON: b,
		Tags:        item.Tags,
		Source:      item.Source,
		AuthorID:    item.AuthorID,
		Status:      item.Status,
		Rating:      item.Rating,
		RD:          item.RD,
		Volatility:  item.Volatility,
		VoteScore:   item.VoteScore,
		Version:     item.Version,
		ImageURL:    item.ImageURL,
		SeriesSlug:  item.SeriesSlug,
		SeriesOrder: item.SeriesOrder,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func entityToContent(slug string, e *contentEntity) (*model.ContentItem, error) {
	item := &model.ContentItem{
		Slug:        slug,
		Type:        e.Type,
		Tags:        e.Tags,
		Source:      e.Source,
		AuthorID:    e.AuthorID,
		Status:      e.Status,
		Rating:      e.Rating,
		RD:          e.RD,
		Volatility:  e.Volatility,
		VoteScore:   e.VoteScore,
		Version:     e.Version,
		ImageURL:    e.ImageURL,
		SeriesSlug:  e.SeriesSlug,
		SeriesOrder: e.SeriesOrder,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
	if len(e.ContentJSON) > 0 {
		if err := json.Unmarshal(e.ContentJSON, &item.Content); err != nil {
			return nil, err
		}
	}
	return item, nil
}

type ContentStore struct {
	db *datastore.Client
}

func NewContentStore(db *datastore.Client) *ContentStore {
	return &ContentStore{db: db}
}

func (s *ContentStore) contentKey(slug string) *datastore.Key {
	return datastore.NameKey(contentKind, slug, nil)
}

func (s *ContentStore) Create(ctx context.Context, item *model.ContentItem) error {
	now := time.Now()
	item.CreatedAt = now
	item.UpdatedAt = now
	e, err := contentToEntity(item)
	if err != nil {
		return fmt.Errorf("content_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.contentKey(item.Slug), e)
	return err
}

func (s *ContentStore) GetBySlug(ctx context.Context, slug string) (*model.ContentItem, error) {
	var e contentEntity
	if err := s.db.Get(ctx, s.contentKey(slug), &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, fmt.Errorf("content_store: GetBySlug %s: %w", slug, err)
	}
	return entityToContent(slug, &e)
}

func (s *ContentStore) Update(ctx context.Context, item *model.ContentItem) error {
	item.UpdatedAt = time.Now()
	e, err := contentToEntity(item)
	if err != nil {
		return fmt.Errorf("content_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.contentKey(item.Slug), e)
	return err
}

func (s *ContentStore) ListByType(ctx context.Context, typ string, limit int) ([]*model.ContentItem, error) {
	q := datastore.NewQuery(contentKind).
		FilterField("status", "=", "approved").
		FilterField("type", "=", typ).
		Limit(limit)
	return s.runContentQuery(ctx, q)
}

func (s *ContentStore) Delete(ctx context.Context, slug string) error {
	return s.db.Delete(ctx, s.contentKey(slug))
}

func (s *ContentStore) ListApproved(ctx context.Context, limit int) ([]*model.ContentItem, error) {
	q := datastore.NewQuery(contentKind).
		FilterField("status", "=", "approved").
		Limit(limit)
	return s.runContentQuery(ctx, q)
}

func (s *ContentStore) ListByRatingRange(ctx context.Context, minR, maxR float64, limit int) ([]*model.ContentItem, error) {
	// Datastore allows equality on one field + range on another with a composite index.
	q := datastore.NewQuery(contentKind).
		FilterField("status", "=", "approved").
		FilterField("rating", ">=", minR).
		FilterField("rating", "<=", maxR).
		Order("rating").
		Limit(limit)
	return s.runContentQuery(ctx, q)
}

func (s *ContentStore) ListForAdmin(ctx context.Context, statusFilter string, limit int) ([]*model.ContentItem, error) {
	q := datastore.NewQuery(contentKind).Limit(limit)
	if statusFilter != "" {
		q = q.FilterField("status", "=", statusFilter)
	}
	return s.runContentQuery(ctx, q)
}

func (s *ContentStore) UpdateRating(ctx context.Context, slug string, rating, rd, volatility float64) error {
	key := s.contentKey(slug)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e contentEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Rating = rating
		e.RD = rd
		e.Volatility = volatility
		e.UpdatedAt = time.Now()
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *ContentStore) UpdateVoteScore(ctx context.Context, slug string, delta int) error {
	key := s.contentKey(slug)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e contentEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.VoteScore += delta
		e.UpdatedAt = time.Now()
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *ContentStore) runContentQuery(ctx context.Context, q *datastore.Query) ([]*model.ContentItem, error) {
	var entities []contentEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("content_store: query: %w", err)
	}
	items := make([]*model.ContentItem, 0, len(entities))
	for i, e := range entities {
		item, err := entityToContent(keys[i].Name, &e)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
