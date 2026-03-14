package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
)

const commentKind = "Comment"

type commentEntity struct {
	UserID        string    `datastore:"user_id"`
	ContentItemID string    `datastore:"content_item_id"`
	Text          string    `datastore:"text,noindex"`
	Approved      bool      `datastore:"approved"`
	AutoApproved  bool      `datastore:"auto_approved"`
	CreatedAt     time.Time `datastore:"created_at"`
}

type CommentStore struct {
	db *datastore.Client
}

func NewCommentStore(db *datastore.Client) *CommentStore {
	return &CommentStore{db: db}
}

func (s *CommentStore) Create(ctx context.Context, c *model.Comment) error {
	c.CreatedAt = time.Now()
	e := &commentEntity{
		UserID:        c.UserID,
		ContentItemID: c.ContentItemID,
		Text:          c.Text,
		Approved:      c.Approved,
		AutoApproved:  c.AutoApproved,
		CreatedAt:     c.CreatedAt,
	}
	key := datastore.IncompleteKey(commentKind, nil)
	key, err := s.db.Put(ctx, key, e)
	if err != nil {
		return fmt.Errorf("comment_store: Create: %w", err)
	}
	c.ID = strconv.FormatInt(key.ID, 10)
	return nil
}

func (s *CommentStore) ListApprovedByContent(ctx context.Context, contentID string) ([]*model.Comment, error) {
	q := datastore.NewQuery(commentKind).
		FilterField("content_item_id", "=", contentID).
		FilterField("approved", "=", true).
		Order("created_at")
	return s.runQuery(ctx, q)
}

func (s *CommentStore) ListPending(ctx context.Context, limit int) ([]*model.Comment, error) {
	q := datastore.NewQuery(commentKind).
		FilterField("approved", "=", false).
		Order("created_at").
		Limit(limit)
	return s.runQuery(ctx, q)
}

func (s *CommentStore) Approve(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("comment_store: bad id %s: %w", id, err)
	}
	key := datastore.IDKey(commentKind, n, nil)
	_, err = s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e commentEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Approved = true
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *CommentStore) Reject(ctx context.Context, id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("comment_store: bad id %s: %w", id, err)
	}
	return s.db.Delete(ctx, datastore.IDKey(commentKind, n, nil))
}

func (s *CommentStore) runQuery(ctx context.Context, q *datastore.Query) ([]*model.Comment, error) {
	var entities []commentEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("comment_store: query: %w", err)
	}
	out := make([]*model.Comment, len(entities))
	for i, e := range entities {
		out[i] = &model.Comment{
			ID:            strconv.FormatInt(keys[i].ID, 10),
			UserID:        e.UserID,
			ContentItemID: e.ContentItemID,
			Text:          e.Text,
			Approved:      e.Approved,
			AutoApproved:  e.AutoApproved,
			CreatedAt:     e.CreatedAt,
		}
	}
	return out, nil
}
