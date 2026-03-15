package model

import "time"

// Translation holds a community-contributed definition in a specific language.
type Translation struct {
	ID        string    `datastore:"-"`
	TargetID  string    `datastore:"target_id"`  // content item slug
	Language  string    `datastore:"language"`
	Text      string    `datastore:"text,noindex"`
	AuthorID       string    `datastore:"author_id"`
	AuthorUsername string    `datastore:"author_username,noindex"`
	VoteScore      int       `datastore:"vote_score"`
	CreatedAt time.Time `datastore:"created_at"`
}
