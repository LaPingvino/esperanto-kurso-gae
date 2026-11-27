package model

import (
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/tolerant"
)

// ModMessage is a message sent to the mod/admin team by a user.
type ModMessage struct {
	ID            string    `datastore:"-"`
	UserID        string    `datastore:"user_id"`
	Username      string    `datastore:"username,noindex"`
	Text          string    `datastore:"text,noindex"`
	ContentItemID string    `datastore:"content_item_id,noindex"`
	Read          bool      `datastore:"read"`
	CreatedAt     time.Time `datastore:"created_at"`

	// Unknown carries the stored properties this version does not know
	// (internal/tolerant), so saving it back keeps them.
	Unknown tolerant.Unknown `datastore:"-" json:"-"`
}

func (m *ModMessage) Load(ps []datastore.Property) error {
	return tolerant.Load("ModMessage", m, ps, &m.Unknown)
}

func (m *ModMessage) Save() ([]datastore.Property, error) { return tolerant.Save(m, &m.Unknown) }
