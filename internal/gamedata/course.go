package gamedata

import (
	"fmt"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/lingvoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/locale"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The course as the game sees it. The structure is the site's own:
//
//   - levels (KER A0 … C2), each series in the level of its first item's
//     rating, sorted by label — as /serioj groups them (ShowSeriesBrowser);
//   - a series' places, in series order: a reading is a place of its own
//     (its text, then its child series' exercises, word cards from its text
//     and sentences to build from it); a video is a place (watched on the
//     site); the other exercises go in places of up to placeSize;
//   - items in no series: by type, in an "Aliaj" series in the level of
//     their rating; vocabulary tagged with a reading's slug is that
//     reading's words, not exercises of their own.
//
// Place slugs: a reading's own slug (zagr-l01-teksto — the account's stars
// are kept under them), a video's own slug, else "<series>.<n>".

const placeSize = 10

// Levels in order (model.RatingToCEFR's).
var Levels = []string{"A0", "A1", "A2", "B1", "B2", "C1", "C2"}

// Course is the whole course for the game.
type Course struct {
	Version string
	Series  []*Series // in map order: level, then label
	Places  []*Place  // all, in map order
	Langs   []string  // languages the meanings can be shown in, "en" first
	place   map[string]*Place
	of      map[string]string // item slug → its place's slug

	derivedOnce sync.Once
	derived     []*WordEntry // Words' derived ones, made once
}

// Series is one of the site's series (or "Aliaj" per level).
type Series struct {
	Slug, Label, Level string
	Places             []*Place
}

// Place is a stop on a series' path.
type Place struct {
	Slug, Title string
	Kind        byte // 'L' a reading's lesson, 'E' exercises, 'V' a video
	Series      *Series
	Text        string // a reading's
	Video       string // a video item's slug (watched at /ekzerco/<slug>)
	Ex          []*Exercise
	Items       []string // the site items it shows (for the "play as a game" links)
}

// Exercise is one encounter. Kinds: 'C' a word card (which meaning?),
// 'F' a fill-in, 'M' multiple choice, 'Q' a vocabulary item (the meaning
// given: which Esperanto word?), 'H' a question with a typed answer (the
// phrasebook, and any other type that has a question and an answer),
// 'B' a sentence of the text to put in order. C and B are the game's own
// (derived, never sent to the site); the others are the site's items,
// judged by its rules (handler.checkAnswer).
type Exercise struct {
	Kind     byte
	Slug     string // the site item's ('F','M','Q','H')
	Question string
	Hint     string
	Answers  []string   // F: GapAnswers; Q: the word; H: Answer()
	Alts     []string   // Alternatives() (not for F: the site checks none there)
	Tiles    [][]string // F: per gap; Q, H, B: one set
	Options  []string   // M: the item's options
	Correct  int        // M: CorrectIndex()
	Defs     map[string]string
	Revo     *revo.Hit // Q: Reta Vortaro's entry for the word, for Meaning
	Word     *Word     // C
	Sentence string    // B
	Bare     bool      // Q: a bare root or affix (tagged radiko, sufikso, prefikso), not a word to guess
}

// Word is a card's word: the form met in the text, its vocabulary root, the
// meanings, and the other words whose meanings the card offers.
type Word struct {
	Base, Root string
	Defs       map[string]string
	Revo       *revo.Hit
	Others     []*Word
}

// Place finds a place by slug.
func (c *Course) Place(slug string) *Place { return c.place[slug] }

// PlaceOf is the place showing a site item ("" for none).
func (c *Course) PlaceOf(itemSlug string) string { return c.of[itemSlug] }

// Build makes the course from the approved items (any order).
func Build(all []*model.ContentItem) *Course {
	items := make([]*model.ContentItem, 0, len(all))
	for _, it := range all {
		if it.Status == "" || it.Status == "approved" {
			items = append(items, it)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })

	c := &Course{place: map[string]*Place{}, of: map[string]string{}}
	vocab := map[string]*model.ContentItem{} // words with a meaning, for cards
	readings := map[string]bool{}
	children := map[string][]*model.ContentItem{}
	for _, it := range items {
		if it.Type == "vocab" && len(defsOf(it)) > 0 {
			if w := strings.ToLower(strings.TrimSpace(it.Word())); w != "" {
				if old := vocab[w]; old == nil || len(defsOf(it)) > len(defsOf(old)) {
					vocab[w] = it
				}
			}
		}
		if it.Type == "reading" {
			readings[it.Slug] = true
		}
		if it.SeriesParent != "" {
			children[it.SeriesParent] = append(children[it.SeriesParent], it)
		}
	}

	// Series in the site's way: the first item (slug order) gives label and level.
	bySeries := map[string]*Series{}
	members := map[string][]*model.ContentItem{}
	var order []string
	for _, it := range items {
		if it.SeriesParent != "" {
			continue // a reading's child series: in the reading's place
		}
		slug := it.SeriesSlug
		if slug == "" {
			if it.Type == "vocab" && (taggedReading(it, readings) || strings.HasPrefix(it.Slug, "voc-auto-")) {
				continue // a reading's words (internal/legvortoj): in its text, not on their own
			}
			if !usable(it) {
				continue
			}
			slug = "aliaj-" + model.RatingToCEFR(it.Rating)
		}
		if bySeries[slug] == nil {
			label := it.SeriesLabel
			if it.SeriesSlug == "" {
				label = "Aliaj"
			} else if label == "" {
				label = slug
			}
			bySeries[slug] = &Series{Slug: slug, Label: label, Level: model.RatingToCEFR(it.Rating)}
			order = append(order, slug)
		}
		members[slug] = append(members[slug], it)
	}
	for _, lvl := range Levels {
		var in []*Series
		for _, s := range order {
			if bySeries[s].Level == lvl {
				in = append(in, bySeries[s])
			}
		}
		sort.SliceStable(in, func(i, j int) bool {
			if (in[i].Label == "Aliaj") != (in[j].Label == "Aliaj") {
				return in[j].Label == "Aliaj" // "Aliaj" last in its level
			}
			return in[i].Label < in[j].Label
		})
		c.Series = append(c.Series, in...)
	}

	seenWord := map[string]bool{}
	for _, s := range c.Series {
		ms := members[s.Slug]
		sort.SliceStable(ms, func(i, j int) bool { return ms[i].SeriesOrder < ms[j].SeriesOrder })
		var chunk []*model.ContentItem
		nE := 0 // exercise places so far
		flush := func() {
			for len(chunk) > 0 {
				n := min(placeSize, len(chunk))
				nE++
				p := &Place{Slug: fmt.Sprintf("%s.%d", s.Slug, nE), Kind: 'E', Series: s}
				p.Title = fmt.Sprintf("%s %d", s.Label, nE)
				for _, it := range chunk[:n] {
					if e := exerciseOf(it); e != nil {
						p.Ex = append(p.Ex, e)
						p.Items = append(p.Items, it.Slug)
					}
				}
				chunk = chunk[n:]
				if len(p.Ex) > 0 {
					c.add(s, p)
				}
			}
		}
		for _, it := range ms {
			switch it.Type {
			case "reading":
				flush()
				p := &Place{Slug: it.Slug, Kind: 'L', Series: s, Title: it.Title(), Text: it.Text(), Items: []string{it.Slug}}
				if p.Title == "" {
					p.Title = fmt.Sprintf("%s %d", s.Label, len(s.Places)+1)
				}
				p.Ex = append(p.Ex, wordCards(p.Text, vocab, seenWord)...)
				kids := children[it.Slug]
				sort.SliceStable(kids, func(i, j int) bool { return kids[i].SeriesOrder < kids[j].SeriesOrder })
				for _, k := range kids {
					if e := exerciseOf(k); e != nil {
						p.Ex = append(p.Ex, e)
						p.Items = append(p.Items, k.Slug)
					}
				}
				for i, sent := range sentences(p.Text) {
					if i == 2 {
						break
					}
					p.Ex = append(p.Ex, &Exercise{Kind: 'B', Sentence: strings.Join(sent, " "), Tiles: [][]string{shuffled(sent, it.Slug)}})
				}
				c.add(s, p)
			case "video":
				flush()
				if it.VideoURL() == "" {
					continue
				}
				p := &Place{Slug: it.Slug, Kind: 'V', Series: s, Title: it.Title(), Video: it.Slug, Items: []string{it.Slug}}
				if p.Title == "" {
					p.Title = fmt.Sprintf("%s %d", s.Label, len(s.Places)+1)
				}
				c.add(s, p)
			default:
				if usable(it) {
					chunk = append(chunk, it)
				}
			}
		}
		flush()
	}
	// drop series left without places
	kept := c.Series[:0]
	for _, s := range c.Series {
		if len(s.Places) > 0 {
			kept = append(kept, s)
		}
	}
	c.Series = kept

	tiles(c)
	c.Langs = meaningLangs(items)
	h := fnv.New64a()
	for _, it := range items {
		fmt.Fprint(h, it.Slug, it.Type, it.Content, it.SeriesSlug, it.SeriesOrder, it.SeriesParent, it.SeriesLabel, it.Tags, model.RatingToCEFR(it.Rating), "\x00")
	}
	c.Version = fmt.Sprintf("%x", h.Sum64())
	return c
}

func (c *Course) add(s *Series, p *Place) {
	if c.place[p.Slug] != nil {
		return
	}
	s.Places = append(s.Places, p)
	c.Places = append(c.Places, p)
	c.place[p.Slug] = p
	for _, it := range p.Items {
		if c.of[it] == "" {
			c.of[it] = p.Slug
		}
	}
}

// usable: an item the game has an encounter for.
func usable(it *model.ContentItem) bool {
	switch it.Type {
	case "reading":
		return it.Text() != ""
	case "video":
		return it.VideoURL() != ""
	}
	return exerciseOf(it) != nil
}

func taggedReading(it *model.ContentItem, readings map[string]bool) bool {
	for _, t := range it.Tags {
		if readings[t] {
			return true
		}
	}
	return false
}

// exerciseOf is a site item's encounter, with the site's answer data; nil
// when the game has none for it.
func exerciseOf(it *model.ContentItem) *Exercise {
	clean := func(xs []string) []string {
		var out []string
		for _, x := range xs {
			if strings.TrimSpace(x) != "" {
				out = append(out, strings.TrimSpace(x))
			}
		}
		return out
	}
	switch it.Type {
	case "fillin":
		q := it.Question()
		gaps := strings.Count(q, "___")
		ans := it.GapAnswers()
		if gaps == 0 || len(ans) == 0 {
			return nil
		}
		return &Exercise{Kind: 'F', Slug: it.Slug, Question: q, Hint: it.Hint(), Answers: ans}
	case "multiplechoice":
		opts := it.Options()
		ci := it.CorrectIndex()
		if it.Question() == "" || len(opts) < 2 || ci < 0 || ci >= len(opts) {
			return nil
		}
		return &Exercise{Kind: 'M', Slug: it.Slug, Question: it.Question(), Options: opts, Correct: ci, Alts: clean(it.Alternatives())}
	case "vocab":
		w := strings.TrimSpace(it.Word())
		d := defsOf(it)
		hit := lookup(it)
		if w == "" || len(d) == 0 && hit == nil {
			return nil
		}
		bare := false
		for _, t := range it.Tags {
			bare = bare || t == "radiko" || t == "sufikso" || t == "prefikso"
		}
		return &Exercise{Kind: 'Q', Slug: it.Slug, Answers: []string{w}, Alts: clean(it.Alternatives()), Defs: d, Revo: hit, Hint: it.Hint(), Bare: bare}
	case "reading", "video":
		return nil
	}
	// The phrasebook and any other type with a question and an answer: the
	// site compares the answer (checkAnswer's default).
	if it.Question() != "" && strings.TrimSpace(it.Answer()) != "" && it.ImageURL == "" && it.AudioURL() == "" {
		return &Exercise{Kind: 'H', Slug: it.Slug, Question: it.Question(), Hint: it.Hint(), Answers: []string{strings.TrimSpace(it.Answer())}, Alts: clean(it.Alternatives())}
	}
	return nil
}

// defsOf: a vocabulary item's meanings by language — definitions[lang],
// the flat "definition" and the old "en" field as English.
func defsOf(it *model.ContentItem) map[string]string {
	d := definitions(it)
	if d["en"] == "" {
		if s := strings.TrimSpace(it.Definition()); s != "" {
			d["en"] = s
		} else if s, _ := it.Content["en"].(string); strings.TrimSpace(s) != "" {
			d["en"] = strings.TrimSpace(s)
		}
	}
	// Generated definitions copied from another language (Romanian = the
	// French text…) are not meanings in that language.
	var copies []string
	for l := range lingvoj.CopiedFrom {
		if lingvoj.CopySource(d, l) != "" {
			copies = append(copies, l)
		}
	}
	for _, l := range copies {
		delete(d, l)
	}
	return d
}

// Meaning is a card's meaning in lang, by the site's fallback chain: the
// course's definition in lang, Reta Vortaro's translation, then English
// (the course's, then ReVo's), the Esperanto definition (the course's, then
// ReVo's), and last any
// course definition. A meaning not in lang carries its language in front,
// "(en) boy", so a card never passes off another language as lang's. Text
// in another script than lang's (Greek on a German card) is not lang's.
func Meaning(d map[string]string, hit *revo.Hit, lang string) string {
	if d[lang] != "" && !lingvoj.WrongScript(d[lang], lang) {
		return d[lang]
	}
	if t := hit.Translation(lang); t != "" && !lingvoj.WrongScript(t, lang) {
		return t
	}
	mark := func(l, s string) string {
		if l == lang {
			return s
		}
		return "(" + l + ") " + s
	}
	if d["en"] != "" {
		return mark("en", d["en"])
	}
	if t := hit.Translation("en"); t != "" {
		return mark("en", t)
	}
	if d["eo"] != "" {
		return mark("eo", d["eo"])
	}
	if hit != nil && hit.Def != "" {
		return mark("eo", hit.Def)
	}
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if d[k] != "" && !lingvoj.WrongScript(d[k], k) {
			return mark(k, d[k])
		}
	}
	return ""
}

// lookup is ReVo's entry for a vocab item's word (roots and affixes looked
// up as roots, as on the site).
func lookup(it *model.ContentItem) *revo.Hit {
	root := false
	for _, t := range it.Tags {
		if t == "radiko" || t == "sufikso" || t == "prefikso" {
			root = true
		}
	}
	return revo.Lookup(it.Word(), root)
}

// wordCards: up to five words of a reading's text that have a meaning
// (eo.ExtractWords, matched to the vocabulary by root — DERIVED), each word
// once in the whole course, plain forms first.
func wordCards(text string, vocab map[string]*model.ContentItem, seen map[string]bool) []*Exercise {
	bases := eo.ExtractWords(text)
	rank := func(base string) int {
		r, _ := findRoot(base, vocab)
		switch n := len(base) - len(r); {
		case n == 1:
			return 0
		case n == 0:
			return 1
		}
		return 2
	}
	sort.SliceStable(bases, func(i, j int) bool { return rank(bases[i]) < rank(bases[j]) })
	var out []*Exercise
	for _, base := range bases {
		root, v := findRoot(base, vocab)
		if v == nil || seen[root] {
			continue
		}
		seen[root] = true
		w := &Word{Base: base, Root: strings.TrimSpace(v.Word()), Defs: defsOf(v), Revo: lookup(v)}
		out = append(out, &Exercise{Kind: 'C', Word: w})
		if len(out) == 5 {
			break
		}
	}
	return out
}

// Shown is how a word card names the word: the form met in the text when it
// is the root with an ending (amik → amiko), else the root itself (patr- —
// "father" — for patrino, so the meaning shown is the root's), with the
// text's form beside it (seen).
func (w *Word) Shown() (show, seen string) {
	if w.Base == w.Root {
		if strings.ContainsAny(w.Root[len(w.Root)-1:], "aeiou") || strings.HasSuffix(w.Root, "ŭ") || strings.Contains(w.Root, " ") {
			return w.Root, "" // a whole word (mia, ankaŭ)
		}
		return w.Root + "-", "" // a root met only in a participle (lern- from lernanto)
	}
	for _, end := range []string{"o", "a", "i", "e"} {
		if w.Base == w.Root+end {
			return w.Base, ""
		}
	}
	return w.Root + "-", w.Base
}

// tiles gives every exercise its choices, DERIVED from the answers around
// it (never made up): a fill-in gap's other gap answers of the same kind
// (an ending or a word), the same place first, then the series, then all;
// a vocabulary item's or a question's other answers, the same way; a word
// card's other words' meanings (chosen when the text is written, by
// language).
func tiles(c *Course) {
	type cand struct {
		s      string
		place  *Place
		kind   int // 0 a word, 1 an ending, 2 a vocabulary word, 3 a question's answer
		series *Series
	}
	var pool []cand
	seen := map[string]bool{}
	addPool := func(p *Place, kind int, s string) {
		k := fmt.Sprintf("%s/%d/%s", p.Slug, kind, strings.ToLower(s))
		if !seen[k] {
			seen[k] = true
			pool = append(pool, cand{s, p, kind, p.Series})
		}
	}
	var words []*Word
	wordPlace := map[*Word]*Place{}
	for _, p := range c.Places {
		for _, e := range p.Ex {
			switch e.Kind {
			case 'F':
				for g := 0; g < strings.Count(e.Question, "___"); g++ {
					addPool(p, b2i(gapIsSuffix(e.Question, g)), strings.TrimSpace(e.Answers[g%len(e.Answers)]))
				}
			case 'Q':
				addPool(p, 2, e.Answers[0])
			case 'H':
				addPool(p, 3, e.Answers[0])
			case 'C':
				words = append(words, e.Word)
				wordPlace[e.Word] = p
			}
		}
	}
	pick := func(p *Place, kind int, right string, avoid []string, key string) []string {
		out := []string{right}
		if len(pool) == 0 {
			return out
		}
		h := int(hash(key, 0) % uint64(len(pool)))
		for pass := 0; pass < 3 && len(out) < 4; pass++ {
			for i := 0; i < len(pool) && len(out) < 4; i++ {
				x := pool[(h+i)%len(pool)]
				ok := [3]bool{x.place == p && x.kind == kind, x.series == p.Series && x.kind == kind, x.kind == kind}[pass]
				t := x.s
				if kind < 2 {
					t = matchCase(t, right)
				}
				if ok && !containsFold(out, t) && !containsFold(avoid, t) {
					out = append(out, t)
				}
			}
		}
		return shuffled(out, key)
	}
	for _, p := range c.Places {
		for _, e := range p.Ex {
			switch e.Kind {
			case 'F':
				gaps := strings.Count(e.Question, "___")
				e.Tiles = make([][]string, gaps)
				for g := 0; g < gaps; g++ {
					right := strings.TrimSpace(e.Answers[g%len(e.Answers)])
					e.Tiles[g] = pick(p, b2i(gapIsSuffix(e.Question, g)), right, nil, fmt.Sprintf("%s/%d", e.Slug, g))
				}
			case 'Q':
				e.Tiles = [][]string{pick(p, 2, e.Answers[0], e.Alts, e.Slug)}
			case 'H':
				e.Tiles = [][]string{pick(p, 3, e.Answers[0], e.Alts, e.Slug)}
			}
		}
	}
	// word cards: three other words, the same place first, then the course
	for i, w := range words {
		h := int(hash(w.Root, 0) % uint64(len(words)))
		for pass := 0; pass < 2 && len(w.Others) < 3; pass++ {
			for k := 0; k < len(words) && len(w.Others) < 3; k++ {
				j := (h + k) % len(words)
				o := words[j]
				if j == i || pass == 0 && wordPlace[o] != wordPlace[w] || o.Defs["en"] == w.Defs["en"] || o.Root == w.Root {
					continue
				}
				dup := false
				for _, x := range w.Others {
					dup = dup || x == o
				}
				if !dup {
					w.Others = append(w.Others, o)
				}
			}
		}
	}
}

// meaningLangs: the site's interface languages the meanings are in whose
// script the game's font draws (Latin, Greek, Cyrillic) and that aren't
// mostly copies of the English; English first. A visitor in another
// language gets English meanings.
func meaningLangs(items []*model.ContentItem) []string {
	count := map[string]int{}
	same := map[string]int{}
	bad := map[string]bool{}
	for _, it := range items {
		if it.Type != "vocab" {
			continue
		}
		d := defsOf(it)
		for k, v := range d {
			count[k]++
			if v == d["en"] {
				same[k]++
			}
			for _, r := range v {
				if unicode.IsLetter(r) && !unicode.In(r, unicode.Latin, unicode.Greek, unicode.Cyrillic) {
					bad[k] = true
					break
				}
			}
		}
	}
	var out []string
	for k, n := range count {
		if k != "en" && locale.Supported(k) && !bad[k] && same[k]*2 <= n && n >= count["en"]/2 {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return append([]string{"en"}, out...)
}
