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

// Every exercise's tiles hold its right answer exactly once, and there are
// at most four.
func TestTilesHoldTheAnswer(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	c := Build(items)
	for _, p := range c.Places {
		for _, e := range p.Ex {
			for g, ts := range e.Tiles {
				right := ""
				switch e.Kind {
				case 'F':
					right = e.Answers[g%len(e.Answers)]
				case 'Q', 'H':
					right = e.Answers[0]
				case 'B':
					continue
				}
				n := 0
				for _, x := range ts {
					if x == right {
						n++
					}
				}
				if n != 1 || len(ts) > 4 {
					t.Errorf("%s %c gap %d: tiles %q, answer %q", e.Slug, e.Kind, g, ts, right)
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
