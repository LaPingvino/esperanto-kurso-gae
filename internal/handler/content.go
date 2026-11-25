package handler

import (
	"context"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
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
	attempts     *store.AttemptStore
	// GamePlace is the game's place showing an item, for the "play as a
	// game" link (GameAPI.PlaceOf); nil: GameLesson's Zagreb rule.
	GamePlace func(ctx context.Context, itemSlug string) string
}

// NewContentHandler creates a ContentHandler.
func NewContentHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	votes *store.VoteStore,
	translations *store.TranslationStore,
	users *store.UserStore,
	attempts *store.AttemptStore,
) *ContentHandler {
	return &ContentHandler{
		tmpl:         tmpl,
		content:      content,
		comments:     comments,
		votes:        votes,
		translations: translations,
		users:        users,
		attempts:     attempts,
	}
}

// RandomVocab handles GET /vorto — redirects to a random vocab exercise near the user's level.
// Optional query param ?etikedo=tag restricts to a specific tag (topic context).
func (h *ContentHandler) RandomVocab(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	var rating float64 = 1500
	if u != nil {
		rating = u.Rating
	}
	tag := r.URL.Query().Get("etikedo")
	var items []*model.ContentItem
	if tag != "" {
		// Try as tag first, then as series slug (covers both use cases).
		tagged, _ := h.content.ListByTag(r.Context(), tag, 100)
		seriesItems, _ := h.content.ListBySeries(r.Context(), tag)
		seen := map[string]bool{}
		for _, it := range append(tagged, seriesItems...) {
			if seen[it.Slug] {
				continue
			}
			seen[it.Slug] = true
			if it.Type == "vocab" && it.Rating >= rating-400 && it.Rating <= rating+400 {
				items = append(items, it)
			}
		}
	}
	read := h.readSet(r.Context(), u)
	if len(keepStandalone(items, read)) == 0 {
		items, _ = h.content.ListByTypeAndRatingRange(r.Context(), "vocab", rating-300, rating+300, 0)
	}
	if len(keepStandalone(items, read)) == 0 {
		items, _ = h.content.ListByType(r.Context(), "vocab", 0)
	}
	items = keepStandalone(items, read)
	if len(items) > 0 {
		http.Redirect(w, r, "/ekzerco/"+items[rand.Intn(len(items))].Slug, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/vortaro", http.StatusSeeOther)
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
	allVocab, _ := h.content.ListByType(r.Context(), "vocab", 0)

	// A shared reading-word card is listed for a learner who has read its
	// text (internal/legvortoj); nothing here makes cards any more.
	allVocab = keepStandalone(allVocab, h.readSet(r.Context(), u))

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
	userLang := "en"
	if u != nil && u.Lang != "" {
		userLang = u.Lang
	}
	meanings := make(map[string]Meaning, len(pageItems))
	for _, it := range pageItems {
		meanings[it.Slug] = MeaningFor(it, userLang, "")
	}

	data := map[string]interface{}{
		"User":       u,
		"Items":      pageItems,
		"Meanings":   meanings,
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
	// An old per-text word card folded into the word's one card (the
	// legvortoj cleanup): its links and stars lead there.
	if item.Status == legvortoj.MergedStatus {
		to := "/vortaro"
		if into, _ := item.Content[legvortoj.MergedIntoKey].(string); into != "" {
			to = "/ekzerco/" + into
		}
		http.Redirect(w, r, to, http.StatusFound)
		return
	}

	u := UserFromContext(r.Context())

	comments, _ := h.comments.ListApprovedByContent(r.Context(), slug)
	h.users.ResolveUsernames(r.Context(), comments)
	translations, _ := h.translations.ListByTarget(r.Context(), slug)
	h.users.ResolveTranslationAuthors(r.Context(), translations)

	var currentVote *model.Vote
	userLang := "en"
	if u != nil {
		currentVote, _ = h.votes.GetByUserAndContent(r.Context(), u.ID, slug)
		if u.Lang != "" {
			userLang = u.Lang
		}
	}

	userID := ""
	if u != nil {
		userID = u.ID
	}
	votes := buildVoteMap(r.Context(), h.translations, userID, translations)
	tradukData := buildTradukData(slug, userLang, translations, votes)
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
				// Only the display name: leaving AuthorID/ID empty keeps the
				// edit/vote controls (which need a real ID) hidden until the
				// stored entry becomes queryable.
				synthetic := &model.Translation{Language: addedLang, Text: addedDef}
				if u != nil {
					synthetic.AuthorUsername = u.DisplayName()
				}
				tradukData["MyLangTranslations"] = append([]*model.Translation{synthetic}, existing...)
			}
		}
	}

	// The meaning in the learner's language, by the fallback chain (course →
	// community → Reta Vortaro → English → Esperanto), and the dictionary links.
	var meaning Meaning
	if item.Type == "vocab" {
		community := ""
		if trs, _ := tradukData["MyLangTranslations"].([]*model.Translation); len(trs) > 0 {
			community = trs[0].Text
		}
		meaning = MeaningFor(item, userLang, community)
		if community == "" && len(meaning.Suggestions) > 0 {
			tradukData["Suggestions"] = meaning.Suggestions
		}
	}

	// Series navigation.
	var prevInSeries, nextInSeries *model.ContentItem
	seriesTotal := 0
	seriesPos := item.SeriesOrder // fallback to stored order
	var seriesItems []*model.ContentItem
	if item.SeriesSlug != "" {
		seriesItems, _ = h.content.ListBySeries(r.Context(), item.SeriesSlug)
		seriesTotal = len(seriesItems)
		for i, si := range seriesItems {
			if si.Slug == slug {
				seriesPos = i + 1 // actual position in sorted list
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

	// A reading's words: tappable in the text, the new ones in a glossary,
	// and a practice round of them (internal/legvortoj).
	var glossary []*GlossWord
	practice := 0
	if item.Type == "reading" {
		glossary = Glossary(item, wordIndex(r.Context(), h.content), userLang)
		practice = len(Practice(item.Slug, glossary))
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
		// A word with a picture needs no shared language: its picture is the
		// prompt (typing mode). Otherwise show the word first.
		if !hasDef && vocabPicture(item) == nil {
			vocabModo = "karto-def"
		}
	}

	isFavorite := false
	isRadiko := false
	vocabKind := ""
	isFavoriteSeries := false
	if u != nil {
		isFavorite = u.IsFavorite(slug)
		if item.SeriesSlug != "" {
			isFavoriteSeries = u.IsFavoriteSeries(item.SeriesSlug)
		}
	}
	for _, t := range item.Tags {
		if t == "radiko" || t == "sufikso" || t == "prefikso" {
			isRadiko = (t == "radiko")
			vocabKind = t
			break
		}
	}

	// Parent item: the reading/lesson this exercise belongs to (if SeriesParent is set).
	var parentItem *model.ContentItem
	if item.SeriesParent != "" {
		parentItem, _ = h.content.GetBySlug(r.Context(), item.SeriesParent)
	}

	// Child series: series whose SeriesParent == this item's slug (e.g. vocab/quiz series for a reading).
	childSeries, _ := h.content.ListBySeriesParent(r.Context(), slug)
	// Deduplicate to one entry per series slug (the first item of each series).
	// Split into vocab children (shown near vocab trainer) and other children.
	seenChildSeries := map[string]bool{}
	var uniqueChildSeries []*model.ContentItem
	var vocabChildSeries []*model.ContentItem
	for _, cs := range childSeries {
		if !seenChildSeries[cs.SeriesSlug] {
			seenChildSeries[cs.SeriesSlug] = true
			if cs.Type == "vocab" {
				vocabChildSeries = append(vocabChildSeries, cs)
			} else {
				uniqueChildSeries = append(uniqueChildSeries, cs)
			}
		}
	}

	// Count comments in user's languages for the peek button badge.
	uiLang := UILangFor(u)
	commentCountMyLang := 0
	for _, c := range comments {
		if c.Language == userLang || c.Language == uiLang {
			commentCountMyLang++
		}
	}

	// Sort comments: user's translate lang first, then UI lang, then rest.
	sortedComments := make([]*model.Comment, 0, len(comments))
	for _, c := range comments {
		if c.Language == userLang {
			sortedComments = append(sortedComments, c)
		}
	}
	for _, c := range comments {
		if c.Language != userLang && c.Language == uiLang {
			sortedComments = append(sortedComments, c)
		}
	}
	for _, c := range comments {
		if c.Language != userLang && c.Language != uiLang {
			sortedComments = append(sortedComments, c)
		}
	}

	data := map[string]interface{}{
		"User":               u,
		"Item":               item,
		"Comments":           sortedComments,
		"CommentCount":       len(comments),
		"CommentCountMyLang": commentCountMyLang,
		"CurrentVote":        currentVote,
		"TradukData":         tradukData,
		"PrevInSeries":       prevInSeries,
		"NextInSeries":       nextInSeries,
		"Glossary":           glossary,
		"NewWords":           newWords(glossary),
		"KnownWords":         knownWords(glossary),
		"Practice":           practice,
		"GameLesson":         h.gameLesson(r.Context(), item),
		"SeriesTotal":        seriesTotal,
		"SeriesPos":          seriesPos,
		"VocabModo":          vocabModo,
		"IsFavorite":         isFavorite,
		"IsFavoriteSeries":   isFavoriteSeries,
		"IsRadiko":           isRadiko,
		"VocabKind":          vocabKind,
		"Meaning":            meaning,
		"ChildSeries":        uniqueChildSeries,
		"VocabChildSeries":   vocabChildSeries,
		"ParentItem":         parentItem,
		"SeriesItems":        seriesItems,
		"UILang":             UILangFor(u),
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
		items, err = h.content.ListByTag(r.Context(), tag, 0)
		if err != nil {
			items = nil
		}
		items = keepStandalone(items, h.readSet(r.Context(), u))
		if len(items) > 200 {
			items = items[:200]
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
		items, err = h.content.ListByType(r.Context(), typ, 0)
	case cefr != "":
		minR, maxR := cefrToRatingRange(cefr)
		items, err = h.content.ListByRatingRange(r.Context(), minR, maxR, 0)
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	items = keepStandalone(items, h.readSet(r.Context(), u))
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

// ToggleFavoriteSeries handles POST /serio/{series_slug}/steli — adds/removes a series favorite.
func (h *ContentHandler) ToggleFavoriteSeries(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
		return
	}
	seriesSlug := r.PathValue("series_slug")
	if seriesSlug == "" {
		http.NotFound(w, r)
		return
	}
	_, err := h.users.ToggleFavorite(r.Context(), u.ID, "series:"+seriesSlug)
	if err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// ToggleFavoriteTag handles POST /etikedo/{tag}/steli — adds/removes a tag favorite.
func (h *ContentHandler) ToggleFavoriteTag(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
		return
	}
	tag := r.PathValue("tag")
	if tag == "" {
		http.NotFound(w, r)
		return
	}
	_, err := h.users.ToggleFavorite(r.Context(), u.ID, "tag:"+tag)
	if err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// ShowFavorites handles GET /steloj — lists the user's starred exercises, series, and tags.
func (h *ContentHandler) ShowFavorites(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
		return
	}
	type seriesEntry struct {
		Slug      string
		Label     string
		FirstSlug string
	}
	var exercises []*model.ContentItem
	var seriesEntries []seriesEntry
	var tags []string
	for _, fav := range u.Favorites {
		if strings.HasPrefix(fav, "series:") {
			slug := strings.TrimPrefix(fav, "series:")
			items, _ := h.content.ListBySeries(r.Context(), slug)
			label := slug
			firstSlug := slug
			if len(items) > 0 {
				if items[0].SeriesLabel != "" {
					label = items[0].SeriesLabel
				}
				firstSlug = items[0].Slug
			}
			seriesEntries = append(seriesEntries, seriesEntry{Slug: slug, Label: label, FirstSlug: firstSlug})
		} else if strings.HasPrefix(fav, "tag:") {
			tags = append(tags, strings.TrimPrefix(fav, "tag:"))
		} else {
			item, _ := h.content.GetBySlug(r.Context(), fav)
			if item != nil {
				exercises = append(exercises, item)
			}
		}
	}
	data := map[string]interface{}{
		"User":          u,
		"Exercises":     exercises,
		"SeriesEntries": seriesEntries,
		"Tags":          tags,
		"UILang":        UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "steloj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
