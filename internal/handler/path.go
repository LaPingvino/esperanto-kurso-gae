package handler

import (
	"sort"
	"strings"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// SeriesProgress is one lesson series on the learner's path.
type SeriesProgress struct {
	Slug      string
	Label     string
	CEFR      string
	Rating    float64 // median item rating: the series' level
	Done      int     // distinct items answered correctly
	Total     int
	Percent   int
	NextSlug  string // first item in series order not yet answered correctly
	FirstSlug string
	Started   bool
	Today     bool           // practised today (UTC)
	Kind      string         // the kind of the next item (ItemKind)
	Kinds     map[string]int // items per kind
	Pic       string         // the series' picture (bildoj.ForSeries), or ""
}

// Finished reports whether every item has been answered correctly.
func (sp *SeriesProgress) Finished() bool { return sp.Total > 0 && sp.Done == sp.Total }

// KindList is Kinds in a fixed order, for the map's chips.
func (sp *SeriesProgress) KindList() []KindCount {
	var out []KindCount
	for k, n := range sp.Kinds {
		out = append(out, KindCount{Kind: k, N: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// KindCount is a kind and how many items of it.
type KindCount struct {
	Kind string
	N    int
}

// learningPath groups series by the learner's relation to them.
type learningPath struct {
	Continue  []*SeriesProgress // started but unfinished, closest to done first
	Suggested []*SeriesProgress // not started, near or just above the learner's level
	Completed int
}

// seriesLabel is a series' label, or a readable name made from its slug
// for the older series that never got one.
func seriesLabel(slug, label string) string {
	if label != "" {
		return label
	}
	var n int
	switch {
	case strings.HasPrefix(slug, "zagr-l") && len(slug) == len("zagr-l01"):
		n = int(slug[6]-'0')*10 + int(slug[7]-'0')
		return "Zagreba metodo, leciono " + itoa(n)
	case strings.HasPrefix(slug, "vortaro-"):
		return "Vortoj: " + strings.ReplaceAll(strings.TrimPrefix(slug, "vortaro-"), "-", " ")
	case slug == "pmeg":
		return "PMEG"
	case strings.HasPrefix(slug, "leciono-"):
		return "Leciono " + strings.TrimPrefix(slug, "leciono-")
	}
	s := strings.ReplaceAll(slug, "-", " ")
	s = strings.ReplaceAll(strings.ReplaceAll(s, "cx", "ĉ"), "gx", "ĝ")
	if s == "" {
		return slug
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// allSeries is the progress on every top-level series, in first-seen order.
// Child series (SeriesParent set) belong to a reading and are reached from
// there. A series whose readings all repeat an earlier series' (the old
// literatura / literaturo pair) is left out.
func allSeries(items []*model.ContentItem, attempts []*model.Attempt, now time.Time) []*SeriesProgress {
	attempted := map[string]bool{}
	correct := map[string]bool{}
	today := map[string]bool{}
	day := now.UTC().Truncate(24 * time.Hour)
	for _, a := range attempts {
		attempted[a.ContentItemID] = true
		if a.Correct {
			correct[a.ContentItemID] = true
		}
		if !a.Timestamp.IsZero() && !a.Timestamp.UTC().Before(day) {
			today[a.ContentItemID] = true
		}
	}

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

	seenTitles := map[string]bool{}
	var out []*SeriesProgress
	for _, slug := range order {
		sItems := bySeries[slug]
		sort.SliceStable(sItems, func(i, j int) bool { return sItems[i].SeriesOrder < sItems[j].SeriesOrder })

		// Duplicate series: every titled item already seen elsewhere.
		dup, titled := true, 0
		for _, it := range sItems {
			if t := it.Title(); t != "" {
				titled++
				if !seenTitles[t] {
					dup = false
				}
			} else {
				dup = false
			}
		}
		if titled > 0 && dup {
			continue
		}
		for _, it := range sItems {
			if t := it.Title(); t != "" {
				seenTitles[t] = true
			}
		}

		sp := &SeriesProgress{Slug: slug, Total: len(sItems), Kinds: map[string]int{}, FirstSlug: sItems[0].Slug, Pic: bildoj.ForSeries(slug)}
		ratings := make([]float64, 0, len(sItems))
		var label string
		for _, it := range sItems {
			if label == "" {
				label = it.SeriesLabel
			}
			ratings = append(ratings, it.Rating)
			sp.Kinds[ItemKind(it)]++
			if attempted[it.Slug] {
				sp.Started = true
			}
			if today[it.Slug] {
				sp.Today = true
			}
			if correct[it.Slug] {
				sp.Done++
			} else if sp.NextSlug == "" {
				sp.NextSlug = it.Slug
				sp.Kind = ItemKind(it)
			}
		}
		if sp.NextSlug == "" { // finished: going again starts at the top
			sp.NextSlug = sp.FirstSlug
			sp.Kind = ItemKind(sItems[0])
		}
		sp.Label = seriesLabel(slug, label)
		sort.Float64s(ratings)
		sp.Rating = ratings[len(ratings)/2]
		sp.CEFR = model.RatingToCEFR(sp.Rating)
		sp.Percent = sp.Done * 100 / sp.Total
		out = append(out, sp)
	}
	return out
}

// buildPathFromItems picks what to continue and what to start next. An item
// counts as done once it has been answered correctly, so the path lingers on
// exercises the learner still gets wrong.
func buildPathFromItems(items []*model.ContentItem, attempts []*model.Attempt, rating float64) learningPath {
	return pathFrom(allSeries(items, attempts, time.Now()), rating)
}

func pathFrom(series []*SeriesProgress, rating float64) learningPath {
	var path learningPath
	for _, sp := range series {
		switch {
		case sp.Finished():
			path.Completed++
		case sp.Started:
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

// TodayRow is one line of the home page's Hodiaŭ queue.
type TodayRow struct {
	*SeriesProgress
	State string // "done" (Farita), "now" (Nun), "part" (started), "new" (Nova)
}

// homeView is what the home page shows: one big "next" and today's queue.
type homeView struct {
	Next  *SeriesProgress
	Today []TodayRow
	Intro bool // a newcomer: the A0 picture series first
}

// buildHome decides the home page. A learner who is not placed yet (see
// User.Placed) starts with the A0 picture series in their topic order, which
// need no shared language; everyone else continues their path.
func buildHome(series []*SeriesProgress, placed bool, rating float64) homeView {
	var hv homeView
	if !placed {
		hv.Intro = true
		bySlug := map[string]*SeriesProgress{}
		for _, sp := range series {
			bySlug[sp.Slug] = sp
		}
		for _, topic := range bildoj.Topics() {
			sp := bySlug["bildoj-"+topic]
			if sp == nil {
				continue
			}
			state := "new"
			switch {
			case sp.Finished():
				state = "done"
			case hv.Next == nil:
				hv.Next, state = sp, "now"
			case sp.Started:
				state = "part"
			}
			if state == "done" && !sp.Today {
				continue
			}
			hv.Today = append(hv.Today, TodayRow{sp, state})
		}
		if hv.Next != nil {
			hv.Today = trimToday(hv.Today)
			return hv
		}
		hv.Intro = false // no picture series (an empty course): the path
		hv.Today = nil
	}

	path := pathFrom(series, rating)
	switch {
	case len(path.Continue) > 0:
		hv.Next = path.Continue[0]
	case len(path.Suggested) > 0:
		hv.Next = path.Suggested[0]
	}
	for _, sp := range series {
		if sp.Finished() && sp.Today {
			hv.Today = append(hv.Today, TodayRow{sp, "done"})
		}
	}
	if hv.Next != nil {
		hv.Today = append(hv.Today, TodayRow{hv.Next, "now"})
	}
	for _, sp := range path.Continue {
		if sp != hv.Next {
			hv.Today = append(hv.Today, TodayRow{sp, "part"})
		}
	}
	for _, sp := range path.Suggested {
		if sp != hv.Next {
			hv.Today = append(hv.Today, TodayRow{sp, "new"})
		}
	}
	hv.Today = trimToday(hv.Today)
	return hv
}

// trimToday keeps the queue short: at most two finished lines, five in all.
func trimToday(rows []TodayRow) []TodayRow {
	var out []TodayRow
	done := 0
	for _, r := range rows {
		if r.State == "done" {
			if done == 2 {
				continue
			}
			done++
		}
		out = append(out, r)
		if len(out) == 5 {
			break
		}
	}
	return out
}

// Station is one KER level on the course map.
type Station struct {
	Level  string
	State  string // "done", "here", "ahead", "empty"
	Series []*SeriesProgress
	Here   *SeriesProgress // at the current station: the next set
}

// buildMap lays the series out on the line A0–C2, each under its level.
// here is the learner's level ("A0" for a learner not placed yet).
func buildMap(series []*SeriesProgress, here string, next *SeriesProgress) []Station {
	byLevel := map[string][]*SeriesProgress{}
	for _, sp := range series {
		byLevel[sp.CEFR] = append(byLevel[sp.CEFR], sp)
	}
	var out []Station
	passed := true
	for _, l := range Levels {
		st := Station{Level: l, Series: byLevel[l]}
		sort.SliceStable(st.Series, func(i, j int) bool { return st.Series[i].Rating < st.Series[j].Rating })
		switch {
		case l == here:
			st.State = "here"
			passed = false
			if next != nil && next.CEFR == l {
				st.Here = next
			} else {
				for _, sp := range st.Series {
					if !sp.Finished() {
						st.Here = sp
						break
					}
				}
			}
		case len(st.Series) == 0:
			st.State = "empty"
		case passed:
			st.State = "done"
		default:
			st.State = "ahead"
		}
		out = append(out, st)
	}
	return out
}
