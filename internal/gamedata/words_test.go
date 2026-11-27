package gamedata

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
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

// TestBlocks: Konstrui's words — each word's tiles spell it, with the
// split legvortoj gives (one root, an affix or more, the ending), and every
// level's pool holds only words of that level or below, enough to play.
func TestBlocks(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	ws := Build(items).Words()
	lv := map[string]int{}
	for i, l := range Levels {
		lv[l] = i
	}
	level := map[string]string{}
	for _, w := range ws {
		level[w.Word] = w.Level
		if w.Blocks == "" {
			continue
		}
		if got := strings.NewReplacer("|", "", "-", "").Replace(w.Blocks); got != w.Word {
			t.Errorf("%s: blocks %q spell %q", w.Word, w.Blocks, got)
		}
		if ps := strings.Split(w.Blocks, "|"); len(ps) < 3 || !strings.HasPrefix(ps[len(ps)-1], "-") {
			t.Errorf("%s: blocks %q", w.Word, w.Blocks)
		}
	}
	for _, l := range Levels {
		p := BuildPool(ws, l)
		if len(p) < 6 {
			t.Errorf("%s: a pool of %d", l, len(p))
		}
		for _, w := range p {
			if lv[level[w]] > lv[l] {
				t.Errorf("%s: %s is of %s", l, w, level[w])
			}
		}
		t.Logf("%s: %d words", l, len(p))
	}
	t.Log("\n" + poolStats(ws))
}

// TestDerived: the words built from the course's roots and affixes — each a
// ReVo headword split beyond doubt, not a course word, every piece taught at
// or below its level and the hardest at it, for Konstrui alone; and each
// level's mix: prefixes from A1, no affix over PerAffix.
func TestDerived(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	c := Build(items)
	course := c.courseWords()
	at := taught(course)
	own := map[string]bool{}
	for _, w := range course {
		own[w.Word] = true
	}
	n := map[string]int{}
	prefixed := map[string]bool{}
	for _, w := range c.Words()[len(course):] {
		if !w.Derived || own[w.Word] || w.Five || w.Affix != "" || revo.Exact(w.Word) == nil {
			t.Errorf("%s: %+v", w.Word, w)
		}
		ps := legvortoj.Blocks(w.Word)
		if BlockLabels(ps) != w.Blocks || ps == nil {
			t.Errorf("%s: blocks %q", w.Word, w.Blocks)
			continue
		}
		hard := 0
		for _, p := range ps[:len(ps)-1] {
			hard = max(hard, at[p.Text])
			if p.Kind != "radiko" {
				n[w.Level+" "+p.Text]++
			}
			if p.Kind == "prefikso" {
				prefixed[w.Level] = true
			}
		}
		if Levels[hard] != w.Level {
			t.Errorf("%s: of %s, its hardest piece of %s", w.Word, w.Level, Levels[hard])
		}
	}
	for k, v := range n {
		if v > PerAffix {
			t.Errorf("%s: %d words", k, v)
		}
	}
	if !prefixed["A1"] || !prefixed["B1"] {
		t.Errorf("prefixes: %v", prefixed)
	}
}

// poolStats: Konstrui's words per level (the level's own, as BuildPool
// takes them) and per affix (a word of two affixes under each).
func poolStats(ws []*WordEntry) string {
	var b strings.Builder
	for _, l := range Levels {
		n, aff := 0, map[string]int{}
		for _, w := range ws {
			if w.Level != l || w.Blocks == "" {
				continue
			}
			n++
			ps := strings.Split(w.Blocks, "|")
			for _, p := range ps[:len(ps)-1] {
				if strings.Contains(p, "-") {
					aff[p]++
				}
			}
		}
		var ks []string
		for k := range aff {
			ks = append(ks, k)
		}
		sort.Slice(ks, func(i, j int) bool { return aff[ks[i]] > aff[ks[j]] || aff[ks[i]] == aff[ks[j]] && ks[i] < ks[j] })
		fmt.Fprintf(&b, "%s: %d own, pool %d;", l, n, len(BuildPool(ws, l)))
		for _, k := range ks {
			fmt.Fprintf(&b, " %s %d", k, aff[k])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// TestMemoPool: Memoro's words — every level's pool enough for a board of
// six pairs, of that level or below, each with a picture or a definition
// short enough for a card, never a word derived for Konstrui.
func TestMemoPool(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	ws := Build(items).Words()
	lv := map[string]int{}
	for i, l := range Levels {
		lv[l] = i
	}
	by := map[string]*WordEntry{}
	for _, w := range ws {
		if by[w.Word] == nil {
			by[w.Word] = w
		}
	}
	for _, l := range Levels {
		p := MemoPool(ws, l)
		if len(p) < 12 {
			t.Errorf("%s: a pool of %d", l, len(p))
		}
		for _, x := range p {
			w := by[x]
			if lv[w.Level] > lv[l] || w.Derived {
				t.Errorf("%s: %s (%s, derived %v)", l, x, w.Level, w.Derived)
			}
			if PictureKey(x) == "" && (eoDef(w.Ex, x) == "" || len([]rune(eoDef(w.Ex, x))) > MemoDefMax) || !memoShort(x) {
				t.Errorf("%s: %s neither picture nor a short definition", l, x)
			}
		}
		t.Logf("%s: %d words", l, len(p))
	}
}

// TestTrainPool: Frazotrajno's sentences — every level's pool enough for a
// round of five, of that level or below, each of three to seven words with
// no comma and no kaj or aŭ (orders the text's alone can't judge), once.
func TestTrainPool(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	ss := Build(items).Sentences()
	lv := map[string]int{}
	for i, l := range Levels {
		lv[l] = i
	}
	by := map[string]*Sentence{}
	for _, s := range ss {
		if by[s.Text] != nil {
			t.Errorf("%q twice", s.Text)
		}
		by[s.Text] = s
	}
	for _, l := range Levels {
		p := TrainPool(ss, l)
		if len(p) < 10 {
			t.Errorf("%s: a pool of %d", l, len(p))
		}
		for _, x := range p {
			n := len(strings.Fields(x))
			if lv[by[x].Level] > lv[l] || n < TrainMin || n > TrainMax || strings.ContainsAny(x, ",.") || strings.Contains(" "+strings.ToLower(x)+" ", " kaj ") {
				t.Errorf("%s: %q (%s)", l, x, by[x].Level)
			}
		}
		t.Logf("%s: %d sentences", l, len(p))
	}
	if TrainOK("Patro kaj frato venis") || TrainOK("Mi vidis, ke ŝi venis") || TrainOK("Jen avo") || !TrainOK("Jen mia avo") {
		t.Error("TrainOK")
	}
}

// TestRainPool: Vortpluvo's words — every level's pool of twenty or more,
// of that level or below, each one word of two to twelve small letters.
func TestRainPool(t *testing.T) {
	items, err := ReadSeedDir("../../seed")
	if err != nil {
		t.Fatal(err)
	}
	ws := Build(items).Words()
	lv := map[string]int{}
	for i, l := range Levels {
		lv[l] = i
	}
	by := map[string]*WordEntry{}
	for _, w := range ws {
		if by[w.Word] == nil {
			by[w.Word] = w
		}
	}
	for _, l := range Levels {
		p := RainPool(ws, l)
		if len(p) < 20 {
			t.Errorf("%s: a pool of %d", l, len(p))
		}
		for _, x := range p {
			n := len([]rune(x))
			if lv[by[x].Level] > lv[l] || by[x].Derived || n < RainMin || n > RainMax || !letters(x) {
				t.Errorf("%s: %q (%s)", l, x, by[x].Level)
			}
		}
		t.Logf("%s: %d words", l, len(p))
	}
	if RainOK(&WordEntry{Word: "Zamenhof"}) || RainOK(&WordEntry{Word: "bona tago"}) || RainOK(&WordEntry{Word: "a"}) || !RainOK(&WordEntry{Word: "ĉu"}) {
		t.Error("RainOK")
	}
}
