package model

// Vote records a +1 or -1 vote by a user on a content item.
type Vote struct {
	UserID        string `firestore:"user_id"`
	ContentItemID string `firestore:"content_item_id"`
	Value         int    `firestore:"value"` // +1 or -1
}
