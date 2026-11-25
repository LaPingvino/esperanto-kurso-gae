// Package legvortoj is the words of a reading ("legvortoj"): the words a
// learner meets in a text, found in the text itself at render time, each
// in its dictionary form, linked to the one card the course has for it.
//
// It replaces the voc-auto-* cards made one per word per text (Joop,
// 2026-11-25: "the idea was good, the execution created a mess"). The idea
// stays: words learnt from the texts you read. What changed:
//
//   - one card per word: a word links to the core card when the course has
//     one (the word, or its root with one ending: amiko → amik), else to
//     one shared card (voc-auto-<word>), else to none; never one per text,
//     never one per inflected form ("boaj" is boao, not a word "boa");
//   - Esperanto first: a word comes with its sentence, its parts (mal|san|
//     ul|ej|o, each root linked to its card), the Esperanto definition of
//     the sense the text means and a picture where one exists; the
//     translation is one aid, revealed on request;
//   - the sense: a word of several senses (boao: the snake, the fur scarf)
//     gets the one the text means — reviewed picks in sencoj.json, else a
//     proposal from its sentence;
//   - its level from the word (its official standing in ReVo: Fundamento,
//     Official Additions), not from the text it was first seen in.
package legvortoj

import (
	_ "embed"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

// SharedPrefix is the slug prefix of the shared cards for words the core
// vocabulary lacks (the old per-text cards kept this prefix).
const SharedPrefix = "voc-auto-"

// IsShared reports whether an item is a shared reading-word card.
func IsShared(it *model.ContentItem) bool {
	return it != nil && it.Type == "vocab" && strings.HasPrefix(it.Slug, SharedPrefix)
}

// SharedSlug is the shared card's slug for a word.
func SharedSlug(lemma string) string { return SharedPrefix + eo.WordToSlug(lemma) }

// ---- word parts ----

// Part is one piece of a word: a root, an affix, the ending, or the
// linking o of a compound.
type Part struct {
	Text string
	Kind string // "radiko", "prefikso", "sufikso", "finaĵo", "ligilo"
}

var prefixes = set("mal", "ge", "re", "dis", "ek", "eks", "bo", "fi", "pra", "mis", "vic")

var suffixes = set("aĉ", "ad", "aĵ", "an", "ar", "ebl", "ec", "eg", "ej", "em", "end", "er", "estr",
	"et", "id", "ig", "iĝ", "il", "in", "ind", "ing", "ism", "ist", "obl", "on", "op", "uj", "ul", "um",
	"ant", "int", "ont", "at", "it", "ot", "ĉj", "nj")

// A word ends in one of these (the dictionary form, or an inflected one).
var endings = set("o", "a", "e", "i", "oj", "aj", "on", "an", "ojn", "ajn", "en", "as", "is", "os", "us", "u")

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

type seg struct {
	cost  int
	parts []Part
	ok    bool
}

// split segments s into known pieces with the fewest pieces (a root of two
// letters counts double: they are easily found by accident). tail: s must
// end in an ending; else s is the part before a known root (no ending, a
// linking o may close it). afterRoot: s follows a root, so it may start
// with a suffix or the ending.
func split(s string, tail, afterRoot bool) ([]Part, bool) {
	r := []rune(s)
	n := len(r)
	best := make([]seg, n+1)
	best[0] = seg{ok: true}
	for i := 0; i < n; i++ {
		if !best[i].ok {
			continue
		}
		prevRoot := afterRoot
		if k := len(best[i].parts); k > 0 {
			prevRoot = best[i].parts[k-1].Kind == "radiko" || best[i].parts[k-1].Kind == "sufikso"
		}
		for j := i + 1; j <= n && j-i <= 12; j++ {
			p := string(r[i:j])
			var kind string
			cost := 1
			switch {
			case tail && j == n && endings[p] && prevRoot:
				kind = "finaĵo"
			case p == "o" && prevRoot && (j < n || !tail):
				kind = "ligilo"
			case suffixes[p] && prevRoot:
				kind = "sufikso"
			case prefixes[p] && i == 0 && !afterRoot:
				kind = "prefikso"
			case j-i >= 2 && revo.IsRoot(p):
				kind = "radiko"
				if j-i == 2 {
					cost = 2
				}
			default:
				continue
			}
			c := best[i].cost + cost
			if !best[j].ok || c < best[j].cost {
				parts := append(append([]Part{}, best[i].parts...), Part{p, kind})
				best[j] = seg{cost: c, parts: parts, ok: true}
			}
		}
	}
	if !best[n].ok {
		return nil, false
	}
	return best[n].parts, true
}

func hasRoot(ps []Part) bool {
	for _, p := range ps {
		if p.Kind == "radiko" {
			return true
		}
	}
	return false
}

// Parts breaks a word into its pieces: mal|san|ul|ej|o. A word in ReVo is
// split around the root its marker names (san.mal0ulejo: the root san,
// mal before it, ulejo after); another word only when it splits wholly
// into known roots and affixes with at least one root (abon|pag|o). Nil
// when it can't be split, or is one piece and an ending.
func Parts(lemma string) []Part {
	w := revo.Normalize(lemma)
	if w == "" || strings.ContainsAny(w, " -") {
		return nil
	}
	if h := revo.Exact(w); h != nil {
		if root, before, after, ok := revo.MrkRoot(h.Mrk); ok && before+root+after == w && root != "" {
			var out []Part
			if before != "" {
				ps, ok := split(before, false, false)
				if !ok {
					ps = []Part{{before, "radiko"}}
				}
				out = append(out, ps...)
			}
			out = append(out, Part{root, "radiko"})
			if after != "" {
				ps, ok := split(after, true, true)
				if !ok {
					return nil
				}
				out = append(out, ps...)
			}
			if len(out) < 2 {
				return nil
			}
			return out
		}
	}
	ps, ok := split(w, true, false)
	if !ok || !hasRoot(ps) || len(ps) < 2 {
		return nil
	}
	return ps
}

// ---- dictionary forms ----

// classes: an inflectional ending, its word class's dictionary ending.
var classes = []struct{ end, base string }{
	{"ojn", "o"}, {"ajn", "a"}, {"oj", "o"}, {"aj", "a"}, {"on", "o"}, {"an", "a"}, {"en", "e"},
	{"as", "i"}, {"is", "i"}, {"os", "i"}, {"us", "i"}, {"u", "i"},
	{"o", "o"}, {"a", "a"}, {"e", "e"}, {"i", "i"},
}

// Lemma is a text word's dictionary form and its ReVo entry. via: the
// form is of a related ReVo word (aĉetita → aĉeti, bele → bela). ok false: no real Esperanto word behind it — not in ReVo, not
// an inflection of a ReVo word, and not splittable into known roots and
// affixes (abon|pag|o is; "boa" as made from "boaj" is not a noun).
func Lemma(token string) (lemma string, hit *revo.Hit, via bool, ok bool) {
	w := revo.Normalize(token)
	if len([]rune(w)) < 3 || strings.ContainsFunc(w, func(r rune) bool { return !isLetter(r) }) {
		return "", nil, false, false
	}
	// a plural or accusative whose singular is a word: the singular, even
	// where ReVo lists the plural too (homoj, bestoj, kelkaj)
	for _, c := range classes[:6] {
		if stem, cut := strings.CutSuffix(w, c.end); cut && len([]rune(stem)) >= 2 {
			if h := revo.Exact(stem + c.base); h != nil {
				return stem + c.base, h, false, true
			}
		}
	}
	if h := revo.Exact(w); h != nil {
		return w, h, false, true
	}
	for _, c := range classes {
		stem, cut := strings.CutSuffix(w, c.end)
		if !cut || len([]rune(stem)) < 2 {
			continue
		}
		base := stem + c.base
		if h := revo.Exact(base); h != nil {
			return base, h, false, true // an inflected form
		}
		if c.base == "a" || c.base == "e" {
			// an adjectival or adverbial participle: its verb
			for _, p := range []string{"ant", "int", "ont", "at", "it", "ot"} {
				if v, cut := strings.CutSuffix(stem, p); cut && len([]rune(v)) >= 2 {
					if h := revo.Exact(v + "i"); h != nil {
						return v + "i", h, true, true
					}
				}
			}
		}
		if c.base == "e" {
			if h := revo.Exact(stem + "a"); h != nil { // an adverb: its adjective
				return stem + "a", h, true, true
			}
		}
		// a derived word or compound ReVo lacks: itself, if it splits
		if Parts(base) != nil {
			return base, nil, false, true
		}
		return "", nil, false, false
	}
	return "", nil, false, false
}

// ---- the course's cards ----

// Index finds the card for a word among the course's vocab cards.
type Index struct {
	word   map[string]*model.ContentItem // a core card by its word
	root   map[string]*model.ContentItem // a core root/affix card by its root
	shared map[string]*model.ContentItem // a shared card by its word
}

func coreRank(slug string) int {
	switch {
	case strings.HasPrefix(slug, "bildo-"):
		return 0
	case strings.HasPrefix(slug, "vortaro-"):
		return 1
	case strings.HasPrefix(slug, "voc-zagr-"):
		return 2
	case strings.HasPrefix(slug, "voc-"):
		return 3
	}
	return 4
}

func rootCard(it *model.ContentItem) bool {
	for _, t := range it.Tags {
		if t == "radiko" || t == "sufikso" || t == "prefikso" {
			return true
		}
	}
	return false
}

// NewIndex indexes the approved vocab cards among items.
func NewIndex(items []*model.ContentItem) *Index {
	x := &Index{word: map[string]*model.ContentItem{}, root: map[string]*model.ContentItem{}, shared: map[string]*model.ContentItem{}}
	better := func(m map[string]*model.ContentItem, k string, it *model.ContentItem, rank func(string) int) {
		if old := m[k]; old == nil || rank(it.Slug) < rank(old.Slug) || (rank(it.Slug) == rank(old.Slug) && it.Slug < old.Slug) {
			m[k] = it
		}
	}
	for _, it := range items {
		if it.Type != "vocab" || (it.Status != "" && it.Status != "approved") {
			continue
		}
		w := revo.Normalize(it.Word())
		if w == "" {
			continue
		}
		switch {
		case IsShared(it):
			if it.Slug == SharedSlug(w) {
				x.shared[w] = it
			}
		case rootCard(it):
			better(x.root, strings.Trim(w, "-"), it, func(s string) int {
				if strings.HasPrefix(s, "voc-zagr-") {
					return 0
				}
				return coreRank(s)
			})
		default:
			better(x.word, w, it, coreRank)
		}
	}
	return x
}

// Card is the card for a word: a core card (core) or the shared one, or
// nil.
func (x *Index) Card(lemma string) (it *model.ContentItem, core bool) {
	if x == nil {
		return nil, false
	}
	if it := x.word[lemma]; it != nil {
		return it, true
	}
	// a root card and one ending: amiko → amik, ejo → ej
	for _, e := range []string{"o", "a", "e", "i"} {
		if r, ok := strings.CutSuffix(lemma, e); ok && len([]rune(r)) >= 2 {
			if it := x.root[r]; it != nil {
				return it, true
			}
		}
	}
	if it := x.shared[lemma]; it != nil {
		return it, false
	}
	return nil, false
}

// RootCard is the core card of a root or affix, or nil.
func (x *Index) RootCard(root string) *model.ContentItem {
	if x == nil {
		return nil
	}
	return x.root[root]
}

// ---- level ----

// ofcRating is the starting rating of a word by its official standing:
// the Fundamento B1, the first three Official Additions B2, later ones
// and unofficial words C1. (Officiality is not frequency, but the
// Fundamento's roots are the common ones; the core vocabulary covers the
// A levels.)
func ofcRating(ofc string) float64 {
	switch ofc {
	case "*":
		return 1500
	case "1", "2", "3":
		return 1700
	}
	return 1900
}

// Rating is a shared card's starting rating: the word's own mark, else
// its roots' (a derived word one band later; a compound the hardest of
// its roots').
func Rating(lemma string, hit *revo.Hit) float64 {
	if hit != nil && hit.Ofc != "" && hit.HasHead(lemma) {
		return ofcRating(hit.Ofc)
	}
	r := 0.0
	for _, p := range Parts(lemma) {
		if p.Kind == "radiko" {
			if v := ofcRating(revo.RootOfc(p.Text)); v > r {
				r = v
			}
		}
	}
	if r == 0 {
		return 1900
	}
	return min(r+200, 1900)
}

// ---- senses ----

// Pick is the sense a text means by a word of several senses.
type Pick struct {
	Sense    string   `json:"senco"`                // ReVo's sense marker
	Checked  bool     `json:"kontrolita,omitempty"` // reviewed; else a proposal
	Sentence string   `json:"frazo,omitempty"`      // the word's sentence, for the review
	Choices  []string `json:"elektoj,omitempty"`    // the senses there were: "marker: definition"
}

//go:embed sencoj.json
var picksJSON []byte

var (
	picksOnce sync.Once
	picks     map[string]map[string]Pick // reading slug → word → pick
)

// Picks are the recorded sense picks (sencoj.json): per reading, per word.
func Picks() map[string]map[string]Pick {
	picksOnce.Do(func() {
		picks = map[string]map[string]Pick{}
		if err := json.Unmarshal(picksJSON, &picks); err != nil {
			log.Printf("legvortoj: sencoj.json: %v", err)
		}
	})
	return picks
}

// stems are a text's content-word stems (endings off, four letters or
// more), for comparing a sentence with a definition.
func stems(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range tokens(s) {
		w := strings.ToLower(t)
		if eo.IsStopword(w) {
			continue
		}
		st := strings.TrimRight(w, "jn")
		st = strings.TrimRight(st, "oaeiu")
		if len([]rune(st)) >= 4 {
			out[st] = true
		}
	}
	return out
}

// ProposeSense picks the sense of a word of several senses that its
// sentence means: ReVo's main sense (Entry.Main: its first that is not a
// styled use, FIG, ARK…, nor the taxon), unless another sense's
// definition shares at least two more words with the sentence. It also
// says whether the pick is open to doubt (another sense came close or won).
func ProposeSense(hit *revo.Hit, sentence, lemma string) (mrk string, doubt bool) {
	if hit == nil || len(hit.Senses) < 2 {
		return "", false
	}
	ctx := stems(sentence)
	for st := range stems(lemma) {
		delete(ctx, st)
	}
	overlap := func(s revo.Sense) int {
		n := 0
		for st := range stems(s.Def) {
			if ctx[st] {
				n++
			}
		}
		return n
	}
	main := hit.Main()
	base := overlap(hit.Senses[main])
	pick, best := main, base
	for i, s := range hit.Senses {
		if o := overlap(s); i != main && o >= base+2 && o > best {
			pick, best = i, o
		}
	}
	for i, s := range hit.Senses {
		if i != pick && overlap(s) > 0 && overlap(s) >= best {
			doubt = true
		}
	}
	return hit.Senses[pick].Mrk, doubt || pick != main
}

// ---- the words of a text ----

// Use is one word of a text.
type Use struct {
	Lemma    string    // dictionary form: boao
	Forms    []string  // as written in the text: boaon, boaj, boao
	Sentence string    // its first sentence in the text
	Hit      *revo.Hit // its ReVo entry (a related word's when Via)
	Via      bool      // ReVo has a related word, not this one
	Parts    []Part    // mal|san|ul|ej|o
	Sense    string    // the sense meant, for a word of several senses
	Checked  bool      // the sense pick is reviewed (sencoj.json)
	Doubt    bool      // an unreviewed pick open to doubt: listed for review
	Card     *model.ContentItem
	Core     bool // Card is a core card
}

// Ambiguous: the word has several senses in ReVo.
func (u *Use) Ambiguous() bool { return u.Hit != nil && len(u.Hit.Senses) > 1 }

// SenseEntry is the sense meant, or nil.
func (u *Use) SenseEntry() *revo.Sense {
	if u.Hit == nil {
		return nil
	}
	return u.Hit.Sense(u.Sense)
}

// EoDef is the Esperanto definition of the word in the sense meant.
func (u *Use) EoDef() string {
	if s := u.SenseEntry(); s != nil && s.Def != "" {
		return s.Def
	}
	if u.Hit != nil {
		return u.Hit.Def
	}
	return ""
}

func isLetter(r rune) bool { return unicode.IsLetter(r) }

// tokens are the letter runs of s (hyphens, arrows and digits split words).
func tokens(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return !isLetter(r) })
}

// Sentences splits a text into sentences (at . ! ? … and line breaks).
func Sentences(text string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			out = append(out, s)
		}
		b.Reset()
	}
	rs := []rune(text)
	for i, r := range rs {
		if r == '\n' {
			flush()
			continue
		}
		b.WriteRune(r)
		if r == '.' || r == '!' || r == '?' || r == '…' {
			if i+1 >= len(rs) || unicode.IsSpace(rs[i+1]) {
				flush()
			}
		}
	}
	flush()
	return out
}

// elided is the noun a word stands for when it is a noun's root with the
// plural or accusative but no o ("la boaj" in La eta princo, which has
// boaon and boaoj too: boao, not the adjective boa, "of the in-laws"),
// given the words the text has; else "".
func elided(w string, inText map[string]bool) string {
	for _, end := range []string{"jn", "j", "n"} {
		if r, cut := strings.CutSuffix(w, end); cut && revo.IsRoot(r) && inText[r+"o"] {
			return r + "o"
		}
	}
	return ""
}

// Words are a text's content words in the order they first appear, each
// once, in its dictionary form, with its card and sense. Function words,
// names (capitalised wherever they occur) and strings that are no
// Esperanto word are left out.
func Words(readingSlug, text string, x *Index) []*Use {
	capital, total := map[string]int{}, map[string]int{}
	for _, t := range tokens(text) {
		l := strings.ToLower(t)
		total[l]++
		if unicode.IsUpper([]rune(t)[0]) {
			capital[l]++
		}
	}
	recorded := Picks()[readingSlug]
	inText := map[string]bool{}
	for _, t := range tokens(text) {
		if l, _, _, ok := Lemma(strings.ToLower(t)); ok {
			inText[l] = true
		}
	}
	byLemma := map[string]*Use{}
	var out []*Use
	for _, sent := range Sentences(text) {
		for _, t := range tokens(sent) {
			l := strings.ToLower(t)
			if len([]rune(l)) < 3 || eo.IsStopword(l) || capital[l] == total[l] {
				continue
			}
			lemma, hit, via, ok := Lemma(l)
			if n := elided(l, inText); n != "" {
				lemma, hit, via, ok = n, revo.Exact(n), true, true
			}
			if !ok || eo.IsStopword(lemma) {
				continue
			}
			if u := byLemma[lemma]; u != nil {
				if !contains(u.Forms, l) {
					u.Forms = append(u.Forms, l)
				}
				continue
			}
			u := &Use{Lemma: lemma, Forms: []string{l}, Sentence: sent, Hit: hit, Via: via, Parts: Parts(lemma)}
			u.Card, u.Core = x.Card(lemma)
			if u.Ambiguous() {
				if p, ok := recorded[lemma]; ok && hit.Sense(p.Sense) != nil {
					// a recorded pick: reviewed, or still a proposal on the list
					u.Sense, u.Checked, u.Doubt = p.Sense, p.Checked, !p.Checked
				} else {
					u.Sense, u.Doubt = ProposeSense(hit, sent, lemma)
				}
			}
			byLemma[lemma] = u
			out = append(out, u)
		}
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// ---- the plan for the old cards ----

// Action is what becomes of one voc-auto-* card.
type Action struct {
	Slug   string   `json:"slug"`
	Word   string   `json:"word"`
	Kind   string   `json:"kind"`             // keep | core | lemma | junk | create
	Into   string   `json:"into,omitempty"`   // core/lemma: the card it folds into
	Lemma  string   `json:"lemma,omitempty"`  // its dictionary form
	Rating float64  `json:"rating,omitempty"` // keep/create: the starting rating
	Tags   []string `json:"tags,omitempty"`   // keep/create: vortaro, legvorto, its readings
	Note   string   `json:"note,omitempty"`
}

// Plan sorts the old per-text cards (every voc-auto-* vocab item) into a
// disjoint plan: keep as the shared card of its word (re-rated by the
// word, tagged with the readings it occurs in); fold into the core card
// (core); fold into the card of its dictionary form (lemma, creating the
// shared card when there is none: create); or retire as no word (junk:
// "ami→malami"). A card someone gave a meaning or wrote by hand folds only
// into a core card; else it is kept, even when its word is odd.
func Plan(items []*model.ContentItem) []Action {
	var cards, core []*model.ContentItem
	var readings []*model.ContentItem
	done := map[string]bool{} // shared cards a run already handled: they exist
	var retired []*model.ContentItem
	for _, it := range items {
		switch {
		case IsShared(it):
			// not yet folded or kept by an earlier run (whose undo comes first)
			switch {
			case it.Content[UndoKey] != nil:
				done[it.Slug] = true
				if into, _ := it.Content[MergedIntoKey].(string); it.Status == MergedStatus && into == "" {
					retired = append(retired, it) // retired by a run: a later rule may find its word
				}
			case it.Status == "" || it.Status == "approved":
				cards = append(cards, it)
			}
		case it.Type == "reading" && (it.Status == "" || it.Status == "approved"):
			readings = append(readings, it)
		case it.Type == "vocab":
			core = append(core, it)
		}
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Slug < cards[j].Slug }) // the same plan for the same items
	sort.Slice(readings, func(i, j int) bool { return readings[i].Slug < readings[j].Slug })
	x := NewIndex(core) // core cards only: which words the course already has
	inReading := map[string][]string{}
	lemmasOf := map[string]map[string]bool{}  // reading → its words
	oldBase := map[string]map[string]string{} // reading → the old extraction's form → the word
	for _, r := range readings {
		lemmasOf[r.Slug], oldBase[r.Slug] = map[string]bool{}, map[string]string{}
		for _, u := range Words(r.Slug, r.Text(), x) {
			inReading[u.Lemma] = append(inReading[u.Lemma], r.Slug)
			lemmasOf[r.Slug][u.Lemma] = true
			for _, f := range u.Forms {
				if b := eo.ToBaseForm(f); b != "" {
					oldBase[r.Slug][b] = u.Lemma
				}
			}
		}
	}
	// what the card's own texts make of its word: the word itself, the
	// word they really have ("boa" from "la boaj": boao), or nothing known
	fromTexts := func(c *model.ContentItem, w string) (string, bool) {
		known := false
		for _, t := range c.Tags {
			if lemmasOf[t] == nil {
				continue
			}
			known = true
			if lemmasOf[t][w] {
				return w, true
			}
		}
		for _, t := range c.Tags {
			if l := oldBase[t][w]; l != "" {
				return l, true
			}
		}
		return "", known
	}
	tagsFor := func(lemma string) []string {
		rs := append([]string{}, inReading[lemma]...)
		sort.Strings(rs)
		return append([]string{"vortaro", "legvorto"}, rs...)
	}
	// the words of a card's own texts (its reading tags): a stem is
	// resolved only to a word its text has — never from the dictionary
	// alone ("diam" could be an abbreviation of diametro)
	inOwnText := func(c *model.ContentItem, word string) bool {
		for _, t := range c.Tags {
			if lemmasOf[t][word] {
				return true
			}
		}
		return false
	}
	stemInText := func(c *model.ContentItem, stem string) bool {
		for _, t := range c.Tags {
			for l := range lemmasOf[t] {
				if strings.HasPrefix(l, stem) {
					return true
				}
			}
		}
		return false
	}
	bySlug := map[string]*model.ContentItem{}
	for _, c := range cards {
		bySlug[c.Slug] = c
	}
	var out []Action
	created := map[string]bool{}
	for _, c := range cards {
		w := revo.Normalize(c.Word())
		a := Action{Slug: c.Slug, Word: c.Word()}
		lemma, hit, _, ok := Lemma(w)
		if strings.ContainsAny(w, " -→") {
			ok = false // "ami→malami", "dek-dua": pieces of an example, not a word
		}
		isHead := revo.Exact(w) != nil
		l, known := fromTexts(c, w)
		// a stem without an ending the old extractor cut from a longer
		// form (bat from batanto, ven from venonta, kv from kvanto) — not
		// a word, though ReVo may list it as an interjection (bat!)
		bare := !strings.ContainsAny(w, " -→") && !strings.ContainsAny(string([]rune(w)[len([]rune(w))-1:]), "oaeiu") &&
			(!ok || (known && l != w && revo.IsRoot(w)))
		switch {
		case bare:
			// the word it was cut from when that is a word of its own, not
			// a participle (kvanto, diamanto: the old extractor took -anto
			// for one); else the root's base word — its verb, else noun,
			// else adjective — never a derived form; for two letters or
			// none: no word
			ok, lemma = false, ""
			if !stemInText(c, w) {
				a.Note = "review: no word of its text begins with " + w
				break
			}
			if cw := cutFrom(w, x, func(cw string) bool { return inOwnText(c, cw) }); cw != "" {
				lemma, hit, ok = cw, revo.Exact(cw), true
				a.Note = "cut from " + cw
				break
			}
			for _, e := range []string{"i", "o", "a"} {
				if len([]rune(w)) < 3 {
					break
				}
				if h := revo.Exact(w + e); h != nil {
					lemma, hit, ok = w+e, h, true
					a.Note = "the base word of the root " + w
					break
				}
			}
		case l != "" && l != lemma && ok && (!isHead || l == w+"o"):
			// its texts have another word: the one it was cut from (ele
			// from elen). A real word folds only into the noun it stood
			// for ("la boaj": boa → boao), never into another headword
			// (supre stays supre, though its text has supren).
			lemma, hit, ok = l, revo.Exact(l), true
			a.Note = "its text has " + l
		case known && l == "" && ok && lemma == w:
			a.Note = "not in its text any more"
		}
		precious := hasDefs(c) || c.AuthorID != ""
		if !ok {
			lemma = ""
		}
		a.Lemma = lemma
		target, isCore := x.Card(lemma)
		exactCore := x.word[w] != nil
		switch {
		case exactCore:
			a.Kind, a.Into = "core", x.word[w].Slug
		case ok && target != nil && isCore:
			a.Kind, a.Into = "core", target.Slug
		case precious:
			a.Kind, a.Rating, a.Tags, a.Note = "keep", Rating(w, revo.Exact(w)), tagsFor(w), "has a meaning or an author"
			if lemma != "" {
				a.Tags = tagsFor(lemma)
			}
		case !ok:
			a.Kind = "junk"
		case target != nil:
			a.Kind, a.Into = "lemma", target.Slug
		case lemma != w:
			a.Kind, a.Into = "lemma", SharedSlug(lemma)
			if bySlug[a.Into] == nil && !done[a.Into] && !created[lemma] {
				created[lemma] = true
				out = append(out, Action{Slug: a.Into, Word: lemma, Kind: "create", Lemma: lemma,
					Rating: Rating(lemma, hit), Tags: tagsFor(lemma), Note: "the dictionary form of " + c.Slug})
			}
		default:
			a.Kind, a.Rating, a.Tags = "keep", Rating(lemma, hit), tagsFor(lemma)
		}
		out = append(out, a)
	}
	// cards a run retired as no word whose word a later rule finds (kv,
	// cut from kvanto): fold them into it now (refold)
	sort.Slice(retired, func(i, j int) bool { return retired[i].Slug < retired[j].Slug })
	for _, c := range retired {
		w := revo.Normalize(c.Word())
		cw := cutFrom(w, x, func(cw string) bool { return inOwnText(c, cw) })
		if cw == "" {
			continue
		}
		a := Action{Slug: c.Slug, Word: c.Word(), Kind: "refold", Lemma: cw, Note: "cut from " + cw}
		if t, core := x.Card(cw); t != nil && core {
			a.Into = t.Slug
		} else {
			a.Into = SharedSlug(cw)
			if bySlug[a.Into] == nil && !done[a.Into] && !created[cw] {
				created[cw] = true
				out = append(out, Action{Slug: a.Into, Word: cw, Kind: "create", Lemma: cw,
					Rating: Rating(cw, revo.Exact(cw)), Tags: tagsFor(cw), Note: "the word " + c.Slug + " was cut from"})
			}
		}
		out = append(out, a)
	}
	// a lemma card that exists but would itself fold away: point at its target
	final := map[string]string{}
	for _, a := range out {
		if a.Kind == "core" || a.Kind == "lemma" || a.Kind == "junk" {
			final[a.Slug] = a.Into
		}
	}
	for i := range out {
		if out[i].Kind == "lemma" {
			for n := 0; n < 4; n++ {
				if next, ok := final[out[i].Into]; ok && next != "" {
					out[i].Into = next
					continue
				}
				break
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// cutFrom is the word a stem without an ending was cut from by the old
// extractor, which stripped -anto, -anta, -inta, -onta… as participles:
// when stem+that is a ReVo word or a core card's, no participle (the
// stem has no verb: kvanto, diamanto — not batanto, of bati), and in the
// card's own text (inText). "" none.
func cutFrom(stem string, x *Index, inText func(string) bool) string {
	if stem == "" || revo.Exact(stem+"i") != nil {
		return ""
	}
	for _, suf := range []string{"anto", "anta", "into", "inta", "onto", "onta"} {
		if c := stem + suf; inText(c) && (revo.Exact(c) != nil || (x != nil && x.word[c] != nil)) {
			return c
		}
	}
	return ""
}

func hasDefs(it *model.ContentItem) bool {
	if s, _ := it.Content["definition"].(string); strings.TrimSpace(s) != "" {
		return true
	}
	d, _ := it.Content["definitions"].(map[string]interface{})
	for _, v := range d {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

// The cleanup of the old cards (the builder API's /builder-api/legvortoj)
// marks a folded card with these, and keeps what it changed under UndoKey.
const (
	MergedStatus  = "merged"      // folded away: hidden everywhere, its page leads to MergedInto
	MergedIntoKey = "merged_into" // content: the slug of the card it folded into ("" none)
	UndoKey       = "legvortoj_undo"
)
