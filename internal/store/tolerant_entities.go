package store

import (
	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/tolerant"
)

// Every entity this package stores loads leniently and saves strictly
// (internal/tolerant): a version of the site reads what a newer or older one
// wrote, and writes back the properties it does not know. A new entity type
// gets the same two methods; TestEveryEntityIsTolerant checks the list.

func (e *userEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(userKind, e, ps, &e.unknown)
}
func (e *userEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *userAliasEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(userAliasKind, e, ps, &e.unknown)
}
func (e *userAliasEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *contentEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(contentKind, e, ps, &e.unknown)
}
func (e *contentEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *contentMeta) Load(ps []datastore.Property) error {
	return tolerant.Load(contentMetaKind, e, ps, &e.unknown)
}
func (e *contentMeta) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *attemptEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(attemptKind, e, ps, &e.unknown)
}
func (e *attemptEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *commentEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(commentKind, e, ps, &e.unknown)
}
func (e *commentEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *voteEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(voteKind, e, ps, &e.unknown)
}
func (e *voteEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

func (e *translationVoteEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(translationVoteKind, e, ps, &e.unknown)
}
func (e *translationVoteEntity) Save() ([]datastore.Property, error) {
	return tolerant.Save(e, &e.unknown)
}
