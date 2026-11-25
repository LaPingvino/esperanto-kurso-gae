package handler

import (
	"testing"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func pics() []*model.ContentItem {
	return []*model.ContentItem{
		item("o-1", "bildoj-objektoj", 1, 820), item("o-2", "bildoj-objektoj", 2, 820),
		item("b-1", "bildoj-bestoj", 1, 830),
		item("z-1", "zagr-l01", 1, 1000),
		item("p-1", "participoj", 1, 1500), item("p-2", "participoj", 2, 1520),
	}
}

// A newcomer starts with the A0 picture series in topic order, whatever
// their (unplaced, B1-looking) starting rating says.
func TestHomeNewcomer(t *testing.T) {
	hv := buildHome(allSeries(pics(), nil, time.Now()), false, 1500)
	if !hv.Intro || hv.Next == nil || hv.Next.Slug != "bildoj-objektoj" {
		t.Fatalf("newcomer's next = %+v, want the first picture series", hv.Next)
	}
	if len(hv.Today) != 2 || hv.Today[0].State != "now" || hv.Today[1].Slug != "bildoj-bestoj" || hv.Today[1].State != "new" {
		t.Errorf("newcomer's today = %+v", hv.Today)
	}
}

func TestHomePlaced(t *testing.T) {
	now := time.Now()
	at := []*model.Attempt{
		{ContentItemID: "z-1", Correct: true, Timestamp: now},
		{ContentItemID: "p-1", Correct: true, Timestamp: now},
	}
	hv := buildHome(allSeries(pics(), at, now), true, 1450)
	if hv.Intro || hv.Next == nil || hv.Next.Slug != "participoj" || hv.Next.NextSlug != "p-2" {
		t.Fatalf("next = %+v, want participoj at p-2", hv.Next)
	}
	if hv.Today[0].Slug != "zagr-l01" || hv.Today[0].State != "done" {
		t.Errorf("today starts with %+v, want the series finished today", hv.Today[0])
	}
}

func TestMapStations(t *testing.T) {
	st := buildMap(allSeries(pics(), nil, time.Now()), "A1", nil)
	want := map[string]string{"A0": "done", "A1": "here", "A2": "empty", "B1": "ahead", "C2": "empty"}
	for _, s := range st {
		if w, ok := want[s.Level]; ok && s.State != w {
			t.Errorf("%s is %s, want %s", s.Level, s.State, w)
		}
	}
	if st[1].Here == nil || st[1].Here.Slug != "zagr-l01" {
		t.Errorf("A1 opens into %+v, want zagr-l01", st[1].Here)
	}
}

// Two series of the same readings (literatura / literaturo) show once.
func TestDuplicateSeries(t *testing.T) {
	r := func(slug, series, title string) *model.ContentItem {
		return &model.ContentItem{Slug: slug, SeriesSlug: series, Type: "reading", Content: map[string]interface{}{"title": title}}
	}
	s := allSeries([]*model.ContentItem{r("a", "literatura", "La Eta Princo"), r("b", "literaturo", "La Eta Princo")}, nil, time.Now())
	if len(s) != 1 {
		t.Errorf("got %d series, want 1", len(s))
	}
}

func TestFormatProse(t *testing.T) {
	b := FormatProse("Kvin komencoj.\n\nAntaŭpartoj:\nKI- = demanda\nTI- = montra\n\nEkzemploj: Kiu venis? Neniu venis.\n\nNun-tempo — AS-finaĵo: la ago estas reala.")
	kinds := ""
	for _, x := range b {
		kinds += x.Kind + " "
	}
	if kinds != "p h table h examples p " {
		t.Fatalf("blocks %q", kinds)
	}
	if b[2].Rows[0] != [2]string{"KI-", "demanda"} || len(b[4].Items) != 2 || b[5].Term != "Nun-tempo — AS-finaĵo:" {
		t.Errorf("blocks %+v", b)
	}
}
