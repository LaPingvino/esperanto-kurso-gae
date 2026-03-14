package model

import "time"

// Attempt records a single user answer to a content item.
type Attempt struct {
	ID            string    `firestore:"-"`
	UserID        string    `firestore:"user_id"`
	ContentItemID string    `firestore:"content_item_id"`
	Correct       bool      `firestore:"correct"`
	Answer        string    `firestore:"answer"`
	TimeMS        int64     `firestore:"time_taken_ms"`
	Timestamp     time.Time `firestore:"timestamp"`
}
