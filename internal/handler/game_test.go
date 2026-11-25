package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The merge rule: the larger per lesson, so nothing is lost either way.
func TestMergeGameStars(t *testing.T) {
	got := model.MergeGameStars(map[string]int{"a": 2, "b": 3}, map[string]int{"a": 3, "b": 1, "c": 1, "d": 9})
	if got["a"] != 3 || got["b"] != 3 || got["c"] != 1 || got["d"] != 3 {
		t.Errorf("merged %v", got)
	}
	if got := model.MergeGameStars(nil, map[string]int{"a": 1}); got["a"] != 1 {
		t.Errorf("into nothing: %v", got)
	}
}

func TestParseFormatGameProgress(t *testing.T) {
	s := ParseGameStars([]string{"zagr-l01-teksto=3,zagr-l02-teksto=1", "x=0,y=4", "=2", "bad", "", "zagr-l02-teksto=2"})
	if len(s) != 2 || s["zagr-l01-teksto"] != 3 || s["zagr-l02-teksto"] != 2 {
		t.Errorf("parsed %v", s)
	}
	if got := FormatGameProgress(s, []string{"zagr-l01-fi02", "zagr-l01-fi01"}); got != "S zagr-l01-teksto 3\nS zagr-l02-teksto 2\nF zagr-l01-fi01\nF zagr-l01-fi02\n" {
		t.Errorf("formatted %q", got)
	}
}

func TestGameLesson(t *testing.T) {
	for _, c := range []struct {
		item *model.ContentItem
		want string
	}{
		{&model.ContentItem{Slug: "zagr-l01-teksto", Type: "reading", SeriesSlug: "zagr-metodo"}, "zagr-l01-teksto"},
		{&model.ContentItem{Slug: "zagr-l01-fi01", Type: "fillin", SeriesParent: "zagr-l01-teksto"}, "zagr-l01-teksto"},
		{&model.ContentItem{Slug: "zagr-l02-teksto", Type: "reading"}, "zagr-l02-teksto"},
		{&model.ContentItem{Slug: "zagr-l12-fi03", Type: "fillin"}, "zagr-l12-teksto"},
		{&model.ContentItem{Slug: "saluton", Type: "multiplechoice"}, ""},
		{&model.ContentItem{Slug: "zagr-vok-x", Type: "vocab"}, ""},
	} {
		if got := GameLesson(c.item); got != c.want {
			t.Errorf("%s: %q", c.item.Slug, got)
		}
	}
}

// fakeGameStore is the account side in memory.
type fakeGameStore struct {
	stars    map[string]int
	attempts []*model.Attempt
}

func (f *fakeGameStore) MergeGameStars(_ context.Context, _ string, s map[string]int) (map[string]int, error) {
	f.stars = model.MergeGameStars(f.stars, s)
	return f.stars, nil
}

func (f *fakeGameStore) ListByUser(context.Context, string, int) ([]*model.Attempt, error) {
	return f.attempts, nil
}

// /ludo-progreso merges by the larger, answers with the account's stars and
// the exercises answered rightly; no account, nothing.
func TestGameProgressHandler(t *testing.T) {
	f := &fakeGameStore{stars: map[string]int{"zagr-l01-teksto": 3}, attempts: []*model.Attempt{
		{ContentItemID: "zagr-l02-fi01", Correct: true}, {ContentItemID: "zagr-l02-fi02", Correct: false}}}
	h := NewGameHandler(f, f)
	req := httptest.NewRequest("POST", "/ludo-progreso", strings.NewReader(url.Values{"steloj": {"zagr-l01-teksto=1", "zagr-l03-teksto=2"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &model.User{ID: "u"}))
	w := httptest.NewRecorder()
	h.Progress(w, req)
	if got := w.Body.String(); got != "L en\nS zagr-l01-teksto 3\nS zagr-l03-teksto 2\nF zagr-l02-fi01\n" || w.Code != http.StatusOK {
		t.Errorf("%d %q", w.Code, got)
	}
	w = httptest.NewRecorder()
	h.Progress(w, httptest.NewRequest("GET", "/ludo-progreso", nil))
	if w.Body.Len() != 0 {
		t.Errorf("without an account: %q", w.Body.String())
	}
}
