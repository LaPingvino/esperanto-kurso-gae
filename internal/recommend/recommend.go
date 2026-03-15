// Package recommend selects content items suited to a learner's current level.
package recommend

import (
	"context"
	"sort"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

// GetForUser returns up to limit content items appropriate for the user's rating.
// Items are sourced within ±200 rating points of the user's rating, then ranked
// by RD descending so that poorly-calibrated items are prioritised (more info gained).
// For beginners (rating < 1200) items that have images are preferred.
func GetForUser(
	ctx context.Context,
	userRating float64,
	userRD float64,
	cs *store.ContentStore,
	limit int,
) ([]*model.ContentItem, error) {

	minR := userRating - 200
	maxR := userRating + 200

	items, err := cs.ListByRatingRange(ctx, minR, maxR, limit*3)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		// Fall back to all approved items if no items in range.
		items, err = cs.ListApproved(ctx, limit)
		if err != nil {
			return nil, err
		}
	}

	// For new/uncertain users (high RD means little is known about their level),
	// always include at least one very easy item so true beginners have somewhere
	// to start regardless of where their rating currently sits.
	if userRD > 200 && minR > 1100 {
		easyItems, _ := cs.ListByRatingRange(ctx, 0, 1300, 3)
		if len(easyItems) > 0 {
			// Add easy items not already present.
			seen := make(map[string]bool, len(items))
			for _, it := range items {
				seen[it.Slug] = true
			}
			for _, ei := range easyItems {
				if !seen[ei.Slug] {
					items = append(items, ei)
					break
				}
			}
		}
	}

	beginner := userRating < 1200

	// Score each item: lower is better.
	// Primary sort: prefer high RD (less certain difficulty → more rating info).
	// Secondary: for beginners prefer items with images.
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		// Beginners: bump items with images up.
		if beginner {
			aHasImg := a.ImageURL != ""
			bHasImg := b.ImageURL != ""
			if aHasImg != bHasImg {
				return aHasImg
			}
		}
		// Prefer higher RD (more to learn from).
		return a.RD > b.RD
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// GetHarder returns items rated +200 to +600 above the user's rating.
func GetHarder(ctx context.Context, userRating float64, cs *store.ContentStore, limit int, exclude string) ([]*model.ContentItem, error) {
	items, err := cs.ListByRatingRange(ctx, userRating+200, userRating+600, limit*2)
	if err != nil || len(items) == 0 {
		items, err = cs.ListByRatingRange(ctx, userRating+50, userRating+1000, limit*2)
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RD > items[j].RD })
	return firstExcluding(items, exclude, limit), nil
}

// GetEasier returns items rated -600 to -200 below the user's rating.
func GetEasier(ctx context.Context, userRating float64, cs *store.ContentStore, limit int, exclude string) ([]*model.ContentItem, error) {
	items, err := cs.ListByRatingRange(ctx, userRating-600, userRating-200, limit*2)
	if err != nil || len(items) == 0 {
		items, err = cs.ListByRatingRange(ctx, userRating-1000, userRating-50, limit*2)
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RD > items[j].RD })
	return firstExcluding(items, exclude, limit), nil
}

func firstExcluding(items []*model.ContentItem, exclude string, limit int) []*model.ContentItem {
	var out []*model.ContentItem
	for _, it := range items {
		if it.Slug != exclude {
			out = append(out, it)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}
