package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/ripetoj"
)

// The repetition trace survives a whole-user write (Update, MergeUsers go
// through userToEntity): a field not carried by model.User would be wiped.
func TestSpuroRoundTrip(t *testing.T) {
	u := model.NewUser("u", "tok")
	u.Spuro = ripetoj.Note(ripetoj.Note(nil, "a", 3, time.Now()), "b", 4, time.Now())
	e, err := userToEntity(u)
	if err != nil {
		t.Fatal(err)
	}
	back, err := entityToUser("u", e)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back.Spuro, u.Spuro) || back.SpuroN() != 2 {
		t.Errorf("trace lost: %v", back.Spuro)
	}
}
