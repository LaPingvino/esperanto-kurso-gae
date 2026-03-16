package model

import "time"

// Comment is a user-submitted comment on a content item.
type Comment struct {
	ID            string    `firestore:"-"`
	UserID        string    `firestore:"user_id"`
	ContentItemID string    `firestore:"content_item_id"`
	Text          string    `firestore:"text"`
	Approved      bool      `firestore:"approved"`
	AutoApproved  bool      `firestore:"auto_approved"`
	Language      string    `firestore:"language"` // UI language of the author at submission time
	CreatedAt     time.Time `firestore:"created_at"`
}
