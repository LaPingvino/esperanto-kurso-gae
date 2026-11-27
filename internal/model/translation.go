package model

import (
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/tolerant"
)

// Translation holds a community-contributed definition in a specific language.
type Translation struct {
	ID             string    `datastore:"-"`
	TargetID       string    `datastore:"target_id"` // content item slug
	Language       string    `datastore:"language"`
	Text           string    `datastore:"text,noindex"`
	AuthorID       string    `datastore:"author_id"`
	AuthorUsername string    `datastore:"author_username,noindex"`
	VoteScore      int       `datastore:"vote_score"`
	CreatedAt      time.Time `datastore:"created_at"`

	// Unknown carries the stored properties this version does not know
	// (internal/tolerant), so saving it back keeps them.
	Unknown tolerant.Unknown `datastore:"-" json:"-"`
}

func (t *Translation) Load(ps []datastore.Property) error {
	return tolerant.Load("Translation", t, ps, &t.Unknown)
}

func (t *Translation) Save() ([]datastore.Property, error) { return tolerant.Save(t, &t.Unknown) }
