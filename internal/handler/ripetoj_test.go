package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/ripetoj"
)

type deckJSON struct {
	V string                 `json:"v"`
	C [][][2]json.RawMessage `json:"c"`
}

// /ripetoj-api/kartoj: every card of the seed but the texts and videos, a
// series as one chain in its authored order, cacheable (ETag, 304), no
// cookie; built once for the same content.
func TestRipetojCards(t *testing.T) {
	items := seedSet(t)
	api := NewRipetojAPI(&seedSource{items: items})
	mux := http.NewServeMux()
	api.Routes(mux)
	get := func(inm string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/ripetoj-api/kartoj", nil)
		if inm != "" {
			r.Header.Set("If-None-Match", inm)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := get("")
	if w.Code != 200 || w.Header().Get("Set-Cookie") != "" || !strings.Contains(w.Header().Get("Cache-Control"), "public") {
		t.Fatalf("cards: %d %v", w.Code, w.Header())
	}
	var d deckJSON
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]*model.ContentItem{}
	want := 0
	for _, it := range items {
		bySlug[it.Slug] = it
		if IsCard(it) {
			want++
		}
	}
	got := 0
	for _, ch := range d.C {
		prev := -1 << 30
		for _, c := range ch {
			var slug string
			json.Unmarshal(c[0], &slug)
			it := bySlug[slug]
			if it == nil || !IsCard(it) {
				t.Fatalf("card %q is no card item", slug)
			}
			if it.SeriesSlug != bySlug[mustSlug(ch[0])].SeriesSlug {
				t.Errorf("chain mixes series: %s", slug)
			}
			if it.SeriesOrder < prev {
				t.Errorf("chain of %s not in authored order at %s", it.SeriesSlug, slug)
			}
			prev = it.SeriesOrder
			got++
		}
	}
	if got != want || want < 1000 {
		t.Errorf("%d cards, want %d", got, want)
	}
	t.Logf("%d cards in %d chains, %d bytes", got, len(d.C), w.Body.Len())
	if w2 := get(w.Header().Get("ETag")); w2.Code != http.StatusNotModified {
		t.Errorf("same version: %d, want 304", w2.Code)
	}
	if api.cache.get(items) != api.cache.get(items) {
		t.Error("the deck is built again for the same content")
	}
}

func mustSlug(c [2]json.RawMessage) string {
	var s string
	json.Unmarshal(c[0], &s)
	return s
}

type fakeSpuro struct{ cleared []string }

func (f *fakeSpuro) ClearSpuro(_ context.Context, id string) error {
	f.cleared = append(f.cleared, id)
	return nil
}

// /ripetoj/spuro: the learner's trace by slug, items gone left out, never
// cached, the owner named; /ripetoj/forgesi clears it.
func TestRipetojTrace(t *testing.T) {
	items := []*model.ContentItem{
		{Slug: "mc", Type: "multiplechoice", Status: "approved"},
		{Slug: "karto", Type: "vocab", Status: "approved"},
		{Slug: "teksto", Type: "reading", Status: "approved"},
	}
	now := time.Now()
	var b []byte
	b = ripetoj.Note(b, "mc", ripetoj.Good, now)
	b = ripetoj.Note(b, "mc", ripetoj.Again, now)
	b = ripetoj.Note(b, "karto", ripetoj.Easy, now)
	b = ripetoj.Note(b, "forigita", ripetoj.Good, now)
	u := &model.User{ID: "u1", Spuro: b}
	fs := &fakeSpuro{}
	h := NewRipetojHandler(nopRenderer{}, &seedSource{items: items}, fs)
	mux := http.NewServeMux()
	h.Routes(mux)
	do := func(method, path string, u *model.User) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if u != nil {
			r = r.WithContext(context.WithValue(r.Context(), UserContextKey, u))
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	w := do("GET", "/ripetoj/spuro", u)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("trace: %d %v", w.Code, w.Header())
	}
	var got struct {
		O string
		E [][4]any
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	day := float64(now.UTC().Unix() / 86400)
	if got.O != SRSOwner(u) || got.O == "" || strings.Contains(got.O, "u1") {
		t.Errorf("owner %q", got.O)
	}
	if len(got.E) != 2 || got.E[0][0] != "karto" || got.E[0][3] != 4.0 || got.E[1][0] != "mc" || got.E[1][2] != 2.0 || got.E[1][3] != 1.0 || got.E[1][1] != day {
		t.Errorf("entries: %v", got.E)
	}
	if w := do("GET", "/ripetoj/spuro", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"e":[]`) {
		t.Errorf("no user: %d %s", w.Code, w.Body)
	}
	if w := do("POST", "/ripetoj/forgesi", u); w.Code != http.StatusNoContent || len(fs.cleared) != 1 || fs.cleared[0] != "u1" {
		t.Errorf("forget: %d %v", w.Code, fs.cleared)
	}
	if w := do("GET", "/ripetoj", u); w.Code != 200 {
		t.Errorf("page: %d", w.Code)
	}
}

// An answer writes its grade into the trace in the same store call that
// records the streak: a hit Good, a miss Again; the judgements ne sciis /
// malfacile / bone / facile Again / Hard / Good / Easy.
func TestVerdictWritesTrace(t *testing.T) {
	mux, _, fu, _ := newRatingHandler()
	u := post(t, mux, fu, nil, "/ekzerco/mc/provo", url.Values{"answer": {"jes"}})
	rec := func(slug string) ripetoj.Record {
		for _, r := range ripetoj.Decode(fu.m[u.ID].Spuro) {
			if r.Hash == ripetoj.Hash(slug) {
				return r
			}
		}
		return ripetoj.Record{}
	}
	if r := rec("mc"); r.Count != 1 || r.Grade != ripetoj.Good {
		t.Errorf("a hit: %+v", r)
	}
	post(t, mux, fu, u, "/ekzerco/mc/provo", url.Values{"answer": {"ne"}})
	if r := rec("mc"); r.Count != 2 || r.Grade != ripetoj.Again {
		t.Errorf("a miss: %+v", r)
	}
	for j, g := range map[string]int{"ne-sciis": 1, "malfacile": 2, "bone": 3, "facile": 4} {
		post(t, mux, fu, u, "/ekzerco/karto/jugxo", url.Values{"judgment": {j}})
		if r := rec("karto"); r.Grade != g {
			t.Errorf("%s: grade %d, want %d", j, r.Grade, g)
		}
	}
	if n := ripetoj.Len(fu.m[u.ID].Spuro); n != 2 {
		t.Errorf("%d records for two items", n)
	}
}

// IsCard: texts and videos are read or watched, not repeated.
func TestIsCard(t *testing.T) {
	for typ, want := range map[string]bool{"multiplechoice": true, "fillin": true, "vocab": true, "phrasebook": true,
		"listening": true, "image": true, "reading": false, "video": false} {
		if IsCard(&model.ContentItem{Type: typ}) != want {
			t.Errorf("IsCard(%s) != %v", typ, want)
		}
	}
	if IsCard(nil) {
		t.Error("nil")
	}
}
