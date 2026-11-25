package handler

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The pages around the stage (kurso-design DESIGN.md "Page types around the
// stage"): home, the course map, grammar and the hand-off to angla.org.
// They all read the cached approved content (ContentStore.ApprovedShared),
// never a fresh scan.

// handoffSeenCookie marks that the learner has seen /vojkrucigxo: the full
// page is offered once, after that the offer is a home card and the footer.
const handoffSeenCookie = "vojkrucigxo"

// progress is the learner's per-series progress on the whole course.
func (h *ContentHandler) progress(ctx context.Context, u *model.User) ([]*SeriesProgress, error) {
	items, err := h.content.ApprovedShared(ctx)
	if err != nil {
		return nil, err
	}
	var attempts []*model.Attempt
	if u != nil {
		attempts, _ = h.attempts.ListByUser(ctx, u.ID, 500)
	}
	return allSeries(items, attempts, time.Now()), nil
}

func placed(u *model.User) bool { return u != nil && u.Placed() }

// hereLevel is the learner's station: their level once placed, else A0.
func hereLevel(u *model.User) string {
	if placed(u) {
		return u.CEFRLevel()
	}
	return "A0"
}

// ShowHome handles GET /: greeting and streak, one big "next" card, the
// Hodiaŭ queue, the level bar, the game, and at B1 the way on to angla.org.
// A newcomer meets the A0 picture series first; nobody is sent elsewhere.
func (h *ContentHandler) ShowHome(w http.ResponseWriter, r *http.Request) {
	// "GET /" is the mux catch-all; don't render (and query for) the home
	// page for every probed path like /wp-login.php.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	u := UserFromContext(r.Context())
	series, err := h.progress(r.Context(), u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rating := 1500.0
	if u != nil {
		rating = u.Rating
	}
	hv := buildHome(series, placed(u), rating)
	_, noCookie := r.Cookie(handoffSeenCookie)
	data := map[string]interface{}{
		"User":        u,
		"UILang":      UILangFor(u),
		"Next":        hv.Next,
		"Today":       hv.Today,
		"Intro":       hv.Intro,
		"Placed":      placed(u),
		"FromAngla":   r.URL.Query().Get("de") == "angla",
		"HandoffSeen": noCookie == nil,
		"Angla":       AnglaArrival,
	}
	if err := h.tmpl.ExecuteTemplate(w, "hejmo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowMap handles GET /mapo: the course as a transit line, one station per
// KER level, series as stops; levels with nothing in them say so.
func (h *ContentHandler) ShowMap(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	series, err := h.progress(r.Context(), u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rating := 1500.0
	if u != nil {
		rating = u.Rating
	}
	hv := buildHome(series, placed(u), rating)
	data := map[string]interface{}{
		"User":     u,
		"UILang":   UILangFor(u),
		"Stations": buildMap(series, hereLevel(u), hv.Next),
		"Angla":    AnglaArrival,
	}
	if err := h.tmpl.ExecuteTemplate(w, "mapo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowHandoff handles GET /vojkrucigxo: at B1, angla.org (English explained
// in Esperanto) is offered, with a way to stay. Shown once as a full page.
func (h *ContentHandler) ShowHandoff(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u != nil && u.ReachedB1() {
		http.SetCookie(w, &http.Cookie{Name: handoffSeenCookie, Value: "1", Path: "/", MaxAge: 5 * 365 * 24 * 3600, SameSite: http.SameSiteLaxMode})
	}
	data := map[string]interface{}{
		"User":    u,
		"UILang":  UILangFor(u),
		"Reached": u != nil && u.ReachedB1(),
		"Angla":   AnglaArrival,
	}
	if err := h.tmpl.ExecuteTemplate(w, "vojkrucigxo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// GrammarTopic is one grammar page: a PMEG excerpt (when there is one) and
// the series that practise the same point, in order of preference.
type GrammarTopic struct {
	Slug     string
	Title    string
	Reading  string   // slug of the reading that explains it
	Intro    string   // the site's own short text when there is no reading
	Practice []string // series slugs
}

// grammarTopics: at first the PMEG excerpts and the grammar series
// (kurso-design DESIGN.md, "New templates"), each point with its practice.
var grammarTopics = []GrammarTopic{
	{Slug: "substantivoj", Title: "Substantivoj (O-vortoj)", Reading: "pmeg-substantivoj", Practice: []string{"gramatiko-mc"}},
	{Slug: "adjektivoj", Title: "Adjektivoj (A-vortoj)", Reading: "pmeg-adjektivoj", Practice: []string{"gramatiko-mc"}},
	{Slug: "verboj", Title: "Verbaj finaĵoj: tempoj kaj modoj", Reading: "pmeg-verboj", Practice: []string{"gramatiko-tempoj-modoj-ekz"}},
	{Slug: "pronomoj", Title: "Pronomoj", Reading: "pmeg-pronomoj", Practice: []string{"vortaro-pronomo"}},
	{Slug: "prepozicioj", Title: "Prepozicioj kaj la akuzativo de direkto", Reading: "pmeg-prepozicioj", Practice: []string{"gramatiko-direkto-ekz", "vortaro-prepozicio"}},
	{Slug: "nombroj", Title: "Nombroj kaj multenombro", Reading: "pmeg-nombroj", Practice: []string{"bildoj-nombroj", "vortaro-cifero"}},
	{Slug: "tabelvortoj", Title: "Tabelvortoj", Reading: "pmeg-tabelvortoj", Practice: []string{"gramatiko-tabelvortoj-ekz", "vortaro-tabelvorto"}},
	{Slug: "participoj", Title: "Participoj", Reading: "pmeg-participoj", Practice: []string{"gramatiko-participoj-ekz"}},
	{Slug: "vortfarado", Title: "Afiksoj kaj vortfarado", Reading: "pmeg-vortfarado", Practice: []string{"gramatiko-afiksoj-ekz", "vortaro-sufikso", "vortaro-prefikso"}},
	{Slug: "terminoj", Title: "Gramatikaj terminoj",
		Intro:    "La vortoj, per kiuj oni parolas pri lingvo: substantivo, verbo, akuzativo, participo, subfrazo… Ilin uzas ankaŭ la klarigoj de angla.org, kiu instruas la anglan en Esperanto.",
		Practice: []string{"gramatikaj-terminoj-ekz", "vortaro-gramatikaj-terminoj"}},
}

// grammarView is one topic as a page or an index line.
type grammarView struct {
	GrammarTopic
	Level    string
	Text     *model.ContentItem
	Blocks   []ProseBlock
	Practice []*SeriesProgress
}

func (h *ContentHandler) grammar(ctx context.Context, u *model.User) []*grammarView {
	items, _ := h.content.ApprovedShared(ctx)
	bySlug := map[string]*model.ContentItem{}
	for _, it := range items {
		bySlug[it.Slug] = it
	}
	series, _ := h.progress(ctx, u)
	bySeries := map[string]*SeriesProgress{}
	for _, sp := range series {
		bySeries[sp.Slug] = sp
	}
	var out []*grammarView
	for _, t := range grammarTopics {
		gv := &grammarView{GrammarTopic: t, Text: bySlug[t.Reading]}
		for _, s := range t.Practice {
			if sp := bySeries[s]; sp != nil {
				gv.Practice = append(gv.Practice, sp)
			}
		}
		switch {
		case gv.Text != nil:
			gv.Level = model.RatingToCEFR(gv.Text.Rating)
		case len(gv.Practice) > 0:
			gv.Level = gv.Practice[0].CEFR
		default:
			continue // nothing to read or practise: not a page yet
		}
		out = append(out, gv)
	}
	return out
}

// ShowGrammarIndex handles GET /gramatiko.
func (h *ContentHandler) ShowGrammarIndex(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	data := map[string]interface{}{
		"User":   u,
		"UILang": UILangFor(u),
		"Topics": h.grammar(r.Context(), u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "gramatiko.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowGrammar handles GET /gramatiko/{topic}: the one page built for
// reading, ending in a practice card for the same point.
func (h *ContentHandler) ShowGrammar(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	slug := r.PathValue("topic")
	for _, gv := range h.grammar(r.Context(), u) {
		if gv.Slug != slug {
			continue
		}
		if gv.Text != nil {
			gv.Blocks = FormatProse(gv.Text.Text())
		}
		data := map[string]interface{}{
			"User":   u,
			"UILang": UILangFor(u),
			"Topic":  gv,
		}
		if err := h.tmpl.ExecuteTemplate(w, "gramatikero.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	http.NotFound(w, r)
}

// ProseBlock is a piece of a grammar text: a paragraph ("p", with an
// optional lead-in Term), a heading ("h"), a table of term → explanation
// ("table") or a list of examples ("examples").
type ProseBlock struct {
	Kind  string
	Term  string
	Text  string
	Rows  [][2]string
	Items []string
}

var sentenceEnd = regexp.MustCompile(`([.!?])\s+`)

// splitSentences splits example text into sentences ("Kiu venis? Neniu.").
func splitSentences(s string) []string {
	var out []string
	last := 0
	for _, m := range sentenceEnd.FindAllStringSubmatchIndex(s, -1) {
		out = append(out, strings.TrimSpace(s[last:m[3]]))
		last = m[1]
	}
	if rest := strings.TrimSpace(s[last:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func isExamples(head string) bool {
	return strings.HasPrefix(strings.ToLower(head), "ekzemplo")
}

// FormatProse turns a PMEG excerpt's plain text into reading blocks: a block
// whose first line ends in ":" becomes a heading with its lines as a table
// (term — explanation, or term = explanation) or as examples; "Ekzemploj: …"
// becomes an example list; "Term — x: …" leads its paragraph in bold.
func FormatProse(text string) []ProseBlock {
	var out []ProseBlock
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		lines := strings.Split(para, "\n")
		if len(lines) > 1 && strings.HasSuffix(strings.TrimSpace(lines[0]), ":") {
			head := strings.TrimSuffix(strings.TrimSpace(lines[0]), ":")
			out = append(out, ProseBlock{Kind: "h", Text: head})
			body := lines[1:]
			if isExamples(head) {
				out = append(out, ProseBlock{Kind: "examples", Items: trimAll(body)})
				continue
			}
			tb := ProseBlock{Kind: "table"}
			for _, l := range body {
				l = strings.TrimSpace(l)
				if k, v, ok := cutTerm(l); ok {
					tb.Rows = append(tb.Rows, [2]string{k, v})
				} else {
					tb.Rows = append(tb.Rows, [2]string{"", l})
				}
			}
			out = append(out, tb)
			continue
		}
		joined := strings.Join(trimAll(lines), " ")
		if head, rest, ok := strings.Cut(joined, ":"); ok && isExamples(head) && len(head) < 12 {
			out = append(out, ProseBlock{Kind: "h", Text: strings.TrimSpace(head)})
			out = append(out, ProseBlock{Kind: "examples", Items: splitSentences(rest)})
			continue
		}
		if i := strings.Index(joined, " — "); i > 0 && i < 40 {
			if j := strings.Index(joined, ": "); j > i && j < 70 {
				out = append(out, ProseBlock{Kind: "p", Term: joined[:j+1], Text: joined[j+2:]})
				continue
			}
		}
		out = append(out, ProseBlock{Kind: "p", Text: joined})
	}
	return out
}

func trimAll(ss []string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// cutTerm splits "KI- = demandovorto" or "ANT — la ago…" at its first
// separator.
func cutTerm(l string) (string, string, bool) {
	for _, sep := range []string{" = ", " — "} {
		if k, v, ok := strings.Cut(l, sep); ok && len([]rune(k)) <= 24 {
			return strings.TrimSpace(k), strings.TrimSpace(v), true
		}
	}
	return "", "", false
}
