package store

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/tolerant"
)

// oldUserEntity is userEntity as it was before the Ripetoj deploy: no
// srs_trace (and, for good measure, no last_practice_at).
type oldUserEntity struct {
	Token         string    `datastore:"token"`
	Username      string    `datastore:"username"`
	Rating        float64   `datastore:"rating"`
	RD            float64   `datastore:"rd"`
	Volatility    float64   `datastore:"volatility"`
	Role          string    `datastore:"role"`
	Lang          string    `datastore:"lang"`
	UILang        string    `datastore:"ui_lang"`
	PasskeysJSON  []byte    `datastore:"passkeys_json,noindex"`
	ProgressJSON  []byte    `datastore:"progress_json,noindex"`
	FavoritesJSON []byte    `datastore:"favorites_json,noindex"`
	GameStarsJSON []byte    `datastore:"game_stars_json,noindex"`
	KeepDataDays  int       `datastore:"keep_data_days"`
	StreakDays    int       `datastore:"streak_days"`
	StreakStartAt time.Time `datastore:"streak_start_at"`
	CreatedAt     time.Time `datastore:"created_at"`
	LastSeenAt    time.Time `datastore:"last_seen_at"`

	unknown tolerant.Unknown
}

func (e *oldUserEntity) Load(ps []datastore.Property) error {
	return tolerant.Load(userKind, e, ps, &e.unknown)
}
func (e *oldUserEntity) Save() ([]datastore.Property, error) { return tolerant.Save(e, &e.unknown) }

// The old struct reads a user the new version saved, changes it and saves it;
// the new version then finds both the change and its trace.
func TestOldStructReadsNewUser(t *testing.T) {
	trace := []byte{1, 2, 3, 4, 5, 6, 7}
	practiced := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	newer, err := userToEntity(&model.User{ID: "u1", Username: "ana", Rating: 1500, Spuro: trace, LastPracticeAt: practiced})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := newer.Save()
	if err != nil {
		t.Fatal(err)
	}

	var old oldUserEntity
	if err := old.Load(stored); err != nil {
		t.Fatalf("old struct loading a new user: %v", err)
	}
	if old.Username != "ana" {
		t.Errorf("old struct read username %q", old.Username)
	}
	old.Rating = 1600
	stored, err = old.Save()
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := propNamed(stored, "srs_trace"); !ok || !p.NoIndex {
		t.Fatalf("the old version's save dropped the trace or indexed it: %+v", p)
	}

	var back userEntity
	if err := back.Load(stored); err != nil {
		t.Fatal(err)
	}
	if back.Rating != 1600 || !bytes.Equal(back.Spuro, trace) || !back.LastPracticeAt.Equal(practiced) {
		t.Errorf("new version after the old one's save: rating %v, trace %v, practiced %v", back.Rating, back.Spuro, back.LastPracticeAt)
	}
	if back.unknown.Len() != 0 {
		t.Errorf("the new version knows every property, yet holds %d unknown", back.unknown.Len())
	}
}

// UserStore.Update and ContentStore.Update rebuild the entity from the model:
// what the model was loaded with must survive that.
func TestUnknownSurvivesTheModel(t *testing.T) {
	future := datastore.Property{Name: "future_blob", Value: []byte("x"), NoIndex: true}

	var ue userEntity
	if err := ue.Load([]datastore.Property{{Name: "username", Value: "ana"}, future}); err != nil {
		t.Fatal(err)
	}
	u, err := entityToUser("u1", &ue)
	if err != nil {
		t.Fatal(err)
	}
	u.Username = "bea"
	e, err := userToEntity(u)
	if err != nil {
		t.Fatal(err)
	}
	ps, _ := e.Save()
	if p, ok := propNamed(ps, "future_blob"); !ok || !p.NoIndex {
		t.Errorf("user: future_blob lost through the model: %+v", ps)
	}

	var ce contentEntity
	if err := ce.Load([]datastore.Property{{Name: "type", Value: "vocab"}, future}); err != nil {
		t.Fatal(err)
	}
	item, err := entityToContent("s", &ce)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := contentToEntity(item)
	if err != nil {
		t.Fatal(err)
	}
	ps, _ = e2.Save()
	if _, ok := propNamed(ps, "future_blob"); !ok {
		t.Errorf("content: future_blob lost through the model: %+v", ps)
	}
}

func propNamed(ps []datastore.Property, name string) (datastore.Property, bool) {
	for _, p := range ps {
		if p.Name == name {
			return p, true
		}
	}
	return datastore.Property{}, false
}

// tolerantEntities is every type stored in Datastore, by name.
var tolerantEntities = map[string]any{
	"userEntity":            &userEntity{},
	"userAliasEntity":       &userAliasEntity{},
	"contentEntity":         &contentEntity{},
	"contentMeta":           &contentMeta{},
	"attemptEntity":         &attemptEntity{},
	"commentEntity":         &commentEntity{},
	"voteEntity":            &voteEntity{},
	"translationVoteEntity": &translationVoteEntity{},
	"ModMessage":            &model.ModMessage{},
	"Translation":           &model.Translation{},
}

// Every struct with datastore tags here and in internal/model loads through
// internal/tolerant, and saves exactly its schema when nothing is unknown.
func TestEveryEntityIsTolerant(t *testing.T) {
	notStored := map[string]bool{"User": true} // model.User: stored as userEntity
	for _, dir := range []string{".", "../model"} { // auth.sessionEntity: tested in package auth
		for _, name := range datastoreStructs(t, dir) {
			if notStored[name] || name == "oldUserEntity" {
				continue
			}
			v, ok := tolerantEntities[name]
			if !ok {
				t.Errorf("%s/%s has datastore tags but no entry in tolerantEntities (and maybe no Load/Save)", dir, name)
				continue
			}
			if _, ok := v.(datastore.PropertyLoadSaver); !ok {
				t.Errorf("%s does not implement datastore.PropertyLoadSaver through internal/tolerant", name)
				continue
			}
			got, err := v.(datastore.PropertyLoadSaver).Save()
			want, err2 := datastore.SaveStruct(v)
			if err != nil || err2 != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("%s: Save is not exactly SaveStruct: %v / %v", name, err, err2)
			}
			if err := v.(datastore.PropertyLoadSaver).Load([]datastore.Property{{Name: "no_such_property_anywhere", Value: int64(1)}}); err != nil {
				t.Errorf("%s: Load of an unknown property failed: %v", name, err)
			}
		}
	}
}

// datastoreStructs names the struct types declared in dir (tests excluded)
// with at least one datastore struct tag.
func datastoreStructs(t *testing.T, dir string) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var out []string
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fl := range st.Fields.List {
				if fl.Tag != nil && strings.Contains(fl.Tag.Value, `datastore:"`) &&
					!strings.Contains(fl.Tag.Value, `datastore:"-"`) {
					out = append(out, ts.Name.Name)
					break
				}
			}
			return true
		})
	}
	return out
}
