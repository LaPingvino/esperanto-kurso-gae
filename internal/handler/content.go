package handler

import (
	"net/http"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/recommend"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

// ContentHandler handles exercise display and the home page.
type ContentHandler struct {
	tmpl         Renderer
	content      *store.ContentStore
	comments     *store.CommentStore
	votes        *store.VoteStore
	translations *store.TranslationStore
}

// NewContentHandler creates a ContentHandler.
func NewContentHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	votes *store.VoteStore,
	translations *store.TranslationStore,
) *ContentHandler {
	return &ContentHandler{
		tmpl:         tmpl,
		content:      content,
		comments:     comments,
		votes:        votes,
		translations: translations,
	}
}

// ShowHome handles GET /. Redirects immediately to the top recommended exercise.
func (h *ContentHandler) ShowHome(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	var rating, rd float64 = 1500, 350
	if u != nil {
		rating = u.Rating
		rd = u.RD
	}

	items, err := recommend.GetForUser(r.Context(), rating, rd, h.content, 10)
	if err != nil || len(items) == 0 {
		items, _ = h.content.ListApproved(r.Context(), 10)
	}

	if len(items) > 0 {
		http.Redirect(w, r, "/ekzerco/"+items[0].Slug, http.StatusSeeOther)
		return
	}

	// Fallback: no exercises yet.
	data := map[string]interface{}{
		"User":   u,
		"Items":  items,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "hejmo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowVortaro handles GET /vortaro — lists vocab items as a dictionary, optionally filtered by tag.
func (h *ContentHandler) ShowVortaro(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	tag := r.URL.Query().Get("tag")

	var items []*model.ContentItem
	var err error
	if tag != "" {
		items, err = h.content.ListByTag(r.Context(), tag, 500)
		if err != nil {
			items = nil
		}
		// Filter to vocab only when tag-filtered.
		var vocabOnly []*model.ContentItem
		for _, it := range items {
			if it.Type == "vocab" {
				vocabOnly = append(vocabOnly, it)
			}
		}
		items = vocabOnly
	} else {
		items, err = h.content.ListByType(r.Context(), "vocab", 500)
		if err != nil {
			items = nil
		}
	}

	// Auto-generate vocab items when the tag is a reading slug and nothing exists yet.
	if tag != "" && len(items) == 0 {
		if reading, _ := h.content.GetBySlug(r.Context(), tag); reading != nil && reading.Type == "reading" {
			text := reading.Text()
			if text != "" {
				// Check existing vocab words so we don't re-create them.
				allVocab, _ := h.content.ListByType(r.Context(), "vocab", 2000)
				existing := make(map[string]bool)
				for _, v := range allVocab {
					if w, ok := v.Content["word"].(string); ok {
						existing[strings.ToLower(strings.TrimSpace(w))] = true
					}
				}
				for _, word := range eo.ExtractWords(text) {
					if existing[word] {
						continue
					}
					vocSlug := "voc-auto-" + eo.WordToSlug(word)
					if ex, _ := h.content.GetBySlug(r.Context(), vocSlug); ex != nil {
						continue
					}
					tags := append([]string{"vortaro", reading.Slug}, reading.Tags...)
					seen := make(map[string]bool)
					var uniqueTags []string
					for _, t := range tags {
						if !seen[t] {
							seen[t] = true
							uniqueTags = append(uniqueTags, t)
						}
					}
					voc := &model.ContentItem{
						Slug:    vocSlug,
						Type:    "vocab",
						Content: map[string]interface{}{"word": word},
						Tags:    uniqueTags,
						Source:  reading.Source,
						Status:  "approved",
						Rating:  reading.Rating,
						RD:      200,
					}
					_ = h.content.Create(r.Context(), voc)
				}
				// Re-query after generation.
				items, _ = h.content.ListByTag(r.Context(), tag, 500)
				var vocabOnly []*model.ContentItem
				for _, it := range items {
					if it.Type == "vocab" {
						vocabOnly = append(vocabOnly, it)
					}
				}
				items = vocabOnly
			}
		}
	}

	data := map[string]interface{}{
		"User":   u,
		"Items":  items,
		"Tag":    tag,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "vortaro.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowExercise handles GET /ekzerco/{slug}.
func (h *ContentHandler) ShowExercise(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())

	comments, _ := h.comments.ListApprovedByContent(r.Context(), slug)
	translations, _ := h.translations.ListByTarget(r.Context(), slug)

	var currentVote *model.Vote
	userLang := "en"
	if u != nil {
		currentVote, _ = h.votes.GetByUserAndContent(r.Context(), u.ID, slug)
		userLang = u.Lang
	}

	userID := ""
	if u != nil {
		userID = u.ID
	}
	votes := buildVoteMap(r.Context(), h.translations, userID, translations)
	tradukData := buildTradukData(slug, userLang, userID, translations, votes)
	tradukData["User"] = u
	tradukData["UILang"] = UILangFor(u)

	// Inject a just-added community translation passed via query params (avoids
	// eventual-consistency gap where the Datastore query misses the new entry).
	if addedLang := r.URL.Query().Get("added_lang"); addedLang == userLang {
		if addedDef := r.URL.Query().Get("added_def"); addedDef != "" {
			existing, _ := tradukData["MyLangTranslations"].([]*model.Translation)
			// Prepend only if not already present (idempotent on refresh).
			already := false
			for _, tr := range existing {
				if tr.Text == addedDef {
					already = true
					break
				}
			}
			if !already {
				synthetic := &model.Translation{Language: addedLang, Text: addedDef}
				tradukData["MyLangTranslations"] = append([]*model.Translation{synthetic}, existing...)
			}
		}
	}

	// Series navigation.
	var prevInSeries, nextInSeries *model.ContentItem
	seriesTotal := 0
	if item.SeriesSlug != "" {
		seriesItems, _ := h.content.ListBySeries(r.Context(), item.SeriesSlug)
		seriesTotal = len(seriesItems)
		for i, si := range seriesItems {
			if si.Slug == slug {
				if i > 0 {
					prevInSeries = seriesItems[i-1]
				}
				if i < len(seriesItems)-1 {
					nextInSeries = seriesItems[i+1]
				}
				break
			}
		}
	}

	// For reading exercises, link to vocab training using the reading's slug as tag.
	vocabTag := ""
	if item.Type == "reading" {
		vocabTag = item.Slug
	}

	data := map[string]interface{}{
		"User":          u,
		"Item":          item,
		"Comments":      comments,
		"CurrentVote":   currentVote,
		"TradukData":    tradukData,
		"PrevInSeries":  prevInSeries,
		"NextInSeries":  nextInSeries,
		"VocabTag":      vocabTag,
		"SeriesTotal":   seriesTotal,
		"VocabModo":     r.URL.Query().Get("modo"),
		"UILang":        UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "ekzerco.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Browse handles GET /sercxi — filter exercises by tag, type, or CEFR level.
// For tag filters it shows a list page; for type/cefr it redirects to the first match.
func (h *ContentHandler) Browse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tag := q.Get("etikedo")
	typ := q.Get("tipo")
	cefr := q.Get("cefr")
	u := UserFromContext(r.Context())

	var items []*model.ContentItem
	var err error

	switch {
	case tag != "":
		items, err = h.content.ListByTag(r.Context(), tag, 200)
		if err != nil {
			items = nil
		}
		data := map[string]interface{}{
			"User":   u,
			"Items":  items,
			"Filter": tag,
			"Kind":   "etikedo",
			"UILang": UILangFor(u),
		}
		if err := h.tmpl.ExecuteTemplate(w, "sercxi.html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	case typ != "":
		items, err = h.content.ListByType(r.Context(), typ, 50)
	case cefr != "":
		minR, maxR := cefrToRatingRange(cefr)
		items, err = h.content.ListByRatingRange(r.Context(), minR, maxR, 50)
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err != nil || len(items) == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/ekzerco/"+items[0].Slug, http.StatusSeeOther)
}

// ShowEtikedoj handles GET /etikedoj — lists all tags with exercise counts.
func (h *ContentHandler) ShowEtikedoj(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	counts, err := h.content.ListAllTags(r.Context())
	if err != nil {
		counts = nil
	}
	data := map[string]interface{}{
		"User":   u,
		"Tags":   counts,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "etikedoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func cefrToRatingRange(cefr string) (float64, float64) {
	ranges := map[string][2]float64{
		"A0": {0, 1000},
		"A1": {1000, 1200},
		"A2": {1200, 1400},
		"B1": {1400, 1600},
		"B2": {1600, 1800},
		"C1": {1800, 2000},
		"C2": {2000, 9999},
	}
	if r, ok := ranges[cefr]; ok {
		return r[0], r[1]
	}
	return 0, 9999
}
