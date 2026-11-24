// Package gamedata makes the game's data (game/, a GameViber project) from
// the course's own content: the Zagreb-method lessons, their fill-in
// exercises, sentences to build from each lesson's text, and the lesson's
// words with their definitions — written as a GML script of global arrays,
// since GML has no JSON reader. Nothing is retyped: every string comes from
// the site's seed data, read through the site's own model and eo code.
package gamedata

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// seedItem is a seed file's row (the restructured files use the Go field names).
type seedItem struct {
	Slug, Type, SeriesSlug, SeriesParent, Status, Source string
	SeriesOrder                                          int
	Content                                              map[string]any
	Tags                                                 []string
}

// ReadSeed reads a seed file into the site's content items.
func ReadSeed(path string) ([]*model.ContentItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []seedItem
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make([]*model.ContentItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, &model.ContentItem{Slug: r.Slug, Type: r.Type, Content: r.Content, Tags: r.Tags, Status: r.Status, Source: r.Source,
			SeriesSlug: r.SeriesSlug, SeriesOrder: r.SeriesOrder, SeriesParent: r.SeriesParent})
	}
	return out, nil
}

// ReadLocale reads one of the site's interface-string files.
func ReadLocale(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	return m, json.Unmarshal(b, &m)
}

// Course is what the game gets.
type Course struct {
	Name    string // the readings' source: the method the lessons come from
	Lessons []*Lesson
	Words   []*Word  // every lesson's words, in lesson order, each once
	Langs   []string // definition languages, "en" first
}

type Lesson struct {
	Slug, Title, Text string
	Fill              []*Fill
	Build             []*Build
	Words             []int // indexes into Course.Words: first met in this lesson
}

// Fill is one of the site's fill-in exercises, as the site checks it.
type Fill struct {
	Slug, Question string
	Gaps           int      // "___" in the question
	Answers        []string // GapAnswers(): one per gap, or one for every gap
	Alts           []string // Alternatives()
	Tiles          [][]string
}

// Build is a sentence of the lesson's text, its words to put in order.
type Build struct {
	Sentence string
	Tiles    []string // the words, shuffled
}

type Word struct {
	Base, Root string            // the form met in the text (eo.ExtractWords); the vocabulary's root
	Defs       map[string]string // language → definition
	Lesson     int
	Options    []int // four words whose meanings the card offers (itself among them)
}

// Counts are the fill-ins, sentences and word cards.
func (c *Course) Counts() (f, b, v int) {
	for _, l := range c.Lessons {
		f += len(l.Fill)
		b += len(l.Build)
		v += len(l.Words)
	}
	return
}

// MakeCourse makes the course from the seed items: the readings of the
// zagr-metodo series in order, each with its child series of fill-ins.
func MakeCourse(items []*model.ContentItem) (*Course, error) {
	var readings []*model.ContentItem
	children := map[string][]*model.ContentItem{}
	vocab := map[string]*model.ContentItem{} // the zagr-vok roots, by word
	for _, it := range items {
		if it.Status != "approved" {
			continue
		}
		switch {
		case it.SeriesSlug == "zagr-metodo" && it.Type == "reading":
			readings = append(readings, it)
		case it.SeriesParent != "" && it.Type == "fillin":
			children[it.SeriesParent] = append(children[it.SeriesParent], it)
		case it.SeriesSlug == "zagr-vok" && it.Type == "vocab":
			vocab[strings.ToLower(it.Word())] = it
		}
	}
	if len(readings) == 0 {
		return nil, fmt.Errorf("no readings in the zagr-metodo series")
	}
	sort.SliceStable(readings, func(i, j int) bool { return readings[i].SeriesOrder < readings[j].SeriesOrder })
	c := &Course{Name: readings[0].Source}
	langs := map[string]int{}
	seenWord := map[string]bool{}
	for li, r := range readings {
		l := &Lesson{Slug: r.Slug, Title: r.Title(), Text: r.Text()}
		kids := children[r.Slug]
		sort.SliceStable(kids, func(i, j int) bool { return kids[i].SeriesOrder < kids[j].SeriesOrder })
		for _, it := range kids {
			f := &Fill{Slug: it.Slug, Question: it.Question(), Gaps: strings.Count(it.Question(), "___"), Answers: it.GapAnswers(), Alts: it.Alternatives()}
			if f.Gaps == 0 || len(f.Answers) == 0 {
				log.Printf("skip %s: no gap or no answer", it.Slug)
				continue
			}
			l.Fill = append(l.Fill, f)
		}
		for i, s := range sentences(l.Text) {
			if i == 2 {
				break
			}
			l.Build = append(l.Build, &Build{Sentence: strings.Join(s, " "), Tiles: shuffled(s, r.Slug)})
		}
		// the plain forms first (patro before patrino), so a root shows as
		// the word it makes when the text has that word
		bases := eo.ExtractWords(l.Text)
		rank := func(base string) int { // 0 root + ending, 1 the root alone, 2 with affixes
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
		for _, base := range bases {
			root, v := findRoot(base, vocab)
			if v == nil || seenWord[root] || definitions(v)["en"] == "" { // a card needs a definition to show
				continue
			}
			seenWord[root] = true
			w := &Word{Base: base, Root: v.Word(), Defs: definitions(v), Lesson: li}
			for k := range w.Defs {
				langs[k]++
			}
			l.Words = append(l.Words, len(c.Words))
			c.Words = append(c.Words, w)
		}
		c.Lessons = append(c.Lessons, l)
	}
	fillTiles(c.Lessons)
	wordOptions(c)
	// The languages every word has a definition in that the game's font can
	// draw (Latin, Greek, Cyrillic), English first.
	for k, n := range langs {
		if n == len(c.Words) && k != "en" && SiteLangs[k] && drawable(c.Words, k) && !placeholder(c.Words, k) {
			c.Langs = append(c.Langs, k)
		}
	}
	sort.Strings(c.Langs)
	c.Langs = append([]string{"en"}, c.Langs...)
	for _, w := range c.Words {
		if w.Defs["en"] == "" {
			return nil, fmt.Errorf("word %s has no English definition", w.Root)
		}
	}
	return c, nil
}

// fillTiles gives each gap four answers to choose from: its own and three
// other gap answers (derived from the data, never made up) — of the same
// kind (an ending glued to a word, or a whole word), from the same lesson
// first, then the other lessons; capitalised as the right answer is; in a
// fixed order per exercise.
func fillTiles(ls []*Lesson) {
	type cand struct {
		s      string
		lesson int
		suffix bool
	}
	var pool []cand
	seen := map[string]bool{}
	for li, l := range ls {
		for _, f := range l.Fill {
			for g := 0; g < f.Gaps; g++ {
				a := strings.TrimSpace(f.Answers[g%len(f.Answers)])
				k := fmt.Sprintf("%d/%s", li, strings.ToLower(a))
				if !seen[k] {
					seen[k] = true
					pool = append(pool, cand{a, li, gapIsSuffix(f.Question, g)})
				}
			}
		}
	}
	for li, l := range ls {
		for _, f := range l.Fill {
			f.Tiles = make([][]string, f.Gaps)
			for g := 0; g < f.Gaps; g++ {
				right := strings.TrimSpace(f.Answers[g%len(f.Answers)])
				suffix := gapIsSuffix(f.Question, g)
				tiles := []string{right}
				h := int(hash(f.Slug, g) % uint64(len(pool)))
				for pass := 0; pass < 3 && len(tiles) < 4; pass++ {
					for i := 0; i < len(pool) && len(tiles) < 4; i++ {
						c := pool[(h+i)%len(pool)]
						ok := [3]bool{c.lesson == li && c.suffix == suffix, c.suffix == suffix, true}[pass]
						t := matchCase(c.s, right)
						if ok && !containsFold(tiles, t) && !containsFold(f.Alts, t) {
							tiles = append(tiles, t)
						}
					}
				}
				f.Tiles[g] = shuffled(tiles, fmt.Sprintf("%s/%d", f.Slug, g))
			}
		}
	}
}

// Shown is how a word card names the word: the form met in the text when it
// is the root with an ending (amik → amiko), else the root itself (patr- —
// "father" — for patrino, so the meaning shown is the root's), with the
// text's form beside it (seen).
func (w *Word) Shown() (show, seen string) {
	if w.Base == w.Root {
		if strings.ContainsAny(w.Root[len(w.Root)-1:], "aeiou") || strings.HasSuffix(w.Root, "ŭ") {
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

// wordOptions gives each word card four meanings to choose from: its own and
// three other words', from its lesson first, then the lessons around it.
func wordOptions(c *Course) {
	for i, w := range c.Words {
		opts := []int{i}
		h := int(hash(w.Root, 0) % uint64(len(c.Words)))
		for pass := 0; pass < 2 && len(opts) < 4; pass++ {
			for k := 0; k < len(c.Words) && len(opts) < 4; k++ {
				j := (h + k) % len(c.Words)
				o := c.Words[j]
				if pass == 0 && o.Lesson != w.Lesson || j == i || o.Defs["en"] == w.Defs["en"] {
					continue
				}
				dup := false
				for _, x := range opts {
					dup = dup || x == j
				}
				if !dup {
					opts = append(opts, j)
				}
			}
		}
		key := make([]string, len(opts))
		for k, o := range opts {
			key[k] = fmt.Sprint(o)
		}
		for _, k := range shuffled(key, w.Root) {
			var n int
			fmt.Sscan(k, &n)
			w.Options = append(w.Options, n)
		}
	}
}

// token is a piece of a fill-in's question as the game lays it out: a word
// (with its punctuation) or a gap; space: a space comes before it.
type token struct {
	s     string
	gap   int // -1: text; else the gap's number
	space bool
}

// tokens splits a question into words and gaps, keeping a gap glued to the
// letters around it (lernant___kaj: "lernant", gap, "kaj", no spaces).
func tokens(q string) []token {
	var out []token
	gap := 0
	for wi, word := range strings.Fields(q) {
		first := true // the word's first piece takes the space before it
		add := func(t token) {
			t.space = first && wi > 0
			first = false
			out = append(out, t)
		}
		for pi, p := range strings.Split(word, "___") {
			if pi > 0 {
				add(token{"", gap, false})
				gap++
			}
			if p != "" {
				add(token{p, -1, false})
			}
		}
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// gapIsSuffix: the g-th gap follows a letter (lernant___): an ending.
func gapIsSuffix(q string, g int) bool {
	parts := strings.Split(q, "___")
	if g >= len(parts) {
		return false
	}
	rs := []rune(parts[g])
	return len(rs) > 0 && unicode.IsLetter(rs[len(rs)-1])
}

// matchCase writes s with its first letter as like's is.
func matchCase(s, like string) string {
	rs, lr := []rune(s), []rune(like)
	if len(rs) == 0 || len(lr) == 0 {
		return s
	}
	if unicode.IsUpper(lr[0]) {
		rs[0] = unicode.ToUpper(rs[0])
	} else {
		rs[0] = unicode.ToLower(rs[0])
	}
	return string(rs)
}

// sentences are the lesson text's plain sentences of 3 to 7 words (no
// quotes, dashes or digits), as word lists without the final stop.
func sentences(text string) [][]string {
	var out [][]string
	start := 0
	rs := []rune(text)
	for i, r := range rs {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		s := strings.TrimSpace(string(rs[start:i]))
		start = i + 1
		if r != '.' || strings.ContainsAny(s, "\"'„“”«»—–-:;()0123456789\n") {
			continue
		}
		ws := strings.Fields(strings.ReplaceAll(s, ",", ""))
		if len(ws) >= 3 && len(ws) <= 7 {
			out = append(out, ws)
		}
	}
	return out
}

// shuffled is a fixed shuffle of xs (by key), never the order given when
// another order exists.
func shuffled(xs []string, key string) []string {
	out := append([]string(nil), xs...)
	h := hash(key, 0)
	for i := len(out) - 1; i > 0; i-- {
		h = h*6364136223846793005 + 1442695040888963407
		j := int((h >> 33) % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	if strings.Join(out, "\x00") == strings.Join(xs, "\x00") && len(out) > 1 {
		for i := range out { // rotate by one
			out[i] = xs[(i+1)%len(xs)]
		}
	}
	return out
}

func hash(s string, n int) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s#%d", s, n)
	return h.Sum64()
}

func containsFold(xs []string, s string) bool {
	for _, x := range xs {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

// findRoot finds the vocabulary entry for a base form from eo.ExtractWords:
// the word itself, or its root without the ending, or with common suffixes
// taken off (instruistino → instru). DERIVED: the course links no words to
// lessons, so this is the game's own matching, not the site's.
func findRoot(base string, vocab map[string]*model.ContentItem) (string, *model.ContentItem) {
	if v := vocab[base]; v != nil {
		return base, v
	}
	stem := base
	if n := len(stem); n > 1 && strings.ContainsRune("oaie", rune(stem[n-1])) {
		stem = stem[:n-1]
	}
	for {
		if v := vocab[stem]; v != nil && len([]rune(stem)) >= 2 {
			return stem, v
		}
		cut := false
		for _, suf := range []string{"in", "ist", "ej", "ul", "ar", "et", "eg", "il", "an", "ec", "aĵ", "ad", "ig", "iĝ", "ebl", "ind", "em", "er", "estr", "id"} {
			if strings.HasSuffix(stem, suf) && len([]rune(stem))-len([]rune(suf)) >= 2 {
				stem = strings.TrimSuffix(stem, suf)
				cut = true
				break
			}
		}
		if !cut {
			return "", nil
		}
	}
}

func definitions(v *model.ContentItem) map[string]string {
	out := map[string]string{}
	if m, ok := v.Content["definitions"].(map[string]any); ok {
		for k, d := range m {
			if s, ok := d.(string); ok && strings.TrimSpace(s) != "" {
				out[k] = strings.TrimSpace(s)
			}
		}
	}
	return out
}

// SiteLangs are the site's interface languages (internal/locale) (the definitions offered are among them).
var SiteLangs = map[string]bool{}

// placeholder: a language whose definitions are mostly the English ones
// copied (the source fills gaps that way), so not really translated.
func placeholder(ws []*Word, lang string) bool {
	same := 0
	for _, w := range ws {
		if w.Defs[lang] == w.Defs["en"] {
			same++
		}
	}
	return same*2 > len(ws)
}

func drawable(ws []*Word, lang string) bool {
	for _, w := range ws {
		for _, r := range w.Defs[lang] {
			if unicode.IsLetter(r) && !unicode.In(r, unicode.Latin, unicode.Greek, unicode.Cyrillic) {
				return false
			}
		}
	}
	return true
}

// gmlStr writes s as a GML string expression: GameMaker 5's strings have no
// escapes, so a double quote is spliced in from a single-quoted one; '#' is
// GML's line break, so a real one is written \#, and a newline becomes '#'.
func gmlStr(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "#", `\#`)
	s = strings.ReplaceAll(s, "\n", "#")
	parts := strings.Split(s, `"`)
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, `+'"'+`)
}

// uiKeys are the site's own interface strings the game shows (eo.json).
var uiKeys = []string{"exercise.choose_answer", "exercise.what_means", "exercise.dont_know", "exercise.send",
	"home.start_cta", "home.continue_cta", "exercise.series_next_fallback", "exercise.series_prev_fallback",
	"exercise.answer_placeholder", "exercise.vocab_mode_type"}

// GML is the data script.
func GML(c *Course, ui map[string]string) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...); b.WriteByte('\n') }
	b.WriteString("// Code generated by cmd/export-game from the site's seed data; DO NOT EDIT.\n")
	for _, k := range uiKeys {
		if ui[k] == "" {
			log.Fatalf("eo.json has no %s", k)
		}
		w("global.t_%s = %s", k[strings.LastIndex(k, ".")+1:], gmlStr(ui[k])) // exercise.send → global.t_send
	}
	w("global.course_name = %s", gmlStr(c.Name))
	w("global.nlang = %d", len(c.Langs))
	for i, l := range c.Langs {
		w("global.lang[%d] = %s", i, gmlStr(l))
	}
	w("global.nwords = %d", len(c.Words))
	for i, x := range c.Words {
		show, seen := x.Shown()
		w("global.w_show[%d] = %s; global.w_seen[%d] = %s; global.w_root[%d] = %s; global.w_lesson[%d] = %d", i, gmlStr(show), i, gmlStr(seen), i, gmlStr(x.Root), i, x.Lesson)
		for k, o := range x.Options {
			w("global.w_opt[%d,%d] = %d", i, k, o)
		}
		for k, l := range c.Langs {
			w("global.w_def[%d,%d] = %s", i, k, gmlStr(x.Defs[l]))
		}
	}
	w("global.nl = %d", len(c.Lessons))
	for i, l := range c.Lessons {
		w("global.l_slug[%d] = %s; global.l_title[%d] = %s; global.l_text[%d] = %s", i, gmlStr(l.Slug), i, gmlStr(l.Title), i, gmlStr(l.Text))
		w("global.l_nf[%d] = %d; global.l_nb[%d] = %d; global.l_nw[%d] = %d", i, len(l.Fill), i, len(l.Build), i, len(l.Words))
		for k, f := range l.Fill {
			w("global.f_slug[%d,%d] = %s; global.f_q[%d,%d] = %s; global.f_ngap[%d,%d] = %d; global.f_nans[%d,%d] = %d; global.f_nalt[%d,%d] = %d",
				i, k, gmlStr(f.Slug), i, k, gmlStr(f.Question), i, k, f.Gaps, i, k, len(f.Answers), i, k, len(f.Alts))
			e := i*100 + k // one exercise's lists, by lesson*100 + exercise
			for a, s := range f.Answers {
				w("global.f_ans[%d,%d] = %s", e, a, gmlStr(s))
			}
			for a, s := range f.Alts {
				w("global.f_alt[%d,%d] = %s", e, a, gmlStr(s))
			}
			for t, tok := range tokens(f.Question) {
				w("global.f_tok[%d,%d] = %s; global.f_tgap[%d,%d] = %d; global.f_tsp[%d,%d] = %d", e, t, gmlStr(tok.s), e, t, tok.gap, e, t, b2i(tok.space))
			}
			w("global.f_ntok[%d,%d] = %d", i, k, len(tokens(f.Question)))
			for g, ts := range f.Tiles {
				w("global.f_nt[%d,%d] = %d", e, g, len(ts))
				for t, s := range ts {
					w("global.f_tile[%d,%d] = %s", e, g*10+t, gmlStr(s))
				}
			}
		}
		for k, x := range l.Build {
			e := i*100 + k
			w("global.b_sent[%d,%d] = %s; global.b_nt[%d,%d] = %d", i, k, gmlStr(x.Sentence), i, k, len(x.Tiles))
			for t, s := range x.Tiles {
				w("global.b_tile[%d,%d] = %s", e, t, gmlStr(s))
			}
		}
		for k, wi := range l.Words {
			w("global.l_word[%d,%d] = %d", i, k, wi)
		}
	}
	return b.String()
}
