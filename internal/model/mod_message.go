package model

import "time"

// ModMessage is a message sent to the mod/admin team by a user.
type ModMessage struct {
	ID            string    `datastore:"-"`
	UserID        string    `datastore:"user_id"`
	Username      string    `datastore:"username,noindex"`
	Text          string    `datastore:"text,noindex"`
	ContentItemID string    `datastore:"content_item_id,noindex"`
	Read          bool      `datastore:"read"`
	CreatedAt     time.Time `datastore:"created_at"`
}
