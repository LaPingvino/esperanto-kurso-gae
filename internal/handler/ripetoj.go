package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/ripetoj"
)

// Spaced repetition (Ripetoj), as on angla.org: the scheduling runs in the
// browser (static/fsrs.js, FSRS-6; static/ripetoj.js, the cards' order;
// static/eo-ripetoj.js, storage, the session, the header count), its state
// in localStorage. The server gives the cards and keeps a rough trace per
// learner (internal/ripetoj), written where the answer is already saved:
//
//	GET  /ripetoj-api/kartoj   the cards: every approved item but texts and
//	                           videos, by series in authored order, with
//	                           their ratings. The same for everyone, outside
//	                           AuthMiddleware, cached publicly (ETag).
//	GET  /ripetoj              the session page
//	GET  /ripetoj/spuro        the learner's trace by slug (never cached):
//	                           what a browser without the state rebuilds from
//	POST /ripetoj/forgesi      forget the trace (the page's reset)

// IsCard reports whether an item is a repetition card: everything one
// answers or judges, not the texts and videos one reads or watches.
func IsCard(item *model.ContentItem) bool {
	if item == nil {
		return false
	}
	switch item.Type {
	case "reading", "video", "":
		return false
	}
	return true
}

// JudgmentGrade is a self-judgement's FSRS grade.
func JudgmentGrade(j string) int {
	switch j {
	case "facile":
		return ripetoj.Easy
	case "bone":
		return ripetoj.Good
	case "malfacile":
		return ripetoj.Hard
	}
	return ripetoj.Again
}

// SRSOwner names whose state a browser holds without giving the account
// away: the first 12 hex digits of the ID's SHA-256 (eo-ripetoj.js treats a
// state of another owner as not its own).
func SRSOwner(u *model.User) string {
	if u == nil || u.ID == "" {
		return ""
	}
	h := sha256.Sum256([]byte("ripetoj:" + u.ID))
	return hex.EncodeToString(h[:6])
}

// deck is the cards as JSON, built once per content set.
type deck struct {
	etag string
	body []byte
}

type deckCache struct {
	mu    sync.Mutex
	first *model.ContentItem
	n     int
	d     *deck
}

func (c *deckCache) get(items []*model.ContentItem) *deck {
	c.mu.Lock()
	defer c.mu.Unlock()
	var first *model.ContentItem
	if len(items) > 0 {
		first = items[0]
	}
	if c.d == nil || first != c.first || len(items) != c.n {
		c.d, c.first, c.n = buildDeck(items), first, len(items)
	}
	return c.d
}

// buildDeck: {"v": version, "c": [[[slug, rating], …], …]}: one chain per
// series in its authored order (series_order), then one chain per item
// outside a series; chains by their first card's rating.
func buildDeck(items []*model.ContentItem) *deck {
	bySeries := map[string][]*model.ContentItem{}
	var chains [][]*model.ContentItem
	for _, it := range items {
		if !IsCard(it) {
			continue
		}
		if it.SeriesSlug == "" {
			chains = append(chains, []*model.ContentItem{it})
			continue
		}
		bySeries[it.SeriesSlug] = append(bySeries[it.SeriesSlug], it)
	}
	for _, s := range bySeries {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].SeriesOrder != s[j].SeriesOrder {
				return s[i].SeriesOrder < s[j].SeriesOrder
			}
			return s[i].Slug < s[j].Slug
		})
		chains = append(chains, s)
	}
	sort.SliceStable(chains, func(i, j int) bool {
		a, b := chains[i][0], chains[j][0]
		if a.Rating != b.Rating {
			return a.Rating < b.Rating
		}
		return a.Slug < b.Slug
	})
	out := make([][][2]any, len(chains))
	for i, ch := range chains {
		out[i] = make([][2]any, len(ch))
		for j, it := range ch {
			out[i][j] = [2]any{it.Slug, int(it.Rating + 0.5)}
		}
	}
	c, _ := json.Marshal(out)
	h := sha256.Sum256(c)
	v := hex.EncodeToString(h[:8])
	body, _ := json.Marshal(map[string]any{"v": v, "c": json.RawMessage(c)})
	return &deck{etag: v, body: body}
}

// RipetojAPI serves the cards (/ripetoj-api/).
type RipetojAPI struct {
	src   approvedSource
	cache deckCache
}

func NewRipetojAPI(src approvedSource) *RipetojAPI { return &RipetojAPI{src: src} }

// Cards handles GET /ripetoj-api/kartoj.
func (a *RipetojAPI) Cards(w http.ResponseWriter, r *http.Request) {
	items, err := a.src.ApprovedShared(r.Context())
	if err != nil {
		http.Error(w, "la enhavo ne legeblas", http.StatusServiceUnavailable)
		return
	}
	d := a.cache.get(items)
	if cached(w, r, d.etag) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(d.body)
}

// Routes are the API's routes, to mount outside AuthMiddleware.
func (a *RipetojAPI) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ripetoj-api/kartoj", a.Cards)
}

type ripetojUsers interface {
	ClearSpuro(ctx context.Context, userID string) error
}

// RipetojHandler serves the page and the learner's trace.
type RipetojHandler struct {
	tmpl  Renderer
	src   approvedSource
	users ripetojUsers
}

func NewRipetojHandler(tmpl Renderer, src approvedSource, users ripetojUsers) *RipetojHandler {
	return &RipetojHandler{tmpl: tmpl, src: src, users: users}
}

// Page handles GET /ripetoj (?plu=1: straight on to the next card).
func (h *RipetojHandler) Page(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	data := map[string]any{"User": u, "UILang": UILangFor(u), "Plu": r.URL.Query().Get("plu") == "1"}
	if err := h.tmpl.ExecuteTemplate(w, "ripetoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Trace handles GET /ripetoj/spuro: {"o": owner, "e": [[slug, unix day,
// answers, last grade], …]}, the items of the trace still in the course.
func (h *RipetojHandler) Trace(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	entries := [][4]any{}
	if u != nil && len(u.Spuro) > 0 {
		items, err := h.src.ApprovedShared(r.Context())
		if err != nil {
			http.Error(w, "la enhavo ne legeblas", http.StatusServiceUnavailable)
			return
		}
		slugs := make([]string, 0, len(items))
		for _, it := range items {
			if IsCard(it) {
				slugs = append(slugs, it.Slug)
			}
		}
		for _, e := range ripetoj.Resolve(u.Spuro, slugs) {
			entries = append(entries, [4]any{e.Slug, e.Day, e.Count, e.Grade})
		}
	}
	json.NewEncoder(w).Encode(map[string]any{"o": SRSOwner(u), "e": entries})
}

// Forget handles POST /ripetoj/forgesi: the trace goes (the browser clears
// its own state), so nothing rebuilds what was just forgotten.
func (h *RipetojHandler) Forget(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.users.ClearSpuro(r.Context(), u.ID); err != nil {
		http.Error(w, "ne eblis forgesi", http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Forgesitaj", strconv.Itoa(ripetoj.Len(u.Spuro)))
	w.WriteHeader(http.StatusNoContent)
}

// Routes are the per-learner routes, inside AuthMiddleware.
func (h *RipetojHandler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ripetoj", h.Page)
	mux.HandleFunc("GET /ripetoj/spuro", h.Trace)
	mux.HandleFunc("POST /ripetoj/forgesi", h.Forget)
}
