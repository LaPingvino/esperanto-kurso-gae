package handler

import (
	"math/rand"
	"net/http"
	"sort"
	"strconv"
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
	users        *store.UserStore
}

// NewContentHandler creates a ContentHandler.
func NewContentHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	votes *store.VoteStore,
	translations *store.TranslationStore,
	users *store.UserStore,
) *ContentHandler {
	return &ContentHandler{
		tmpl:         tmpl,
		content:      content,
		comments:     comments,
		votes:        votes,
		translations: translations,
		users:        users,
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
		pick := items[rand.Intn(len(items))]
		http.Redirect(w, r, "/ekzerco/"+pick.Slug, http.StatusSeeOther)
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

const vortaroPageSize = 50

// ShowVortaro handles GET /vortaro — searchable, tag-filtered, paginated vocab list.
func (h *ContentHandler) ShowVortaro(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	q := r.URL.Query()
	tag := q.Get("tag")
	search := strings.ToLower(strings.TrimSpace(q.Get("s")))
	page, _ := strconv.Atoi(q.Get("p"))
	if page < 1 {
		page = 1
	}

	// Fetch all vocab items in one pass — used for both the tag-filter UI and
	// the filtered result set.
	allVocab, _ := h.content.ListByType(r.Context(), "vocab", 2000)

	// Auto-generate vocab items when the tag is a reading slug and nothing exists yet.
	if tag != "" {
		var taggedVocab []*model.ContentItem
		for _, it := range allVocab {
			for _, t := range it.Tags {
				if t == tag {
					taggedVocab = append(taggedVocab, it)
					break
				}
			}
		}
		if len(taggedVocab) == 0 {
			if reading, _ := h.content.GetBySlug(r.Context(), tag); reading != nil && reading.Type == "reading" {
				text := reading.Text()
				if text != "" {
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
						iTags := append([]string{"vortaro", reading.Slug}, reading.Tags...)
						seen := make(map[string]bool)
						var uniqueTags []string
						for _, t := range iTags {
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
					// Refresh after generation.
					allVocab, _ = h.content.ListByType(r.Context(), "vocab", 2000)
				}
			}
		}
	}

	// Build tag counts from full vocab set for the filter UI.
	tagCounts := make(map[string]int)
	for _, it := range allVocab {
		for _, t := range it.Tags {
			tagCounts[t]++
		}
	}
	// Sort tags by count descending for the UI pills.
	type tagCount struct {
		Tag   string
		Count int
	}
	var tagList []tagCount
	for t, c := range tagCounts {
		tagList = append(tagList, tagCount{t, c})
	}
	sort.Slice(tagList, func(i, j int) bool {
		if tagList[i].Count != tagList[j].Count {
			return tagList[i].Count > tagList[j].Count
		}
		return tagList[i].Tag < tagList[j].Tag
	})

	// Apply tag and search filters.
	var filtered []*model.ContentItem
	for _, it := range allVocab {
		if tag != "" {
			hasTag := false
			for _, t := range it.Tags {
				if t == tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		if search != "" {
			word := strings.ToLower(it.Word())
			if !strings.Contains(word, search) {
				continue
			}
		}
		filtered = append(filtered, it)
	}

	// Sort filtered results alphabetically by word.
	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Word()) < strings.ToLower(filtered[j].Word())
	})

	// Paginate.
	total := len(filtered)
	totalPages := (total + vortaroPageSize - 1) / vortaroPageSize
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * vortaroPageSize
	end := start + vortaroPageSize
	if end > total {
		end = total
	}
	pageItems := filtered[start:end]

	data := map[string]interface{}{
		"User":       u,
		"Items":      pageItems,
		"Tag":        tag,
		"Search":     q.Get("s"),
		"Page":       page,
		"TotalPages": totalPages,
		"Total":      total,
		"Tags":       tagList,
		"UILang":     UILangFor(u),
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
	var vocabItems []*model.ContentItem
	if item.Type == "reading" {
		vocabTag = item.Slug
		// Combine reading-specific vocab (tagged with slug) and global zagr roots.
		specific, _ := h.content.ListByTag(r.Context(), item.Slug, 500)
		zagr, _ := h.content.ListByTag(r.Context(), "zagr", 700)
		seen := make(map[string]bool)
		for _, v := range specific {
			seen[v.Slug] = true
			vocabItems = append(vocabItems, v)
		}
		for _, v := range zagr {
			if !seen[v.Slug] {
				vocabItems = append(vocabItems, v)
			}
		}
	}

	vocabModo := r.URL.Query().Get("modo")
	// For vocab exercises with no definition in the user's language, default to
	// karto-def mode so the word is shown first and the user can add a definition.
	if item.Type == "vocab" && vocabModo == "" {
		hasDef := false
		if defs, ok := item.Content["definitions"]; ok {
			if defsMap, ok := defs.(map[string]interface{}); ok {
				if v, ok := defsMap[userLang].(string); ok && v != "" {
					hasDef = true
				}
			}
		}
		// Also check legacy flat "definition" for English users.
		if !hasDef && userLang == "en" {
			if v, ok := item.Content["definition"].(string); ok && v != "" {
				hasDef = true
			}
		}
		// Check community translations for this user's language.
		if !hasDef {
			for _, tr := range translations {
				if tr.Language == userLang {
					hasDef = true
					break
				}
			}
		}
		if !hasDef {
			vocabModo = "karto-def"
		}
	}

	isFavorite := false
	isRadiko := false
	if u != nil {
		isFavorite = u.IsFavorite(slug)
	}
	for _, t := range item.Tags {
		if t == "radiko" {
			isRadiko = true
			break
		}
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
		"VocabItems":    vocabItems,
		"SeriesTotal":   seriesTotal,
		"VocabModo":     vocabModo,
		"IsFavorite":    isFavorite,
		"IsRadiko":      isRadiko,
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

// ToggleFavorite handles POST /ekzerco/{slug}/steli — adds/removes a favorite.
func (h *ContentHandler) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
		return
	}
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	_, err := h.users.ToggleFavorite(r.Context(), u.ID, slug)
	if err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/ekzerco/"+slug, http.StatusSeeOther)
}

// ShowFavorites handles GET /steloj — lists the user's starred exercises.
func (h *ContentHandler) ShowFavorites(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
		return
	}
	var items []*model.ContentItem
	for _, slug := range u.Favorites {
		item, err := h.content.GetBySlug(r.Context(), slug)
		if err == nil && item != nil {
			items = append(items, item)
		}
	}
	data := map[string]interface{}{
		"User":   u,
		"Items":  items,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "steloj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowHonorListo handles GET /honorlisto — hall of fame, top rated named users.
func (h *ContentHandler) ShowHonorListo(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	top, err := h.users.ListTopUsers(r.Context(), 100)
	if err != nil {
		top = nil
	}
	data := map[string]interface{}{
		"User":   u,
		"Top":    top,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "honorlisto.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
