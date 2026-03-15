package store

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

const modMsgKind = "ModMessage"

type ModMessageStore struct {
	db *datastore.Client
}

func NewModMessageStore(db *datastore.Client) *ModMessageStore {
	return &ModMessageStore{db: db}
}

func (s *ModMessageStore) Create(ctx context.Context, m *model.ModMessage) error {
	m.CreatedAt = time.Now()
	key := datastore.IncompleteKey(modMsgKind, nil)
	newKey, err := s.db.Put(ctx, key, m)
	if err != nil {
		return fmt.Errorf("mod_message_store: Create: %w", err)
	}
	m.ID = fmt.Sprintf("%d", newKey.ID)
	return nil
}

func (s *ModMessageStore) ListUnread(ctx context.Context, limit int) ([]*model.ModMessage, error) {
	// Avoid querying on boolean false (zero-value Datastore issue); fetch recent
	// messages and filter in Go instead.
	q := datastore.NewQuery(modMsgKind).
		Order("-created_at").
		Limit(limit * 10)
	var msgs []*model.ModMessage
	keys, err := s.db.GetAll(ctx, q, &msgs)
	if err != nil {
		return nil, fmt.Errorf("mod_message_store: ListUnread: %w", err)
	}
	var unread []*model.ModMessage
	for i, k := range keys {
		msgs[i].ID = fmt.Sprintf("%d", k.ID)
		if !msgs[i].Read {
			unread = append(unread, msgs[i])
			if len(unread) >= limit {
				break
			}
		}
	}
	return unread, nil
}

func (s *ModMessageStore) MarkRead(ctx context.Context, id string) error {
	key, err := datastore.DecodeKey(id)
	if err != nil {
		// Try as int64 ID
		var intID int64
		if _, e := fmt.Sscanf(id, "%d", &intID); e != nil {
			return fmt.Errorf("mod_message_store: bad id: %s", id)
		}
		key = datastore.IDKey(modMsgKind, intID, nil)
	}
	_, err = s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var m model.ModMessage
		if err := tx.Get(key, &m); err != nil {
			return err
		}
		m.Read = true
		_, err := tx.Put(key, &m)
		return err
	})
	return err
}
