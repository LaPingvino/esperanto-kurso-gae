package store

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

const commentKind = "Comment"

type commentEntity struct {
	UserID        string    `datastore:"user_id"`
	ContentItemID string    `datastore:"content_item_id"`
	Text          string    `datastore:"text,noindex"`
	Approved      bool      `datastore:"approved"`
	AutoApproved  bool      `datastore:"auto_approved"`
	Language      string    `datastore:"language,noindex"`
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
		Language:      c.Language,
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
	// Keys-only query to get all comment keys for this content item.
	// The keys are eventually consistent for new entities, but already-existing
	// entities (like those awaiting moderation approval) will appear.
	q := datastore.NewQuery(commentKind).
		FilterField("content_item_id", "=", contentID).
		KeysOnly()
	keys, err := s.db.GetAll(ctx, q, nil)
	if err != nil {
		return nil, fmt.Errorf("comment_store: list keys: %w", err)
	}
	if len(keys) == 0 {
		return nil, nil
	}
	// GetMulti by key is strongly consistent — reflects the current approved state.
	entities := make([]commentEntity, len(keys))
	if err := s.db.GetMulti(ctx, keys, entities); err != nil {
		if me, ok := err.(datastore.MultiError); ok {
			for _, e := range me {
				if e != nil && e != datastore.ErrNoSuchEntity {
					return nil, fmt.Errorf("comment_store: get multi: %w", e)
				}
			}
		} else {
			return nil, fmt.Errorf("comment_store: get multi: %w", err)
		}
	}
	var approved []*model.Comment
	for i, e := range entities {
		if !e.Approved {
			continue
		}
		approved = append(approved, &model.Comment{
			ID:            strconv.FormatInt(keys[i].ID, 10),
			UserID:        e.UserID,
			ContentItemID: e.ContentItemID,
			Text:          e.Text,
			Approved:      e.Approved,
			AutoApproved:  e.AutoApproved,
			Language:      e.Language,
			CreatedAt:     e.CreatedAt,
		})
	}
	sort.Slice(approved, func(i, j int) bool {
		return approved[i].CreatedAt.Before(approved[j].CreatedAt)
	})
	return approved, nil
}

func (s *CommentStore) ListPending(ctx context.Context, limit int) ([]*model.Comment, error) {
	// Single-field filter, sort in Go — no composite index needed.
	q := datastore.NewQuery(commentKind).FilterField("approved", "=", false).Limit(limit * 3)
	all, err := s.runQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// ListAllApproved returns all approved comments, sorted newest first, up to limit.
func (s *CommentStore) ListAllApproved(ctx context.Context, limit int) ([]*model.Comment, error) {
	q := datastore.NewQuery(commentKind).FilterField("approved", "=", true).Limit(limit)
	all, err := s.runQuery(ctx, q)
	if err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	return all, nil
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
			Language:      e.Language,
			CreatedAt:     e.CreatedAt,
		}
	}
	return out, nil
}
