package store

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

const voteKind = "Vote"

type voteEntity struct {
	UserID        string `datastore:"user_id"`
	ContentItemID string `datastore:"content_item_id"`
	Value         int    `datastore:"value"`
}

type VoteStore struct {
	db *datastore.Client
}

func NewVoteStore(db *datastore.Client) *VoteStore {
	return &VoteStore{db: db}
}

func voteKey(userID, contentItemID string) *datastore.Key {
	return datastore.NameKey(voteKind, userID+"_"+contentItemID, nil)
}

// Toggle applies an up/down vote with toggle-off semantics: voting the same
// value again removes the vote. The vote entity and the content item's score
// are updated in a single transaction so rapid repeat clicks cannot drift the
// score. Returns the user's effective vote value and the new total score.
func (s *VoteStore) Toggle(ctx context.Context, userID, contentID string, newValue int) (int, int, error) {
	vKey := voteKey(userID, contentID)
	cKey := datastore.NameKey(contentKind, contentID, nil)
	var effective, score int
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		existing := 0
		var ve voteEntity
		switch err := tx.Get(vKey, &ve); err {
		case nil:
			existing = ve.Value
		case datastore.ErrNoSuchEntity:
		default:
			return err
		}

		var delta int
		if existing == newValue {
			// Toggle off: remove existing vote.
			effective, delta = 0, -existing
		} else {
			effective, delta = newValue, newValue-existing
		}

		if effective == 0 {
			if err := tx.Delete(vKey); err != nil {
				return err
			}
		} else {
			ve := &voteEntity{UserID: userID, ContentItemID: contentID, Value: effective}
			if _, err := tx.Put(vKey, ve); err != nil {
				return err
			}
		}

		var ce contentEntity
		if err := tx.Get(cKey, &ce); err != nil {
			return err
		}
		if delta != 0 {
			ce.VoteScore += delta
			ce.UpdatedAt = time.Now()
			if _, err := tx.Put(cKey, &ce); err != nil {
				return err
			}
		}
		score = ce.VoteScore
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("vote_store: Toggle: %w", err)
	}
	return effective, score, nil
}

func (s *VoteStore) GetByUserAndContent(ctx context.Context, userID, contentID string) (*model.Vote, error) {
	var e voteEntity
	if err := s.db.Get(ctx, voteKey(userID, contentID), &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, fmt.Errorf("vote_store: Get: %w", err)
	}
	return &model.Vote{
		UserID:        e.UserID,
		ContentItemID: e.ContentItemID,
		Value:         e.Value,
	}, nil
}
