package handler

import (
	"context"
	"sort"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// SeriesProgress is one lesson series on the learner's path.
type SeriesProgress struct {
	Slug     string
	Label    string
	CEFR     string
	Rating   float64 // median item rating
	Done     int     // distinct items answered correctly
	Total    int
	Percent  int
	NextSlug string // first item in series order not yet answered correctly
}

// learningPath groups series by the learner's relation to them.
type learningPath struct {
	Continue  []*SeriesProgress // started but unfinished, closest to done first
	Suggested []*SeriesProgress // not started, near or just above the learner's level
	Completed int
}

// buildLearningPath derives per-series progress from the user's recent
// attempts and picks what to continue and what to start next.
func (h *ContentHandler) buildLearningPath(ctx context.Context, attempts []*model.Attempt, rating float64) learningPath {
	items, err := h.content.ListForAdmin(ctx, "approved", 50000)
	if err != nil {
		return learningPath{}
	}
	return buildPathFromItems(items, attempts, rating)
}

// buildPathFromItems is the pure core of buildLearningPath. An item counts as
// done once it has been answered correctly, so the path lingers on exercises
// the learner still gets wrong.
func buildPathFromItems(items []*model.ContentItem, attempts []*model.Attempt, rating float64) learningPath {
	var path learningPath

	attempted := map[string]bool{}
	correct := map[string]bool{}
	for _, a := range attempts {
		attempted[a.ContentItemID] = true
		if a.Correct {
			correct[a.ContentItemID] = true
		}
	}

	// Group top-level series; child series (SeriesParent set) belong to a
	// specific reading and are reached from there, not from the dashboard.
	bySeries := map[string][]*model.ContentItem{}
	var order []string
	for _, it := range items {
		if it.SeriesSlug == "" || it.SeriesParent != "" {
			continue
		}
		if _, ok := bySeries[it.SeriesSlug]; !ok {
			order = append(order, it.SeriesSlug)
		}
		bySeries[it.SeriesSlug] = append(bySeries[it.SeriesSlug], it)
	}

	for _, slug := range order {
		sItems := bySeries[slug]
		sort.Slice(sItems, func(i, j int) bool { return sItems[i].SeriesOrder < sItems[j].SeriesOrder })

		sp := &SeriesProgress{Slug: slug, Total: len(sItems)}
		ratings := make([]float64, 0, len(sItems))
		started := false
		for _, it := range sItems {
			if sp.Label == "" && it.SeriesLabel != "" {
				sp.Label = it.SeriesLabel
			}
			ratings = append(ratings, it.Rating)
			if attempted[it.Slug] {
				started = true
			}
			if correct[it.Slug] {
				sp.Done++
			} else if sp.NextSlug == "" {
				sp.NextSlug = it.Slug
			}
		}
		if sp.Label == "" {
			sp.Label = slug
		}
		sort.Float64s(ratings)
		sp.Rating = ratings[len(ratings)/2]
		sp.CEFR = model.RatingToCEFR(sp.Rating)
		sp.Percent = sp.Done * 100 / sp.Total

		switch {
		case sp.Done == sp.Total:
			path.Completed++
		case started:
			path.Continue = append(path.Continue, sp)
		case sp.Rating >= rating-150 && sp.Rating <= rating+300:
			path.Suggested = append(path.Suggested, sp)
		}
	}

	// Nearly-finished first: finishing something is the strongest pull.
	sort.SliceStable(path.Continue, func(i, j int) bool {
		return path.Continue[i].Percent > path.Continue[j].Percent
	})
	// The suggestion sweet spot sits slightly above the current level.
	target := rating + 100
	dist := func(sp *SeriesProgress) float64 {
		d := sp.Rating - target
		if d < 0 {
			d = -d
		}
		return d
	}
	sort.SliceStable(path.Suggested, func(i, j int) bool {
		return dist(path.Suggested[i]) < dist(path.Suggested[j])
	})

	if len(path.Continue) > 3 {
		path.Continue = path.Continue[:3]
	}
	if len(path.Suggested) > 3 {
		path.Suggested = path.Suggested[:3]
	}
	return path
}
