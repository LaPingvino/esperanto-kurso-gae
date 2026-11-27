package gamedata

import (
	_ "embed"
	"sort"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

// Bonvorto: a Vortludo on La Bona Lingvo's pairs (the "Bona lingvo"
// series): the word with a root of its own given (trajno), the word of the
// same meaning built from basic roots to find (vagonaro), with its shape,
// the roots hidden (…·ar·o), beside the board. It plays the pairs the course
// has, one line each in the word list:
//
//	B <neo> <built> <shape> <divided> <meaning>
//
// shape: the division with each root as …; divided: vagon·ar·o, shown at
// the end; meaning: in the list's language, the hint after three misses.

// bonaTSV is cmd/bonalingvo's -dividoj: neo, built, the project's division.
//
//go:embed bonalingvo.tsv
var bonaTSV string

// BonaMin and BonaMax are the letters of a built word Bonvorto plays: the
// board of Vortludo's long words, nine cells at most on a phone.
const (
	BonaMin = 6
	BonaMax = 9
)

// Pair is one round of Bonvorto.
type Pair struct {
	Neo, Built, Divided string
}

var bonaDiv = func() map[string]Pair {
	m := map[string]Pair{}
	for _, l := range strings.Split(bonaTSV, "\n") {
		f := strings.Split(l, "\t")
		if len(f) == 3 {
			m[f[1]] = Pair{f[0], f[1], f[2]}
		}
	}
	return m
}()

// BonaOK: a pair Bonvorto plays — the built word of the alphabet's small
// letters, six to nine of them, its division spelling it.
func BonaOK(p Pair) bool {
	n := len([]rune(p.Built))
	return letters(p.Built) && n >= BonaMin && n <= BonaMax && strings.ReplaceAll(p.Divided, "·", "") == p.Built
}

// Shape is a division with its roots hidden (mal·…·ul·ej·o): affixes, the
// linking o and the ending stay. A word with no root but affixes has the
// first as its root (eks·iĝ·i, obl·ig·i: …·iĝ·i).
func Shape(divided string) string {
	ps := strings.Split(divided, "·")
	hid := false
	for i, p := range ps[:len(ps)-1] {
		if p != "o" && legvortoj.AffixKind(p) == "" {
			ps[i] = "…"
			hid = true
		}
	}
	if !hid {
		ps[0] = "…"
	}
	return strings.Join(ps, "·")
}

// Pairs are Bonvorto's rounds: the course's Bona lingvo exercises that
// ask for the built word (bona-lingvo-*-al-konstruita: its right option),
// each with the project's division, BonaOK; sorted by the built word.
// The game plays them all, whatever the learner's level: the series is
// C1–C2, and the board with the shape is help enough.
func (c *Course) Pairs() []Pair {
	var out []Pair
	seen := map[string]bool{}
	for _, p := range c.Places {
		for _, e := range p.Ex {
			if e.Kind != 'M' || !strings.HasPrefix(e.Slug, "bona-lingvo-") || !strings.HasSuffix(e.Slug, "-al-konstruita") ||
				e.Correct < 0 || e.Correct >= len(e.Options) {
				continue
			}
			pr, ok := bonaDiv[e.Options[e.Correct]]
			if !ok || seen[pr.Built] || !BonaOK(pr) || !strings.Contains(e.Question, "„"+pr.Neo+"“") {
				continue
			}
			seen[pr.Built] = true
			out = append(out, pr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Built < out[j].Built })
	return out
}

// bonaMeaning: the pair's meaning in lang, by ReVo (the built word's, else
// the other's).
func bonaMeaning(p Pair, lang string) string {
	if h := revo.Exact(p.Built); h != nil {
		if m := Meaning(nil, h, lang); m != "" {
			return m
		}
	}
	return Meaning(nil, revo.Exact(p.Neo), lang)
}
