package handler

import (
	"context"
	"net/http"
	"net/url"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// seedSource serves the seed as the approved content (one slice, as the
// store's cache does).
type seedSource struct{ items []*model.ContentItem }

func (s *seedSource) ApprovedShared(context.Context) ([]*model.ContentItem, error) {
	return s.items, nil
}

func seedSet(t *testing.T) []*model.ContentItem {
	items, err := gamedata.ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	return items
}

// /ludo-api/: the map and a lesson as text, cacheable by version (ETag,
// 304), meanings in the language asked for, no cookie, an unknown place 404;
// the course built once for the same content.
func TestGameAPI(t *testing.T) {
	src := &seedSource{items: seedSet(t)}
	api := NewGameAPI(src)
	mux := http.NewServeMux()
	api.Routes(mux)
	get := func(path, inm string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if inm != "" {
			r.Header.Set("If-None-Match", inm)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	m := get("/ludo-api/kurso", "")
	body := m.Body.String()
	if m.Code != 200 || !strings.HasPrefix(body, "KURSO\t") || !strings.HasSuffix(body, "FINO\n") || !strings.Contains(body, "P\tzagr-l01-teksto\tL\t") {
		t.Fatalf("map: %d %q", m.Code, body[:min(200, len(body))])
	}
	if m.Header().Get("Set-Cookie") != "" || !strings.Contains(m.Header().Get("Cache-Control"), "public") {
		t.Errorf("map headers: %v", m.Header())
	}
	if w := get("/ludo-api/kurso", m.Header().Get("ETag")); w.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: %d", w.Code)
	}
	nl := get("/ludo-api/leciono/zagr-l01-teksto?lingvo=nl", "").Body.String()
	en := get("/ludo-api/leciono/zagr-l01-teksto?lingvo=en", "").Body.String()
	if !strings.Contains(nl, "\tvriend") || !strings.Contains(en, "\tfriend") || !strings.HasSuffix(nl, "FINO\n") {
		t.Errorf("meanings by language:\n%s\n%s", nl, en)
	}
	if w := get("/ludo-api/leciono/ne-ekzistas", ""); w.Code != 404 {
		t.Errorf("unknown place: %d", w.Code)
	}
	c1 := api.Course(context.Background())
	if c2 := api.Course(context.Background()); c1 != c2 {
		t.Error("the course was built again for the same content")
	}
	if api.PlaceOf(context.Background(), "zagr-l01-fi01") != "zagr-l01-teksto" || api.PlaceOf(context.Background(), "vortaro-koloro-blanka") != "vortaro-koloro.1" {
		t.Errorf("places of items: %q %q", api.PlaceOf(context.Background(), "zagr-l01-fi01"), api.PlaceOf(context.Background(), "vortaro-koloro-blanka"))
	}
}

// Without the series fields (as cmd/seed stores items), the course still
// holds every exercise, in the "Aliaj" places of their levels.
func TestCourseWithoutSeries(t *testing.T) {
	var flat []*model.ContentItem
	n := 0
	for _, it := range seedSet(t) {
		cp := *it
		cp.SeriesSlug, cp.SeriesParent, cp.SeriesLabel, cp.SeriesOrder = "", "", "", 0
		flat = append(flat, &cp)
	}
	c := gamedata.Build(flat)
	for _, p := range c.Places {
		for _, e := range p.Ex {
			if e.Slug != "" {
				n++
			}
		}
	}
	if n < 1000 || len(c.Series) == 0 {
		t.Errorf("%d exercises in %d series", n, len(c.Series))
	}
}

// /ludo-api/vorto: a guess is a word or not (the x-system read too), the
// answer cacheable and without a cookie.
func TestGameAPIWord(t *testing.T) {
	mux := http.NewServeMux()
	NewGameAPI(&seedSource{}).Routes(mux)
	for v, want := range map[string]string{
		"domo": "1", "domojn": "1", "domŝipo": "1", "malbonulejo": "1", "tiujn": "1", "trancxiloj": "1", "tranĉiloj": "1",
		"bzzrk": "0", "domok": "0", "": "0", strings.Repeat("domo", 5): "0",
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", "/ludo-api/vorto?v="+url.QueryEscape(v), nil))
		if w.Code != 200 || w.Body.String() != want {
			t.Errorf("%q: %d %q, want %q", v, w.Code, w.Body.String(), want)
		}
		if w.Header().Get("Set-Cookie") != "" || !strings.Contains(w.Header().Get("Cache-Control"), "public") {
			t.Errorf("%q headers: %v", v, w.Header())
		}
	}
}
