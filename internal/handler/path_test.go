package handler

import (
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func item(slug, series string, order int, rating float64) *model.ContentItem {
	return &model.ContentItem{Slug: slug, SeriesSlug: series, SeriesLabel: series, SeriesOrder: order, Rating: rating}
}

func attempt(slug string, correct bool) *model.Attempt {
	return &model.Attempt{ContentItemID: slug, Correct: correct}
}

func TestBuildPathFromItems(t *testing.T) {
	items := []*model.ContentItem{
		// Series "salutoj": 3 items around 1000.
		item("sal-1", "salutoj", 1, 990),
		item("sal-2", "salutoj", 2, 1000),
		item("sal-3", "salutoj", 3, 1010),
		// Series "manĝoj": 2 items around 1500 — near the test user's level.
		item("man-1", "mangxoj", 1, 1490),
		item("man-2", "mangxoj", 2, 1560),
		// Series "literaturo": far above level.
		item("lit-1", "literaturo", 1, 1900),
		// A child series and a series-less item must be ignored.
		{Slug: "kid-1", SeriesSlug: "sub", SeriesParent: "man-1", Rating: 1500},
		{Slug: "solo-1", Rating: 1500},
	}

	attempts := []*model.Attempt{
		attempt("sal-1", true),
		attempt("sal-2", false), // tried but not yet mastered
	}

	path := buildPathFromItems(items, attempts, 1500)

	if len(path.Continue) != 1 || path.Continue[0].Slug != "salutoj" {
		t.Fatalf("Continue = %+v, want just salutoj", path.Continue)
	}
	c := path.Continue[0]
	if c.Done != 1 || c.Total != 3 {
		t.Errorf("salutoj progress = %d/%d, want 1/3", c.Done, c.Total)
	}
	if c.NextSlug != "sal-2" {
		t.Errorf("salutoj next = %s, want sal-2 (still unmastered)", c.NextSlug)
	}

	if len(path.Suggested) != 1 || path.Suggested[0].Slug != "mangxoj" {
		t.Fatalf("Suggested = %+v, want just mangxoj (literaturo is out of range)", path.Suggested)
	}
	if path.Suggested[0].NextSlug != "man-1" {
		t.Errorf("mangxoj next = %s, want man-1", path.Suggested[0].NextSlug)
	}

	if path.Completed != 0 {
		t.Errorf("Completed = %d, want 0", path.Completed)
	}
}

func TestBuildPathCompletedSeries(t *testing.T) {
	items := []*model.ContentItem{
		item("a-1", "a", 1, 1200),
		item("a-2", "a", 2, 1200),
	}
	attempts := []*model.Attempt{attempt("a-1", true), attempt("a-2", true)}

	path := buildPathFromItems(items, attempts, 1200)
	if path.Completed != 1 {
		t.Errorf("Completed = %d, want 1", path.Completed)
	}
	if len(path.Continue) != 0 || len(path.Suggested) != 0 {
		t.Errorf("finished series must not appear in Continue (%d) or Suggested (%d)", len(path.Continue), len(path.Suggested))
	}
}

func TestBuildPathOrdering(t *testing.T) {
	items := []*model.ContentItem{
		item("x-1", "x", 1, 1200), item("x-2", "x", 2, 1200),
		item("y-1", "y", 1, 1200), item("y-2", "y", 2, 1200), item("y-3", "y", 3, 1200), item("y-4", "y", 4, 1200),
	}
	// x: 1/2 done (50%), y: 3/4 done (75%) — y should come first.
	attempts := []*model.Attempt{
		attempt("x-1", true),
		attempt("y-1", true), attempt("y-2", true), attempt("y-3", true),
	}
	path := buildPathFromItems(items, attempts, 1200)
	if len(path.Continue) != 2 || path.Continue[0].Slug != "y" {
		t.Fatalf("Continue order = %+v, want y (closest to done) first", path.Continue)
	}
}
