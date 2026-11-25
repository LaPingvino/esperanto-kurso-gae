package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// ratingContent is an in-memory exerciseContent with no recommendations.
type ratingContent struct {
	m   map[string]*model.ContentItem
	err error // returned by UpdateRating
}

func (f *ratingContent) GetBySlug(_ context.Context, slug string) (*model.ContentItem, error) {
	it, ok := f.m[slug]
	if !ok {
		return nil, nil
	}
	c := *it
	return &c, nil
}
func (f *ratingContent) UpdateRating(_ context.Context, slug string, r, rd, vol float64) error {
	if f.err != nil {
		return f.err
	}
	it := f.m[slug]
	it.Rating, it.RD, it.Volatility = r, rd, vol
	return nil
}
func (f *ratingContent) ApprovedShared(context.Context) ([]*model.ContentItem, error) {
	return nil, nil
}
func (f *ratingContent) ListBySeries(context.Context, string) ([]*model.ContentItem, error) {
	return nil, nil
}
func (f *ratingContent) ListApproved(context.Context, int) ([]*model.ContentItem, error) {
	return nil, nil
}
func (f *ratingContent) ListByRatingRange(context.Context, float64, float64, int) ([]*model.ContentItem, error) {
	return nil, nil
}

// ratingUsers is an in-memory exerciseUsers.
type ratingUsers struct {
	m   map[string]*model.User
	err error // returned by UpdateRating
}

func (f *ratingUsers) Create(_ context.Context, u *model.User) error {
	c := *u
	f.m[u.ID] = &c
	return nil
}
func (f *ratingUsers) UpdateRating(_ context.Context, id string, r, rd, vol float64) error {
	if f.err != nil {
		return f.err
	}
	u := f.m[id]
	u.Rating, u.RD, u.Volatility = r, rd, vol
	return nil
}
func (f *ratingUsers) UpdateStreakAndSeen(context.Context, string) (int, error) { return 1, nil }
func (f *ratingUsers) UpdateUILang(context.Context, string, string) error       { return nil }

type ratingAttempts struct {
	n   int
	err error
}

func (f *ratingAttempts) Create(context.Context, *model.Attempt) error {
	f.n++
	return f.err
}

type ratingVotes struct{}

func (ratingVotes) GetByUserAndContent(context.Context, string, string) (*model.Vote, error) {
	return nil, nil
}

type nopRenderer struct{}

func (nopRenderer) ExecuteTemplate(io.Writer, string, interface{}) error { return nil }

func newRatingHandler() (*http.ServeMux, *ratingContent, *ratingUsers, *ratingAttempts) {
	fc := &ratingContent{m: map[string]*model.ContentItem{
		"mc": {Slug: "mc", Type: "multiplechoice", Status: "approved", Rating: 1500, RD: 350, Volatility: 0.06,
			Content: map[string]any{"question": "?", "options": []string{"jes", "ne"}, "correct_index": 0}},
		"karto": {Slug: "karto", Type: "vocab", Status: "approved", Rating: 1500, RD: 350, Volatility: 0.06,
			Content: map[string]any{"word": "hundo"}},
	}}
	fu := &ratingUsers{m: map[string]*model.User{}}
	fa := &ratingAttempts{}
	h := NewExerciseHandler(nopRenderer{}, fc, fu, fa, ratingVotes{})
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ekzerco/{slug}/provo", h.SubmitAttempt)
	mux.HandleFunc("POST /ekzerco/{slug}/jugxo", h.JudgeExercise)
	return mux, fc, fu, fa
}

// post sends a form as the given user (nil: a new anonymous learner) and
// returns the learner the handler created or used.
func post(t *testing.T, mux *http.ServeMux, fu *ratingUsers, u *model.User, path string, form url.Values) *model.User {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if u != nil {
		c := *fu.m[u.ID]
		r = r.WithContext(context.WithValue(r.Context(), UserContextKey, &c))
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("%s: status %d: %s", path, w.Code, w.Body)
	}
	if u != nil {
		return fu.m[u.ID]
	}
	if len(fu.m) != 1 {
		t.Fatalf("want one anonymous user, have %d", len(fu.m))
	}
	for _, nu := range fu.m {
		return nu
	}
	return nil
}

// step checks one verdict: the learner moves by userDir, the item the other
// way, and both RDs shrink.
func step(t *testing.T, name string, before, after model.User, itemBefore, itemAfter model.ContentItem, userDir float64) {
	t.Helper()
	if (after.Rating-before.Rating)*userDir <= 0 {
		t.Errorf("%s: user rating %v -> %v, want direction %v", name, before.Rating, after.Rating, userDir)
	}
	if (itemAfter.Rating-itemBefore.Rating)*userDir >= 0 {
		t.Errorf("%s: item rating %v -> %v, want direction %v", name, itemBefore.Rating, itemAfter.Rating, -userDir)
	}
	if after.RD >= before.RD || itemAfter.RD >= itemBefore.RD {
		t.Errorf("%s: RDs must shrink: user %v -> %v, item %v -> %v", name, before.RD, after.RD, itemBefore.RD, itemAfter.RD)
	}
}

func TestVerdictUpdatesBothRatings(t *testing.T) {
	mux, fc, fu, fa := newRatingHandler()

	// A right answer from a new learner.
	item0 := *fc.m["mc"]
	u := post(t, mux, fu, nil, "/ekzerco/mc/provo", url.Values{"answer": {"jes"}})
	start := *model.NewUser("", "")
	u1, item1 := *u, *fc.m["mc"]
	step(t, "right answer", start, u1, item0, item1, +1)

	// Then a wrong one.
	post(t, mux, fu, u, "/ekzerco/mc/provo", url.Values{"answer": {"ne"}})
	u2, item2 := *fu.m[u.ID], *fc.m["mc"]
	step(t, "wrong answer", u1, u2, item1, item2, -1)

	// A flashcard self-judgement: known, then not known.
	card0 := *fc.m["karto"]
	post(t, mux, fu, u, "/ekzerco/karto/jugxo", url.Values{"judgment": {"bone"}})
	u3, card1 := *fu.m[u.ID], *fc.m["karto"]
	step(t, "judged bone", u2, u3, card0, card1, +1)

	post(t, mux, fu, u, "/ekzerco/karto/jugxo", url.Values{"judgment": {"ne-sciis"}})
	u4, card2 := *fu.m[u.ID], *fc.m["karto"]
	step(t, "judged ne-sciis", u3, u4, card1, card2, -1)

	if fa.n != 4 {
		t.Errorf("attempts recorded: %d, want 4", fa.n)
	}
}

// TestVerdictSurvivesStoreErrors: failing stores are logged, and the learner
// still gets the verdict.
func TestVerdictSurvivesStoreErrors(t *testing.T) {
	mux, fc, fu, fa := newRatingHandler()
	fa.err = errors.New("attempt store down")
	fc.err = errors.New("content store down")
	fu.err = errors.New("user store down")
	post(t, mux, fu, nil, "/ekzerco/mc/provo", url.Values{"answer": {"jes"}})
	if fc.m["mc"].Rating != 1500 {
		t.Errorf("item rating changed despite the store error")
	}
}
