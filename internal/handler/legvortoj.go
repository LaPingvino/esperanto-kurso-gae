package handler

// The words of a reading (internal/legvortoj), on the site: the text's
// words tappable, opening an Esperanto-first panel; a glossary of the
// words new to the course under the text; and "Vortoj el la teksto", a
// short practice that fills each word back into its own sentence — the
// sentence and the Esperanto definition are the prompt, so it needs no
// shared language. A word's card (core or shared) learns from the answers.
//
// The balance (Joop: the course stands apart from other languages, and
// some languages still lack translations): every panel and question leads
// with Esperanto — the sentence, the parts, the definition, a picture —
// and the translation is one more aid, folded away and marked with its
// language when it is a fallback.

import (
	"context"
	"hash/fnv"
	"html"
	"html/template"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// GlossWord is one word of a reading as the page shows it.
type GlossWord struct {
	*legvortoj.Use
	ID    string // the glossary entry's anchor: "v-knabo"
	M     Meaning
	Pic   *bildoj.Picture
	Parts []GlossPart
	New   bool // not a core word: listed in the glossary under the text
}

// GlossPart is one piece of a word, with its root's card when the course
// has one.
type GlossPart struct {
	Text, Kind, Slug string
}

// PartsText is the word in its pieces: mal·san·ul·ej·o.
func (g *GlossWord) PartsText() string {
	var ps []string
	for _, p := range g.Parts {
		ps = append(ps, p.Text)
	}
	return strings.Join(ps, "·")
}

// HintDef is the Esperanto definition as a hint: the word itself (its
// stem) written ~, as ReVo does, so the hint doesn't give it away.
func (g *GlossWord) HintDef() string {
	def := g.EoDef()
	stem := g.Lemma
	if r := []rune(stem); len(r) > 1 && strings.ContainsRune("oaei", r[len(r)-1]) {
		stem = string(r[:len(r)-1])
	}
	if len([]rune(stem)) < 3 || def == "" {
		return def
	}
	re := regexp.MustCompile(`(?i)(^|[^\p{L}])` + regexp.QuoteMeta(stem))
	return re.ReplaceAllString(def, "${1}~")
}

// ShowParts: the pieces say more than the word: a derived word or a
// compound, or a root with its card.
func (g *GlossWord) ShowParts() bool {
	if len(g.Parts) > 2 {
		return true
	}
	for _, p := range g.Parts {
		if p.Slug != "" && (g.Card == nil || p.Slug != g.Card.Slug) {
			return true
		}
	}
	return false
}

// wordIndexCache keeps the index of the course's cards for the cached
// approved items it was built from.
var wordIndexCache struct {
	sync.Mutex
	from []*model.ContentItem
	idx  *legvortoj.Index
}

// wordIndex is the index of the course's vocab cards.
func wordIndex(ctx context.Context, cs interface {
	ApprovedShared(context.Context) ([]*model.ContentItem, error)
}) *legvortoj.Index {
	items, err := cs.ApprovedShared(ctx)
	if err != nil || len(items) == 0 {
		return legvortoj.NewIndex(nil)
	}
	c := &wordIndexCache
	c.Lock()
	defer c.Unlock()
	if c.idx == nil || len(c.from) != len(items) || &c.from[0] != &items[0] {
		c.from, c.idx = items, legvortoj.NewIndex(items)
	}
	return c.idx
}

// Glossary is a reading's words for the page, in a learner's language.
func Glossary(reading *model.ContentItem, x *legvortoj.Index, lang string) []*GlossWord {
	var out []*GlossWord
	for _, u := range legvortoj.Words(reading.Slug, reading.Text(), x) {
		g := &GlossWord{Use: u, ID: "v-" + eo.WordToSlug(u.Lemma), New: !u.Core}
		// the meaning: the card's own (course, then ReVo) or the word's in
		// ReVo, in the sense the text means
		it := &model.ContentItem{Type: "vocab", Content: map[string]interface{}{"word": u.Lemma}}
		if u.Card != nil {
			cp := *u.Card
			cp.Content = map[string]interface{}{}
			for k, v := range u.Card.Content {
				cp.Content[k] = v
			}
			if revoWordOf(u.Card) != u.Lemma {
				cp.Content["word"] = u.Lemma // a root card (amik): the meaning of the word itself
			}
			it = &cp
		}
		if u.Sense != "" {
			it.Content["senco"] = u.Sense
		}
		g.M = MeaningFor(it, lang, "")
		if def := u.EoDef(); def != "" {
			g.M.EoDef = def
		}
		g.Pic = bildoj.ForVocab(u.Lemma, "")
		for _, p := range u.Parts {
			gp := GlossPart{Text: p.Text, Kind: p.Kind}
			if p.Kind == "radiko" || p.Kind == "prefikso" || p.Kind == "sufikso" {
				if c := x.RootCard(p.Text); c != nil {
					gp.Slug = c.Slug
				}
			}
			g.Parts = append(g.Parts, gp)
		}
		out = append(out, g)
	}
	return out
}

func revoWordOf(it *model.ContentItem) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(it.Word()), "-"))
}

// GlossText is a reading's text with its words tappable: each opens the
// word's panel (eo.js), or without JavaScript leads to its glossary entry.
func GlossText(text string, words []*GlossWord) template.HTML {
	byForm := map[string]*GlossWord{}
	for _, g := range words {
		for _, f := range g.Forms {
			byForm[f] = g
		}
	}
	var b strings.Builder
	rs := []rune(text)
	for i := 0; i < len(rs); {
		j := i
		for j < len(rs) && unicode.IsLetter(rs[j]) {
			j++
		}
		if j > i {
			tok := string(rs[i:j])
			if g := byForm[strings.ToLower(tok)]; g != nil {
				b.WriteString(`<a class="eo-word" href="#` + g.ID + `" data-v="` + g.ID + `">` + html.EscapeString(tok) + `</a>`)
			} else {
				b.WriteString(html.EscapeString(tok))
			}
			i = j
			continue
		}
		if rs[i] == '\n' {
			b.WriteString("<br>")
		} else {
			b.WriteString(html.EscapeString(string(rs[i])))
		}
		i++
	}
	return template.HTML(b.String())
}

// ---- who has read what ----

// readSet is the readings a learner has read (judged on the stage):
// nil for no learner.
func (h *ContentHandler) readSet(ctx context.Context, u *model.User) map[string]bool {
	if u == nil || h.attempts == nil {
		return nil
	}
	atts, _ := h.attempts.ListByUser(ctx, u.ID, 500)
	read := map[string]bool{}
	for _, a := range atts {
		read[a.ContentItemID] = true
	}
	return read
}

// standalone reports whether a card may show on its own (the vocabulary
// list, a random word, search): every card but a shared reading-word
// card, which shows only to a learner who has read one of its texts (its
// tags name them). In its texts it is always there, in the glossary.
func standalone(it *model.ContentItem, read map[string]bool) bool {
	if !legvortoj.IsShared(it) {
		return true
	}
	for _, t := range it.Tags {
		if read[t] {
			return true
		}
	}
	return false
}

// keepStandalone filters a list by standalone.
func keepStandalone(items []*model.ContentItem, read map[string]bool) []*model.ContentItem {
	out := items[:0:0]
	for _, it := range items {
		if standalone(it, read) {
			out = append(out, it)
		}
	}
	return out
}

// ---- the practice: Vortoj el la teksto ----

// practiceMax is how many words one round asks.
const practiceMax = 10

// PracticeQ is one question: the word's sentence with a gap where it was.
type PracticeQ struct {
	Word    *GlossWord
	Form    string   // the answer: the word as the sentence has it
	Before  string   // the sentence up to the gap
	After   string   // and after it
	Options []string // four forms from the text, the answer among them
}

// formRE-free gap: the first whole-word occurrence of form in s.
func gapAt(s, form string) (before, after string, ok bool) {
	rs := []rune(s)
	fr := []rune(form)
	for i := 0; i+len(fr) <= len(rs); i++ {
		if !strings.EqualFold(string(rs[i:i+len(fr)]), form) {
			continue
		}
		if (i > 0 && unicode.IsLetter(rs[i-1])) || (i+len(fr) < len(rs) && unicode.IsLetter(rs[i+len(fr)])) {
			continue
		}
		return string(rs[:i]), string(rs[i+len(fr):]), true
	}
	return "", "", false
}

// ending is a form's inflection, for picking distractors of the same kind.
func ending(f string) string {
	for _, e := range []string{"ojn", "ajn", "oj", "aj", "on", "an", "en", "as", "is", "os", "us", "o", "a", "e", "i", "u"} {
		if strings.HasSuffix(f, e) {
			return e
		}
	}
	return ""
}

// Practice is the questions for a reading: its words new to the course
// first (then core words, to make up a round), each in its own sentence,
// with three other words of the text in the same form as distractors.
// The same reading always gives the same round.
func Practice(slug string, words []*GlossWord) []*PracticeQ {
	var pool []*GlossWord
	for _, g := range words {
		if g.New {
			pool = append(pool, g)
		}
	}
	for _, g := range words {
		if !g.New && len(pool) < practiceMax {
			pool = append(pool, g)
		}
	}
	h := fnv.New64a()
	h.Write([]byte(slug))
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	var allForms []string
	seen := map[string]bool{}
	for _, g := range words {
		for _, f := range g.Forms {
			if !seen[f] {
				seen[f] = true
				allForms = append(allForms, f)
			}
		}
	}
	var out []*PracticeQ
	for _, g := range pool {
		if len(out) == practiceMax {
			break
		}
		var q *PracticeQ
		for _, f := range g.Forms {
			if b, a, ok := gapAt(g.Sentence, f); ok {
				q = &PracticeQ{Word: g, Form: f, Before: b, After: a}
				break
			}
		}
		if q == nil {
			continue
		}
		var same, other []string
		for _, f := range allForms {
			if f == q.Form || containsStr(g.Forms, f) {
				continue
			}
			if ending(f) == ending(q.Form) {
				same = append(same, f)
			} else {
				other = append(other, f)
			}
		}
		rng.Shuffle(len(same), func(i, j int) { same[i], same[j] = same[j], same[i] })
		rng.Shuffle(len(other), func(i, j int) { other[i], other[j] = other[j], other[i] })
		opts := append([]string{q.Form}, append(same, other...)...)
		if len(opts) > 4 {
			opts = opts[:4]
		}
		if len(opts) < 2 {
			continue
		}
		rng.Shuffle(len(opts), func(i, j int) { opts[i], opts[j] = opts[j], opts[i] })
		q.Options = opts
		out = append(out, q)
	}
	return out
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// readingPractice loads a reading and its round.
func readingPractice(ctx context.Context, cs interface {
	GetBySlug(context.Context, string) (*model.ContentItem, error)
	ApprovedShared(context.Context) ([]*model.ContentItem, error)
}, slug, lang string) (*model.ContentItem, []*PracticeQ) {
	reading, err := cs.GetBySlug(ctx, slug)
	if err != nil || reading == nil || reading.Type != "reading" {
		return nil, nil
	}
	return reading, Practice(slug, Glossary(reading, wordIndex(ctx, cs), lang))
}

func learnerLang(u *model.User) string {
	if u != nil && u.Lang != "" {
		return u.Lang
	}
	return "en"
}

// ShowReadingPractice handles GET /legado/{slug}/vortoj?n=: question n.
func (h *ContentHandler) ShowReadingPractice(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	reading, qs := readingPractice(r.Context(), h.content, r.PathValue("slug"), learnerLang(u))
	if reading == nil || len(qs) == 0 {
		http.NotFound(w, r)
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n < 1 || n > len(qs) {
		n = 1
	}
	data := map[string]interface{}{
		"User": u, "UILang": UILangFor(u), "Lang": learnerLang(u),
		"Reading": reading, "Q": qs[n-1], "N": n, "Total": len(qs),
	}
	if err := h.tmpl.ExecuteTemplate(w, "legvortoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// SubmitReadingPractice handles POST /legado/{slug}/vortoj (n, answer):
// the verdict; a right or wrong answer counts for the word's card, when
// it has one, like an answer on the card itself.
func (h *ExerciseHandler) SubmitReadingPractice(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	u := UserFromContext(ctx)
	slug := r.PathValue("slug")
	reading, qs := readingPractice(ctx, h.content, slug, learnerLang(u))
	n, _ := strconv.Atoi(r.FormValue("n"))
	if reading == nil || n < 1 || n > len(qs) {
		http.NotFound(w, r)
		return
	}
	q := qs[n-1]
	answer := strings.TrimSpace(r.FormValue("answer"))
	correct := strings.EqualFold(answer, q.Form)
	newToken := ""
	if u == nil {
		var err error
		if u, newToken, err = h.createAnonymousUser(ctx); err != nil {
			http.Error(w, "Ne eblis krei uzanton", http.StatusInternalServerError)
			return
		}
	}
	score := 0.0
	if correct {
		score = 1
	}
	var streak int
	var milestone bool
	if q.Word.Card != nil {
		_, _, streak, milestone = h.rate(ctx, u, q.Word.Card, score, correct, answer)
	}
	cont := "/ekzerco/" + slug
	if n < len(qs) {
		cont = "/legado/" + slug + "/vortoj?n=" + strconv.Itoa(n+1)
	}
	why := q.Word.EoDef()
	if q.Word.ShowParts() {
		why = q.Word.PartsText() + " — " + why
	}
	data := map[string]interface{}{
		"User": u, "Correct": correct, "CorrectAnswer": q.Form, "YourAnswer": answer,
		"Why": why, "Gloss": "", "ContinueHref": cont, "UILang": u.UILangOrDefault(),
		"StreakDays": streak, "StreakMilestone": milestone, "NoAlt": true,
	}
	if q.Word.Card != nil {
		data["Item"] = q.Word.Card
		data["VoteScore"] = q.Word.Card.VoteScore
	}
	if newToken != "" {
		w.Header().Set("X-New-Token", newToken)
	}
	if err := h.tmpl.ExecuteTemplate(w, "rezulto.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// newWords is the glossary's list: the words new to the course, in the
// order of the text.
func newWords(words []*GlossWord) []*GlossWord {
	var out []*GlossWord
	for _, g := range words {
		if g.New {
			out = append(out, g)
		}
	}
	return out
}

// knownWords: the core words, alphabetically (their panels).
func knownWords(words []*GlossWord) []*GlossWord {
	var out []*GlossWord
	for _, g := range words {
		if !g.New {
			out = append(out, g)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Lemma < out[j].Lemma })
	return out
}
