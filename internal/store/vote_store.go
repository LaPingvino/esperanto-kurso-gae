package store

import (
	"context"
	"fmt"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
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

func (s *VoteStore) Upsert(ctx context.Context, v *model.Vote) error {
	e := &voteEntity{
		UserID:        v.UserID,
		ContentItemID: v.ContentItemID,
		Value:         v.Value,
	}
	_, err := s.db.Put(ctx, voteKey(v.UserID, v.ContentItemID), e)
	if err != nil {
		return fmt.Errorf("vote_store: Upsert: %w", err)
	}
	return nil
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
