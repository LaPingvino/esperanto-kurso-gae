package auth

import (
	"testing"

	"cloud.google.com/go/datastore"
)

func TestSessionEntityIsTolerant(t *testing.T) {
	var e sessionEntity
	var _ datastore.PropertyLoadSaver = &e
	err := e.Load([]datastore.Property{
		{Name: "data", Value: []byte("{}")},
		{Name: "future", Value: "x"},
		{Name: "expires_at", Value: "tomorrow"},
	})
	if err != nil || string(e.Data) != "{}" || !e.ExpiresAt.IsZero() {
		t.Fatalf("Load: %v, %+v", err, e)
	}
	ps, _ := e.Save()
	found := false
	for _, p := range ps {
		found = found || p.Name == "future"
	}
	if !found {
		t.Errorf("Save dropped the unknown property: %v", ps)
	}
}
