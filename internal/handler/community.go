package handler

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

// CommunityHandler handles voting, commenting, translations, and mod messages.
type CommunityHandler struct {
	tmpl         Renderer
	content      *store.ContentStore
	votes        *store.VoteStore
	comments     *store.CommentStore
	translations *store.TranslationStore
	modMessages  *store.ModMessageStore
	users        *store.UserStore
}

// NewCommunityHandler creates a CommunityHandler.
func NewCommunityHandler(
	tmpl Renderer,
	content *store.ContentStore,
	votes *store.VoteStore,
	comments *store.CommentStore,
	translations *store.TranslationStore,
	modMessages *store.ModMessageStore,
	users *store.UserStore,
) *CommunityHandler {
	return &CommunityHandler{
		tmpl:         tmpl,
		content:      content,
		votes:        votes,
		comments:     comments,
		translations: translations,
		modMessages:  modMessages,
		users:        users,
	}
}

// VoteCompact handles POST /vochdonado/{contentID}/kompakta — same as Vote but returns compact template.
func (h *CommunityHandler) VoteCompact(w http.ResponseWriter, r *http.Request) {
	h.vote(w, r, "vote-kompakta.html")
}

// Vote handles POST /vochdonado/{contentID}.
func (h *CommunityHandler) Vote(w http.ResponseWriter, r *http.Request) {
	h.vote(w, r, "vochdonado.html")
}

func (h *CommunityHandler) vote(w http.ResponseWriter, r *http.Request, tmplName string) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Bonvolu ensaluti", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	valueStr := r.FormValue("value")
	var newValue int
	switch valueStr {
	case "1":
		newValue = 1
	case "-1":
		newValue = -1
	default:
		http.Error(w, "Nevalida voĉo", http.StatusBadRequest)
		return
	}

	// Determine delta from existing vote.
	// Clicking the same button again removes the vote (toggle to 0).
	existing, _ := h.votes.GetByUserAndContent(r.Context(), u.ID, contentID)
	var delta int
	var effectiveValue int
	if existing != nil && existing.Value == newValue {
		// Toggle off: remove existing vote.
		effectiveValue = 0
		delta = -existing.Value
	} else if existing != nil {
		effectiveValue = newValue
		delta = newValue - existing.Value
	} else {
		effectiveValue = newValue
		delta = newValue
	}

	vote := &model.Vote{
		UserID:        u.ID,
		ContentItemID: contentID,
		Value:         effectiveValue,
	}
	if err := h.votes.Upsert(r.Context(), vote); err != nil {
		http.Error(w, "Ne eblis konservi voĉon", http.StatusInternalServerError)
		return
	}

	if delta != 0 {
		_ = h.content.UpdateVoteScore(r.Context(), contentID, delta)
	}

	item, _ := h.content.GetBySlug(r.Context(), contentID)
	var voteScore int
	if item != nil {
		voteScore = item.VoteScore + delta
	}

	var currentVote *model.Vote
	if effectiveValue != 0 {
		currentVote = vote
	}

	data := map[string]interface{}{
		"ContentID":   contentID,
		"VoteScore":   voteScore,
		"CurrentVote": currentVote,
		"User":        u,
		"UILang":      UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, tmplName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// AddComment handles POST /komentoj/{contentID}.
func (h *CommunityHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Bonvolu ensaluti", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	text := r.FormValue("text")
	if text == "" {
		http.Error(w, "Malplena komento", http.StatusBadRequest)
		return
	}

	// Auto-approve if user is well-calibrated and close in rating to content.
	item, _ := h.content.GetBySlug(r.Context(), contentID)
	autoApprove := false
	if item != nil && u.RD < 100 && math.Abs(u.Rating-item.Rating) < 300 {
		autoApprove = true
	}

	comment := &model.Comment{
		UserID:        u.ID,
		ContentItemID: contentID,
		Text:          text,
		Approved:      autoApprove,
		AutoApproved:  autoApprove,
		Language:      UILangFor(u),
	}
	if err := h.comments.Create(r.Context(), comment); err != nil {
		http.Error(w, "Ne eblis konservi komenton", http.StatusInternalServerError)
		return
	}

	comments, _ := h.comments.ListApprovedByContent(r.Context(), contentID)
	if autoApprove {
		comment.Username = u.Username
		comments = append(comments, comment)
	}
	h.users.ResolveUsernames(r.Context(), comments)

	data := map[string]interface{}{
		"ContentID": contentID,
		"Comments":  comments,
		"User":      u,
		"UILang":    UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "komentoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// buildVoteMap returns a map of translationID → current user vote value.
func buildVoteMap(ctx context.Context, ts *store.TranslationStore, userID string, translations []*model.Translation) map[string]int {
	votes := map[string]int{}
	for _, t := range translations {
		if v, _ := ts.GetVote(ctx, userID, t.ID); v != 0 {
			votes[t.ID] = v
		}
	}
	return votes
}

// buildTradukData builds the data map for the traduko.html partial.
func buildTradukData(contentID, userLang, userID string, translations []*model.Translation, votes map[string]int) map[string]interface{} {
	var mine, other []*model.Translation
	for _, t := range translations {
		if t.Language == userLang {
			mine = append(mine, t)
		} else {
			other = append(other, t)
		}
	}
	return map[string]interface{}{
		"ContentID":            contentID,
		"UserLang":             userLang,
		"MyLangTranslations":   mine,
		"OtherTranslations":    other,
		"TranslationVotes":     votes,
	}
}

// AddTranslation handles POST /tradukoj/{contentID}.
func (h *CommunityHandler) AddTranslation(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Bonvolu ensaluti", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	lang := strings.TrimSpace(r.FormValue("language"))
	text := strings.TrimSpace(r.FormValue("text"))
	if lang == "" || text == "" {
		http.Error(w, "Mankas lingvo aŭ teksto", http.StatusBadRequest)
		return
	}

	t := &model.Translation{
		TargetID:       contentID,
		Language:       lang,
		Text:           text,
		AuthorID:       u.ID,
		AuthorUsername: u.DisplayName(),
	}
	if err := h.translations.Create(r.Context(), t); err != nil {
		http.Error(w, "Ne eblis konservi tradukon", http.StatusInternalServerError)
		return
	}

	// When submitted from the exercise page, redirect so the full page reloads
	// with the new definition visible. Pass lang+text as query params so the
	// handler can inject the translation immediately (Datastore queries are
	// eventually consistent and may not return the new entry right away).
	if r.FormValue("from") == "ekzerco" {
		u2 := url.Values{}
		u2.Set("added_lang", lang)
		u2.Set("added_def", text)
		http.Redirect(w, r, "/ekzerco/"+contentID+"?"+u2.Encode(), http.StatusSeeOther)
		return
	}

	translations, _ := h.translations.ListByTarget(r.Context(), contentID)
	votes := buildVoteMap(r.Context(), h.translations, u.ID, translations)
	userLang := "en"
	if u != nil {
		userLang = u.Lang
	}
	data := buildTradukData(contentID, userLang, u.ID, translations, votes)
	data["User"] = u
	data["UILang"] = UILangFor(u)
	if err := h.tmpl.ExecuteTemplate(w, "traduko.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// VoteTranslation handles POST /tradukoj/{contentID}/vochdoni/{id}.
func (h *CommunityHandler) VoteTranslation(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	translationID := r.PathValue("id")
	if contentID == "" || translationID == "" {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Bonvolu ensaluti", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	valueStr := r.FormValue("value")
	var newValue int
	switch valueStr {
	case "1":
		newValue = 1
	case "-1":
		newValue = -1
	default:
		http.Error(w, "Nevalida voĉo", http.StatusBadRequest)
		return
	}

	if _, err := h.translations.Vote(r.Context(), u.ID, translationID, newValue); err != nil {
		http.Error(w, "Ne eblis voĉdoni", http.StatusInternalServerError)
		return
	}

	translations, _ := h.translations.ListByTarget(r.Context(), contentID)
	votes := buildVoteMap(r.Context(), h.translations, u.ID, translations)
	userLang := "en"
	if u != nil {
		userLang = u.Lang
	}
	data := buildTradukData(contentID, userLang, u.ID, translations, votes)
	data["User"] = u
	data["UILang"] = UILangFor(u)
	if err := h.tmpl.ExecuteTemplate(w, "traduko.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// EditTranslation handles POST /tradukoj/{contentID}/redakti/{id}.
// The author of the translation (or a mod/admin) can update its text.
func (h *CommunityHandler) EditTranslation(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	translationID := r.PathValue("id")
	if contentID == "" || translationID == "" {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Bonvolu ensaluti", http.StatusUnauthorized)
		return
	}

	// Check ownership or mod/admin role.
	existing, err := h.translations.GetByID(r.Context(), translationID)
	if err != nil {
		http.Error(w, "Traduko ne trovita", http.StatusNotFound)
		return
	}
	if existing.AuthorID != u.ID && u.Role != "mod" && u.Role != "admin" {
		http.Error(w, "Vi ne rajtas redakti ĉi tiun tradukon", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}
	newText := strings.TrimSpace(r.FormValue("text"))
	if newText == "" {
		http.Error(w, "Teksto ne povas esti malplena", http.StatusBadRequest)
		return
	}

	if err := h.translations.UpdateText(r.Context(), translationID, newText); err != nil {
		http.Error(w, "Ne eblis konservi ŝanĝon", http.StatusInternalServerError)
		return
	}

	// Re-render the translation section.
	translations, _ := h.translations.ListByTarget(r.Context(), contentID)
	votes := buildVoteMap(r.Context(), h.translations, u.ID, translations)
	userLang := "en"
	if u != nil {
		userLang = u.Lang
	}
	data := buildTradukData(contentID, userLang, u.ID, translations, votes)
	data["User"] = u
	data["UILang"] = UILangFor(u)
	if err := h.tmpl.ExecuteTemplate(w, "traduko.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// SuggestAlternative handles POST /ekzerco/{slug}/alternativo.
// Any logged-in user can suggest their wrong answer should be accepted as correct.
// The suggestion is queued as a mod message for admin review.
func (h *CommunityHandler) SuggestAlternative(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	answer := strings.TrimSpace(r.FormValue("answer"))
	if answer == "" || len(answer) > 500 {
		http.Error(w, "Malvalida respondo", http.StatusBadRequest)
		return
	}
	msg := &model.ModMessage{
		UserID:        u.ID,
		Username:      u.DisplayName(),
		ContentItemID: slug,
		Text:          "[alternativo] ekzerco: " + slug + "\nSuggestita respondo: " + answer,
	}
	if err := h.modMessages.Create(r.Context(), msg); err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	uiLang := UILangFor(u)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "alternativo-konfirmo.html", map[string]interface{}{
		"UILang": uiLang,
	})
}

// FlagExercise handles POST /ekzerco/{slug}/flagi.
// Any logged-in user can flag an exercise as having an error; queued as mod message.
func (h *CommunityHandler) FlagExercise(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	comment := strings.TrimSpace(r.FormValue("comment"))
	note := "Uzanto raportis problemon en ĉi tiu ekzerco."
	if comment != "" && len(comment) <= 500 {
		note = comment
	}
	msg := &model.ModMessage{
		UserID:        u.ID,
		Username:      u.DisplayName(),
		ContentItemID: slug,
		Text:          "[eraro-raporto] ekzerco: " + slug + "\n" + note,
	}
	if err := h.modMessages.Create(r.Context(), msg); err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	uiLang := UILangFor(u)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.ExecuteTemplate(w, "flag-konfirmo.html", map[string]interface{}{
		"UILang": uiLang,
	})
}

// SendModMessage handles POST /kontaktu — logged-in users send a message to mods.
func (h *CommunityHandler) SendModMessage(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(r.FormValue("teksto"))
	if text == "" || len(text) > 2000 {
		http.Error(w, "Mesaĝo devas havi 1–2000 signojn", http.StatusBadRequest)
		return
	}
	msg := &model.ModMessage{
		UserID:   u.ID,
		Username: u.DisplayName(),
		Text:     text,
	}
	if err := h.modMessages.Create(r.Context(), msg); err != nil {
		http.Error(w, "Eraro dum sendado", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/enskribi?sendita=1", http.StatusSeeOther)
}
