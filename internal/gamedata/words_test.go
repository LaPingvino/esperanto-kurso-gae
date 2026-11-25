package gamedata

import (
	"strings"
	"testing"
)

// TestWords: the bonus games' words from the seed — five-letter words in
// every level the pool needs, the day's word in its pool, a definition never
// giving its word away.
func TestWords(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	c := Build(items)
	ws := c.Words()
	n, pics := 0, 0
	aff := map[string]string{}
	for _, w := range ws {
		if w.Affix != "" {
			aff[w.Word] = w.Affix
			if n := len([]rune(w.Word)); n < 6 || n > 9 || !strings.Contains(w.Word[1:len(w.Word)-1], w.Affix) && !strings.HasPrefix(w.Word, w.Affix) {
				t.Errorf("%s: affix %q", w.Word, w.Affix)
			}
		}
		if w.Five {
			n++
			if PictureKey(w.Word) != "" {
				pics++
			}
			if d := eoDef(w.Ex, w.Word); strings.Contains(strings.ToLower(d), strings.TrimRight(w.Word, "oaie")) {
				t.Errorf("%s: its definition shows it: %q", w.Word, d)
			}
		}
	}
	t.Logf("%d words, %d for Vortludo, %d with a picture", len(ws), n, pics)
	for _, l := range Levels {
		p := Pool(ws, l)
		if len(p) < 10 {
			t.Errorf("%s: a pool of %d", l, len(p))
		}
		d := Daily(p, "2026-11-25", l)
		long := l >= Levels[LongFrom]
		if (!long && !IsFive(d)) || (long && aff[d] == "") || Daily(p, "2026-11-25", l) != d {
			t.Errorf("%s: day's word %q", l, d)
		}
		t.Logf("%s: %d words, %v", l, len(p), p)
	}
	if IsFive("Hundo") || IsFive("hund") || !IsFive("ŝtono") || !IsFive("aĉeti") {
		t.Error("IsFive")
	}
	if got := maskFold("Hundo estas besto; hundoj bojas.", "hund"); got != "~ estas besto; ~ bojas." {
		t.Errorf("maskFold: %q", got)
	}
}
