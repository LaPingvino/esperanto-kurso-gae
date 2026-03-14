package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/datastore"
	"esperanto-kurso-gae/internal/model"
	"github.com/go-webauthn/webauthn/webauthn"
)

const userKind = "User"

type userEntity struct {
	Token        string    `datastore:"token"`
	Rating       float64   `datastore:"rating"`
	RD           float64   `datastore:"rd"`
	Volatility   float64   `datastore:"volatility"`
	Role         string    `datastore:"role"`
	Lang         string    `datastore:"lang"`
	PasskeysJSON []byte    `datastore:"passkeys_json,noindex"`
	ProgressJSON []byte    `datastore:"progress_json,noindex"`
	CreatedAt    time.Time `datastore:"created_at"`
	LastSeenAt   time.Time `datastore:"last_seen_at"`
}

func userToEntity(u *model.User) (*userEntity, error) {
	pkJSON, err := json.Marshal(u.Passkeys)
	if err != nil {
		return nil, err
	}
	prJSON, err := json.Marshal(u.Progress)
	if err != nil {
		return nil, err
	}
	lang := u.Lang
	if lang == "" {
		lang = "en"
	}
	return &userEntity{
		Token:        u.Token,
		Rating:       u.Rating,
		RD:           u.RD,
		Volatility:   u.Volatility,
		Role:         u.Role,
		Lang:         lang,
		PasskeysJSON: pkJSON,
		ProgressJSON: prJSON,
		CreatedAt:    u.CreatedAt,
		LastSeenAt:   u.LastSeenAt,
	}, nil
}

func entityToUser(id string, e *userEntity) (*model.User, error) {
	lang := e.Lang
	if lang == "" {
		lang = "en"
	}
	u := &model.User{
		ID:         id,
		Token:      e.Token,
		Rating:     e.Rating,
		RD:         e.RD,
		Volatility: e.Volatility,
		Role:       e.Role,
		Lang:       lang,
		CreatedAt:  e.CreatedAt,
		LastSeenAt: e.LastSeenAt,
		Progress:   make(map[string]bool),
	}
	if len(e.PasskeysJSON) > 0 {
		if err := json.Unmarshal(e.PasskeysJSON, &u.Passkeys); err != nil {
			return nil, err
		}
	}
	if len(e.ProgressJSON) > 0 {
		if err := json.Unmarshal(e.ProgressJSON, &u.Progress); err != nil {
			return nil, err
		}
	}
	return u, nil
}

type UserStore struct {
	db *datastore.Client
}

func NewUserStore(db *datastore.Client) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) userKey(id string) *datastore.Key {
	return datastore.NameKey(userKind, id, nil)
}

func (s *UserStore) Create(ctx context.Context, u *model.User) error {
	e, err := userToEntity(u)
	if err != nil {
		return fmt.Errorf("user_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.userKey(u.ID), e)
	return err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var e userEntity
	if err := s.db.Get(ctx, s.userKey(id), &e); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, fmt.Errorf("user_store: GetByID %s: %w", id, err)
	}
	return entityToUser(id, &e)
}

func (s *UserStore) GetByToken(ctx context.Context, token string) (*model.User, error) {
	q := datastore.NewQuery(userKind).FilterField("token", "=", token).Limit(1)
	var entities []userEntity
	keys, err := s.db.GetAll(ctx, q, &entities)
	if err != nil {
		return nil, fmt.Errorf("user_store: GetByToken: %w", err)
	}
	if len(entities) == 0 {
		return nil, nil
	}
	return entityToUser(keys[0].Name, &entities[0])
}

func (s *UserStore) Update(ctx context.Context, u *model.User) error {
	e, err := userToEntity(u)
	if err != nil {
		return fmt.Errorf("user_store: marshal: %w", err)
	}
	_, err = s.db.Put(ctx, s.userKey(u.ID), e)
	return err
}

func (s *UserStore) UpdateRating(ctx context.Context, userID string, rating, rd, volatility float64) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Rating = rating
		e.RD = rd
		e.Volatility = volatility
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) AddPasskey(ctx context.Context, userID string, cred webauthn.Credential) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		var passkeys []webauthn.Credential
		if len(e.PasskeysJSON) > 0 {
			if err := json.Unmarshal(e.PasskeysJSON, &passkeys); err != nil {
				return err
			}
		}
		passkeys = append(passkeys, cred)
		b, err := json.Marshal(passkeys)
		if err != nil {
			return err
		}
		e.PasskeysJSON = b
		_, err = tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) UpdateLang(ctx context.Context, userID, lang string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.Lang = lang
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}

func (s *UserStore) UpdateLastSeen(ctx context.Context, userID string) error {
	key := s.userKey(userID)
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var e userEntity
		if err := tx.Get(key, &e); err != nil {
			return err
		}
		e.LastSeenAt = time.Now()
		_, err := tx.Put(key, &e)
		return err
	})
	return err
}
