package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// fakeContent is an in-memory builderStore.
type fakeContent struct{ m map[string]*model.ContentItem }

func (f *fakeContent) GetBySlug(_ context.Context, slug string) (*model.ContentItem, error) {
	it, ok := f.m[slug]
	if !ok {
		return nil, nil
	}
	c := *it
	return &c, nil
}
func (f *fakeContent) Create(_ context.Context, it *model.ContentItem) error {
	c := *it
	f.m[it.Slug] = &c
	return nil
}
func (f *fakeContent) PatchContentFields(_ context.Context, it *model.ContentItem) error {
	old, ok := f.m[it.Slug]
	if !ok {
		return f.Create(nil, it)
	}
	c := *it
	c.Rating, c.RD, c.Volatility, c.VoteScore, c.AuthorID = old.Rating, old.RD, old.Volatility, old.VoteScore, old.AuthorID
	if c.Status == "" {
		c.Status = old.Status
	}
	f.m[it.Slug] = &c
	return nil
}
func (f *fakeContent) ListForAdmin(_ context.Context, status string, _ int) ([]*model.ContentItem, error) {
	var out []*model.ContentItem
	for _, it := range f.m {
		if status == "" || it.Status == status {
			c := *it
			out = append(out, &c)
		}
	}
	return out, nil
}

const testKey = "test-builder-key"

func newTestBuilder() (*BuilderAPI, *fakeContent, *http.ServeMux) {
	fc := &fakeContent{m: map[string]*model.ContentItem{
		"zagr-l01-teksto": {Slug: "zagr-l01-teksto", Type: "reading", Status: "approved", AuthorID: "seed", Rating: 1000,
			Content: map[string]any{"title": "Leciono 1", "text": "Marko estas mia amiko."}},
	}}
	b := NewBuilderAPI(fc, builderSHA256(testKey))
	mux := http.NewServeMux()
	b.Routes(mux)
	return b, fc, mux
}

func call(mux *http.ServeMux, method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

const readingJSON = `{"slug":"c2-test-teksto","type":"reading","level":"C2","source":"test",
 "content":{"title":"Testo","text":"Ĉi tio estas teksto."},"series_slug":"c2-test"}`
const mcJSON = `{"slug":"c2-test-mc1","type":"multiplechoice","level":"C2","series_slug":"c2-test-ekz","series_parent":"c2-test-teksto","series_order":1,
 "content":{"question":"Kio?","options":["unu","du","tri"],"correct_index":2}}`

func TestBuilderKey(t *testing.T) {
	_, _, mux := newTestBuilder()
	for _, c := range []struct {
		key  string
		want int
	}{{"", 401}, {"wrong", 401}, {testKey + "x", 401}, {testKey, 200}} {
		if w := call(mux, "GET", "/builder-api/content", c.key, ""); w.Code != c.want {
			t.Errorf("key %q: %d, want %d", c.key, w.Code, c.want)
		}
	}
	// writes too
	if w := call(mux, "POST", "/builder-api/content", "wrong", readingJSON); w.Code != 401 {
		t.Errorf("write with a wrong key: %d", w.Code)
	}
	if w := call(mux, "POST", "/builder-api/unpublish", "", `{"all":true}`); w.Code != 401 {
		t.Errorf("unpublish without a key: %d", w.Code)
	}
	// off: no hash configured
	off := NewBuilderAPI(&fakeContent{m: map[string]*model.ContentItem{}}, "")
	m2 := http.NewServeMux()
	off.Routes(m2)
	if w := call(m2, "GET", "/builder-api/content", testKey, ""); w.Code != 403 {
		t.Errorf("API off: %d, want 403", w.Code)
	}
	if w := call(m2, "GET", "/builder-api/content", "", ""); w.Code != 403 {
		t.Errorf("API off, no key: %d, want 403", w.Code)
	}
}

func TestBuilderWrongKeysLimited(t *testing.T) {
	_, _, mux := newTestBuilder()
	got := 0
	for range 25 {
		got = call(mux, "GET", "/builder-api/content", "nope", "").Code
	}
	if got != 429 {
		t.Errorf("after 25 wrong keys: %d, want 429", got)
	}
}

func TestBuilderAddReadPublishUnpublish(t *testing.T) {
	_, fc, mux := newTestBuilder()
	w := call(mux, "POST", "/builder-api/content", testKey, "["+readingJSON+","+mcJSON+"]")
	if w.Code != 200 {
		t.Fatalf("add: %d %s", w.Code, w.Body)
	}
	// pending, builder-made, at its level
	it := fc.m["c2-test-teksto"]
	if it == nil || it.Status != "pending" || it.AuthorID != BuilderOrigin || model.RatingToCEFR(it.Rating) != "C2" || it.RD != 200 {
		t.Fatalf("added: %+v", it)
	}
	// read back
	w = call(mux, "GET", "/builder-api/content/c2-test-mc1", testKey, "")
	var back BuilderItem
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &back) != nil || back.Status != "pending" ||
		back.SeriesParent != "c2-test-teksto" || back.Content["correct_index"].(float64) != 2 {
		t.Fatalf("read back: %d %s", w.Code, w.Body)
	}
	if w := call(mux, "GET", "/builder-api/content/nothing-here", testKey, ""); w.Code != 404 {
		t.Errorf("missing item: %d", w.Code)
	}
	// the outline, builder items only
	w = call(mux, "GET", "/builder-api/content?origin=builder", testKey, "")
	var outline struct {
		Count  int
		Counts map[string]map[string]int
	}
	if json.Unmarshal(w.Body.Bytes(), &outline) != nil || outline.Count != 2 || outline.Counts["status"]["pending"] != 2 {
		t.Errorf("outline: %s", w.Body)
	}
	// publish a series, then undo everything in one call
	if w := call(mux, "POST", "/builder-api/publish", testKey, `{"series":"c2-test-ekz"}`); w.Code != 200 || fc.m["c2-test-mc1"].Status != "approved" || fc.m["c2-test-teksto"].Status != "pending" {
		t.Fatalf("publish series: %d %s", w.Code, w.Body)
	}
	if w := call(mux, "POST", "/builder-api/publish", testKey, `{"all":true}`); w.Code != 200 || fc.m["c2-test-teksto"].Status != "approved" {
		t.Fatalf("publish all: %d %s", w.Code, w.Body)
	}
	if w := call(mux, "POST", "/builder-api/unpublish", testKey, `{"all":true}`); w.Code != 200 {
		t.Fatalf("unpublish: %d %s", w.Code, w.Body)
	}
	for _, s := range []string{"c2-test-teksto", "c2-test-mc1"} {
		if fc.m[s].Status != "pending" {
			t.Errorf("%s after unpublish: %s", s, fc.m[s].Status)
		}
	}
	if fc.m["zagr-l01-teksto"].Status != "approved" {
		t.Errorf("unpublish all touched a seed item")
	}
	// unpublish by slug
	call(mux, "POST", "/builder-api/publish", testKey, `{"slugs":["c2-test-mc1"]}`)
	if w := call(mux, "POST", "/builder-api/review", testKey, `{"slugs":["c2-test-mc1"]}`); w.Code != 200 || fc.m["c2-test-mc1"].Status != "pending" {
		t.Errorf("review by slug: %d %s", w.Code, w.Body)
	}
}

// The approval flow: pending unless trusted; an update keeps the status and
// the learned rating; trusted on an update publishes.
func TestBuilderApprovalFlow(t *testing.T) {
	_, fc, mux := newTestBuilder()
	trusted := strings.Replace(readingJSON, `"level":"C2"`, `"level":"C2","trusted":true`, 1)
	if w := call(mux, "POST", "/builder-api/content", testKey, trusted); w.Code != 200 || fc.m["c2-test-teksto"].Status != "approved" {
		t.Fatalf("trusted add: %d %s", w.Code, w.Body)
	}
	call(mux, "POST", "/builder-api/content", testKey, mcJSON)
	if fc.m["c2-test-mc1"].Status != "pending" {
		t.Fatalf("untrusted add: %s", fc.m["c2-test-mc1"].Status)
	}
	fc.m["c2-test-mc1"].Rating = 2345 // learned from attempts
	upd := strings.Replace(mcJSON, `"Kio?"`, `"Kio do?"`, 1)
	if w := call(mux, "POST", "/builder-api/content", testKey, upd); w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if it := fc.m["c2-test-mc1"]; it.Status != "pending" || it.Rating != 2345 || it.Content["question"] != "Kio do?" {
		t.Errorf("updated: %+v", it)
	}
	// an approved item updated without trusted stays approved (no silent unpublish)
	if w := call(mux, "POST", "/builder-api/content", testKey, readingJSON); w.Code != 200 || fc.m["c2-test-teksto"].Status != "approved" {
		t.Errorf("update of a published item: %s", fc.m["c2-test-teksto"].Status)
	}
}

func TestBuilderOnlyOwnItems(t *testing.T) {
	_, fc, mux := newTestBuilder()
	over := `{"slug":"zagr-l01-teksto","type":"reading","content":{"title":"X","text":"Y"}}`
	if w := call(mux, "POST", "/builder-api/content", testKey, over); w.Code != 400 || fc.m["zagr-l01-teksto"].Content["title"] != "Leciono 1" {
		t.Errorf("overwrite a seed item: %d %s", w.Code, w.Body)
	}
	if w := call(mux, "POST", "/builder-api/unpublish", testKey, `{"slugs":["zagr-l01-teksto"]}`); w.Code != 409 || fc.m["zagr-l01-teksto"].Status != "approved" {
		t.Errorf("unpublish a seed item: %d %s", w.Code, w.Body)
	}
	if w := call(mux, "POST", "/builder-api/publish", testKey, `{"series":"zagr-metodo"}`); w.Code != 200 || strings.Contains(w.Body.String(), "zagr-l01") {
		t.Errorf("publish someone's series: %d %s", w.Code, w.Body)
	}
}

func TestBuilderValidation(t *testing.T) {
	_, fc, mux := newTestBuilder()
	for name, body := range map[string]string{
		"bad slug":         `{"slug":"Big Slug","type":"reading","content":{"title":"a","text":"b"}}`,
		"bad type":         `{"slug":"x","type":"essay","content":{}}`,
		"mc dup options":   `{"slug":"x","type":"multiplechoice","content":{"question":"q","options":["Jes","jes"],"correct_index":0}}`,
		"mc index":         `{"slug":"x","type":"multiplechoice","content":{"question":"q","options":["a","b"],"correct_index":2}}`,
		"fillin no gap":    `{"slug":"x","type":"fillin","content":{"question":"q","answer":"a"}}`,
		"fillin count":     `{"slug":"x","type":"fillin","content":{"question":"a ___ b ___ c ___","answers":["x","y"]}}`,
		"missing parent":   `{"slug":"x","type":"fillin","series_parent":"nenio","content":{"question":"a ___","answer":"b"}}`,
		"bad level":        `{"slug":"x","type":"reading","level":"D1","content":{"title":"a","text":"b"}}`,
		"one bad in batch": `[` + readingJSON + `,{"slug":"y","type":"vocab","content":{}}]`,
	} {
		if w := call(mux, "POST", "/builder-api/content", testKey, body); w.Code != 400 {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	if len(fc.m) != 1 {
		t.Errorf("a refused batch wrote items: %d", len(fc.m))
	}
}
