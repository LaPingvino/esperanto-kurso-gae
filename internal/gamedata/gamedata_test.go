package gamedata

import (
	"fmt"
	"testing"
)

func TestTokens(t *testing.T) {
	show := func(q string) string {
		s := ""
		for _, tk := range tokens(q) {
			if tk.space {
				s += " "
			}
			if tk.gap >= 0 {
				s += fmt.Sprintf("[%d]", tk.gap)
			} else {
				s += tk.s
			}
		}
		return s
	}
	for q, want := range map[string]string{
		"Marko kaj Aleksandro estas lernant___kaj sportist___.": "Marko kaj Aleksandro estas lernant[0]kaj sportist[1].",
		"___ sidas sur seĝo.":          "[0] sidas sur seĝo.",
		"Ili estas ___amikoj.":         "Ili estas [0]amikoj.",
		"Laboristo ___. Lernanto ___.": "Laboristo [0]. Lernanto [1].",
	} {
		if got := show(q); got != want {
			t.Errorf("%q: %q, want %q", q, got, want)
		}
	}
}

// Every gap's tiles hold its right answer exactly once.
func TestTilesHoldTheAnswer(t *testing.T) {
	items, err := ReadSeed("../../seed/ekzercoj-zagr-restructured.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := MakeCourse(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range c.Lessons {
		for _, f := range l.Fill {
			for g, ts := range f.Tiles {
				right := f.Answers[g%len(f.Answers)]
				n := 0
				for _, x := range ts {
					if x == right {
						n++
					}
				}
				if n != 1 || len(ts) != 4 {
					t.Errorf("%s gap %d: tiles %q, answer %q", f.Slug, g, ts, right)
				}
			}
		}
	}
}

func TestShown(t *testing.T) {
	for _, c := range []struct{ base, root, show, seen string }{
		{"amiko", "amik", "amiko", ""}, {"patrino", "patr", "patr-", "patrino"},
		{"lern", "lern", "lern-", ""}, {"mia", "mia", "mia", ""}, {"ankaŭ", "ankaŭ", "ankaŭ", ""},
	} {
		show, seen := (&Word{Base: c.base, Root: c.root}).Shown()
		if show != c.show || seen != c.seen {
			t.Errorf("%s/%s: %q %q", c.base, c.root, show, seen)
		}
	}
}
