package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

// AdminHandler bundles all admin HTTP handlers.
type AdminHandler struct {
	tmpl         Renderer
	content      *store.ContentStore
	comments     *store.CommentStore
	users        *store.UserStore
	modMessages  *store.ModMessageStore
	translations *store.TranslationStore
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	users *store.UserStore,
	modMessages *store.ModMessageStore,
	translations *store.TranslationStore,
) *AdminHandler {
	return &AdminHandler{
		tmpl:         tmpl,
		content:      content,
		comments:     comments,
		users:        users,
		modMessages:  modMessages,
		translations: translations,
	}
}

// Dashboard handles GET /admin.
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	approved, _ := h.content.ListForAdmin(r.Context(), "approved", 1000)
	pending, _ := h.content.ListForAdmin(r.Context(), "pending", 1000)
	pendingComments, _ := h.comments.ListPending(r.Context(), 100)
	unreadMessages, _ := h.modMessages.ListUnread(r.Context(), 50)
	pendingTranslations, _ := h.translations.ListAll(r.Context(), 200)

	data := map[string]interface{}{
		"User":               u,
		"ApprovedCount":      len(approved),
		"PendingCount":       len(pending),
		"CommentCount":       len(pendingComments),
		"ModMessageCount":    len(unreadMessages),
		"TranslationCount":   len(pendingTranslations),
		"SeedResult":         r.URL.Query().Get("seed"),
		"ImportResult":       r.URL.Query().Get("import"),
		"NukeResult":         r.URL.Query().Get("nuke"),
		"UILang":             UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "admin_dashboard.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ListContent handles GET /admin/enhavo.
func (h *AdminHandler) ListContent(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	statusFilter := r.URL.Query().Get("status")

	items, err := h.content.ListForAdmin(r.Context(), statusFilter, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":         u,
		"Items":        items,
		"StatusFilter": statusFilter,
		"UILang":       UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "listo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// NewContentForm handles GET /admin/enhavo/nova.
func (h *AdminHandler) NewContentForm(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	data := map[string]interface{}{
		"User": u,
		"Item": &model.ContentItem{
			Rating:     1500,
			RD:         350,
			Volatility: 0.06,
			Status:     "draft",
		},
		"IsNew":  true,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "redaktilo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CreateContent handles POST /admin/enhavo.
func (h *AdminHandler) CreateContent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	u := UserFromContext(r.Context())
	authorID := ""
	if u != nil {
		authorID = u.ID
	}

	item := buildContentItem(r, authorID)
	if item.Slug == "" {
		http.Error(w, "Identigilo estas deviga", http.StatusBadRequest)
		return
	}

	if err := h.content.Create(r.Context(), item); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/enhavo", http.StatusSeeOther)
}

// EditContentForm handles GET /admin/enhavo/{slug}/redakti.
func (h *AdminHandler) EditContentForm(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	data := map[string]interface{}{
		"User":   u,
		"Item":   item,
		"IsNew":  false,
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "redaktilo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// UpdateContent handles POST /admin/enhavo/{slug}.
func (h *AdminHandler) UpdateContent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	slug := r.PathValue("slug")
	existing, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || existing == nil {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	authorID := existing.AuthorID
	if authorID == "" && u != nil {
		authorID = u.ID
	}

	updated := buildContentItem(r, authorID)
	updated.Slug = slug // keep original slug
	updated.Rating = existing.Rating
	updated.RD = existing.RD
	updated.Volatility = existing.Volatility
	updated.VoteScore = existing.VoteScore
	updated.CreatedAt = existing.CreatedAt
	updated.Version = existing.Version + 1

	if err := h.content.Update(r.Context(), updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/enhavo", http.StatusSeeOther)
}

// VocabFromReading handles GET /admin/enhavo/{slug}/vortaro.
// Shows words extracted from the reading text, split into "already in dictionary"
// and "not yet added". The admin can select words and POST to create vocab items.
func (h *AdminHandler) VocabFromReading(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	text := item.Text()
	extracted := eo.ExtractWords(text)

	// Find which words already have a vocab item (slug voc-auto-WORD or voc-ANYTHING with matching word).
	// We use a simple heuristic: scan all vocab items and collect their word fields.
	allItems, _ := h.content.ListByType(r.Context(), "vocab", 2000)
	existing := make(map[string]bool)
	for _, v := range allItems {
		if w, ok := v.Content["word"].(string); ok {
			existing[strings.ToLower(strings.TrimSpace(w))] = true
		}
	}

	type wordEntry struct {
		Word    string
		Slug    string
		Exists  bool
	}
	var words []wordEntry
	for _, w := range extracted {
		words = append(words, wordEntry{
			Word:   w,
			Slug:   "voc-auto-" + eo.WordToSlug(w),
			Exists: existing[w],
		})
	}

	u := UserFromContext(r.Context())
	created, _ := strconv.Atoi(r.URL.Query().Get("created"))
	skipped, _ := strconv.Atoi(r.URL.Query().Get("skipped"))
	data := map[string]interface{}{
		"User":    u,
		"Item":    item,
		"Words":   words,
		"Created": created,
		"Skipped": skipped,
		"UILang":  UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "admin_vortaro_gen.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CreateVocabFromReading handles POST /admin/enhavo/{slug}/vortaro.
// Creates vocab items for the selected words.
func (h *AdminHandler) CreateVocabFromReading(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta formularo", http.StatusBadRequest)
		return
	}

	slug := r.PathValue("slug")
	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	authorID := ""
	if u != nil {
		authorID = u.ID
	}

	words := r.Form["word"]
	created := 0
	skipped := 0
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		vocSlug := "voc-auto-" + eo.WordToSlug(word)
		existing, _ := h.content.GetBySlug(r.Context(), vocSlug)
		if existing != nil {
			skipped++
			continue
		}
		// Tag with "vortaro", the reading's slug (so /vortaro?tag=slug works), and the reading's own tags.
		tags := append([]string{"vortaro", item.Slug}, item.Tags...)
		seen := make(map[string]bool)
		var uniqueTags []string
		for _, t := range tags {
			if !seen[t] {
				seen[t] = true
				uniqueTags = append(uniqueTags, t)
			}
		}
		voc := &model.ContentItem{
			Slug:     vocSlug,
			Type:     "vocab",
			Content:  map[string]interface{}{"word": word},
			Tags:     uniqueTags,
			Source:   item.Source,
			AuthorID: authorID,
			Status:   "approved",
			Rating:   item.Rating,
			RD:       200,
		}
		if err := h.content.Create(r.Context(), voc); err == nil {
			created++
		}
	}

	redir := fmt.Sprintf("/admin/enhavo/%s/vortaro?created=%d&skipped=%d", slug, created, skipped)
	http.Redirect(w, r, redir, http.StatusSeeOther)
}

// ModerationQueue handles GET /admin/moderigo.
func (h *AdminHandler) ModerationQueue(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	comments, err := h.comments.ListPending(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allMessages, _ := h.modMessages.ListUnread(r.Context(), 100)
	translations, _ := h.translations.ListAll(r.Context(), 200)

	// Split mod messages into contact messages vs. automated reports.
	var contactMessages, reportMessages []*model.ModMessage
	for _, m := range allMessages {
		if strings.HasPrefix(m.Text, "[alternativo]") || strings.HasPrefix(m.Text, "[eraro-raporto]") {
			reportMessages = append(reportMessages, m)
		} else {
			contactMessages = append(contactMessages, m)
		}
	}

	data := map[string]interface{}{
		"User":            u,
		"Comments":        comments,
		"ContactMessages": contactMessages,
		"ReportMessages":  reportMessages,
		"Translations":    translations,
		"UILang":          UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "moderigo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ApproveTranslation handles POST /admin/tradukoj/{id}/aprobi.
// It writes the translation text to content["definitions"]["lang"] and deletes the Translation entry.
func (h *AdminHandler) ApproveTranslation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	t, err := h.translations.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Traduko ne trovita", http.StatusNotFound)
		return
	}
	if err := h.content.UpdateDefinition(r.Context(), t.TargetID, t.Language, t.Text); err != nil {
		http.Error(w, "Ne eblis ĝisdatigi enhavon: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.translations.Delete(r.Context(), id)
	http.Redirect(w, r, "/admin/moderigo", http.StatusSeeOther)
}

// DeleteTranslation handles POST /admin/tradukoj/{id}/forigi.
func (h *AdminHandler) DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	_ = h.translations.Delete(r.Context(), id)
	http.Redirect(w, r, "/admin/moderigo", http.StatusSeeOther)
}

// ModerateComment handles POST /admin/moderigo/{id}.
func (h *AdminHandler) ModerateComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	action := r.FormValue("action")
	var err error
	switch action {
	case "aprobi":
		err = h.comments.Approve(r.Context(), id)
	case "malakcepti":
		err = h.comments.Reject(r.Context(), id)
	default:
		http.Error(w, "Nekonata ago", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/moderigo", http.StatusSeeOther)
}

// buildContentItem constructs a ContentItem from an HTTP form.
func buildContentItem(r *http.Request, authorID string) *model.ContentItem {
	tagsRaw := r.FormValue("tags")
	var tags []string
	for _, t := range strings.Split(tagsRaw, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}

	contentMap := map[string]interface{}{
		"question":  r.FormValue("question"),
		"answer":    r.FormValue("answer"),
		"hint":      r.FormValue("hint"),
		"audio_url": r.FormValue("audio_url"),
		"video_url": r.FormValue("video_url"),
		"word":      r.FormValue("word"),
		"title":     r.FormValue("title"),
		"text":      r.FormValue("text"),
	}

	// Parse multilingual definitions (format: "lang: text" per line).
	if rawDefs := r.FormValue("definitions"); rawDefs != "" {
		defs := map[string]interface{}{}
		for _, line := range strings.Split(rawDefs, "\n") {
			line = strings.TrimSpace(line)
			if idx := strings.Index(line, ":"); idx > 0 {
				lang := strings.TrimSpace(line[:idx])
				text := strings.TrimSpace(line[idx+1:])
				if lang != "" && text != "" {
					defs[lang] = text
				}
			}
		}
		if len(defs) > 0 {
			contentMap["definitions"] = defs
		}
	}

	// Parse options (newline-separated).
	optionsRaw := r.FormValue("options")
	var options []string
	for _, o := range strings.Split(optionsRaw, "\n") {
		o = strings.TrimSpace(o)
		if o != "" {
			options = append(options, o)
		}
	}
	if len(options) > 0 {
		contentMap["options"] = options
	}

	// Parse gap answers for fill-in exercises (newline-separated, one per blank).
	if r.FormValue("type") == "fillin" {
		var gapAnswers []string
		for _, a := range strings.Split(r.FormValue("gap_answers"), "\n") {
			a = strings.TrimSpace(a)
			if a != "" {
				gapAnswers = append(gapAnswers, a)
			}
		}
		delete(contentMap, "answer")
		if len(gapAnswers) == 1 {
			contentMap["answer"] = gapAnswers[0]
		} else if len(gapAnswers) > 1 {
			contentMap["answers"] = gapAnswers
		}
	}

	correctIndex, _ := strconv.Atoi(r.FormValue("correct_index"))
	contentMap["correct_index"] = correctIndex

	seriesOrder, _ := strconv.Atoi(r.FormValue("series_order"))
	return &model.ContentItem{
		Slug:        r.FormValue("slug"),
		Type:        r.FormValue("type"),
		Content:     contentMap,
		Tags:        tags,
		Source:      r.FormValue("source"),
		AuthorID:    authorID,
		Status:      r.FormValue("status"),
		Rating:      1500,
		RD:          350,
		Volatility:  0.06,
		ImageURL:    r.FormValue("image_url"),
		SeriesSlug:  r.FormValue("series_slug"),
		SeriesOrder: seriesOrder,
		UpdatedAt:   time.Now(),
	}
}

// DeleteContent handles POST /admin/enhavo/{slug}/forigi.
func (h *AdminHandler) DeleteContent(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	if err := h.content.Delete(r.Context(), slug); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/enhavo", http.StatusSeeOther)
}

// SeedContent handles POST /admin/seed — loads embedded seed data into Datastore.
// Idempotent: skips items that already exist. Redirects back to dashboard with result.
func (h *AdminHandler) SeedContent(w http.ResponseWriter, r *http.Request) {
	loaded, skipped, failed := 0, 0, 0
	allSeedItems := append(seedItems(), seedContentItems()...)
	allSeedItems = append(allSeedItems, seedVideoItems()...)
	allSeedItems = append(allSeedItems, seedExtraItems()...)
	allSeedItems = append(allSeedItems, seedFillinItems()...)
	for _, item := range allSeedItems {
		existing, _ := h.content.GetBySlug(r.Context(), item.Slug)
		if existing != nil {
			skipped++
			continue
		}
		if err := h.content.Create(r.Context(), item); err != nil {
			failed++
			continue
		}
		loaded++
	}
	msg := fmt.Sprintf("Semo: %d ŝargitaj, %d preterlasitaj, %d malsukcesaj", loaded, skipped, failed)
	http.Redirect(w, r, "/admin?seed="+url.QueryEscape(msg), http.StatusSeeOther)
}

// PatchSeedContent handles POST /admin/patch-seed — updates only the content/tags fields
// of existing seed items (preserving ratings, votes, etc.) and creates missing ones.
func (h *AdminHandler) PatchSeedContent(w http.ResponseWriter, r *http.Request) {
	updated, created, failed := 0, 0, 0
	allSeedItems := append(seedItems(), seedContentItems()...)
	allSeedItems = append(allSeedItems, seedVideoItems()...)
	allSeedItems = append(allSeedItems, seedExtraItems()...)
	allSeedItems = append(allSeedItems, seedFillinItems()...)
	for _, item := range allSeedItems {
		existing, _ := h.content.GetBySlug(r.Context(), item.Slug)
		if err := h.content.PatchContentFields(r.Context(), item); err != nil {
			failed++
			continue
		}
		if existing == nil {
			created++
		} else {
			updated++
		}
	}
	msg := fmt.Sprintf("Flikaĵo: %d ĝisdatigitaj, %d novaj, %d malsukcesaj", updated, created, failed)
	http.Redirect(w, r, "/admin?seed="+url.QueryEscape(msg), http.StatusSeeOther)
}

// ExportContent handles GET /admin/eksporti — streams all content items as a JSON array.
func (h *AdminHandler) ExportContent(w http.ResponseWriter, r *http.Request) {
	items, err := h.content.ListForAdmin(r.Context(), "", 10000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="ekzercoj.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		// headers already sent, can't change status
		return
	}
}

// ImportContent handles POST /admin/importi — imports a JSON array of content items.
// Existing slugs are overwritten; new slugs are created.
func (h *AdminHandler) ImportContent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 32<<20) // 32 MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusBadRequest)
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Mankas dosiero: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "Eraro de legado: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var items []*model.ContentItem
	if err := json.Unmarshal(data, &items); err != nil {
		http.Error(w, "Nevalida JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	created, updated, failed := 0, 0, 0
	for _, item := range items {
		if item.Slug == "" {
			failed++
			continue
		}
		existing, _ := h.content.GetBySlug(ctx, item.Slug)
		if existing != nil {
			item.CreatedAt = existing.CreatedAt
			if err := h.content.Update(ctx, item); err != nil {
				failed++
			} else {
				updated++
			}
		} else {
			if err := h.content.Create(ctx, item); err != nil {
				failed++
			} else {
				created++
			}
		}
	}
	msg := fmt.Sprintf("Importo: %d novaj, %d ĝisdatigitaj, %d malsukcesaj", created, updated, failed)
	http.Redirect(w, r, "/admin?import="+url.QueryEscape(msg), http.StatusSeeOther)
}

// NukeContent handles POST /admin/forigi-cion — deletes ALL content items.
func (h *AdminHandler) NukeContent(w http.ResponseWriter, r *http.Request) {
	items, err := h.content.ListForAdmin(r.Context(), "", 10000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deleted, failed := 0, 0
	for _, item := range items {
		if err := h.content.Delete(r.Context(), item.Slug); err != nil {
			failed++
		} else {
			deleted++
		}
	}
	msg := fmt.Sprintf("Forigitaj: %d, malsukcesaj: %d", deleted, failed)
	http.Redirect(w, r, "/admin?nuke="+url.QueryEscape(msg), http.StatusSeeOther)
}

// MarkModMessageRead handles POST /admin/mesagxoj/{id}/legita.
func (h *AdminHandler) MarkModMessageRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = h.modMessages.MarkRead(r.Context(), id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// SetUserRole handles POST /admin/uzantoj/{id}/rolo — sets role to "user", "mod", or "admin".
func (h *AdminHandler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	role := r.FormValue("rolo")
	if role != "user" && role != "mod" && role != "admin" {
		http.Error(w, "Nevalida rolo", http.StatusBadRequest)
		return
	}
	target, err := h.users.GetByID(r.Context(), userID)
	if err != nil || target == nil {
		http.Error(w, "Uzanto ne trovita", http.StatusNotFound)
		return
	}
	target.Role = role
	if err := h.users.Update(r.Context(), target); err != nil {
		http.Error(w, "Eraro", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/uzantoj?msg=rolo+ŝanĝita", http.StatusSeeOther)
}

// ListUsers handles GET /admin/uzantoj — search users by username or ID.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	search := r.URL.Query().Get("s")
	var found []*model.User
	if search != "" {
		if byName, err := h.users.GetByUsername(r.Context(), search); err == nil && byName != nil {
			found = append(found, byName)
		} else if byID, err := h.users.GetByID(r.Context(), search); err == nil && byID != nil {
			found = append(found, byID)
		}
	}
	data := map[string]interface{}{
		"User":   u,
		"Search": search,
		"Found":  found,
		"Msg":    r.URL.Query().Get("msg"),
		"UILang": UILangFor(u),
	}
	if err := h.tmpl.ExecuteTemplate(w, "admin_uzantoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// MergeUsers handles POST /admin/uzantoj/kunfandi — merges src into dst.
// dst and src may be user IDs or usernames; usernames are resolved automatically.
func (h *AdminHandler) MergeUsers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	dstRef := strings.TrimSpace(r.FormValue("dst"))
	srcRef := strings.TrimSpace(r.FormValue("src"))
	if dstRef == "" || srcRef == "" {
		http.Error(w, "Bezonatas du uzant-referencoj", http.StatusBadRequest)
		return
	}
	dst, err := h.users.ResolveUserRef(r.Context(), dstRef)
	if err != nil {
		http.Error(w, "Eraro (cela uzanto): "+err.Error(), http.StatusBadRequest)
		return
	}
	src, err := h.users.ResolveUserRef(r.Context(), srcRef)
	if err != nil {
		http.Error(w, "Eraro (fonta uzanto): "+err.Error(), http.StatusBadRequest)
		return
	}
	if dst.ID == src.ID {
		http.Error(w, "La du uzantoj estas la sama konto", http.StatusBadRequest)
		return
	}
	if err := h.users.MergeUsers(r.Context(), dst.ID, src.ID); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/uzantoj?msg=kunfandita", http.StatusSeeOther)
}

// UnlinkUsername handles POST /admin/uzantoj/{id}/nomo-forigi — clears a user's username.
func (h *AdminHandler) UnlinkUsername(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	if err := h.users.ClearUsername(r.Context(), id); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/uzantoj?msg=nomo-forigita", http.StatusSeeOther)
}

// seedItems returns the built-in bootstrap dataset (Zagreba Metodo vortaro).
func seedItems() []*model.ContentItem {
	type si struct {
		slug, typ    string
		content      map[string]interface{}
		tags         []string
		source       string
		rating, rd   float64
		seriesSlug   string
		seriesOrder  int
	}
	items := []si{
		{
			slug: "vortaro-radiko-afer", typ: "vocab",
			content: map[string]interface{}{
				"word": "afer",
				"definition": "matter",
				"definitions": map[string]interface{}{"en": "matter", "nl": "aangelegenheid, zaak", "de": "Angelegenheit, Sache", "fr": "affaire", "es": "asunto, cosa", "pt": "assunto, coisa", "ar": "مسألة, قضية", "be": "дело", "ca": "assumpte, cosa", "cs": "věc, záležitost", "da": "ting", "el": "ζήτημα", "fa": "چیز, کاروبار, موضوع", "frp": "affaire", "ga": "ábhar", "he": "עניין", "hi": "matter", "hr": "stvar", "hu": "dolog", "id": "hal", "it": "cosa", "ja": "ことがら", "kk": "matter", "km": "matter", "ko": "사건, 일, 사무", "ku": "چیز, کاروبار, موضوع", "lo": "matter", "mg": "raharaha", "ms": "perkara, kata akar", "my": "matter", "pl": "sprawa", "ro": "affaire", "ru": "дело", "sk": "vec, záležitosť", "sl": "zadeva", "sv": "sak, företeelse", "sw": "affaire", "th": "เรื่องราว, ข้าวของ", "tok": "matter", "tr": "mesele, konu, şey", "uk": "справа, діло, заняття", "ur": "معاملہ", "vi": "vật, việc", "yo": "matter", "zh-tw": "事情", "zh": "事情,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-radiko-agrabl", typ: "vocab",
			content: map[string]interface{}{
				"word": "agrabl",
				"definition": "pleasant",
				"definitions": map[string]interface{}{"en": "pleasant", "nl": "aangenaam", "de": "angenehm", "fr": "agréable", "es": "agradable", "pt": "agradável", "ar": "ممتع", "be": "приятный", "ca": "agradable", "cs": "příjemný", "da": "behagelig", "el": "ευχάριστος-η-ο", "fa": "دلپذیر", "frp": "agréable", "ga": "taitneamhach", "he": "מעים", "hi": "pleasant", "hr": "ugodan", "hu": "kellemes", "id": "menyenangkan, baik (sifat)", "it": "piacevole", "ja": "気持ちよい", "kk": "pleasant", "km": "pleasant", "ko": "유쾌한", "ku": "دلپذیر", "lo": "pleasant", "mg": "mahafinaritra", "ms": "menyenangkan, kata akar", "my": "pleasant", "pl": "miły", "ro": "agréable", "ru": "приятный", "sk": "príjemný", "sl": "ugodno", "sv": "trevlig", "sw": "agréable", "th": "ยินดี", "tok": "pleasant", "tr": "hoş", "uk": "приємний", "ur": "خوشگوار", "vi": "dễ chịu", "yo": "pleasant", "zh-tw": "愉快", "zh": "愉快,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-radiko-akcept", typ: "vocab",
			content: map[string]interface{}{
				"word": "akcept",
				"definition": "to accept",
				"definitions": map[string]interface{}{"en": "to accept", "nl": "aanvaarden", "de": "akzeptieren", "fr": "accepter", "es": "aceptar", "pt": "aceitar", "ar": "قبول, رضى", "be": "принимать, принять", "ca": "acceptar", "cs": "přijmout", "da": "akceptere", "el": "αποδεκτός-η-ο", "fa": "پذیرفتن", "frp": "accepter", "ga": "glac le", "he": "לקבל", "hi": "accept", "hr": "prihvatiti, primiti", "hu": "fogad", "id": "terima", "it": "accettare", "ja": "受け取る", "kk": "accept", "km": "ទទួលយក", "ko": "받아 들이다", "ku": "پذیرفتن", "lo": "accept", "mg": "manaiky", "ms": "terima, kata akar", "my": "accept", "pl": "akceptować", "ro": "accepter", "ru": "принимать, принять", "sk": "prijať", "sl": "sprejeti", "sv": "acceptera", "sw": "accepter", "th": "ยอมรับ", "tok": "to accept", "tr": "kabul etmek", "uk": "приймати", "ur": "قبول کرنا", "vi": "chấp nhận", "yo": "to accept", "zh-tw": "接受", "zh": "接受,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-radiko-akv", typ: "vocab",
			content: map[string]interface{}{
				"word": "akv",
				"definition": "water",
				"definitions": map[string]interface{}{"en": "water", "nl": "water", "de": "Wasser", "fr": "eau", "es": "agua", "pt": "água", "ar": "ماء", "be": "вода", "ca": "aigua", "cs": "voda", "da": "vand", "el": "νερό", "fa": "آب", "frp": "eau", "ga": "uisce", "he": "מים", "hi": "water", "hr": "voda", "hu": "víz", "id": "air", "it": "acqua", "ja": "水", "kk": "water", "km": "water", "ko": "물", "ku": "آب", "lo": "water", "mg": "rano", "ms": "air, kata akar", "my": "water", "pl": "woda", "ro": "eau", "ru": "вода", "sk": "voda", "sl": "voda", "sv": "vatten", "sw": "eau", "th": "น้ำ", "tok": "water", "tr": "su", "uk": "вода", "ur": "پانی", "vi": "nước", "yo": "water", "zh-tw": "水", "zh": "水,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-radiko-ali", typ: "vocab",
			content: map[string]interface{}{
				"word": "ali",
				"definition": "another",
				"definitions": map[string]interface{}{"en": "another", "nl": "ander", "de": "anderer", "fr": "autre", "es": "otro/a", "pt": "outro", "ar": "آخر", "be": "другой", "ca": "altre/a", "cs": "jiný", "da": "anden", "el": "άλλος-η-ο", "fa": "دیگر", "frp": "autre", "ga": "eile", "he": "אחר", "hi": "another", "hr": "drugi", "hu": "más", "id": "lain", "it": "altro", "ja": "ほかの", "kk": "another", "km": "ផ្សេងទៀត", "ko": "다른", "ku": "دیگر", "lo": "another", "mg": "hafa", "ms": "lain, kata akar", "my": "another", "pl": "inny", "ro": "autre", "ru": "другой", "sk": "iný", "sl": "drugi", "sv": "annan", "sw": "autre", "th": "อื่น ๆ", "tok": "another", "tr": "diğer", "uk": "інший", "ur": "ایک اور", "vi": "khác", "yo": "another", "zh-tw": "另一個", "zh": "另一个,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-radiko-alt", typ: "vocab",
			content: map[string]interface{}{
				"word": "alt",
				"definition": "high",
				"definitions": map[string]interface{}{"en": "high", "nl": "hoog", "de": "hohe/r/s, hoch", "fr": "haut", "es": "alto/a", "pt": "alto, alta", "ar": "عالي", "be": "высокий", "ca": "alt/a", "cs": "výška", "da": "høj", "el": "ψηλός-η-ο", "fa": "بلند", "frp": "haut", "ga": "ard", "he": "גבוה", "hi": "high", "hr": "visok", "hu": "magas", "id": "tinggi", "it": "alto", "ja": "高い", "kk": "high", "km": "ខ្ពស់", "ko": "높은", "ku": "بلند", "lo": "high", "mg": "avo", "ms": "tinggi, kata akar", "my": "high", "pl": "wysoki", "ro": "haut", "ru": "высокий", "sk": "výška", "sl": "visok", "sv": "hög", "sw": "haut", "th": "สูง", "tok": "high", "tr": "yüksek", "uk": "високий", "ur": "اونچا", "vi": "cao", "yo": "high", "zh-tw": "高", "zh": "高,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-radiko-am", typ: "vocab",
			content: map[string]interface{}{
				"word": "am",
				"definition": "to love",
				"definitions": map[string]interface{}{"en": "to love", "nl": "beminnen", "de": "lieben", "fr": "aimer", "es": "amar", "pt": "amor, amar", "ar": "حب", "be": "любовь", "ca": "estimar, amar", "cs": "láska", "da": "kærlighed", "el": "το να αγαπώ", "fa": "عاشق بودن", "frp": "aimer", "ga": "grá", "he": "אהבה", "hi": "love", "hr": "ljubiti, voljeti", "hu": "szeret", "id": "cinta", "it": "amore", "ja": "愛する", "kk": "love", "km": "love", "ko": "사랑", "ku": "عاشق بودن", "lo": "love", "mg": "tia", "ms": "sayang, kata akar, cinta, kata akar", "my": "love", "pl": "miłość", "ro": "aimer", "ru": "любовь", "sk": "láska", "sl": "ljubezen", "sv": "kärlek, älska", "sw": "aimer", "th": "รัก", "tok": "to love", "tr": "sevmek; sevgi, aşk", "uk": "кохати, любити", "ur": "محبت", "vi": "yêu dấu", "yo": "to love", "zh-tw": "愛", "zh": "爱,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-radiko-amik", typ: "vocab",
			content: map[string]interface{}{
				"word": "amik",
				"definition": "friend",
				"definitions": map[string]interface{}{"en": "friend", "nl": "vriend", "de": "Freund", "fr": "ami", "es": "amigo", "pt": "amigo, amiga", "ar": "صديق", "be": "друг", "ca": "amic", "cs": "přítel", "da": "ven", "el": "φίλος-η-ο", "fa": "دوست", "frp": "ami", "ga": "cara", "he": "חבר", "hi": "friend", "hr": "prijatelj", "hu": "barát", "id": "teman", "it": "amico", "ja": "友", "kk": "дос", "km": "friend", "ko": "친구", "ku": "دوست", "lo": "friend", "mg": "sakaiza", "ms": "kawan, kata akar", "my": "friend", "pl": "przyjaciel", "ro": "ami", "ru": "друг", "sk": "priateľ", "sl": "prijatelj", "sv": "vän", "sw": "ami", "th": "เพื่อน", "tok": "friend", "tr": "arkadaş", "uk": "друг", "ur": "دوست", "vi": "bạn bè", "yo": "friend", "zh-tw": "朋友", "zh": "朋友,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-radiko-arb", typ: "vocab",
			content: map[string]interface{}{
				"word": "arb",
				"definition": "tree",
				"definitions": map[string]interface{}{"en": "tree", "nl": "boom", "de": "Baum", "fr": "arbre", "es": "árbol", "pt": "árvore", "ar": "شجرة", "be": "дерево", "ca": "arbre", "cs": "strom", "da": "træ", "el": "δέντρο", "fa": "درخت", "frp": "arbre", "ga": "crann", "he": "עץ", "hi": "tree", "hr": "stablo", "hu": "fa", "id": "pohon", "it": "albero", "ja": "木", "kk": "ағаш", "km": "ដើមឈើ", "ko": "나무", "ku": "درخت", "lo": "tree", "mg": "hazo", "ms": "pokok, kata akar", "my": "tree", "pl": "drzewo", "ro": "arbre", "ru": "дерево", "sk": "strom", "sl": "drevo", "sv": "träd", "sw": "arbre", "th": "ต้นไม้", "tok": "tree", "tr": "ağaç", "uk": "дерево", "ur": "درخت", "vi": "cây cối", "yo": "tree", "zh-tw": "樹", "zh": "树,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-radiko-aspekt", typ: "vocab",
			content: map[string]interface{}{
				"word": "aspekt",
				"definition": "to look, to seem",
				"definitions": map[string]interface{}{"en": "to look, to seem", "nl": "uitzicht", "de": "Aussehen", "fr": "aspect", "es": "parecer (aspecto)", "pt": "aspecto, aparentar", "ar": "مظهر", "be": "look, seem", "ca": "semblar (aspecte)", "cs": "vypadat, vzhled", "da": "se ud som, virke", "el": "το να φαίνομαι", "fa": "ظاهر, جنبه, نما", "frp": "aspect", "ga": "cuma ar", "he": "נראה", "hi": "look, seem", "hr": "izgledati", "hu": "kinéz", "id": "tampak", "it": "sembrare, apparire", "ja": "外見, のようだ", "kk": "look, seem", "km": "look, seem", "ko": "~한 외양이다, ~한 표정이다", "ku": "ظاهر, جنبه, نما", "lo": "look, seem", "mg": "endrika , bika", "ms": "kelihatan ,kata akar, aspek, kata akar", "my": "look, seem", "pl": "wygląd, aspekt", "ro": "aspect", "ru": "look, seem", "sk": "vyzerať, vzhľad", "sl": "pogled, videz", "sv": "se ut", "sw": "aspect", "th": "ลักษณะ", "tok": "to look, to seem", "tr": "görünmek, gözükmek", "uk": "вигляд", "ur": "look, seem", "vi": "diện mạo, khía cạnh", "yo": "to look, to seem", "zh-tw": "外表, 顯得", "zh": "外表,词根, 显得,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-radiko-atend", typ: "vocab",
			content: map[string]interface{}{
				"word": "atend",
				"definition": "to wait",
				"definitions": map[string]interface{}{"en": "to wait", "nl": "wachten", "de": "warten", "fr": "attendre", "es": "esperar", "pt": "esperar", "ar": "انتظر", "be": "ждать", "ca": "esperar", "cs": "čekat", "da": "vente, forvente", "el": "το να περιμένω", "fa": "منتظر چیزی یا کسی بودن, انتظار چیزی یا کسی را داشتن", "frp": "attendre", "ga": "fan", "he": "לחכות", "hi": "to wait", "hr": "čekati", "hu": "vár", "id": "tunggu", "it": "attendere, aspettare", "ja": "待つ", "kk": "to wait", "km": "to wait", "ko": "기다리다", "ku": "منتظر چیزی یا کسی بودن, انتظار چیزی یا کسی را داشتن", "lo": "to wait", "mg": "miandry", "ms": "menunggu, kata akar", "my": "to wait", "pl": "czekać", "ro": "attendre", "ru": "ждать", "sk": "čakať", "sl": "čakati", "sv": "vänta", "sw": "attendre", "th": "คอย", "tok": "to wait", "tr": "beklemek", "uk": "чекати", "ur": "انتظار کرنا", "vi": "chờ đợi", "yo": "to wait", "zh-tw": "等待", "zh": "等待,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-radiko-atent", typ: "vocab",
			content: map[string]interface{}{
				"word": "atent",
				"definition": "to pay attention",
				"definitions": map[string]interface{}{"en": "to pay attention", "nl": "opletten, aandacht", "de": "acht geben", "fr": "prêter attention", "es": "estar atento, prestar atención", "pt": "prestar atenção", "ar": "انتبه", "be": "внимание", "ca": "estar atent, parar atenció", "cs": "dávat pozor", "da": "være opmærksom", "el": "το να προσέχω", "fa": "توجه کردن", "frp": "prêter attention", "ga": "tabhair aire", "he": "לשים לב", "hi": "pay attention", "hr": "paziti", "hu": "figyel", "id": "perhatian", "it": "stare attento", "ja": "気をつける", "kk": "pay attention", "km": "pay attention", "ko": "주의하다", "ku": "توجه کردن", "lo": "pay attention", "mg": "mitandrina", "ms": "memberi perhatian, kata akar", "my": "pay attention", "pl": "uważać", "ro": "prêter attention", "ru": "внимание", "sk": "dávať pozor", "sl": "paziti", "sv": "akta, uppmärksamma, se upp", "sw": "prêter attention", "th": "ระวัง", "tok": "to pay attention", "tr": "dikkat etmek", "uk": "уважний", "ur": "توجہ دینا", "vi": "chú ý, để ý", "yo": "to pay attention", "zh-tw": "注意", "zh": "注意,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-radiko-acxet", typ: "vocab",
			content: map[string]interface{}{
				"word": "aĉet",
				"definition": "to buy",
				"definitions": map[string]interface{}{"en": "to buy", "nl": "kopen", "de": "kaufen", "fr": "acheter", "es": "comprar", "pt": "comprar", "ar": "اشترى", "be": "купить", "ca": "comprar", "cs": "koupit", "da": "købe", "el": "το να αγοράζω", "fa": "خریدن", "frp": "acheter", "ga": "ceannaigh", "he": "לקנות", "hi": "to buy", "hr": "kupiti", "hu": "vesz", "id": "beli", "it": "comprare", "ja": "買う", "kk": "to buy", "km": "to buy", "ko": "사다", "ku": "خریدن", "lo": "to buy", "mg": "mividy", "ms": "membeli, kata akar", "my": "to buy", "pl": "kupować", "ro": "acheter", "ru": "купить", "sk": "kúpiť", "sl": "kupiti", "sv": "köpa", "sw": "acheter", "th": "ซื้อ", "tok": "to buy", "tr": "satın almak", "uk": "купувати", "ur": "خریدنا", "vi": "mua sắm", "yo": "to buy", "zh-tw": "買", "zh": "买,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-radiko-auxd", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭd",
				"definition": "to hear",
				"definitions": map[string]interface{}{"en": "to hear", "nl": "horen", "de": "hören", "fr": "entendre", "es": "oír", "pt": "ouvir", "ar": "سمع", "be": "слышать", "ca": "sentir, oir", "cs": "slyšet", "da": "høre", "el": "το να ακούω", "fa": "شنیدن", "frp": "entendre", "ga": "clois", "he": "שלמוע", "hi": "to hear", "hr": "čuti", "hu": "hall", "id": "dengar", "it": "sentire (udito)", "ja": "聞こえる", "kk": "to hear", "km": "to hear", "ko": "듣다", "ku": "شنیدن", "lo": "to hear", "mg": "mahare", "ms": "mendengar, kata akar", "my": "to hear", "pl": "słyszeć", "ro": "entendre", "ru": "слышать", "sk": "počuť", "sl": "slišati", "sv": "höra", "sw": "entendre", "th": "ได้ยิน", "tok": "to hear", "tr": "duymak", "uk": "чути", "ur": "سننا", "vi": "nghe thấy", "yo": "to hear", "zh-tw": "聽", "zh": "听,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-radiko-auxt", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭt",
				"definition": "car",
				"definitions": map[string]interface{}{"en": "car", "nl": "auto", "de": "Auto", "fr": "voiture", "es": "coche", "pt": "carro", "ar": "سيارة", "be": "машина, автомобиль", "ca": "cotxe", "cs": "auto", "da": "bil", "el": "αυτοκίνητο", "fa": "خودرو", "frp": "voiture", "ga": "carr", "he": "מכונית", "hi": "car", "hr": "auto", "hu": "autó", "id": "mobil", "it": "automobile", "ja": "車", "kk": "car", "km": "ឡាន", "ko": "자동차", "ku": "خودرو", "lo": "car", "mg": "fiara", "ms": "kereta, kata akar", "my": "car", "pl": "samochód", "ro": "voiture", "ru": "машина, автомобиль", "sk": "auto", "sl": "avto", "sv": "bil", "sw": "voiture", "th": "รถ", "tok": "car", "tr": "araba", "uk": "автомобіль", "ur": "کار", "vi": "xe hơi", "yo": "car", "zh-tw": "汽車", "zh": "汽车,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-radiko-auxtoritat", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭtoritat",
				"definition": "authority",
				"definitions": map[string]interface{}{"en": "authority", "nl": "autoriteit", "de": "Autorität", "fr": "autorité", "es": "autoridad", "pt": "autoridade, autorizado", "ar": "سلطة", "be": "авторитет", "ca": "autoritat", "cs": "autorita", "da": "autoritet", "el": "εξουσία", "fa": "قدرت, اقتدار, اختیار, اعتبار, نفوذ, اولیای امور (به صورت جمع)", "frp": "autorité", "ga": "údarás", "he": "סמכות", "hi": "authority", "hr": "autoritet", "hu": "hatalom", "id": "kuasa", "it": "autorità", "ja": "権威", "kk": "authority", "km": "authority", "ko": "권위", "ku": "قدرت, اقتدار, اختیار, اعتبار, نفوذ, اولیای امور (به صورت جمع)", "lo": "authority", "mg": "fanjakana", "ms": "pihak berkuasa, kata akar", "my": "authority", "pl": "autorytet", "ro": "autorité", "ru": "авторитет", "sk": "autorita", "sl": "oblast", "sv": "auktoritet, myndighet", "sw": "autorité", "th": "มีสิทธิ์, มีอำนาจชอบธรรม", "tok": "authority", "tr": "otorite", "uk": "авторитет", "ur": "مقتدرہ", "vi": "chính quyền", "yo": "authority", "zh-tw": "權威", "zh": "权威,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-radiko-bedauxr", typ: "vocab",
			content: map[string]interface{}{
				"word": "bedaŭr",
				"definition": "to regret",
				"definitions": map[string]interface{}{"en": "to regret", "nl": "betreuren", "de": "bedauern", "fr": "regretter", "es": "lamentar", "pt": "lamentar", "ar": "ندم", "be": "сожалеть, жалеть", "ca": "lamentar, deplorar", "cs": "litovat", "da": "beklage, fortryde", "el": "το να λυπάμαι", "fa": "متأسف بودن به خاطر", "frp": "regretter", "ga": "aiféala", "he": "להצטער", "hi": "to regret", "hr": "žaliti", "hu": "sajnál", "id": "sesal", "it": "dispiacere", "ja": "残念に思う", "kk": "to regret", "km": "to regret", "ko": "애석하게 여기다", "ku": "متأسف بودن به خاطر", "lo": "to regret", "mg": "manenina", "ms": "menyesal,kata akar", "my": "to regret", "pl": "żałować", "ro": "regretter", "ru": "сожалеть, жалеть", "sk": "ľutovať", "sl": "obžalovati", "sv": "beklaga", "sw": "regretter", "th": "เสียใจ, เสียดาย", "tok": "to regret", "tr": "pişman olmak, esefle karşılamak", "uk": "жалкувати", "ur": "افسوس ہونا", "vi": "hối hận", "yo": "to regret", "zh-tw": "遺憾", "zh": "遗憾,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-radiko-bel", typ: "vocab",
			content: map[string]interface{}{
				"word": "bel",
				"definition": "beautiful",
				"definitions": map[string]interface{}{"en": "beautiful", "nl": "mooi", "de": "schön", "fr": "beau", "es": "bonito/a, bello/a, lindo/a", "pt": "belo, bela", "ar": "جميل", "be": "красивый", "ca": "bonic/a, bell/a, maco/a", "cs": "krásný", "da": "smuk", "el": "ωραίος-α-ο", "fa": "زیبا", "frp": "beau", "ga": "álainn", "he": "יפה", "hi": "beautiful", "hr": "lijep", "hu": "szép", "id": "cantik", "it": "beautiful", "ja": "美しい", "kk": "beautiful", "km": "ស្រស់ស្អាត", "ko": "아름다운", "ku": "زیبا", "lo": "beautiful", "mg": "tsara tarehy", "ms": "cantik,kata akar", "my": "beautiful", "pl": "ładny", "ro": "beau", "ru": "красивый", "sk": "pekný, krásny", "sl": "čudovit", "sv": "vacker, snygg", "sw": "beau", "th": "สวย", "tok": "beautiful", "tr": "güzel", "uk": "красивий", "ur": "خوبصورت", "vi": "đẹp đẽ", "yo": "beautiful", "zh-tw": "美麗", "zh": "美丽,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-radiko-best", typ: "vocab",
			content: map[string]interface{}{
				"word": "best",
				"definition": "animal",
				"definitions": map[string]interface{}{"en": "animal", "nl": "dier", "de": "Tier", "fr": "animal", "es": "animal", "pt": "animal, bicho", "ar": "حيوان", "be": "зверь", "ca": "animal", "cs": "zvíře", "da": "dyr", "el": "ζώο", "fa": "حیوان", "frp": "animal", "ga": "ainmhí", "he": "חיה", "hi": "animal", "hr": "životinja", "hu": "állat", "id": "hewan, binatang", "it": "animale", "ja": "動物", "kk": "animal", "km": "សត្វ", "ko": "동물", "ku": "حیوان", "lo": "animal", "mg": "biby", "ms": "binatang,kata akar", "my": "animal", "pl": "zwierzę", "ro": "animal", "ru": "зверь", "sk": "zviera", "sl": "žival", "sv": "djur", "sw": "animal", "th": "สัตว์", "tok": "animal", "tr": "hayvan", "uk": "тварина", "ur": "جانور", "vi": "động vật", "yo": "animal", "zh-tw": "動物", "zh": "动物,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-radiko-bezon", typ: "vocab",
			content: map[string]interface{}{
				"word": "bezon",
				"definition": "to need",
				"definitions": map[string]interface{}{"en": "to need", "nl": "nodig hebben, benodigen", "de": "brauchen, benötigen", "fr": "avoir besoin", "es": "necesitar", "pt": "precisar", "ar": "حاجة", "be": "нуждаться", "ca": "necessitar", "cs": "potřebovat", "da": "behøve, have brug for", "el": "το να χρειάζομαι", "fa": "نیاز داشتن به", "frp": "avoir besoin", "ga": "gá", "he": "להצטרך", "hi": "to need", "hr": "trebati", "hu": "szükség", "id": "butuh", "it": "essere necessario", "ja": "必要である", "kk": "to need", "km": "to need", "ko": "필요하다", "ku": "نیاز داشتن به", "lo": "to need", "mg": "mila", "ms": "memerlu,kata akar", "my": "to need", "pl": "potrzebować", "ro": "avoir besoin", "ru": "нуждаться", "sk": "potrebovať", "sl": "potrebovati", "sv": "behöva", "sw": "avoir besoin", "th": "จำเป็น", "tok": "to need", "tr": "ihtiyaç duymak", "uk": "потребувати", "ur": "ضرورت ہونا", "vi": "cần thiết", "yo": "to need", "zh-tw": "需要", "zh": "需要,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-radiko-bien", typ: "vocab",
			content: map[string]interface{}{
				"word": "bien",
				"definition": "estate, land",
				"definitions": map[string]interface{}{"en": "estate, land", "nl": "landgoed, boerderij", "de": "Landgut, Bauerngut", "fr": "propriété", "es": "terreno", "pt": "fazenda", "ar": "ممتلكات", "be": "имение, поместиье", "ca": "terreny, finca", "cs": "statek, země", "da": "grund, land", "el": "περιουσία", "fa": "مُلک", "frp": "propriété", "ga": "eastát, talamh", "he": "אחוזה", "hi": "estate, land", "hr": "imanje, posjed", "hu": "telek", "id": "lahan, tanah", "it": "fattoria, podere", "ja": "地所, 土地", "kk": "estate, land", "km": "estate, land", "ko": "땅, 토지, 농장지", "ku": "مُلک", "lo": "estate, land", "mg": "fananana", "ms": "harta,kata akar", "my": "estate, land", "pl": "farma, gospodarstwo", "ro": "propriété", "ru": "имение, поместиье", "sk": "statok, hospodárstvo", "sl": "posestvo, pokrajina", "sv": "egendom, gård, gods", "sw": "propriété", "th": "ทุ่ง", "tok": "estate, land", "tr": "arsa, malikane", "uk": "маєток", "ur": "estate, land", "vi": "nông trại, đồn điền", "yo": "estate, land", "zh-tw": "地產, 土地", "zh": "产业,词根, 土地,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-radiko-bon", typ: "vocab",
			content: map[string]interface{}{
				"word": "bon",
				"definition": "good",
				"definitions": map[string]interface{}{"en": "good", "nl": "goed", "de": "gut", "fr": "bon", "es": "bueno/a", "pt": "bom, boa", "ar": "جيد", "be": "хороший", "ca": "bo/bona", "cs": "dobré", "da": "god", "el": "καλός-η-ο", "fa": "خوب", "frp": "bon", "ga": "maith", "he": "טוב", "hi": "good", "hr": "dobar", "hu": "jó", "id": "baik", "it": "buono", "ja": "良い", "kk": "good", "km": "ល្អ", "ko": "좋은", "ku": "خوب", "lo": "good", "mg": "tsara", "ms": "bagus,kata akar", "my": "good", "pl": "dobry", "ro": "bon", "ru": "хороший", "sk": "dobrý", "sl": "dobro", "sv": "bra, god", "sw": "bon", "th": "ดี", "tok": "good", "tr": "iyi", "uk": "добрий", "ur": "اچھا، اچھی", "vi": "hay, tốt", "yo": "good", "zh-tw": "好", "zh": "好,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 22,
		},
		{
			slug: "vortaro-radiko-brancx", typ: "vocab",
			content: map[string]interface{}{
				"word": "branĉ",
				"definition": "branch",
				"definitions": map[string]interface{}{"en": "branch", "nl": "tak", "de": "Zweig, Ast", "fr": "branche", "es": "rama", "pt": "ramo, galho", "ar": "غصن", "be": "ветка", "ca": "branca", "cs": "obor, větev", "da": "gren", "el": "κλάδος", "fa": "شاخه", "frp": "branche", "ga": "géag", "he": "ענף", "hi": "branch", "hr": "grana", "hu": "ág", "id": "cabang", "it": "ramo", "ja": "枝", "kk": "branch", "km": "branch", "ko": "가지", "ku": "شاخه", "lo": "branch", "mg": "rantsana", "ms": "cawangan,kata akar", "my": "branch", "pl": "gałąź", "ro": "branche", "ru": "ветка", "sk": "odbor, odvetvie, konár", "sl": "veja", "sv": "gren", "sw": "branche", "th": "กิ่ง", "tok": "branch", "tr": "dal", "uk": "гілка", "ur": "شاخ", "vi": "cành cây, ngả đường", "yo": "branch", "zh-tw": "支部, 枝", "zh": "支部,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 23,
		},
		{
			slug: "vortaro-radiko-bru", typ: "vocab",
			content: map[string]interface{}{
				"word": "bru",
				"definition": "noise",
				"definitions": map[string]interface{}{"en": "noise", "nl": "lawaai", "de": "Lärm", "fr": "bruit", "es": "ruido", "pt": "barulho, fazer barulho", "ar": "صوت", "be": "шум", "ca": "soroll", "cs": "hluk", "da": "støj", "el": "θόρυβος", "fa": "سروصدا", "frp": "bruit", "ga": "fothram", "he": "רעש", "hi": "noise", "hr": "buka", "hu": "zaj", "id": "ribut", "it": "rumore", "ja": "騒音", "kk": "noise", "km": "noise", "ko": "소음", "ku": "سروصدا", "lo": "noise", "mg": "feo", "ms": "bunyian,kata akar", "my": "noise", "pl": "hałas", "ro": "bruit", "ru": "шум", "sk": "hluk", "sl": "hrup", "sv": "buller", "sw": "bruit", "th": "เสียงรบกวน", "tok": "noise", "tr": "gürültü", "uk": "шуміти", "ur": "شور", "vi": "tiếng ồn", "yo": "noise", "zh-tw": "噪音", "zh": "噪音,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 24,
		},
		{
			slug: "vortaro-radiko-busx", typ: "vocab",
			content: map[string]interface{}{
				"word": "buŝ",
				"definition": "mouth",
				"definitions": map[string]interface{}{"en": "mouth", "nl": "mond", "de": "Mund", "fr": "bouche", "es": "boca", "pt": "boca", "ar": "فم", "be": "морда", "ca": "boca", "cs": "ústa", "da": "mund", "el": "στόμα", "fa": "دهان", "frp": "bouche", "ga": "béal", "he": "פה", "hi": "mouth", "hr": "usta", "hu": "száj", "id": "mulut", "it": "bocca", "ja": "口", "kk": "ауыз", "km": "មាត់", "ko": "입", "ku": "دهان", "lo": "mouth", "mg": "vava", "ms": "mulut,kata akar", "my": "mouth", "pl": "usta", "ro": "bouche", "ru": "морда", "sk": "ústa", "sl": "usta", "sv": "mun", "sw": "bouche", "th": "ปาก", "tok": "mouth", "tr": "ağız", "uk": "рот", "ur": "منہ", "vi": "miệng", "yo": "mouth", "zh-tw": "嘴", "zh": "嘴,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 25,
		},
		{
			slug: "vortaro-radiko-cert", typ: "vocab",
			content: map[string]interface{}{
				"word": "cert",
				"definition": "certain",
				"definitions": map[string]interface{}{"en": "certain", "nl": "zeker", "de": "sichere/r/s, sicherlich", "fr": "certain", "es": "cierto/a, seguro/a", "pt": "certo, certa", "ar": "متأكد", "be": "уверенный, несомненный", "ca": "cert/a, segur/a", "cs": "jistý", "da": "sikker", "el": "βέβαιος-η-ο", "fa": "مطمئن", "frp": "certain", "ga": "cinnte", "he": "ודאי, בטוח", "hi": "certain", "hr": "siguran", "hu": "biztos", "id": "yakin", "it": "certo", "ja": "確かである", "kk": "certain", "km": "certain", "ko": "확실한", "ku": "مطمئن", "lo": "certain", "mg": "sasany , marina tokoa", "ms": "tentu,kata akar", "my": "certain", "pl": "pewny, pewien", "ro": "certain", "ru": "уверенный, несомненный", "sk": "istý, určitý", "sl": "gotovo", "sv": "säker", "sw": "certain", "th": "แน่นอน", "tok": "certain", "tr": "kesin, emin olmak", "uk": "упевнений", "ur": "certain", "vi": "chắc chắn", "yo": "certain", "zh-tw": "確定", "zh": "确定,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 26,
		},
		{
			slug: "vortaro-radiko-dank", typ: "vocab",
			content: map[string]interface{}{
				"word": "dank",
				"definition": "to thank",
				"definitions": map[string]interface{}{"en": "to thank", "nl": "dank", "de": "danken", "fr": "remercier", "es": "agradecer (a)", "pt": "agradecer", "ar": "شكر", "be": "благодарить", "ca": "agrair (a)", "cs": "děkovat", "da": "takke", "el": "το να ευχαριστώ", "fa": "تشکر کردن از", "frp": "remercier", "ga": "buíochas", "he": "להודות", "hi": "to thank", "hr": "zahvaliti", "hu": "köszön", "id": "terima kasih", "it": "ringraziamento", "ja": "感謝する", "kk": "to thank", "km": "to thank", "ko": "감사하다", "ku": "تشکر کردن از", "lo": "to thank", "mg": "misaotra", "ms": "terima kasih,kata akar", "my": "to thank", "pl": "dziękować", "ro": "remercier", "ru": "благодарить", "sk": "ďakovať", "sl": "zahvaliti se", "sv": "tacka", "sw": "remercier", "th": "ขอบคุณ", "tok": "to thank", "tr": "teşekkür etmek", "uk": "дякувати", "ur": "شکریہ ادا کرنا", "vi": "cảm ơn", "yo": "to thank", "zh-tw": "謝", "zh": "谢谢,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 27,
		},
		{
			slug: "vortaro-radiko-dauxr", typ: "vocab",
			content: map[string]interface{}{
				"word": "daŭr",
				"definition": "to continue",
				"definitions": map[string]interface{}{"en": "to continue", "nl": "duren", "de": "dauern", "fr": "continuer", "es": "durar", "pt": "continuar, durar", "ar": "استمر", "be": "продолжать", "ca": "durar", "cs": "pokračovat", "da": "fortsætte", "el": "το να διαρκώ", "fa": "ادامه داشتن", "frp": "continuer", "ga": "lean", "he": "להימשך", "hi": "to continue", "hr": "trajati", "hu": "tart", "id": "lanjut", "it": "proseguire, continuare", "ja": "続ける", "kk": "to continue", "km": "to continue", "ko": "지속하다", "ku": "ادامه داشتن", "lo": "to continue", "mg": "manohy", "ms": "menyambung,kata akar", "my": "to continue", "pl": "trwać", "ro": "continuer", "ru": "продолжать", "sk": "pokračovať", "sl": "nadaljevati", "sv": "fortgå, pågå, vara", "sw": "continuer", "th": "ต่อเนื่อง", "tok": "to continue", "tr": "devam etmek", "uk": "продовжуватися", "ur": "جاری رہنا", "vi": "tiếp tục", "yo": "to continue", "zh-tw": "持續", "zh": "继续,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 28,
		},
		{
			slug: "vortaro-radiko-decid", typ: "vocab",
			content: map[string]interface{}{
				"word": "decid",
				"definition": "to decide",
				"definitions": map[string]interface{}{"en": "to decide", "nl": "beslissen", "de": "entscheiden", "fr": "décider", "es": "decidir", "pt": "decidir", "ar": "قرر", "be": "решать, принять решение", "ca": "decidir", "cs": "rozhodnout", "da": "bestemme, tage beslutning, afgøre", "el": "το να αποφασίζω", "fa": "تصمیم گرفتن", "frp": "décider", "ga": "cinn", "he": "להחליט", "hi": "to decide", "hr": "odlučiti", "hu": "elhatároz", "id": "memutuskan", "it": "decidere", "ja": "決める", "kk": "to decide", "km": "to decide", "ko": "결정하다", "ku": "تصمیم گرفتن", "lo": "to decide", "mg": "manapaka", "ms": "memutus,kata akar", "my": "to decide", "pl": "decydować", "ro": "décider", "ru": "решать, принять решение", "sk": "rozhodnúť", "sl": "odločiti", "sv": "bestämma", "sw": "décider", "th": "ตัดสินใจ", "tok": "to decide", "tr": "karar vermek", "uk": "вирішувати", "ur": "فیصلہ کرنا", "vi": "quyết định", "yo": "to decide", "zh-tw": "決定", "zh": "决定,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 29,
		},
		{
			slug: "vortaro-radiko-demand", typ: "vocab",
			content: map[string]interface{}{
				"word": "demand",
				"definition": "to ask",
				"definitions": map[string]interface{}{"en": "to ask", "nl": "vragen", "de": "fragen", "fr": "demander", "es": "preguntar", "pt": "perguntar", "ar": "تطلب", "be": "спросить", "ca": "preguntar", "cs": "požádat", "da": "spørge", "el": "το να ρωτώ", "fa": "پرسیدن", "frp": "demander", "ga": "fiafraigh", "he": "לשאול", "hi": "to ask", "hr": "pitati", "hu": "kérdez", "id": "tanya", "it": "domandare", "ja": "質問する", "kk": "to ask", "km": "to ask", "ko": "묻다", "ku": "پرسیدن", "lo": "to ask", "mg": "mangataka", "ms": "bertanya,kata akar", "my": "to ask", "pl": "pytać", "ro": "demander", "ru": "спросить", "sk": "pýtať sa, požiadať", "sl": "vprašati", "sv": "fråga", "sw": "demander", "th": "ถาม", "tok": "to ask", "tr": "sormak", "uk": "питати", "ur": "پوچھنا", "vi": "hỏi han", "yo": "to ask", "zh-tw": "發問", "zh": "发问,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 30,
		},
		{
			slug: "vortaro-radiko-dev", typ: "vocab",
			content: map[string]interface{}{
				"word": "dev",
				"definition": "to must",
				"definitions": map[string]interface{}{"en": "to must", "nl": "moeten", "de": "müssen", "fr": "devoir", "es": "tener que, deber", "pt": "dever, ter de", "ar": "واجب", "be": "быть должным", "ca": "haver de, deure (estar obligat a)", "cs": "muset", "da": "skulle, være nødt til", "el": "το να πρέπει", "fa": "باید", "frp": "devoir", "ga": "tá...ar", "he": "צריך", "hi": "must", "hr": "morati", "hu": "kell", "id": "harus", "it": "dovere", "ja": "しなければならない", "kk": "must", "km": "must", "ko": "해야한다", "ku": "باید", "lo": "must", "mg": "adidy", "ms": "mesti,kata akar", "my": "must", "pl": "musieć", "ro": "devoir", "ru": "быть должным", "sk": "musieť", "sl": "morati", "sv": "måsta, vara tvungen", "sw": "devoir", "th": "ต้อง", "tok": "to must", "tr": "zorunda olmak", "uk": "мусити, бути зобов'язаним", "ur": "must", "vi": "phải", "yo": "to must", "zh-tw": "必須", "zh": "必需、一定,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 31,
		},
		{
			slug: "vortaro-radiko-dezir", typ: "vocab",
			content: map[string]interface{}{
				"word": "dezir",
				"definition": "to wish",
				"definitions": map[string]interface{}{"en": "to wish", "nl": "wensen, verlangen", "de": "wünschen", "fr": "désirer", "es": "desear", "pt": "desejar, desejo", "ar": "رغبة", "be": "желать", "ca": "desitjar", "cs": "toužit", "da": "ønske", "el": "το να επιθυμώ", "fa": "آرزو داشتن", "frp": "désirer", "ga": "mian", "he": "להשתוקק", "hi": "to wish", "hr": "željeti", "hu": "kíván", "id": "ingin", "it": "desiderare", "ja": "望み", "kk": "to wish", "km": "to wish", "ko": "바라다, 욕망하다", "ku": "آرزو داشتن", "lo": "to wish", "mg": "maniry", "ms": "mengharap,kata akar", "my": "to wish", "pl": "pragnąć", "ro": "désirer", "ru": "желать", "sk": "priať, želať", "sl": "želeti", "sv": "önska", "sw": "désirer", "th": "ปรารถณา", "tok": "to wish", "tr": "dilemek, arzu etmek", "uk": "бажати", "ur": "خواہش رکھنا", "vi": "mong muốn", "yo": "to wish", "zh-tw": "願望", "zh": "愿望,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 32,
		},
		{
			slug: "vortaro-radiko-diabl", typ: "vocab",
			content: map[string]interface{}{
				"word": "diabl",
				"definition": "devil",
				"definitions": map[string]interface{}{"en": "devil", "nl": "duivel", "de": "Teufel", "fr": "diable", "es": "diablo", "pt": "diabo, diabólico, diabólica", "ar": "شيطان", "be": "дьявол", "ca": "diable", "cs": "ďábel", "da": "djævel", "el": "διάβολος", "fa": "شیطان", "frp": "diable", "ga": "diabhal", "he": "שטן", "hi": "devil", "hr": "vrag", "hu": "ördög", "id": "setan", "it": "diavolo", "ja": "悪魔", "kk": "devil", "km": "devil", "ko": "악마", "ku": "شیطان", "lo": "devil", "mg": "devoly", "ms": "iblis,kata akar", "my": "devil", "pl": "diabeł", "ro": "diable", "ru": "дьявол", "sk": "diabol", "sl": "vrag", "sv": "djävul", "sw": "diable", "th": "ปีศาจ", "tok": "devil", "tr": "şeytan", "uk": "диявол", "ur": "شیطان", "vi": "ác quỷ", "yo": "devil", "zh-tw": "妖魔", "zh": "妖魔,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 33,
		},
		{
			slug: "vortaro-radiko-dik", typ: "vocab",
			content: map[string]interface{}{
				"word": "dik",
				"definition": "thick, fat",
				"definitions": map[string]interface{}{"en": "thick, fat", "nl": "dik", "de": "dick", "fr": "gros", "es": "gordo", "pt": "gordo, grosso", "ar": "سمين", "be": "толстый, полный", "ca": "gras/grassa", "cs": "tlustý", "da": "tyk, fed", "el": "παχύς-ιά-ύ", "fa": "چاق, قطور", "frp": "gros", "ga": "ramhar, tiubh", "he": "עבה, שמן", "hi": "thick, fat", "hr": "debeo", "hu": "kövér", "id": "tebal, gemuk", "it": "grasso, spesso", "ja": "厚い, 太い", "kk": "thick, fat", "km": "thick, fat", "ko": "두꺼운, 살찐", "ku": "چاق, قطور", "lo": "thick, fat", "mg": "be vatana , vaventy", "ms": "tebal,kata akar, gemuk,kata akar", "my": "thick, fat", "pl": "gruby, tłusty", "ro": "gros", "ru": "толстый, полный", "sk": "tlstý", "sl": "debel, rejen", "sv": "tjock", "sw": "gros", "th": "หนา, อ้วน", "tok": "thick, fat", "tr": "kalın, şişman", "uk": "товстий", "ur": "گھنا, موٹا", "vi": "dày dặn, mập mạp", "yo": "thick, fat", "zh-tw": "厚, 肥胖", "zh": "厚,词根, 肥胖,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 34,
		},
		{
			slug: "vortaro-radiko-dimancx", typ: "vocab",
			content: map[string]interface{}{
				"word": "dimanĉ",
				"definition": "Sunday",
				"definitions": map[string]interface{}{"en": "Sunday", "nl": "zondag", "de": "Sonntag", "fr": "dimanche", "es": "domingo", "pt": "domingo", "ar": "الأحد", "be": "Воскресение", "ca": "diumenge", "cs": "Neděle", "da": "søndag", "el": "Κυριακή", "fa": "یک‌شنبه", "frp": "dimanche", "ga": "Domhnach", "he": "יום ראשון", "hi": "Sunday", "hr": "nedjelja", "hu": "vasárnap", "id": "Minggu", "it": "domenica", "ja": "日曜日", "kk": "жексенбі", "km": "កាលពីថ្ងៃអាទិត្យ", "ko": "일요일", "ku": "یک‌شنبه", "lo": "Sunday", "mg": "alahady", "ms": "hari Ahad,kata akar", "my": "Sunday", "pl": "niedziela", "ro": "dimanche", "ru": "Воскресение", "sk": "nedeľa", "sl": "nedelja", "sv": "söndag", "sw": "dimanche", "th": "วันอาทิตย์", "tok": "Sunday", "tr": "Pazar", "uk": "неділя", "ur": "اتوار", "vi": "chủ nhật", "yo": "Sunday", "zh-tw": "星期日", "zh": "星期日,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 35,
		},
		{
			slug: "vortaro-radiko-dir", typ: "vocab",
			content: map[string]interface{}{
				"word": "dir",
				"definition": "to say",
				"definitions": map[string]interface{}{"en": "to say", "nl": "zeggen", "de": "sagen", "fr": "dire", "es": "decir", "pt": "dizer", "ar": "أقول", "be": "сказать", "ca": "dir", "cs": "říct", "da": "sige", "el": "το να λέω", "fa": "گفتن", "frp": "dire", "ga": "deir", "he": "להגיד", "hi": "to say", "hr": "reći", "hu": "mond", "id": "bilang", "it": "dire", "ja": "言う", "kk": "to say", "km": "to say", "ko": "얘기하다", "ku": "گفتن", "lo": "to say", "mg": "miteny", "ms": "mengata,kata akar", "my": "to say", "pl": "mówić", "ro": "dire", "ru": "сказать", "sk": "povedať", "sl": "reči", "sv": "säga", "sw": "dire", "th": "บอก", "tok": "to say", "tr": "söylemek, demek", "uk": "казати, мовити, говорити", "ur": "کہنا", "vi": "nói", "yo": "to say", "zh-tw": "說", "zh": "说,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 36,
		},
		{
			slug: "vortaro-radiko-direkt", typ: "vocab",
			content: map[string]interface{}{
				"word": "direkt",
				"definition": "direction",
				"definitions": map[string]interface{}{"en": "direction", "nl": "richting", "de": "Richtung", "fr": "direction", "es": "dirección", "pt": "direção, dirigir", "ar": "اتجاه, جهة", "be": "направление", "ca": "direcció", "cs": "směr", "da": "retning", "el": "κατεύθυνση", "fa": "جهت دادن", "frp": "direction", "ga": "treo", "he": "כיוון", "hi": "direction", "hr": "smjer", "hu": "igazgat", "id": "arah", "it": "direzione", "ja": "方向", "kk": "direction", "km": "direction", "ko": "방향", "ku": "جهت دادن", "lo": "direction", "mg": "fizotra", "ms": "arah,kata akar", "my": "direction", "pl": "kierunek", "ro": "direction", "ru": "направление", "sk": "smer", "sl": "smer", "sv": "riktning", "sw": "direction", "th": "ทิศทาง", "tok": "direction", "tr": "yön", "uk": "направляти, спрямовувати", "ur": "سمت", "vi": "phương hướng", "yo": "direction", "zh-tw": "方向", "zh": "方向,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 37,
		},
		{
			slug: "vortaro-radiko-doktor", typ: "vocab",
			content: map[string]interface{}{
				"word": "doktor",
				"definition": "doctor",
				"definitions": map[string]interface{}{"en": "doctor", "nl": "doctor", "de": "Doktor", "fr": "docteur", "es": "doctor", "pt": "doutor", "ar": "طبيب", "be": "доктор", "ca": "doctor", "cs": "doktor", "da": "doktor", "el": "γιατρός", "fa": "دکتر", "frp": "docteur", "ga": "dochtúir", "he": "דוקטור", "hi": "doctor", "hr": "liječnik", "hu": "orvos", "id": "dokter", "it": "dottore", "ja": "博士", "kk": "дәрігер", "km": "doctor", "ko": "의사", "ku": "دکتر", "lo": "doctor", "mg": "dokotera", "ms": "doktor,kata akar", "my": "doctor", "pl": "doktor, lekarz", "ro": "docteur", "ru": "доктор", "sk": "doktor", "sl": "doktor", "sv": "doktor", "sw": "docteur", "th": "แพทย์, ดอกเตอร์", "tok": "doctor", "tr": "doktor", "uk": "доктор", "ur": "حکیم", "vi": "bác sĩ", "yo": "doctor", "zh-tw": "醫生", "zh": "医生,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 38,
		},
		{
			slug: "vortaro-radiko-dolar", typ: "vocab",
			content: map[string]interface{}{
				"word": "dolar",
				"definition": "dollar",
				"definitions": map[string]interface{}{"en": "dollar", "nl": "dollar", "de": "Dollar", "fr": "dollar", "es": "dólar", "pt": "dólar", "ar": "دولار", "be": "доллар", "ca": "dòlar", "cs": "dolar", "da": "dollar", "el": "δολλάριο", "fa": "دلار", "frp": "dollar", "ga": "dollar", "he": "דולר", "hi": "dollar", "hr": "dolar", "hu": "dollár", "id": "dolar", "it": "dollaro", "ja": "ドル", "kk": "доллар", "km": "dollar", "ko": "달러", "ku": "دلار", "lo": "dollar", "mg": "dolara", "ms": "dolar,kata akar", "my": "dollar", "pl": "dolar", "ro": "dollar", "ru": "доллар", "sk": "dolár", "sl": "dolar", "sv": "dollar", "sw": "dollar", "th": "ดอลล่า", "tok": "dollar", "tr": "dolar", "uk": "долар", "ur": "dollar", "vi": "đô la", "yo": "dollar", "zh-tw": "元 (錢)", "zh": "元 （钱）,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 39,
		},
		{
			slug: "vortaro-radiko-dolor", typ: "vocab",
			content: map[string]interface{}{
				"word": "dolor",
				"definition": "to hurt, pain",
				"definitions": map[string]interface{}{"en": "to hurt, pain", "nl": "pijn", "de": "Schmerz", "fr": "douleur", "es": "doler", "pt": "dor, doer", "ar": "ألم", "be": "боль, доставлять боль", "ca": "fer mal, doler", "cs": "bolest, bolet", "da": "smerte, skade", "el": "το να πονάω", "fa": "به درد آوردن, درد کردن", "frp": "douleur", "ga": "gortaigh, pian", "he": "כאב, לפגוע", "hi": "pain, to hurt", "hr": "boljeti", "hu": "fájdalom", "id": "sakit, menyakiti", "it": "dolore, far male", "ja": "痛み, 痛みを与える", "kk": "pain, to hurt", "km": "pain, to hurt", "ko": "고통, 아프게하다", "ku": "به درد آوردن, درد کردن", "lo": "pain, to hurt", "mg": "fijaliana , fangirifiriana", "ms": "sakit,kata akar, menyakiti,kata akar", "my": "pain, to hurt", "pl": "ból, boleć", "ro": "douleur", "ru": "боль, доставлять боль", "sk": "bolesť, bolieť", "sl": "bolečina", "sv": "smärta, göra ont", "sw": "douleur", "th": "เจ็บ", "tok": "to hurt, pain", "tr": "ağrı, acı, acımak", "uk": "боліти, викликати біль", "ur": "درد, ایزا دینا", "vi": "làm đau, nỗi đau", "yo": "to hurt, pain", "zh-tw": "痛", "zh": "痛,词根, 受伤,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 40,
		},
		{
			slug: "vortaro-radiko-dolcx", typ: "vocab",
			content: map[string]interface{}{
				"word": "dolĉ",
				"definition": "sweet",
				"definitions": map[string]interface{}{"en": "sweet", "nl": "zoet", "de": "süß", "fr": "doux", "es": "dulce", "pt": "doce", "ar": "لطيف", "be": "вкусный", "ca": "dolç/a", "cs": "sladké", "da": "sød", "el": "γλυκός-ιά-ό", "fa": "شیرین", "frp": "doux", "ga": "milis", "he": "מתוק", "hi": "sweet", "hr": "sladak", "hu": "édes", "id": "manis", "it": "dolce", "ja": "甘い", "kk": "sweet", "km": "ផ្អែម", "ko": "달콤한", "ku": "شیرین", "lo": "sweet", "mg": "mamy", "ms": "manis,kata akar", "my": "sweet", "pl": "słodki", "ro": "doux", "ru": "вкусный", "sk": "sladký", "sl": "sladko", "sv": "söt", "sw": "doux", "th": "หวาน", "tok": "sweet", "tr": "tatlı", "uk": "солодкий", "ur": "میٹھا", "vi": "ngọt", "yo": "sweet", "zh-tw": "甜", "zh": "甜,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 41,
		},
		{
			slug: "vortaro-radiko-dom", typ: "vocab",
			content: map[string]interface{}{
				"word": "dom",
				"definition": "house",
				"definitions": map[string]interface{}{"en": "house", "nl": "huis", "de": "Haus", "fr": "maison", "es": "casa", "pt": "casa", "ar": "منزل, بيت", "be": "дом", "ca": "casa, edifici", "cs": "dům", "da": "hus", "el": "σπίτι", "fa": "خانه (ساختمان)", "frp": "maison", "ga": "teach", "he": "בית", "hi": "house", "hr": "kuća", "hu": "ház", "id": "rumah", "it": "casa", "ja": "家", "kk": "house", "km": "house", "ko": "집", "ku": "خانه (ساختمان)", "lo": "house", "mg": "trano", "ms": "rumah,kata akar", "my": "house", "pl": "dom (budynek)", "ro": "maison", "ru": "дом", "sk": "dom", "sl": "hiša", "sv": "hus", "sw": "maison", "th": "บ้าน", "tok": "house", "tr": "ev", "uk": "дім", "ur": "گھر", "vi": "ngôi nhà", "yo": "house", "zh-tw": "房子", "zh": "房子,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 42,
		},
		{
			slug: "vortaro-radiko-don", typ: "vocab",
			content: map[string]interface{}{
				"word": "don",
				"definition": "to give",
				"definitions": map[string]interface{}{"en": "to give", "nl": "geven", "de": "geben", "fr": "donner", "es": "dar", "pt": "dar", "ar": "أعطى", "be": "дать", "ca": "donar", "cs": "dát", "da": "give", "el": "το να δίνω", "fa": "دادن", "frp": "donner", "ga": "tabhair", "he": "לתת", "hi": "to give", "hr": "dati", "hu": "ad", "id": "beri", "it": "dare", "ja": "与える", "kk": "to give", "km": "to give", "ko": "주다", "ku": "دادن", "lo": "to give", "mg": "manome", "ms": "memberi,kata akar", "my": "to give", "pl": "dać", "ro": "donner", "ru": "дать", "sk": "dať", "sl": "dati", "sv": "ge", "sw": "donner", "th": "ให้", "tok": "to give", "tr": "vermek", "uk": "давати", "ur": "دینا", "vi": "đưa cho", "yo": "to give", "zh-tw": "給", "zh": "给,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 43,
		},
		{
			slug: "vortaro-radiko-dorm", typ: "vocab",
			content: map[string]interface{}{
				"word": "dorm",
				"definition": "to sleep",
				"definitions": map[string]interface{}{"en": "to sleep", "nl": "slapen", "de": "schlafen", "fr": "dormir", "es": "dormir", "pt": "dormir", "ar": "نوم", "be": "спать", "ca": "dormir", "cs": "spát", "da": "sove", "el": "το να κοιμάμαι", "fa": "خوابیده بودن, خوابیدن", "frp": "dormir", "ga": "codladh", "he": "לישון", "hi": "sleep", "hr": "spavati", "hu": "alszik", "id": "tidur", "it": "dormire", "ja": "眠る", "kk": "sleep", "km": "sleep", "ko": "자다", "ku": "خوابیده بودن, خوابیدن", "lo": "sleep", "mg": "matory", "ms": "tidur,kata akar", "my": "sleep", "pl": "spać", "ro": "dormir", "ru": "спать", "sk": "spať", "sl": "spati", "sv": "sova", "sw": "dormir", "th": "นอนหลับ", "tok": "to sleep", "tr": "uyku", "uk": "спати", "ur": "sleep", "vi": "đi ngủ", "yo": "to sleep", "zh-tw": "睡", "zh": "睡,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 44,
		},
		{
			slug: "vortaro-radiko-dogx", typ: "vocab",
			content: map[string]interface{}{
				"word": "doĝ",
				"definition": "doge",
				"definitions": map[string]interface{}{"en": "doge", "nl": "doge", "de": "Doge (Oberhaupt)", "fr": "doge", "es": "dux", "pt": "doge", "ar": "دوج", "be": "дож", "ca": "dux", "cs": "dóže", "da": "doge", "el": "δόγης", "fa": "دوج (دوک ونیز)", "frp": "doge", "ga": "dóg", "he": "דוצ'ה, שליט איטלקי", "hi": "doge", "hr": "dužd", "hu": "dózse", "id": "doge", "it": "doge", "ja": "ドージ", "kk": "doge", "km": "doge", "ko": "총독", "ku": "دوج (دوک ونیز)", "lo": "doge", "mg": "doge", "ms": "doge,kata akar", "my": "doge", "pl": "doża", "ro": "doge", "ru": "дож", "sk": "dóža", "sl": "dož", "sv": "doge (hög ämbetsman i Venedig)", "sw": "doge", "th": "สุนัข", "tok": "doge", "tr": "eski venedik başkanı", "uk": "дож", "ur": "doge", "vi": "tổng trấn (từ cũ)", "yo": "doge", "zh-tw": "總督", "zh": "总督,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 45,
		},
		{
			slug: "vortaro-radiko-edz", typ: "vocab",
			content: map[string]interface{}{
				"word": "edz",
				"definition": "husband",
				"definitions": map[string]interface{}{"en": "husband", "nl": "echtgenoot", "de": "Gatte", "fr": "mari", "es": "esposo, marido", "pt": "marido", "ar": "زوج", "be": "муж", "ca": "marit", "cs": "manžel", "da": "mand, ægtemand", "el": "σύζυγος", "fa": "شوهر", "frp": "mari", "ga": "fear céile", "he": "בעל", "hi": "husband", "hr": "suprug", "hu": "férj", "id": "suami", "it": "marito", "ja": "配偶者", "kk": "husband", "km": "ប្តី", "ko": "남편", "ku": "شوهر", "lo": "husband", "mg": "vady lahy", "ms": "suami,kata akar", "my": "husband", "pl": "mąż", "ro": "mari", "ru": "муж", "sk": "manžel", "sl": "mož", "sv": "make, man", "sw": "mari", "th": "สามี", "tok": "husband", "tr": "koca", "uk": "чоловік (у шлюбі)", "ur": "خاونڈ", "vi": "người chồng", "yo": "husband", "zh-tw": "丈夫", "zh": "丈夫,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 46,
		},
		{
			slug: "vortaro-radiko-efektiv", typ: "vocab",
			content: map[string]interface{}{
				"word": "efektiv",
				"definition": "actually",
				"definitions": map[string]interface{}{"en": "actually", "nl": "werkelijk, efficiënt", "de": "wirklich, tatsächlich", "fr": "effectif, réel, véritable", "es": "efectivo", "pt": "na realidade", "ar": "بالفعل", "be": "действительно", "ca": "efectiu", "cs": "opravdu, skutečně", "da": "faktisk", "el": "πραγματικός-η-ο", "fa": "مؤثر, عملی, واقعی", "frp": "effectivement", "ga": "dáiríre", "he": "למעשה", "hi": "actually", "hr": "zapravo", "hu": "tényleges", "id": "sebenarnya, nyata", "it": "invero, a dire il vero, effettivamente", "ja": "実際に", "kk": "actually", "km": "actually", "ko": "효과적인", "ku": "مؤثر, عملی, واقعی", "lo": "actually", "mg": "tena isa, tokoa, marina, tena marina, marina tokoa", "ms": "sebenar,kata akar", "my": "actually", "pl": "efektywnie, faktycznie, rzeczywiście", "ro": "effectif, réel, véritable", "ru": "действительно", "sk": "skutočne, efektívne", "sl": "dejansko", "sv": "i själva verket", "sw": "effectivement", "th": "เป็นจริง", "tok": "actually", "tr": "gerçekte", "uk": "ефективний, діючий, дійсний", "ur": "actually", "vi": "thật ra là", "yo": "actually", "zh-tw": "其實", "zh": "其实,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 47,
		},
		{
			slug: "vortaro-radiko-ekskurs", typ: "vocab",
			content: map[string]interface{}{
				"word": "ekskurs",
				"definition": "excursion",
				"definitions": map[string]interface{}{"en": "excursion", "nl": "uitstap", "de": "Ausflug", "fr": "excursion", "es": "excursión", "pt": "excursão, excursionar", "ar": "نزهة", "be": "экскурсия, прогулка", "ca": "excursió", "cs": "výlet", "da": "ekskursion", "el": "εκδρομή", "fa": "گشت‌وگذار", "frp": "excursion", "ga": "turas", "he": "טיול", "hi": "excursion", "hr": "izlet", "hu": "kirándul", "id": "ekskursi, perjalanan wisata", "it": "escursione", "ja": "遠足", "kk": "excursion", "km": "excursion", "ko": "소풍", "ku": "گشت‌وگذار", "lo": "excursion", "mg": "fandehandehanana", "ms": "melancong,kata akar", "my": "excursion", "pl": "wycieczka", "ro": "excursion", "ru": "экскурсия, прогулка", "sk": "výlet", "sl": "izlet", "sv": "utflykt", "sw": "excursion", "th": "ทัศนศึกษา", "tok": "excursion", "tr": "gezinti", "uk": "екскурсія", "ur": "excursion", "vi": "đi chơi", "yo": "excursion", "zh-tw": "出遊", "zh": "旅游,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 48,
		},
		{
			slug: "vortaro-radiko-esper", typ: "vocab",
			content: map[string]interface{}{
				"word": "esper",
				"definition": "to hope",
				"definitions": map[string]interface{}{"en": "to hope", "nl": "hopen", "de": "hoffen", "fr": "espérer", "es": "esperar (esperanza)", "pt": "ter esperança", "ar": "أمل", "be": "надеяться", "ca": "tenir esperança", "cs": "doufat", "da": "håbe", "el": "το να ελπίζω", "fa": "امید داشتن به", "frp": "espérer", "ga": "dóchas", "he": "תקווה", "hi": "to hope", "hr": "nadati se", "hu": "remél", "id": "berharap", "it": "sperare", "ja": "希望する", "kk": "to hope", "km": "to hope", "ko": "희망하다", "ku": "امید داشتن به", "lo": "to hope", "mg": "manantena", "ms": "mengharap,kata akar", "my": "to hope", "pl": "mieć nadzieję", "ro": "espérer", "ru": "надеяться", "sk": "dúfať", "sl": "upati", "sv": "hoppas", "sw": "espérer", "th": "หวัง", "tok": "to hope", "tr": "ummak, umut etmek", "uk": "сподіватися", "ur": "امید کرنا", "vi": "hy vọng", "yo": "to hope", "zh-tw": "希望", "zh": "希望,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 49,
		},
		{
			slug: "vortaro-radiko-est", typ: "vocab",
			content: map[string]interface{}{
				"word": "est",
				"definition": "to be",
				"definitions": map[string]interface{}{"en": "to be", "nl": "zijn (werkwoord)", "de": "sein (Verb)", "fr": "être", "es": "ser, estar, haber", "pt": "ser, estar, haver", "ar": "يكون", "be": "быть", "ca": "ser, estar, haver-hi", "cs": "být", "da": "være", "el": "το να είμαι", "fa": "بودن", "frp": "être", "ga": "bí", "he": "להיות", "hi": "to be", "hr": "biti", "hu": "van", "id": "adalah (to be), ada", "it": "essere", "ja": "存在する, ～である", "kk": "to be", "km": "to be", "ko": "~이다", "ku": "بودن", "lo": "to be", "mg": "izy , zavatra", "ms": "ialah,kata akar", "my": "to be", "pl": "być", "ro": "être", "ru": "быть", "sk": "byť", "sl": "biti", "sv": "vara, finnas", "sw": "être", "th": "เป็น, อยู่, คือ", "tok": "to be", "tr": "olmak, -dir, -im vs (yardımcı fiil)", "uk": "бути", "ur": "ہونا", "vi": "thì, là", "yo": "to be", "zh-tw": "是", "zh": "是,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 50,
		},
		{
			slug: "vortaro-radiko-facil", typ: "vocab",
			content: map[string]interface{}{
				"word": "facil",
				"definition": "easy",
				"definitions": map[string]interface{}{"en": "easy", "nl": "gemakkelijk", "de": "leicht", "fr": "facile", "es": "fácil", "pt": "fácil", "ar": "سهل", "be": "лёгкий", "ca": "fàcil", "cs": "jednoduchý", "da": "nem", "el": "εύκολος-η-ο", "fa": "آسان", "frp": "facile", "ga": "furasta", "he": "קל", "hi": "easy", "hr": "lagan", "hu": "könnyű", "id": "gampang", "it": "facile", "ja": "やさしい", "kk": "easy", "km": "easy", "ko": "쉬운", "ku": "آسان", "lo": "easy", "mg": "mora", "ms": "senang,kata akar", "my": "easy", "pl": "łatwy", "ro": "facile", "ru": "лёгкий", "sk": "jednoduchý", "sl": "lahko", "sv": "lätt, enkel", "sw": "facile", "th": "ง่าย", "tok": "easy", "tr": "kolay", "uk": "легкий", "ur": "آسان", "vi": "dễ dàng", "yo": "easy", "zh-tw": "容易", "zh": "容易,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 51,
		},
		{
			slug: "vortaro-radiko-fakt", typ: "vocab",
			content: map[string]interface{}{
				"word": "fakt",
				"definition": "fact",
				"definitions": map[string]interface{}{"en": "fact", "nl": "feitelijk", "de": "tatsächlich", "fr": "fait", "es": "hecho", "pt": "fato", "ar": "حقيقة", "be": "факт", "ca": "fet", "cs": "fakt", "da": "faktum", "el": "γεγονός", "fa": "واقعیت, حقیقت", "frp": "fait", "ga": "fíric", "he": "עובדה", "hi": "fact", "hr": "činjenica", "hu": "tény", "id": "fakta", "it": "fatto", "ja": "事実", "kk": "fact", "km": "fact", "ko": "사실", "ku": "واقعیت, حقیقت", "lo": "fact", "mg": "vita", "ms": "fakta,kata akar", "my": "fact", "pl": "fakt", "ro": "fait", "ru": "факт", "sk": "fakt", "sl": "dejstvo", "sv": "fakta", "sw": "fait", "th": "ข้อเท็จจริง", "tok": "fact", "tr": "aslında", "uk": "факт", "ur": "حقیقت", "vi": "sự thật", "yo": "fact", "zh-tw": "事實", "zh": "事实,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 52,
		},
		{
			slug: "vortaro-radiko-fal", typ: "vocab",
			content: map[string]interface{}{
				"word": "fal",
				"definition": "to fall",
				"definitions": map[string]interface{}{"en": "to fall", "nl": "vallen", "de": "fallen", "fr": "tomber", "es": "caer", "pt": "cair, queda", "ar": "سقط", "be": "падать", "ca": "caure", "cs": "padat", "da": "falde", "el": "το να πέφτω", "fa": "افتادن", "frp": "tomber", "ga": "tit", "he": "ליפול", "hi": "to fall", "hr": "pasti", "hu": "esik", "id": "jatuh", "it": "cadere", "ja": "落ちる", "kk": "to fall", "km": "to fall", "ko": "떨어지다", "ku": "افتادن", "lo": "to fall", "mg": "latsaka", "ms": "menjatuh,kata akar", "my": "to fall", "pl": "spaść", "ro": "tomber", "ru": "падать", "sk": "padať", "sl": "pasti", "sv": "falla", "sw": "tomber", "th": "ตก", "tok": "to fall", "tr": "düşmek", "uk": "падати, знижуватися, зменшуватися", "ur": "گرنا", "vi": "rơi", "yo": "to fall", "zh-tw": "掉落", "zh": "掉,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 53,
		},
		{
			slug: "vortaro-radiko-famili", typ: "vocab",
			content: map[string]interface{}{
				"word": "famili",
				"definition": "family",
				"definitions": map[string]interface{}{"en": "family", "nl": "gezin (familie)", "de": "Familie", "fr": "famille", "es": "familiar", "pt": "família", "ar": "عائلة, أسرة", "be": "семья", "ca": "família", "cs": "rodina", "da": "familie", "el": "οικογένεια", "fa": "خانواده", "frp": "famille", "ga": "líon tí", "he": "משפחה", "hi": "family", "hr": "obitelj", "hu": "család", "id": "keluarga", "it": "famiglia", "ja": "家族", "kk": "family", "km": "ក្រុមគ្រួសារ", "ko": "가족", "ku": "خانواده", "lo": "family", "mg": "fianakaviana", "ms": "famili,kata akar", "my": "family", "pl": "rodzina", "ro": "famille", "ru": "семья", "sk": "rodina", "sl": "družina", "sv": "familj", "sw": "famille", "th": "ครอบครัว", "tok": "family", "tr": "aile", "uk": "родина, сім'я", "ur": "خاندان", "vi": "gia đình", "yo": "family", "zh-tw": "家庭", "zh": "家庭,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 54,
		},
		{
			slug: "vortaro-radiko-far", typ: "vocab",
			content: map[string]interface{}{
				"word": "far",
				"definition": "to do",
				"definitions": map[string]interface{}{"en": "to do", "nl": "maken, doen", "de": "machen, tun", "fr": "faire", "es": "hacer", "pt": "fazer", "ar": "يفعل", "be": "делать", "ca": "fer", "cs": "dělat", "da": "gøre, lave", "el": "το να κάνω", "fa": "انجام دادن, درست کردن, کردن", "frp": "faire", "ga": "déan", "he": "לעשות", "hi": "to do", "hr": "napraviti, činiti", "hu": "csinál", "id": "buat, lakukan", "it": "fare", "ja": "作る, する", "kk": "to do", "km": "to do", "ko": "만들다, 짓다, 제조하다", "ku": "انجام دادن, درست کردن, کردن", "lo": "to do", "mg": "manao", "ms": "membuat,kata akar", "my": "to do", "pl": "robić", "ro": "faire", "ru": "делать", "sk": "robiť", "sl": "narediti", "sv": "göra", "sw": "faire", "th": "ทำ", "tok": "to do", "tr": "yapmak", "uk": "робити", "ur": "کرنا", "vi": "làm", "yo": "to do", "zh-tw": "做", "zh": "做,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 55,
		},
		{
			slug: "vortaro-radiko-fart", typ: "vocab",
			content: map[string]interface{}{
				"word": "fart",
				"definition": "to feel, to be",
				"definitions": map[string]interface{}{"en": "to feel, to be", "nl": "gesteld zijn, varen (zich voelen)", "de": "sich fühlen", "fr": "se porter", "es": "estar, sentirse", "pt": "sentir-se", "ar": "يكون", "be": "поживать", "ca": "estar, sentir-se, trobar-se", "cs": "cítit (se), mít se", "da": "have det, føle sig", "el": "το να αισθάνομαι", "fa": "سلامت بودن", "frp": "se porter", "ga": "braith, bí", "he": "להרגיש", "hi": "feel, be", "hr": "osjećati se", "hu": "érzi magát", "id": "merasa, (menerangkan keadaan)", "it": "sentirsi", "ja": "…な健康状態にある, 体調", "kk": "feel, be", "km": "feel, be", "ko": "살아가다, 지내다", "ku": "سلامت بودن", "lo": "feel, be", "mg": "mandeha , mirohotra", "ms": "merasa,kata akar", "my": "feel, be", "pl": "czuć się, mieć się", "ro": "se porter", "ru": "поживать", "sk": "cítiť sa, mať sa", "sl": "čutiti, biti", "sv": "må, känna sig", "sw": "se porter", "th": "สภาพของสุขภาพ, เป็น", "tok": "to feel, to be", "tr": "hissetmek(ruh/sağlık hali), olmak (iyi/kötü)", "uk": "матися, почувати себе, поживати", "ur": "محسوس کرنا, be", "vi": "cảm thấy", "yo": "to feel, to be", "zh-tw": "過活, 康泰", "zh": "过活、康泰,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 56,
		},
		{
			slug: "vortaro-radiko-felicx", typ: "vocab",
			content: map[string]interface{}{
				"word": "feliĉ",
				"definition": "happy",
				"definitions": map[string]interface{}{"en": "happy", "nl": "gelukkig", "de": "glücklich", "fr": "heureux", "es": "feliz", "pt": "feliz", "ar": "سعيد", "be": "счастье", "ca": "feliç", "cs": "štěstí", "da": "lykkelig", "el": "ευτυχής-ες", "fa": "خوش‌حال", "frp": "heureux", "ga": "sona", "he": "מאושר", "hi": "happy", "hr": "sretan", "hu": "boldog", "id": "bahagia, senang", "it": "felice", "ja": "幸せな", "kk": "бақыт", "km": "សប្បាយរីករាយ", "ko": "행복한", "ku": "خوش‌حال", "lo": "happy", "mg": "sambatra", "ms": "gembira,kata akar", "my": "happy", "pl": "szczęśliwy", "ro": "heureux", "ru": "счастье", "sk": "šťastie", "sl": "srečen", "sv": "lycklig", "sw": "heureux", "th": "ความสุข", "tok": "happy", "tr": "mutlu", "uk": "щасливий", "ur": "خوش", "vi": "hài lòng", "yo": "happy", "zh-tw": "幸福, 快樂", "zh": "幸福,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 57,
		},
		{
			slug: "vortaro-radiko-ferm", typ: "vocab",
			content: map[string]interface{}{
				"word": "ferm",
				"definition": "to close",
				"definitions": map[string]interface{}{"en": "to close", "nl": "sluiten", "de": "schließen", "fr": "fermer", "es": "cerrar", "pt": "fechar", "ar": "أغلق", "be": "закрыть", "ca": "tancar", "cs": "zavřít", "da": "lukke", "el": "το να κλείνω", "fa": "بستن", "frp": "fermer", "ga": "dún", "he": "לסגור", "hi": "to close", "hr": "zatvoriti", "hu": "zár", "id": "tutup", "it": "chiudere", "ja": "閉じる", "kk": "to close", "km": "to close", "ko": "닫다", "ku": "بستن", "lo": "to close", "mg": "manakatona", "ms": "menutup,kata akar", "my": "to close", "pl": "zamykać", "ro": "fermer", "ru": "закрыть", "sk": "zavrieť", "sl": "zapreti", "sv": "stänga", "sw": "fermer", "th": "ปิด", "tok": "to close", "tr": "kapatmak", "uk": "закривати, зачиняти", "ur": "بند کرنا", "vi": "đóng lại", "yo": "to close", "zh-tw": "關閉", "zh": "关闭,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 58,
		},
		{
			slug: "vortaro-radiko-fin", typ: "vocab",
			content: map[string]interface{}{
				"word": "fin",
				"definition": "to end",
				"definitions": map[string]interface{}{"en": "to end", "nl": "einde", "de": "beenden", "fr": "finir", "es": "acabar, terminar", "pt": "fim, finalizar", "ar": "ينهى", "be": "закончить", "ca": "acabar", "cs": "ukončit", "da": "færdiggøre", "el": "το να τελειώνω", "fa": "پایان دادن", "frp": "finir", "ga": "críoch", "he": "לסיים", "hi": "to end", "hr": "kraj, završetak", "hu": "vég", "id": "akhir", "it": "finire", "ja": "終える", "kk": "to end", "km": "to end", "ko": "끝내다", "ku": "پایان دادن", "lo": "to end", "mg": "mamarana", "ms": "mengakhiri,kata akar", "my": "to end", "pl": "kończyć", "ro": "finir", "ru": "закончить", "sk": "končiť", "sl": "končati", "sv": "slut", "sw": "finir", "th": "จบ, เลิก", "tok": "to end", "tr": "bitirmek", "uk": "кінчати, закінчувати, завершувати, припиняти", "ur": "ختم کرنا", "vi": "chấm dứt", "yo": "to end", "zh-tw": "結束", "zh": "结束,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 59,
		},
		{
			slug: "vortaro-radiko-flank", typ: "vocab",
			content: map[string]interface{}{
				"word": "flank",
				"definition": "side",
				"definitions": map[string]interface{}{"en": "side", "nl": "zijde (kant)", "de": "Seite", "fr": "côté", "es": "lado", "pt": "lado", "ar": "جانب", "be": "сторона, бок", "ca": "costat", "cs": "strana", "da": "side", "el": "πλευρά", "fa": "سمت", "frp": "côté", "ga": "taobh", "he": "צד, אגף", "hi": "side", "hr": "strana", "hu": "oldal", "id": "sisi", "it": "lato", "ja": "脇の", "kk": "side", "km": "side", "ko": "측면, 면", "ku": "سمت", "lo": "side", "mg": "ila , lafy", "ms": "tepi,kata akar", "my": "side", "pl": "strona", "ro": "côté", "ru": "сторона, бок", "sk": "bok, strana", "sl": "stran", "sv": "sida", "sw": "côté", "th": "ด้าน", "tok": "side", "tr": "taraf", "uk": "сторона", "ur": "سمت", "vi": "khía cạnh", "yo": "side", "zh-tw": "方面, 側", "zh": "方面、侧,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 60,
		},
		{
			slug: "vortaro-radiko-foj", typ: "vocab",
			content: map[string]interface{}{
				"word": "foj",
				"definition": "occasion, time",
				"definitions": map[string]interface{}{"en": "occasion, time", "nl": "keer", "de": "Mal", "fr": "fois", "es": "vez", "pt": "ocasião, vez", "ar": "مرة", "be": "раз, случай", "ca": "vegada, cop", "cs": "krát", "da": "gang, tilfælde", "el": "φορά", "fa": "دفعه, بار", "frp": "fois", "ga": "uair, ócáid", "he": "פעם", "hi": "time, occasion", "hr": "puta (višestrukost), -struk", "hu": "alkalom", "id": "kali, kejadian", "it": "volta, occasione", "ja": "回, 度", "kk": "time, occasion", "km": "time, occasion", "ko": "번, 차", "ku": "دفعه, بار", "lo": "time, occasion", "mg": "indray, in... , im...", "ms": "masa,kata akar, kali,kata akar", "my": "time, occasion", "pl": "raz, pewnego razu", "ro": "fois", "ru": "раз, случай", "sk": "-krát", "sl": "krat, včasih", "sv": "gång, tillfälle", "sw": "fois", "th": "ครั้ง", "tok": "occasion, time", "tr": "sefer, kez, sıklık", "uk": "раз", "ur": "دفعہ, occasion", "vi": "lần, dịp, cơ hội", "yo": "occasion, time", "zh-tw": "有時, 次", "zh": "有时,词根, 次,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 61,
		},
		{
			slug: "vortaro-radiko-forges", typ: "vocab",
			content: map[string]interface{}{
				"word": "forges",
				"definition": "to forget",
				"definitions": map[string]interface{}{"en": "to forget", "nl": "vergeten", "de": "vergessen", "fr": "oublier", "es": "olvidar", "pt": "esquecer", "ar": "ينسى", "be": "забыть", "ca": "oblidar", "cs": "zapomenout", "da": "glemme", "el": "το να ξεχνώ", "fa": "فراموش کردن", "frp": "oublier", "ga": "dearmad", "he": "לשכוח", "hi": "to forget", "hr": "zaboraviti", "hu": "felejt", "id": "lupa", "it": "dimenticare", "ja": "忘れる", "kk": "to forget", "km": "to forget", "ko": "잊다", "ku": "فراموش کردن", "lo": "to forget", "mg": "manadino", "ms": "lupa,kata akar", "my": "to forget", "pl": "zapominać", "ro": "oublier", "ru": "забыть", "sk": "zabudnúť", "sl": "pozabiti", "sv": "glömma", "sw": "oublier", "th": "ลืม", "tok": "to forget", "tr": "unutmak", "uk": "забувати", "ur": "بھول جانا", "vi": "quên", "yo": "to forget", "zh-tw": "忘記", "zh": "忘记,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 62,
		},
		{
			slug: "vortaro-radiko-form", typ: "vocab",
			content: map[string]interface{}{
				"word": "form",
				"definition": "form",
				"definitions": map[string]interface{}{"en": "form", "nl": "vorm", "de": "Form", "fr": "forme", "es": "forma", "pt": "formar", "ar": "شكل", "be": "форма", "ca": "forma", "cs": "forma", "da": "form", "el": "σχήμα", "fa": "شکل", "frp": "forme", "ga": "foirm", "he": "צורה", "hi": "form", "hr": "oblik", "hu": "forma", "id": "bentuk", "it": "formare", "ja": "形", "kk": "form", "km": "form", "ko": "형성하다", "ku": "شکل", "lo": "form", "mg": "tarehy , endrika , bika", "ms": "menjadi,kata akar", "my": "form", "pl": "forma", "ro": "forme", "ru": "форма", "sk": "forma", "sl": "oblika", "sv": "form", "sw": "forme", "th": "รูปแบบ", "tok": "form", "tr": "şekillendirmek, biçimlendirmek", "uk": "форма, вигляд, постать", "ur": "form", "vi": "hình thành", "yo": "form", "zh-tw": "形式", "zh": "形式,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 63,
		},
		{
			slug: "vortaro-radiko-fort", typ: "vocab",
			content: map[string]interface{}{
				"word": "fort",
				"definition": "strong",
				"definitions": map[string]interface{}{"en": "strong", "nl": "sterk", "de": "stark", "fr": "fort", "es": "fuerte", "pt": "forte, força", "ar": "قوى", "be": "сильный", "ca": "fort/a", "cs": "silný", "da": "stærk", "el": "δυνατός-ή-ό", "fa": "قوی", "frp": "fort", "ga": "láidir", "he": "כוח", "hi": "strong", "hr": "jak", "hu": "erős", "id": "kuat", "it": "forte", "ja": "強い", "kk": "strong", "km": "strong", "ko": "강한", "ku": "قوی", "lo": "strong", "mg": "matanjaka", "ms": "kuat,kata akar", "my": "strong", "pl": "silny", "ro": "fort", "ru": "сильный", "sk": "silný", "sl": "močan", "sv": "stark", "sw": "fort", "th": "แข็งแรง", "tok": "strong", "tr": "güçlü", "uk": "сильний, міцний, дужий", "ur": "مضبوط", "vi": "mạnh mẽ", "yo": "strong", "zh-tw": "強壯", "zh": "壮,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 64,
		},
		{
			slug: "vortaro-radiko-fot", typ: "vocab",
			content: map[string]interface{}{
				"word": "fot",
				"definition": "photo",
				"definitions": map[string]interface{}{"en": "photo", "nl": "foto", "de": "Foto", "fr": "photo", "es": "fotografía", "pt": "foto, fotografar", "ar": "صور", "be": "фотография", "ca": "foto", "cs": "foto", "da": "foto", "el": "φωτογραφία", "fa": "عکس", "frp": "photo", "ga": "grianghraf", "he": "לצלם", "hi": "photo", "hr": "fotografija", "hu": "fénykép", "id": "foto", "it": "foto", "ja": "写真", "kk": "photo", "km": "photo", "ko": "사진", "ku": "عکس", "lo": "photo", "mg": "sary", "ms": "foto,kata akar", "my": "photo", "pl": "zdjęcie", "ro": "photo", "ru": "фотография", "sk": "foto", "sl": "slika", "sv": "foto", "sw": "photo", "th": "รูปภาพ", "tok": "photo", "tr": "fotoğraf", "uk": "світлина, фото", "ur": "تصویر", "vi": "ảnh chụp", "yo": "photo", "zh-tw": "照片", "zh": "相片,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 65,
		},
		{
			slug: "vortaro-radiko-frat", typ: "vocab",
			content: map[string]interface{}{
				"word": "frat",
				"definition": "brother",
				"definitions": map[string]interface{}{"en": "brother", "nl": "broer", "de": "Bruder", "fr": "frère", "es": "hermano", "pt": "irmão", "ar": "شقيق, أخ", "be": "брат", "ca": "germà", "cs": "bratr", "da": "bror", "el": "αδελφός-ή-ό", "fa": "برادر", "frp": "frère", "ga": "deartháir, bráthair", "he": "אח", "hi": "brother", "hr": "brat", "hu": "fiútestvér", "id": "saudara", "it": "fratello", "ja": "兄弟", "kk": "аға", "km": "brother", "ko": "형제", "ku": "برادر", "lo": "brother", "mg": "rahalahy", "ms": "adik atau abang,kata akar", "my": "brother", "pl": "brat", "ro": "frère", "ru": "брат", "sk": "brat", "sl": "brat", "sv": "bror", "sw": "frère", "th": "พี่ชาย, น้องชาย", "tok": "brother", "tr": "kardeş", "uk": "брат", "ur": "بھائی", "vi": "anh em trai", "yo": "brother", "zh-tw": "兄弟", "zh": "兄/弟,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 66,
		},
		{
			slug: "vortaro-radiko-fru", typ: "vocab",
			content: map[string]interface{}{
				"word": "fru",
				"definition": "early",
				"definitions": map[string]interface{}{"en": "early", "nl": "vroeg", "de": "früh", "fr": "tôt", "es": "temprano/a", "pt": "cedo", "ar": "مبكرا", "be": "рано", "ca": "d'hora, aviat", "cs": "brzy", "da": "tidlig", "el": "νωρίς", "fa": "زود, زودرس", "frp": "tôt", "ga": "luath", "he": "מוקדם", "hi": "early", "hr": "rano", "hu": "korán", "id": "awal", "it": "presto", "ja": "早い", "kk": "early", "km": "early", "ko": "이른", "ku": "زود, زودرس", "lo": "early", "mg": "faingana", "ms": "awal,kata akar", "my": "early", "pl": "wcześnie", "ro": "tôt", "ru": "рано", "sk": "včas, skoro", "sl": "zgodaj", "sv": "tidig", "sw": "tôt", "th": "ก่อนเวลา", "tok": "early", "tr": "erken", "uk": "рано", "ur": "سویرے/جلدی", "vi": "sớm", "yo": "early", "zh-tw": "早", "zh": "早，反义词是迟,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 67,
		},
		{
			slug: "vortaro-radiko-fusx", typ: "vocab",
			content: map[string]interface{}{
				"word": "fuŝ",
				"definition": "to mess up",
				"definitions": map[string]interface{}{"en": "to mess up", "nl": "prutsen, knoeien, sukkelen", "de": "pfuschen", "fr": "gâcher", "es": "hacer una chapuza", "pt": "bagunçar, bagunçado", "ar": "فَسَدَ", "be": "делать небрежно, кое-как", "ca": "espatllar, malmetre, esguerrar", "cs": "odbýt", "da": "sjuske", "el": "το να μπερδεύω", "fa": "سنبل کردن", "frp": "louper", "ga": "déan praiseach", "he": "לפשל, לקלקל", "hi": "to mess up", "hr": "pokvariti, upropastiti", "hu": "ront", "id": "kacau", "it": "pasticciare, rovinare", "ja": "やり損なう", "kk": "to mess up", "km": "to mess up", "ko": "망치다", "ku": "سنبل کردن", "lo": "to mess up", "mg": "manaonao foana", "ms": "merosakkan,kata akar", "my": "to mess up", "pl": "psuć, rujnować, schrzanić", "ro": "gâcher", "ru": "делать небрежно, кое-как", "sk": "zbabrať, pokaziť", "sl": "pokvariti", "sv": "slarva, fördärva", "sw": "gâcher", "th": "ทำผิด", "tok": "to mess up", "tr": "batırmak, dağıtmak, yüzüne bulaştırmak", "uk": "робити що-небудь наспіх, халтурити, псувати", "ur": "to mess up", "vi": "làm hỏng, phá đám", "yo": "to mess up", "zh-tw": "搞糟", "zh": "搞糟,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 68,
		},
		{
			slug: "vortaro-radiko-gondol", typ: "vocab",
			content: map[string]interface{}{
				"word": "gondol",
				"definition": "gondola",
				"definitions": map[string]interface{}{"en": "gondola", "nl": "gondel", "de": "Gondel", "fr": "gondole", "es": "góndola", "pt": "gôndola", "ar": "جندول", "be": "гондола", "ca": "góndola", "cs": "gondola", "da": "gondol", "el": "γόνδολα", "fa": "گوندولا (نوعی قایق مورد استفاده در ونیز)", "frp": "gondole", "ga": "gondala", "he": "גונדולה", "hi": "gondola", "hr": "gondola", "hu": "gondola", "id": "gondola", "it": "gondola", "ja": "ゴンドラ", "kk": "гондола", "km": "gondola", "ko": "곤돌라", "ku": "گوندولا (نوعی قایق مورد استفاده در ونیز)", "lo": "gondola", "mg": "gondole", "ms": "gondola,kata akar", "my": "gondola", "pl": "gondola", "ro": "gondole", "ru": "гондола", "sk": "gondola", "sl": "gondola", "sv": "gondol", "sw": "gondole", "th": "เรือกอนโดลา", "tok": "gondola", "tr": "gondol", "uk": "ґондола", "ur": "gondola", "vi": "thuyền gondola", "yo": "gondola", "zh-tw": "貢多拉", "zh": "贡多拉,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 69,
		},
		{
			slug: "vortaro-radiko-graf", typ: "vocab",
			content: map[string]interface{}{
				"word": "graf",
				"definition": "earl, count",
				"definitions": map[string]interface{}{"en": "earl, count", "nl": "graaf", "de": "Graf", "fr": "comte", "es": "conde", "pt": "conde", "ar": "الكونت", "be": "граф", "ca": "comte", "cs": "hrabě", "da": "greve", "el": "κόμης", "fa": "اشرافی اروپایی, کُنت", "frp": "comte", "ga": "cúnta", "he": "גרף, תואר אצולה", "hi": "count, earl", "hr": "grof", "hu": "gróf", "id": "count, earl", "it": "conte", "ja": "伯爵", "kk": "count, earl", "km": "count, earl", "ko": "백작, 자작", "ku": "اشرافی اروپایی, کُنت", "lo": "count, earl", "mg": "comte", "ms": "mengira,kata akar", "my": "count, earl", "pl": "hrabia", "ro": "comte", "ru": "граф", "sk": "gróf", "sl": "grof", "sv": "greve", "sw": "comte", "th": "ขุนนาง", "tok": "earl, count", "tr": "Kont (soylu)", "uk": "граф", "ur": "count, earl", "vi": "bá tước", "yo": "earl, count", "zh-tw": "伯爵", "zh": "伯爵,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 70,
		},
		{
			slug: "vortaro-radiko-grand", typ: "vocab",
			content: map[string]interface{}{
				"word": "grand",
				"definition": "big, great",
				"definitions": map[string]interface{}{"en": "big, great", "nl": "groot", "de": "groß", "fr": "grand", "es": "grande, gran", "pt": "grande", "ar": "كبير", "be": "большой, великий", "ca": "gran", "cs": "velký", "da": "stor", "el": "μεγάλος-η-ο", "fa": "بزرگ", "frp": "grand", "ga": "mór", "he": "גדול", "hi": "big, great", "hr": "velik", "hu": "nagy", "id": "besar, hebat", "it": "grande", "ja": "大きい, 偉大な", "kk": "big, great", "km": "big, great", "ko": "커다란, 위대한", "ku": "بزرگ", "lo": "big, great", "mg": "lehibe , vaventy", "ms": "besar,kata akar", "my": "big, great", "pl": "duży, wielki", "ro": "grand", "ru": "большой, великий", "sk": "veľký", "sl": "velik", "sv": "stor", "sw": "grand", "th": "ใหญ่", "tok": "big, great", "tr": "büyük", "uk": "великий", "ur": "بڑا, عظیم", "vi": "to lớn, vĩ đại", "yo": "big, great", "zh-tw": "大, 巨大", "zh": "大,词根, 巨大,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 71,
		},
		{
			slug: "vortaro-radiko-grav", typ: "vocab",
			content: map[string]interface{}{
				"word": "grav",
				"definition": "important",
				"definitions": map[string]interface{}{"en": "important", "nl": "belangrijk", "de": "wichtig", "fr": "important", "es": "importante", "pt": "importante", "ar": "مهم", "be": "важный", "ca": "important", "cs": "důležitý", "da": "vigtig", "el": "σπουδαίος-α-ο", "fa": "مهم", "frp": "important", "ga": "tábhachtach", "he": "חשוב", "hi": "important", "hr": "važan", "hu": "fontos", "id": "penting", "it": "important", "ja": "重要な", "kk": "important", "km": "សំខាន់", "ko": "중요한", "ku": "مهم", "lo": "important", "mg": "mahavita be", "ms": "penting,kata akar", "my": "important", "pl": "ważny", "ro": "important", "ru": "важный", "sk": "dôležitý", "sl": "pomemben", "sv": "viktig", "sw": "important", "th": "สำคัญ", "tok": "important", "tr": "önemli", "uk": "важливий", "ur": "اہم", "vi": "quan trọng", "yo": "important", "zh-tw": "重要", "zh": "重要,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 72,
		},
		{
			slug: "vortaro-radiko-halt", typ: "vocab",
			content: map[string]interface{}{
				"word": "halt",
				"definition": "to stop",
				"definitions": map[string]interface{}{"en": "to stop", "nl": "halthouden", "de": "anhalten", "fr": "s'arrêter", "es": "parar(se)", "pt": "parar", "ar": "توقف", "be": "остановить", "ca": "aturar-se, parar-se", "cs": "zastavit", "da": "stoppe", "el": "το να σταματώ", "fa": "متوقف شدن", "frp": "s'arrêter", "ga": "stopadh", "he": "לעצור", "hi": "to stop", "hr": "zaustaviti", "hu": "megáll", "id": "berhenti", "it": "fermare", "ja": "止まる", "kk": "тоқтату; доғару", "km": "to stop", "ko": "정지하다, 멈추다", "ku": "متوقف شدن", "lo": "to stop", "mg": "mijanona", "ms": "menghentikan,kata akar", "my": "to stop", "pl": "zatrzymać", "ro": "s'arrêter", "ru": "остановить", "sk": "zastaviť", "sl": "ustaviti se", "sv": "stanna", "sw": "s'arrêter", "th": "หยุด", "tok": "to stop", "tr": "durdurmak", "uk": "зупинятися, cтавати, затримуватися", "ur": "سونا", "vi": "dừng lại", "yo": "to stop", "zh-tw": "停止", "zh": "停止,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 73,
		},
		{
			slug: "vortaro-radiko-hav", typ: "vocab",
			content: map[string]interface{}{
				"word": "hav",
				"definition": "to have",
				"definitions": map[string]interface{}{"en": "to have", "nl": "hebben", "de": "haben", "fr": "avoir", "es": "tener", "pt": "ter", "ar": "يملك", "be": "иметь", "ca": "tenir", "cs": "vlastnit", "da": "have", "el": "το να έχω", "fa": "داشتن", "frp": "avoir", "ga": "bheith ....ag", "he": "להיות בעל רכוש או דבר אחר", "hi": "to have", "hr": "imati", "hu": "van neki", "id": "punya", "it": "avere", "ja": "持つ", "kk": "to have", "km": "to have", "ko": "가지다", "ku": "داشتن", "lo": "to have", "mg": "manana", "ms": "mempunyai,kata akar", "my": "to have", "pl": "mieć", "ro": "avoir", "ru": "иметь", "sk": "mať, vlastniť", "sl": "imeti", "sv": "ha", "sw": "avoir", "th": "มี", "tok": "to have", "tr": "sahip olmak", "uk": "мати (дієслово)", "ur": "to have", "vi": "có", "yo": "to have", "zh-tw": "有", "zh": "有,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 74,
		},
		{
			slug: "vortaro-radiko-hejm", typ: "vocab",
			content: map[string]interface{}{
				"word": "hejm",
				"definition": "home",
				"definitions": map[string]interface{}{"en": "home", "nl": "huis, thuis", "de": "Heim, Zuhause", "fr": "foyer", "es": "casa", "pt": "lar, doméstico", "ar": "منزل", "be": "дома", "ca": "casa, llar", "cs": "domov", "da": "hjem", "el": "σπίτι", "fa": "خانه", "frp": "foyer", "ga": "baile", "he": "בית", "hi": "home", "hr": "dom", "hu": "otthon", "id": "rumah", "it": "casa propria", "ja": "家庭", "kk": "үй", "km": "home", "ko": "가정", "ku": "خانه", "lo": "home", "mg": "fatana", "ms": "rumah,kata akar", "my": "home", "pl": "dom", "ro": "foyer", "ru": "дома", "sk": "domov, domácnosť", "sl": "dom", "sv": "hem", "sw": "foyer", "th": "บ้าน", "tok": "home", "tr": "ev", "uk": "дім, домівка, домашнє вогнище", "ur": "گھر", "vi": "nhà", "yo": "home", "zh-tw": "家", "zh": "家,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 75,
		},
		{
			slug: "vortaro-radiko-help", typ: "vocab",
			content: map[string]interface{}{
				"word": "help",
				"definition": "to help",
				"definitions": map[string]interface{}{"en": "to help", "nl": "helpen", "de": "helfen", "fr": "aider", "es": "ayudar", "pt": "ajudar, ajuda", "ar": "مساعدة", "be": "помогать", "ca": "ajudar", "cs": "pomoct", "da": "hjælpe", "el": "το να βοηθώ", "fa": "کمک کردن", "frp": "aider", "ga": "cabhair", "he": "לעזור", "hi": "to help", "hr": "pomoći", "hu": "segít", "id": "bantu", "it": "aiutare", "ja": "助ける", "kk": "to help", "km": "to help", "ko": "돕다", "ku": "کمک کردن", "lo": "to help", "mg": "manampy", "ms": "tolong,kata akar", "my": "to help", "pl": "pomoc", "ro": "aider", "ru": "помогать", "sk": "pomôcť", "sl": "pomagati", "sv": "hjälpa", "sw": "aider", "th": "ช่วย, ช่วยเหลือ", "tok": "to help", "tr": "yardım etmek", "uk": "допомагати", "ur": "مدد کرنا", "vi": "giúp đỡ", "yo": "to help", "zh-tw": "幫助", "zh": "帮助,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 76,
		},
		{
			slug: "vortaro-radiko-hom", typ: "vocab",
			content: map[string]interface{}{
				"word": "hom",
				"definition": "human",
				"definitions": map[string]interface{}{"en": "human", "nl": "mens", "de": "Mensch", "fr": "humain", "es": "persona, ser humano", "pt": "humano, ser humano", "ar": "بشري", "be": "человек", "ca": "persona, ésser humà", "cs": "člověk", "da": "menneske", "el": "άνθρωπος", "fa": "انسان", "frp": "humain", "ga": "duine", "he": "אדם", "hi": "human", "hr": "čovjek", "hu": "ember", "id": "manusia", "it": "umano", "ja": "人", "kk": "адам", "km": "human", "ko": "사람", "ku": "انسان", "lo": "human", "mg": "momba ny olona", "ms": "manusia,kata akar", "my": "human", "pl": "człowiek", "ro": "humain", "ru": "человек", "sk": "človek", "sl": "človek", "sv": "männsika", "sw": "humain", "th": "มนุษย์", "tok": "human", "tr": "insan", "uk": "людина", "ur": "انسان", "vi": "con người", "yo": "human", "zh-tw": "人", "zh": "人类,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 77,
		},
		{
			slug: "vortaro-radiko-hor", typ: "vocab",
			content: map[string]interface{}{
				"word": "hor",
				"definition": "hour",
				"definitions": map[string]interface{}{"en": "hour", "nl": "uur", "de": "Stunde", "fr": "heure", "es": "horo", "pt": "hora", "ar": "ساعة", "be": "час", "ca": "hora", "cs": "hodina", "da": "time, klokken", "el": "ώρα", "fa": "ساعت", "frp": "heure", "ga": "uair an chloig", "he": "שעה", "hi": "hour", "hr": "sat", "hu": "óra", "id": "jam", "it": "ora", "ja": "時間", "kk": "сағат", "km": "hour", "ko": "시간", "ku": "ساعت", "lo": "hour", "mg": "ora", "ms": "jam,kata akar", "my": "hour", "pl": "godzina", "ro": "heure", "ru": "час", "sk": "hodina", "sl": "ura", "sv": "timme", "sw": "heure", "th": "ชั่วโมง", "tok": "hour", "tr": "saat", "uk": "година", "ur": "گھنٹہ", "vi": "giờ", "yo": "hour", "zh-tw": "點鐘, 小時", "zh": "点钟,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 78,
		},
		{
			slug: "vortaro-radiko-hotel", typ: "vocab",
			content: map[string]interface{}{
				"word": "hotel",
				"definition": "hotel",
				"definitions": map[string]interface{}{"en": "hotel", "nl": "hotel", "de": "Hotel", "fr": "hôtel", "es": "hotel", "pt": "hotel", "ar": "فندق", "be": "гостиница, отель", "ca": "hotel", "cs": "hotel", "da": "hotel", "el": "ξενοδοχείο", "fa": "هتل", "frp": "hôtel", "ga": "óstán", "he": "מלון", "hi": "hotel", "hr": "hotel", "hu": "szálloda", "id": "hotel", "it": "hotel", "ja": "ホテル", "kk": "қонақхана", "km": "សណ្ឋាគារ", "ko": "호텔", "ku": "هتل", "lo": "hotel", "mg": "hotely", "ms": "hotel,kata akar", "my": "hotel", "pl": "hotel", "ro": "hôtel", "ru": "гостиница, отель", "sk": "hotel", "sl": "hotel", "sv": "hotell", "sw": "hôtel", "th": "โรงแรม", "tok": "hotel", "tr": "otel", "uk": "готель", "ur": "ہوٹل", "vi": "khách sạn", "yo": "hotel", "zh-tw": "旅館", "zh": "酒店,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 79,
		},
		{
			slug: "vortaro-radiko-ide", typ: "vocab",
			content: map[string]interface{}{
				"word": "ide",
				"definition": "idea",
				"definitions": map[string]interface{}{"en": "idea", "nl": "idee", "de": "Idee", "fr": "idée", "es": "idea", "pt": "ideia", "ar": "فكرة", "be": "идея", "ca": "idea", "cs": "idea", "da": "idé, ide", "el": "ιδέα", "fa": "فکر, اندیشه, ایده", "frp": "idée", "ga": "smaoineamh", "he": "רעיון", "hi": "idea", "hr": "ideja", "hu": "ötlet", "id": "ide", "it": "idea", "ja": "考え", "kk": "idea", "km": "idea", "ko": "아이디어", "ku": "فکر, اندیشه, ایده", "lo": "idea", "mg": "hevitra", "ms": "idea,kata akar", "my": "idea", "pl": "pomysł", "ro": "idée", "ru": "идея", "sk": "idea", "sl": "ideja", "sv": "idé", "sw": "idée", "th": "ความคิด", "tok": "idea", "tr": "fikir", "uk": "ідея", "ur": "خیال", "vi": "ý tưởng", "yo": "idea", "zh-tw": "想法", "zh": "想法,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 80,
		},
		{
			slug: "vortaro-radiko-industri", typ: "vocab",
			content: map[string]interface{}{
				"word": "industri",
				"definition": "industry",
				"definitions": map[string]interface{}{"en": "industry", "nl": "industrie", "de": "Industrie", "fr": "industrie", "es": "industria", "pt": "indústria, industrial", "ar": "صناعة", "be": "индустрия", "ca": "indústria", "cs": "průmysl", "da": "industri", "el": "βιομηχανία", "fa": "صنعت", "frp": "industrie", "ga": "tionscal", "he": "תעשיה", "hi": "industry", "hr": "industrija", "hu": "ipar", "id": "industri", "it": "industria", "ja": "産業, 工業", "kk": "industry", "km": "industry", "ko": "산업", "ku": "صنعت", "lo": "industry", "mg": "taozavatra", "ms": "industri,kata akar", "my": "industry", "pl": "przemysł", "ro": "industrie", "ru": "индустрия", "sk": "priemysel", "sl": "industrija", "sv": "industri", "sw": "industrie", "th": "อุตสาหกรรม", "tok": "industry", "tr": "endüstri", "uk": "індустрія", "ur": "صنعت", "vi": "kinh doanh", "yo": "industry", "zh-tw": "工業", "zh": "工业,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 81,
		},
		{
			slug: "vortaro-radiko-instru", typ: "vocab",
			content: map[string]interface{}{
				"word": "instru",
				"definition": "to teach",
				"definitions": map[string]interface{}{"en": "to teach", "nl": "onderwijzen", "de": "unterrichten", "fr": "enseigner", "es": "enseñar", "pt": "ensinar, ensinamento", "ar": "علم", "be": "обучать", "ca": "ensenyar, instruir", "cs": "učit", "da": "lære, undervise", "el": "το να διδάσκω", "fa": "تدریس کردن, یاد دادن, آموختن", "frp": "enseigner", "ga": "múin", "he": "ללמד", "hi": "to teach", "hr": "podučavati", "hu": "tanít", "id": "mengajar", "it": "insegnare", "ja": "教える", "kk": "to teach", "km": "to teach", "ko": "가르치다", "ku": "تدریس کردن, یاد دادن, آموختن", "lo": "to teach", "mg": "mampianatra", "ms": "mengajar,kata akar", "my": "to teach", "pl": "nauczać", "ro": "enseigner", "ru": "обучать", "sk": "učiť, školiť", "sl": "učiti", "sv": "lära (någon), lära ut, undervisa", "sw": "enseigner", "th": "สอน", "tok": "to teach", "tr": "öğretmek", "uk": "учити, навчати, викладати, наставляти", "ur": "سکھانا", "vi": "giảng dạy", "yo": "to teach", "zh-tw": "教", "zh": "教,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 82,
		},
		{
			slug: "vortaro-radiko-interes", typ: "vocab",
			content: map[string]interface{}{
				"word": "interes",
				"definition": "to interest",
				"definitions": map[string]interface{}{"en": "to interest", "nl": "interesse, belangstelling, interesseren", "de": "Interesse", "fr": "intéresser", "es": "interesar", "pt": "interessar, interessante", "ar": "اهتم", "be": "интересоваться", "ca": "interessar", "cs": "zajímat se", "da": "interessere", "el": "το να ενδιαφέρω", "fa": "علاقه", "frp": "intéresser", "ga": "suim", "he": "לעניין", "hi": "to interest", "hr": "zanimati", "hu": "érdekes", "id": "tertarik", "it": "interessare", "ja": "興味を持たせる", "kk": "to interest", "km": "to interest", "ko": "흥미나게 하다, 관여케 하다", "ku": "علاقه", "lo": "to interest", "mg": "mahaliana", "ms": "manarik minat,kata akar", "my": "to interest", "pl": "zaciekawiać", "ro": "intéresser", "ru": "интересоваться", "sk": "zaujímať sa", "sl": "zanimanje", "sv": "intressera", "sw": "intéresser", "th": "สนใจ", "tok": "to interest", "tr": "ilgilenmek", "uk": "цікавити", "ur": "دلچسپی ہونا", "vi": "quan tâm, thích thú", "yo": "to interest", "zh-tw": "興趣", "zh": "兴趣,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 83,
		},
		{
			slug: "vortaro-radiko-invit", typ: "vocab",
			content: map[string]interface{}{
				"word": "invit",
				"definition": "to invite",
				"definitions": map[string]interface{}{"en": "to invite", "nl": "uitnodigen", "de": "einladen", "fr": "inviter", "es": "invitar", "pt": "convidar, convite", "ar": "دعا", "be": "пригласить", "ca": "invitar, convidar", "cs": "pozvat", "da": "invitere", "el": "το να προσκαλώ", "fa": "دعوت کردن", "frp": "inviter", "ga": "cuireadh", "he": "להזמין", "hi": "to invite", "hr": "pozvati", "hu": "meghív", "id": "undang", "it": "invitare", "ja": "招待する", "kk": "to invite", "km": "to invite", "ko": "초청하다", "ku": "دعوت کردن", "lo": "to invite", "mg": "manasa", "ms": "mengundang,kata akar", "my": "to invite", "pl": "zapraszać", "ro": "inviter", "ru": "пригласить", "sk": "pozvať", "sl": "povabiti", "sv": "bjuda in", "sw": "inviter", "th": "เชิญ", "tok": "to invite", "tr": "davet etmek", "uk": "запрошувати", "ur": "دعوت دینا", "vi": "mời", "yo": "to invite", "zh-tw": "邀請", "zh": "邀请,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 84,
		},
		{
			slug: "vortaro-radiko-ir", typ: "vocab",
			content: map[string]interface{}{
				"word": "ir",
				"definition": "to go",
				"definitions": map[string]interface{}{"en": "to go", "nl": "gaan", "de": "gehen", "fr": "aller", "es": "ir", "pt": "ir, andar", "ar": "يذهب", "be": "идти", "ca": "anar", "cs": "jít", "da": "tage til", "el": "το να πηγαίνω", "fa": "رفتن", "frp": "aller", "ga": "dul", "he": "ללכת", "hi": "to go", "hr": "ići", "hu": "megy", "id": "pergi", "it": "andare", "ja": "行く", "kk": "to go", "km": "to go", "ko": "가다", "ku": "رفتن", "lo": "to go", "mg": "mandeha , mankany", "ms": "pergi,kata akar", "my": "to go", "pl": "iść", "ro": "aller", "ru": "идти", "sk": "ísť", "sl": "iti", "sv": "gå, åka", "sw": "aller", "th": "ไป", "tok": "to go", "tr": "gitmek", "uk": "йти", "ur": "جانا", "vi": "đi", "yo": "to go", "zh-tw": "去", "zh": "去,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 85,
		},
		{
			slug: "vortaro-radiko-itali", typ: "vocab",
			content: map[string]interface{}{
				"word": "itali",
				"definition": "Italy",
				"definitions": map[string]interface{}{"en": "Italy", "nl": "Italië", "de": "Italien", "fr": "Italie", "es": "Italia", "pt": "Itália", "ar": "إيطاليا", "be": "Италия", "ca": "Itàlia", "cs": "Itálie", "da": "Italien", "el": "Ιταλία", "fa": "ایتالیا", "frp": "Italie", "ga": "Iodáil", "he": "איטליה", "hi": "Italy", "hu": "olasz", "id": "Itali", "it": "Italia", "ja": "イタリア", "kk": "Италия", "km": "Italy", "ko": "이탈리아", "ku": "ایتالیا", "lo": "Italy", "mg": "italia", "ms": "Itali,kata akar", "my": "Italy", "pl": "Włochy", "ro": "Italie", "ru": "Италия", "sv": "Italien", "sw": "Italie", "th": "อิตาลี", "tok": "Italy", "tr": "İtalya", "uk": "Італія", "ur": "اطالیہ", "vi": "nước Ý", "yo": "Italy", "zh-tw": "義大利", "zh": "意大利,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 86,
		},
		{
			slug: "vortaro-radiko-jar", typ: "vocab",
			content: map[string]interface{}{
				"word": "jar",
				"definition": "year",
				"definitions": map[string]interface{}{"en": "year", "nl": "jaar", "de": "Jahr", "fr": "année", "es": "año", "pt": "ano, anual", "ar": "عام", "be": "год", "ca": "any", "cs": "rok", "da": "år", "el": "έτος", "fa": "سال", "frp": "année", "ga": "bliain", "he": "שנה", "hi": "year", "hr": "godina", "hu": "év", "id": "tahun", "it": "anno", "ja": "年", "kk": "жыл", "km": "year", "ko": "해, 년", "ku": "سال", "lo": "year", "mg": "taona", "ms": "tahun,kata akar", "my": "year", "pl": "rok", "ro": "année", "ru": "год", "sk": "rok", "sl": "leto", "sv": "år", "sw": "année", "th": "ปี", "tok": "year", "tr": "yıl", "uk": "рік", "ur": "سال", "vi": "năm", "yo": "year", "zh-tw": "年", "zh": "年,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 87,
		},
		{
			slug: "vortaro-radiko-jun", typ: "vocab",
			content: map[string]interface{}{
				"word": "jun",
				"definition": "young",
				"definitions": map[string]interface{}{"en": "young", "nl": "jong", "de": "jung", "fr": "jeune", "es": "joven", "pt": "jovem", "ar": "شاب", "be": "молодой", "ca": "jove", "cs": "mladý", "da": "ung", "el": "νέαρός-η-ο", "fa": "جوان", "frp": "jeune", "ga": "óg", "he": "צעיר", "hi": "young", "hr": "mlad", "hu": "fiatal", "id": "muda", "it": "giovane", "ja": "若い", "kk": "young", "km": "young", "ko": "젊은", "ku": "جوان", "lo": "young", "mg": "tanora", "ms": "muda,kata akar", "my": "young", "pl": "młody", "ro": "jeune", "ru": "молодой", "sk": "mladý", "sl": "mlad", "sv": "ung", "sw": "jeune", "th": "วัยรุ่น", "tok": "young", "tr": "genç", "uk": "молодий", "ur": "نوجوان", "vi": "trẻ trung", "yo": "young", "zh-tw": "年輕", "zh": "年轻,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 88,
		},
		{
			slug: "vortaro-radiko-kaf", typ: "vocab",
			content: map[string]interface{}{
				"word": "kaf",
				"definition": "coffee",
				"definitions": map[string]interface{}{"en": "coffee", "nl": "koffie", "de": "Kaffee", "fr": "café", "es": "café", "pt": "café", "ar": "قهوة", "be": "кофе", "ca": "cafè", "cs": "káva", "da": "kaffe", "el": "καφές", "fa": "قهوه", "frp": "café", "ga": "caife", "he": "קפה", "hi": "coffee", "hr": "kava", "hu": "kávé", "id": "kopi", "it": "caffè", "ja": "コーヒー", "kk": "кофе", "km": "កាហ្វេ", "ko": "커피", "ku": "قهوه", "lo": "coffee", "mg": "kafé", "ms": "kopi,kata akar", "my": "coffee", "pl": "kawa", "ro": "café", "ru": "кофе", "sk": "káva", "sl": "kava", "sv": "kaffe", "sw": "café", "th": "กาแฟ", "tok": "coffee", "tr": "kahve", "uk": "кава", "ur": "coffee", "vi": "cà phê", "yo": "coffee", "zh-tw": "咖啡", "zh": "咖啡,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 89,
		},
		{
			slug: "vortaro-radiko-kalv", typ: "vocab",
			content: map[string]interface{}{
				"word": "kalv",
				"definition": "bald",
				"definitions": map[string]interface{}{"en": "bald", "nl": "kaal", "de": "kahl", "fr": "chauve", "es": "calvo/a", "pt": "calvo, calva", "ar": "أصلع", "be": "лысый", "ca": "calv/a", "cs": "plešatý", "da": "skaldet", "el": "φαλακρός-ή-ό", "fa": "کچل", "frp": "bientôt", "ga": "maol", "he": "קירח", "hi": "bald", "hr": "ćelav", "hu": "kopasz", "id": "botak", "it": "calvo", "ja": "はげ頭の", "kk": "bald", "km": "bald", "ko": "대머리의", "ku": "کچل", "lo": "bald", "mg": "sola", "ms": "botak,kata akar", "my": "bald", "pl": "łysy", "ro": "chauve", "ru": "лысый", "sk": "plešatý", "sl": "plešast", "sv": "flintskallig", "sw": "chauve", "th": "หัวล้าน", "tok": "bald", "tr": "kel", "uk": "лисий", "ur": "گنجا", "vi": "hói", "yo": "bald", "zh-tw": "秃", "zh": "秃,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 90,
		},
		{
			slug: "vortaro-radiko-kap", typ: "vocab",
			content: map[string]interface{}{
				"word": "kap",
				"definition": "head",
				"definitions": map[string]interface{}{"en": "head", "nl": "kop", "de": "Kopf", "fr": "tête", "es": "cabeza", "pt": "cabeça", "ar": "الرأس", "be": "голова", "ca": "cap (part superior del cos)", "cs": "hlava", "da": "hoved", "el": "κεφάλι", "fa": "سر", "frp": "tête", "ga": "ceann", "he": "ראש", "hi": "head", "hr": "glava", "hu": "fej", "id": "kepala", "it": "testa", "ja": "頭", "kk": "бас", "km": "head", "ko": "머리", "ku": "سر", "lo": "head", "mg": "loha", "ms": "kepala,kata akar", "my": "head", "pl": "głowa", "ro": "tête", "ru": "голова", "sk": "hlava", "sl": "glava", "sv": "huvud", "sw": "tête", "th": "ศีรษะ", "tok": "head", "tr": "kafa", "uk": "голова", "ur": "سر", "vi": "cái đầu", "yo": "head", "zh-tw": "頭", "zh": "头,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 91,
		},
		{
			slug: "vortaro-radiko-kapabl", typ: "vocab",
			content: map[string]interface{}{
				"word": "kapabl",
				"definition": "capable",
				"definitions": map[string]interface{}{"en": "capable", "nl": "bekwaam", "de": "fähig", "fr": "capable", "es": "capaz", "pt": "capaz", "ar": "قادر", "be": "способный", "ca": "capaç", "cs": "schopný", "da": "i stand til, egnet", "el": "ικανός-ή-ό", "fa": "قادر", "frp": "capable", "ga": "cumas", "he": "מסוגל, יכול", "hi": "capable", "hr": "sposoban", "hu": "képes", "id": "mampu", "it": "capace", "ja": "能力がある", "kk": "capable", "km": "capable", "ko": "능력이 있는, 유능한, 자격이 있는", "ku": "قادر", "lo": "capable", "mg": "mahavita , mahazaka", "ms": "kebolehan,kata akar", "my": "capable", "pl": "zdolny", "ro": "capable", "ru": "способный", "sk": "schopný", "sl": "sposoben, zmožen", "sv": "kapabel", "sw": "capable", "th": "สามารถ", "tok": "capable", "tr": "yetenekli", "uk": "здатний", "ur": "قابل", "vi": "có khả năng", "yo": "capable", "zh-tw": "有能力", "zh": "有能力,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 92,
		},
		{
			slug: "vortaro-radiko-kapt", typ: "vocab",
			content: map[string]interface{}{
				"word": "kapt",
				"definition": "to catch",
				"definitions": map[string]interface{}{"en": "to catch", "nl": "vangen", "de": "fangen", "fr": "attraper", "es": "atrapar, capturar", "pt": "pegar", "ar": "قبض", "be": "ловить", "ca": "atrapar, capturar, segrestar", "cs": "chytit", "da": "fange", "el": "το να αρπάζω", "fa": "گرفتن", "frp": "attraper", "ga": "breith ar", "he": "לתפוס", "hi": "to catch", "hr": "uloviti, uhvatiti", "hu": "elfog, megfog", "id": "tangkap", "it": "catturare", "ja": "つかまえる", "kk": "to catch", "km": "to catch", "ko": "잡다", "ku": "گرفتن", "lo": "to catch", "mg": "misambotra ,mahazo", "ms": "menangkap,kata akar", "my": "to catch", "pl": "złapać", "ro": "attraper", "ru": "ловить", "sk": "chytiť", "sl": "ujeti", "sv": "fånga", "sw": "attraper", "th": "จับ", "tok": "to catch", "tr": "yakalamak", "uk": "ловити, піймати, хапати, схопити", "ur": "پکڑنا", "vi": "bắt lấy", "yo": "to catch", "zh-tw": "抓捕", "zh": "抓捕,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 93,
		},
		{
			slug: "vortaro-radiko-kar", typ: "vocab",
			content: map[string]interface{}{
				"word": "kar",
				"definition": "dear",
				"definitions": map[string]interface{}{"en": "dear", "nl": "lief, dierbaar, waard", "de": "lieb, teuer, wert", "fr": "cher", "es": "querido/a", "pt": "caro, cara", "ar": "عزيز", "be": "дорогой, милый", "ca": "estimat/da, benvolgut/da", "cs": "drahý (milý)", "da": "kære", "el": "αγαπητός-ή-ό", "fa": "عزیز, گران‌بها", "frp": "cher", "ga": "ionúin", "he": "יקר", "hi": "dear", "hr": "drag", "hu": "kedves, drága", "id": "sayang,", "it": "caro", "ja": "いとしい, 大事な", "kk": "қымбат", "km": "dear", "ko": "친애하는, 사랑하는, 소중한", "ku": "عزیز, گران‌بها", "lo": "dear", "mg": "lafo", "ms": "sayang,kata akar", "my": "dear", "pl": "drogi", "ro": "cher", "ru": "дорогой, милый", "sk": "drahý, milý", "sl": "ljub", "sv": "kär, älskad", "sw": "cher", "th": "สี่", "tok": "dear", "tr": "değerli, kıymetli", "uk": "дорогий, любий, милий", "ur": "پیارا، پیاری", "vi": "thân mến", "yo": "dear", "zh-tw": "珍愛", "zh": "心爱,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 94,
		},
		{
			slug: "vortaro-radiko-kastel", typ: "vocab",
			content: map[string]interface{}{
				"word": "kastel",
				"definition": "castle",
				"definitions": map[string]interface{}{"en": "castle", "nl": "slot, kasteel", "de": "Schloss, Kastell", "fr": "château", "es": "castillo", "pt": "castelo", "ar": "قلعة", "be": "замок", "ca": "castell", "cs": "hrad", "da": "slot", "el": "κάστρο", "fa": "قلعه", "frp": "château", "ga": "caisleán", "he": "מצודה", "hi": "castle", "hr": "dvorac", "hu": "kastély", "id": "kastil", "it": "castello", "ja": "城", "kk": "castle", "km": "castle", "ko": "성", "ku": "قلعه", "lo": "castle", "mg": "lapa", "ms": "istana,kata akar", "my": "castle", "pl": "zamek", "ro": "château", "ru": "замок", "sk": "hrad, zámok", "sl": "grad, dvorec", "sv": "slott", "sw": "château", "th": "ปราสาท", "tok": "castle", "tr": "kale", "uk": "зáмок", "ur": "قلعہ", "vi": "lâu đài", "yo": "castle", "zh-tw": "城堡", "zh": "城堡,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 95,
		},
		{
			slug: "vortaro-radiko-kav", typ: "vocab",
			content: map[string]interface{}{
				"word": "kav",
				"definition": "cave",
				"definitions": map[string]interface{}{"en": "cave", "nl": "hol, groeve", "de": "Höhle, Grube", "fr": "cave", "es": "cavidad", "pt": "caverna, cavidade", "ar": "مغارة", "be": "яма", "ca": "cavitat", "cs": "jeskyně", "da": "hule", "el": "σπηλιά", "fa": "حفره", "frp": "cave", "ga": "pluais", "he": "מערה", "hi": "cave", "hr": "šupljina, rupa", "hu": "pince", "id": "gua", "it": "kava", "ja": "空洞", "kk": "cave", "km": "cave", "ko": "동굴", "ku": "حفره", "lo": "cave", "mg": "lempona", "ms": "gua,kata akar", "my": "cave", "pl": "jaskinia", "ro": "cave", "ru": "яма", "sk": "jaskyňa", "sl": "jama", "sv": "håla, hålighet", "sw": "cave", "th": "ถ้ำ, โพรง", "tok": "cave", "tr": "mağara", "uk": "яма", "ur": "غار", "vi": "hang động", "yo": "cave", "zh-tw": "山洞", "zh": "山洞,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 96,
		},
		{
			slug: "vortaro-radiko-kelk", typ: "vocab",
			content: map[string]interface{}{
				"word": "kelk",
				"definition": "some",
				"definitions": map[string]interface{}{"en": "some", "nl": "enige, enkele", "de": "manch, einige", "fr": "quelques", "es": "alguno/a/os/as", "pt": "alguns, algumas", "ar": "بعض", "be": "некоторый", "ca": "alguns/algunes", "cs": "nějaký, několik", "da": "smule, nogen", "el": "μερικός-ή-ο", "fa": "چند", "frp": "quelques", "ga": "roinnt", "he": "כמה, אחדים", "hi": "some", "hr": "neki, nekoliko", "hu": "néhány", "id": "beberapa", "it": "qualche", "ja": "いくらかの", "kk": "some", "km": "some", "ko": "몇개의", "ku": "چند", "lo": "some", "mg": "tokony", "ms": "beberapa,kata akar", "my": "some", "pl": "niektóry", "ro": "quelques", "ru": "некоторый", "sk": "niektorý, nejaký, trocha", "sl": "nekaj", "sv": "några", "sw": "quelques", "th": "จำนวนหนึ่ง", "tok": "some", "tr": "bazı, bir kaç", "uk": "якийсь певний (про число)", "ur": "کچھ", "vi": "một vài", "yo": "some", "zh-tw": "數個, 若干個", "zh": "一些,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 97,
		},
		{
			slug: "vortaro-radiko-kelner", typ: "vocab",
			content: map[string]interface{}{
				"word": "kelner",
				"definition": "waiter",
				"definitions": map[string]interface{}{"en": "waiter", "nl": "ober", "de": "Kelner", "fr": "serveur", "es": "camarero/a", "pt": "garçom", "ar": "الخادم", "be": "официант, кельнер", "ca": "cambrer/a", "cs": "číšník", "da": "tjener", "el": "σερβιτόρος", "fa": "پیشخدمت", "frp": "serveur", "ga": "freastalaí", "he": "מלצר", "hi": "waiter", "hr": "konobar", "hu": "pincér", "id": "pelayan", "it": "cameriere", "ja": "ウェイター", "kk": "waiter", "km": "waiter", "ko": "웨이터", "ku": "پیشخدمت", "lo": "waiter", "mg": "mpizara", "ms": "pelayan,kata akar", "my": "waiter", "pl": "kelner", "ro": "serveur", "ru": "официант, кельнер", "sk": "čašník", "sl": "natakar", "sv": "kypare, servitör", "sw": "serveur", "th": "บริกร", "tok": "waiter", "tr": "garson", "uk": "кельнер", "ur": "waiter", "vi": "người phục vụ", "yo": "waiter", "zh-tw": "服務員, 侍應生", "zh": "服务员，侍应生,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 98,
		},
		{
			slug: "vortaro-radiko-klar", typ: "vocab",
			content: map[string]interface{}{
				"word": "klar",
				"definition": "clear",
				"definitions": map[string]interface{}{"en": "clear", "nl": "klaar", "de": "klar", "fr": "clair", "es": "claro", "pt": "claro, clara", "ar": "واضح", "be": "ясный, чёткий", "ca": "clar", "cs": "jasný", "da": "klar, ikke tåget", "el": "καθαρός-ή-ό", "fa": "واضح, شفاف", "frp": "clair", "ga": "soiléir", "he": "ברור", "hi": "clear", "hr": "jasan", "hu": "világos, tiszta", "id": "jelas", "it": "chiaro", "ja": "澄んだ, 明るい, はっきりした", "kk": "clear", "km": "clear", "ko": "맑은, 밝은", "ku": "واضح, شفاف", "lo": "clear", "mg": "mazava", "ms": "terang,kata akar", "my": "clear", "pl": "wyraźny", "ro": "clair", "ru": "ясный, чёткий", "sk": "jasný", "sl": "jasno", "sv": "klar, tydlig", "sw": "clair", "th": "กระจ่าง, สะอาด", "tok": "clear", "tr": "net, açık", "uk": "ясний, зрозумілий", "ur": "clear", "vi": "rõ ràng", "yo": "clear", "zh-tw": "解釋", "zh": "解释,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 99,
		},
		{
			slug: "vortaro-radiko-klub", typ: "vocab",
			content: map[string]interface{}{
				"word": "klub",
				"definition": "club",
				"definitions": map[string]interface{}{"en": "club", "nl": "club", "de": "Club", "fr": "club", "es": "club", "pt": "clube", "ar": "ناد", "be": "клуб", "ca": "club", "cs": "klub", "da": "klub", "el": "λέσχη", "fa": "باشگاه", "frp": "club", "ga": "club", "he": "מועדון", "hi": "club", "hr": "klub", "hu": "klub", "id": "klub", "it": "club", "ja": "クラブ", "kk": "клуб", "km": "club", "ko": "클럽", "ku": "باشگاه", "lo": "club", "mg": "fivoriana , fiokoana", "ms": "klub,kata akar", "my": "club", "pl": "klub", "ro": "club", "ru": "клуб", "sk": "klub", "sl": "klub", "sv": "klubb", "sw": "club", "th": "ชมรม", "tok": "club", "tr": "klup", "uk": "клуб", "ur": "کلب", "vi": "câu lạc bộ", "yo": "club", "zh-tw": "俱樂部", "zh": "俱乐部,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 100,
		},
		{
			slug: "vortaro-radiko-knab", typ: "vocab",
			content: map[string]interface{}{
				"word": "knab",
				"definition": "boy",
				"definitions": map[string]interface{}{"en": "boy", "nl": "jongen, knaap", "de": "Knabe", "fr": "garçon", "es": "chico", "pt": "garoto, menino", "ar": "صبي", "be": "мальчик", "ca": "noi", "cs": "kluk", "da": "dreng", "el": "αγόρι", "fa": "پسر", "frp": "garçon", "ga": "buachaill", "he": "נער", "hi": "boy", "hr": "dječak", "hu": "fiú", "id": "anak laki-laki", "it": "ragazzo", "ja": "少年", "kk": "бала", "km": "boy", "ko": "소년", "ku": "پسر", "lo": "boy", "mg": "zazalahy", "ms": "lelaki kecil,kata akar", "my": "boy", "pl": "chłopiec", "ro": "garçon", "ru": "мальчик", "sk": "chlapec", "sl": "deček", "sv": "pojke", "sw": "garçon", "th": "เด็กชาย", "tok": "boy", "tr": "oğlan", "uk": "хлопець", "ur": "لڑکا", "vi": "cậu bé", "yo": "boy", "zh-tw": "男孩", "zh": "小孩,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 101,
		},
		{
			slug: "vortaro-radiko-koler", typ: "vocab",
			content: map[string]interface{}{
				"word": "koler",
				"definition": "anger",
				"definitions": map[string]interface{}{"en": "anger", "nl": "boos, kwaad zijn", "de": "zürnen, zornig sein", "fr": "colère", "es": "estar enfadado", "pt": "raiva, colérico", "ar": "غضب", "be": "злоба, гнев", "ca": "estar enfadat", "cs": "vztek", "da": "vrede", "el": "θυμός", "fa": "عصبانی بودن", "frp": "colère", "ga": "fearg", "he": "כעס", "hi": "anger", "hr": "ljutiti se, ljutnja", "hu": "méreg", "id": "marah", "it": "rabbia", "ja": "怒る", "kk": "anger", "km": "anger", "ko": "화", "ku": "عصبانی بودن", "lo": "anger", "mg": "hatezerana", "ms": "marah,kata akar", "my": "anger", "pl": "złość", "ro": "colère", "ru": "злоба, гнев", "sk": "hnev", "sl": "jeza", "sv": "arg", "sw": "colère", "th": "โกรธ", "tok": "anger", "tr": "sinir", "uk": "сердитись", "ur": "غصہ", "vi": "giận dữ", "yo": "anger", "zh-tw": "生氣", "zh": "生气,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 102,
		},
		{
			slug: "vortaro-radiko-kolor", typ: "vocab",
			content: map[string]interface{}{
				"word": "kolor",
				"definition": "color",
				"definitions": map[string]interface{}{"en": "color", "nl": "kleur", "de": "Farbe", "fr": "couleur", "es": "color", "pt": "cor, colorido, colorida", "ar": "اللون", "be": "цвет", "ca": "color", "cs": "barva", "da": "farve", "el": "χρώμα", "fa": "رنگ", "frp": "couleur", "ga": "dath", "he": "צבע", "hi": "color", "hr": "boja", "hu": "szín", "id": "warna", "it": "colore", "ja": "色", "kk": "түс", "km": "color", "ko": "색깔", "ku": "رنگ", "lo": "color", "mg": "loko", "ms": "warna,kata akar", "my": "color", "pl": "kolor", "ro": "couleur", "ru": "цвет", "sk": "farba", "sl": "barva", "sv": "färg", "sw": "couleur", "th": "สี", "tok": "color", "tr": "renk", "uk": "колір", "ur": "رنگ", "vi": "sắc màu", "yo": "color", "zh-tw": "顏色", "zh": "颜色,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 103,
		},
		{
			slug: "vortaro-radiko-komenc", typ: "vocab",
			content: map[string]interface{}{
				"word": "komenc",
				"definition": "to begin",
				"definitions": map[string]interface{}{"en": "to begin", "nl": "beginnen", "de": "anfangen", "fr": "commencer", "es": "empezar", "pt": "começar, começo", "ar": "بداية", "be": "начинать", "ca": "començar", "cs": "začít", "da": "begynde", "el": "το να αρχίζω", "fa": "آغاز کردن", "frp": "commencer", "ga": "tosaigh", "he": "להתחיל", "hi": "to begin", "hr": "početi", "hu": "kezd", "id": "mulai", "it": "cominciare", "ja": "始める", "kk": "to begin", "km": "to begin", "ko": "시작하다", "ku": "آغاز کردن", "lo": "to begin", "mg": "manomboka", "ms": "mula,kata akar", "my": "to begin", "pl": "zaczynać", "ro": "commencer", "ru": "начинать", "sk": "začať", "sl": "začeti", "sv": "börja, påbörja", "sw": "commencer", "th": "เริ่ม", "tok": "to begin", "tr": "başlamak", "uk": "починати", "ur": "شروع کرنا", "vi": "bắt đầu", "yo": "to begin", "zh-tw": "開始", "zh": "开始,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 104,
		},
		{
			slug: "vortaro-radiko-kompren", typ: "vocab",
			content: map[string]interface{}{
				"word": "kompren",
				"definition": "to understand",
				"definitions": map[string]interface{}{"en": "to understand", "nl": "verstaan, begrijpen", "de": "verstehen", "fr": "comprendre", "es": "entender", "pt": "entender, compreender", "ar": "فهم", "be": "понимать", "ca": "entendre", "cs": "rozumět", "da": "forstå", "el": "το να καταλαβαίνω", "fa": "فهمیدن", "frp": "comprendre", "ga": "tuig", "he": "להבין", "hi": "to understand", "hr": "razumjeti", "hu": "ért", "id": "paham", "it": "capire", "ja": "理解する", "kk": "to understand", "km": "to understand", "ko": "이해하다", "ku": "فهمیدن", "lo": "to understand", "mg": "mahazo ,misy", "ms": "faham,kata akar", "my": "to understand", "pl": "rozumieć", "ro": "comprendre", "ru": "понимать", "sk": "rozumieť", "sl": "razumeti", "sv": "förstå", "sw": "comprendre", "th": "เข้าใจ", "tok": "to understand", "tr": "anlamak, kavramak", "uk": "розуміти", "ur": "سمجھنا", "vi": "hiểu, nắm được", "yo": "to understand", "zh-tw": "理解, 明白", "zh": "理解，明白,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 105,
		},
		{
			slug: "vortaro-radiko-kon", typ: "vocab",
			content: map[string]interface{}{
				"word": "kon",
				"definition": "to know",
				"definitions": map[string]interface{}{"en": "to know", "nl": "kennen", "de": "kennen", "fr": "connaître", "es": "conocer", "pt": "saber, conhecer", "ar": "يعرف", "be": "знать", "ca": "conèixer", "cs": "znát", "da": "kende", "el": "το να γνωρίζω", "fa": "شناختن", "frp": "connaître", "ga": "aithne", "he": "לדעת", "hi": "to know", "hr": "poznavati", "hu": "ismer", "id": "tahu, kenal", "it": "conoscere, sapere", "ja": "知る", "kk": "to know", "km": "to know", "ko": "알다", "ku": "شناختن", "lo": "to know", "mg": "mahay", "ms": "kenal,kata akar", "my": "to know", "pl": "znać", "ro": "connaître", "ru": "знать", "sk": "poznať, vedieť", "sl": "poznati", "sv": "känna (till)", "sw": "connaître", "th": "รู้", "tok": "to know", "tr": "bilmek", "uk": "знати", "ur": "جاننا", "vi": "quen biết", "yo": "to know", "zh-tw": "認得", "zh": "知道 (不是很理解，片面的),词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 106,
		},
		{
			slug: "vortaro-radiko-kontent", typ: "vocab",
			content: map[string]interface{}{
				"word": "kontent",
				"definition": "satisfied",
				"definitions": map[string]interface{}{"en": "satisfied", "nl": "tevreden", "de": "zufrieden", "fr": "content", "es": "contento", "pt": "satisfeito, satisfeita, contente", "ar": "محتوى", "be": "удовлетворённый, довольный", "ca": "satisfet/a, content/a", "cs": "spokojený", "da": "tilfreds", "el": "το να ικανοποιώ", "fa": "راضی", "frp": "content", "ga": "sásta", "he": "מרוצה", "hi": "satisfied", "hr": "zadovoljan", "hu": "elégedett", "id": "puas, senang", "it": "soddisfatto", "ja": "満足した", "kk": "satisfied", "km": "satisfied", "ko": "만족한", "ku": "راضی", "lo": "satisfied", "mg": "faly ,afa-po", "ms": "puas,kata akar", "my": "satisfied", "pl": "usatysfakcjonowany", "ro": "content", "ru": "удовлетворённый, довольный", "sk": "spokojný", "sl": "zadovoljen", "sv": "nöjd", "sw": "content", "th": "พอใจ", "tok": "satisfied", "tr": "tatmin olmak", "uk": "задоволений", "ur": "مطمئن", "vi": "thoả mãn", "yo": "satisfied", "zh-tw": "滿意", "zh": "满意,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 107,
		},
		{
			slug: "vortaro-radiko-kontrol", typ: "vocab",
			content: map[string]interface{}{
				"word": "kontrol",
				"definition": "to check",
				"definitions": map[string]interface{}{"en": "to check", "nl": "nazien", "de": "prüfen", "fr": "vérifier", "es": "controlar, comprobar", "pt": "checar, verificar, controlar", "ar": "تحقق, فحص", "be": "проверять, контроллировать", "ca": "controlar, comprovar", "cs": "kontrolovat", "da": "kontrollere, tjekke", "el": "το να ελέγχω", "fa": "کنترل کردن, بررسی کردن", "frp": "vérifier", "ga": "seiceáil", "he": "לבדוק", "hi": "to check", "hr": "provjeriti", "hu": "ellenőriz", "id": "periksa", "it": "controllare", "ja": "点検する", "kk": "to check", "km": "to check", "ko": "검사하다, 감독하다", "ku": "کنترل کردن, بررسی کردن", "lo": "to check", "mg": "mamotopototra", "ms": "semak,kata akar", "my": "to check", "pl": "sprawdzać", "ro": "vérifier", "ru": "проверять, контроллировать", "sk": "kontrolovať", "sl": "preveriti", "sv": "kontrollera", "sw": "vérifier", "th": "ตรวจสอบ", "tok": "to check", "tr": "kontrol etmek", "uk": "контролювати", "ur": "to check", "vi": "kiểm soát, kiểm tra", "yo": "to check", "zh-tw": "檢查", "zh": "检查,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 108,
		},
		{
			slug: "vortaro-radiko-kor", typ: "vocab",
			content: map[string]interface{}{
				"word": "kor",
				"definition": "heart",
				"definitions": map[string]interface{}{"en": "heart", "nl": "hart", "de": "Herz", "fr": "coeur", "es": "corazón", "pt": "coração, cordial", "ar": "قلب", "be": "сердце", "ca": "cor", "cs": "srdce", "da": "hjerte", "el": "καρδιά", "fa": "قلب", "frp": "coeur", "ga": "croí", "he": "לב", "hi": "heart", "hr": "srce", "hu": "szív", "id": "hati", "it": "cuore", "ja": "心臓, 心", "kk": "жүрек", "km": "heart", "ko": "심장", "ku": "قلب", "lo": "heart", "mg": "fo", "ms": "hati,kata akar", "my": "heart", "pl": "serce", "ro": "coeur", "ru": "сердце", "sk": "srdce", "sl": "srce", "sv": "hjärta", "sw": "coeur", "th": "หัวใจ", "tok": "heart", "tr": "kalp", "uk": "серце", "ur": "دل", "vi": "con tim", "yo": "heart", "zh-tw": "心", "zh": "心,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 109,
		},
		{
			slug: "vortaro-radiko-korp", typ: "vocab",
			content: map[string]interface{}{
				"word": "korp",
				"definition": "body",
				"definitions": map[string]interface{}{"en": "body", "nl": "lichaam", "de": "Körper", "fr": "corps", "es": "cuerpo", "pt": "corpo", "ar": "جسد", "be": "тело", "ca": "cos", "cs": "tělo", "da": "krop", "el": "σώμα", "fa": "بدن", "frp": "corps", "ga": "corp", "he": "גוף", "hi": "body", "hr": "tijelo", "hu": "test", "id": "badan, tubuh", "it": "corpo", "ja": "からだ", "kk": "дене; тән", "km": "body", "ko": "신체", "ku": "بدن", "lo": "body", "mg": "vatana", "ms": "badan,kata akar", "my": "body", "pl": "ciało", "ro": "corps", "ru": "тело", "sk": "telo", "sl": "telo", "sv": "kropp", "sw": "corps", "th": "ร่างกาย", "tok": "body", "tr": "beden", "uk": "тіло", "ur": "جسم", "vi": "thân thể", "yo": "body", "zh-tw": "身體", "zh": "身体,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 110,
		},
		{
			slug: "vortaro-radiko-kost", typ: "vocab",
			content: map[string]interface{}{
				"word": "kost",
				"definition": "to cost",
				"definitions": map[string]interface{}{"en": "to cost", "nl": "kosten", "de": "kosten", "fr": "coûter", "es": "costar", "pt": "custar, custoso", "ar": "يكلف", "be": "стоить, цена", "ca": "costar", "cs": "stát (o ceně)", "da": "koste", "el": "το να κοστίζω", "fa": "ارزیدن", "frp": "coûter", "ga": "costas", "he": "מחיר", "hi": "to cost", "hr": "koštati, stajati", "hu": "kerül", "id": "harga", "it": "costare", "ja": "費用がかかる", "kk": "to cost", "km": "to cost", "ko": "비용", "ku": "ارزیدن", "lo": "to cost", "mg": "vidiny", "ms": "harga,kata akar", "my": "to cost", "pl": "kosztować", "ro": "coûter", "ru": "стоить, цена", "sk": "stáť (o cene)", "sl": "stati (cenovno)", "sv": "kosta", "sw": "coûter", "th": "มีราคา", "tok": "to cost", "tr": "tutmak, etmek (fiyat), maaliyet", "uk": "коштувати", "ur": "to cost", "vi": "giá cả", "yo": "to cost", "zh-tw": "價格", "zh": "价格,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 111,
		},
		{
			slug: "vortaro-radiko-kri", typ: "vocab",
			content: map[string]interface{}{
				"word": "kri",
				"definition": "to scream",
				"definitions": map[string]interface{}{"en": "to scream", "nl": "schreeuwen", "de": "schreien", "fr": "crier", "es": "gritar", "pt": "gritar", "ar": "صرخة", "be": "кричать", "ca": "cridar", "cs": "křičet", "da": "skrige", "el": "το να φωνάζω", "fa": "فریاد زدن, جیغ زدن", "frp": "crier", "ga": "béic", "he": "לצעוק", "hi": "to scream", "hr": "vikati", "hu": "kiált", "id": "teriak", "it": "gridare", "ja": "叫ぶ", "kk": "to scream", "km": "to scream", "ko": "외치다", "ku": "فریاد زدن, جیغ زدن", "lo": "to scream", "mg": "mihorakoraka", "ms": "menjerit,kata akar", "my": "to scream", "pl": "krzyczeć", "ro": "crier", "ru": "кричать", "sk": "kričať", "sl": "kričati", "sv": "skrika", "sw": "crier", "th": "ตะโกน", "tok": "to scream", "tr": "çığlık atmak", "uk": "кричати", "ur": "چیخنا", "vi": "la hét", "yo": "to scream", "zh-tw": "叫喊", "zh": "叫喊,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 112,
		},
		{
			slug: "vortaro-radiko-kuir", typ: "vocab",
			content: map[string]interface{}{
				"word": "kuir",
				"definition": "to cook",
				"definitions": map[string]interface{}{"en": "to cook", "nl": "koken", "de": "kochen", "fr": "cuisiner", "es": "cocinar", "pt": "cozinhar", "ar": "طبخ", "be": "готовить (пищу)", "ca": "cuinar", "cs": "vařit", "da": "tilberede mad, lave mad", "el": "το να μαγειρεύω", "fa": "پختن", "frp": "cuisiner", "ga": "cócaireacht", "he": "לבשל", "hi": "to cook", "hr": "kuhati", "hu": "főz", "id": "masak", "it": "cucinare", "ja": "料理をする", "kk": "to cook", "km": "to cook", "ko": "요리하다", "ku": "پختن", "lo": "to cook", "mg": "mahandro", "ms": "memasak,kata akar", "my": "to cook", "pl": "gotować", "ro": "cuisiner", "ru": "готовить (пищу)", "sk": "variť", "sl": "kuhati", "sv": "laga (mat)", "sw": "cuisiner", "th": "ทำอาหาร, ปรุง, ชง", "tok": "to cook", "tr": "yemek yapmak, pişirmek", "uk": "варити", "ur": "پکانا", "vi": "nấu", "yo": "to cook", "zh-tw": "煮", "zh": "煮,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 113,
		},
		{
			slug: "vortaro-radiko-kuk", typ: "vocab",
			content: map[string]interface{}{
				"word": "kuk",
				"definition": "cake",
				"definitions": map[string]interface{}{"en": "cake", "nl": "koek, cake", "de": "Kuchen", "fr": "gâteau", "es": "pastel, torta", "pt": "bolo", "ar": "كعكة", "be": "торт", "ca": "pastís", "cs": "koláč", "da": "kage", "el": "γλύκισμα", "fa": "کیک", "frp": "gâteau", "ga": "císte", "he": "עוגה", "hi": "cake", "hr": "kolač", "hu": "süti", "id": "kue", "it": "torta", "ja": "菓子", "kk": "cake", "km": "នំ", "ko": "과자", "ku": "کیک", "lo": "cake", "mg": "mofo mamy", "ms": "kek,kata akar", "my": "cake", "pl": "ciasto", "ro": "gâteau", "ru": "торт", "sk": "koláč", "sl": "pecivo", "sv": "kaka", "sw": "gâteau", "th": "เค้ก", "tok": "cake", "tr": "kek", "uk": "пиріг", "ur": "cake", "vi": "bánh kem", "yo": "cake", "zh-tw": "蛋糕", "zh": "蛋糕,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 114,
		},
		{
			slug: "vortaro-radiko-kur", typ: "vocab",
			content: map[string]interface{}{
				"word": "kur",
				"definition": "to run",
				"definitions": map[string]interface{}{"en": "to run", "nl": "lopen, rennen, hardlopen", "de": "laufen", "fr": "courir", "es": "correr", "pt": "correr", "ar": "جري", "be": "бегать", "ca": "córrer", "cs": "běžet", "da": "løbe", "el": "το να τρέχω", "fa": "دویدن", "frp": "courir", "ga": "rith", "he": "לרוץ", "hi": "to run", "hr": "trčati", "hu": "fut", "id": "lari", "it": "correre", "ja": "走る", "kk": "to run", "km": "to run", "ko": "달리다", "ku": "دویدن", "lo": "to run", "mg": "mihazakazaka", "ms": "lari,kata akar", "my": "to run", "pl": "biec", "ro": "courir", "ru": "бегать", "sk": "bežať", "sl": "teči", "sv": "springa", "sw": "courir", "th": "วิ่ง", "tok": "to run", "tr": "koşmak", "uk": "бігти", "ur": "دوڑنا", "vi": "chạy", "yo": "to run", "zh-tw": "跑", "zh": "跑,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 115,
		},
		{
			slug: "vortaro-radiko-labor", typ: "vocab",
			content: map[string]interface{}{
				"word": "labor",
				"definition": "to work",
				"definitions": map[string]interface{}{"en": "to work", "nl": "werken, arbeiden", "de": "arbeiten", "fr": "travailler", "es": "trabajar", "pt": "trabalhar", "ar": "عمل", "be": "работать", "ca": "treballar", "cs": "pracovat", "da": "arbejde", "el": "το να εργάζομαι, το να δουλεύω", "fa": "کار کردن", "frp": "travailler", "ga": "obair", "he": "לעבוד", "hi": "to work", "hr": "raditi", "hu": "munka, dolgoz", "id": "kerja", "it": "lavorare", "ja": "働く", "kk": "to work", "km": "to work", "ko": "일하다", "ku": "کار کردن", "lo": "to work", "mg": "miasa", "ms": "bekerja,kata akar", "my": "to work", "pl": "pracować", "ro": "travailler", "ru": "работать", "sk": "pracovať", "sl": "delati", "sv": "arbete, jobb", "sw": "travailler", "th": "ทำงาน", "tok": "to work", "tr": "çalışmak", "uk": "працювати", "ur": "کام کرنا", "vi": "làm việc", "yo": "to work", "zh-tw": "工作", "zh": "工作,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 116,
		},
		{
			slug: "vortaro-radiko-lakt", typ: "vocab",
			content: map[string]interface{}{
				"word": "lakt",
				"definition": "milk",
				"definitions": map[string]interface{}{"en": "milk", "nl": "melk", "de": "Milch", "fr": "lait", "es": "leche", "pt": "leite", "ar": "حليب", "be": "молоко", "ca": "llet", "cs": "mléko", "da": "mælk", "el": "γάλα", "fa": "شیر", "frp": "lait", "ga": "bainne", "he": "חלב", "hi": "milk", "hr": "mlijeko", "hu": "tej", "id": "susu", "it": "latte", "ja": "ミルク", "kk": "milk", "km": "milk", "ko": "우유", "ku": "شیر", "lo": "milk", "mg": "ronono", "ms": "susu,kata akar", "my": "milk", "pl": "mleko", "ro": "lait", "ru": "молоко", "sk": "mlieko", "sl": "mleko", "sv": "mjölk", "sw": "lait", "th": "นำ้นม", "tok": "milk", "tr": "süt", "uk": "молоко", "ur": "دودھ", "vi": "sữa", "yo": "milk", "zh-tw": "牛奶", "zh": "牛奶,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 117,
		},
		{
			slug: "vortaro-radiko-lamp", typ: "vocab",
			content: map[string]interface{}{
				"word": "lamp",
				"definition": "lamp",
				"definitions": map[string]interface{}{"en": "lamp", "nl": "lamp", "de": "Lampe", "fr": "lampe", "es": "lámpara", "pt": "lâmpada", "ar": "مصباح", "be": "лампа", "ca": "làmpada", "cs": "lampa", "da": "lampe", "el": "λάμπα", "fa": "لامپ", "frp": "lampe", "ga": "lampa", "he": "מנורה", "hi": "lamp", "hr": "svjetiljka", "hu": "lámpa", "id": "lampu", "it": "lampada", "ja": "ランプ", "kk": "lamp", "km": "ចង្កៀង", "ko": "전등", "ku": "لامپ", "lo": "lamp", "mg": "jiro", "ms": "lampu,kata akar,", "my": "lamp", "pl": "lampa", "ro": "lampe", "ru": "лампа", "sk": "lampa", "sl": "luč", "sv": "lampa", "sw": "lampe", "th": "ตะเกียง", "tok": "lamp", "tr": "lamba", "uk": "лямпа", "ur": "لیمپ", "vi": "đèn", "yo": "lamp", "zh-tw": "燈", "zh": "灯,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 118,
		},
		{
			slug: "vortaro-radiko-land", typ: "vocab",
			content: map[string]interface{}{
				"word": "land",
				"definition": "land",
				"definitions": map[string]interface{}{"en": "land", "nl": "land", "de": "Land", "fr": "pays", "es": "país, territorio", "pt": "terra", "ar": "بلد", "be": "страна", "ca": "país, territori", "cs": "země", "da": "land", "el": "χώρα", "fa": "کشور, سرزمین", "frp": "pays", "ga": "talamh, tír", "he": "ארץ", "hi": "land", "hr": "zemlja", "hu": "ország", "id": "negeri", "it": "terra", "ja": "土地, 国", "kk": "land", "km": "land", "ko": "땅", "ku": "کشور, سرزمین", "lo": "land", "mg": "fari-tany , tany", "ms": "tanah,kata akar", "my": "land", "pl": "kraj", "ro": "pays", "ru": "страна", "sk": "krajina, štát", "sl": "dežela", "sv": "land", "sw": "pays", "th": "ประเทศ", "tok": "land", "tr": "toprak", "uk": "країна", "ur": "ملک", "vi": "quốc gia, đất nước", "yo": "land", "zh-tw": "國家", "zh": "国家,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 119,
		},
		{
			slug: "vortaro-radiko-largx", typ: "vocab",
			content: map[string]interface{}{
				"word": "larĝ",
				"definition": "wide",
				"definitions": map[string]interface{}{"en": "wide", "nl": "breed", "de": "breit", "fr": "large", "es": "ancho", "pt": "largo", "ar": "واسع", "be": "широкий, обширный", "ca": "ample/a", "cs": "široký", "da": "bred", "el": "πλατύς-ιά-ύ", "fa": "ستبر, گسترده, گشاد", "frp": "large", "ga": "leathan", "he": "רחב", "hi": "wide", "hr": "širok", "hu": "széles", "id": "luas", "it": "largo", "ja": "広い", "kk": "wide", "km": "wide", "ko": "넓은, 널찍한", "ku": "ستبر, گسترده, گشاد", "lo": "wide", "mg": "malalaka", "ms": "lebar,kata akar", "my": "wide", "pl": "szeroki", "ro": "large", "ru": "широкий, обширный", "sk": "široký", "sl": "široko", "sv": "bred, vid", "sw": "large", "th": "กว้าง", "tok": "wide", "tr": "geniş", "uk": "широкий", "ur": "وسیع", "vi": "rộng lớn, mênh mông, bao la, thênh thang, bát ngát", "yo": "wide", "zh-tw": "寛", "zh": "宽,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 120,
		},
		{
			slug: "vortaro-radiko-last", typ: "vocab",
			content: map[string]interface{}{
				"word": "last",
				"definition": "last",
				"definitions": map[string]interface{}{"en": "last", "nl": "laatst", "de": "letzter", "fr": "dernier", "es": "último", "pt": "último, última", "ar": "آخر", "be": "последний", "ca": "últim/a", "cs": "poslední", "da": "sidste", "el": "τελευταίος-α-ο", "fa": "آخرین", "frp": "dernier", "ga": "deiridh", "he": "אחרון", "hi": "last", "hr": "posljednji, zadnji", "hu": "utolsó", "id": "terakhir", "it": "ultimo", "ja": "最後の, 最近の", "kk": "last", "km": "last", "ko": "마지막", "ku": "آخرین", "lo": "last", "mg": "farany", "ms": "akhir,kata akar", "my": "last", "pl": "ostatni", "ro": "dernier", "ru": "последний", "sk": "posledný", "sl": "zadnje", "sv": "sista", "sw": "dernier", "th": "สุดท้าย", "tok": "last", "tr": "son", "uk": "останній", "ur": "آخری", "vi": "cuối cùng", "yo": "last", "zh-tw": "最後, 最新", "zh": "最后，最新,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 121,
		},
		{
			slug: "vortaro-radiko-lauxt", typ: "vocab",
			content: map[string]interface{}{
				"word": "laŭt",
				"definition": "loud",
				"definitions": map[string]interface{}{"en": "loud", "nl": "luid", "de": "laut", "fr": "fort", "es": "fuerte", "pt": "alto (som), alta (música)", "ar": "عالي", "be": "громкий", "ca": "fort/a (so)", "cs": "hlasitý", "da": "højlydt", "el": "μεγαλόφωνος-η-ο", "fa": "بلند و رسا", "frp": "fort", "ga": "ard", "he": "בקול רם", "hi": "loud", "hr": "glasan", "hu": "hangos", "id": "keras (suara), kencang", "it": "clamoroso", "ja": "（声が）大きい", "kk": "loud", "km": "loud", "ko": "소리가 큰", "ku": "بلند و رسا", "lo": "loud", "mg": "matanjaka", "ms": "suara kuat,kata akar", "my": "loud", "pl": "głośny", "ro": "fort", "ru": "громкий", "sk": "hlasný", "sl": "glasno", "sv": "hödljudd", "sw": "fort", "th": "ดัง", "tok": "loud", "tr": "yüksek(ses)", "uk": "голосний", "ur": "اونچی", "vi": "ầm ĩ, inh ỏi, lớn tiếng, um sùm", "yo": "loud", "zh-tw": "大聲", "zh": "大声,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 122,
		},
		{
			slug: "vortaro-radiko-lecion", typ: "vocab",
			content: map[string]interface{}{
				"word": "lecion",
				"definition": "lesson",
				"definitions": map[string]interface{}{"en": "lesson", "nl": "les", "de": "Lektion", "fr": "leçon", "es": "lección, clase", "pt": "lição, aula", "ar": "درس", "be": "урок", "ca": "lliçó", "cs": "lekce", "da": "lektion", "el": "μάθημα", "fa": "درس", "frp": "leçon", "ga": "ceacht", "he": "שיעור", "hi": "lesson", "hr": "lekcija", "hu": "lecke", "id": "pelajaran", "it": "lezione", "ja": "授業", "kk": "дәріс; сабақ", "km": "lesson", "ko": "학과", "ku": "درس", "lo": "lesson", "mg": "lessona", "ms": "pelajaran,kata akar", "my": "lesson", "pl": "lekcja", "ro": "leçon", "ru": "урок", "sk": "lekcia", "sl": "lekcija", "sv": "lektion", "sw": "leçon", "th": "บทเรียน", "tok": "lesson", "tr": "ders", "uk": "лекція", "ur": "سبق", "vi": "bài học", "yo": "lesson", "zh-tw": "課", "zh": "课,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 123,
		},
		{
			slug: "vortaro-radiko-leg", typ: "vocab",
			content: map[string]interface{}{
				"word": "leg",
				"definition": "to read",
				"definitions": map[string]interface{}{"en": "to read", "nl": "lezen", "de": "lesen", "fr": "lire", "es": "leer", "pt": "ler", "ar": "قرأ", "be": "читать", "ca": "llegir", "cs": "číst", "da": "læse", "el": "το να διαβάζω", "fa": "روخوانی کردن, خواندن, مطالعه کردن", "frp": "lire", "ga": "léigh", "he": "לקרוא", "hi": "to read", "hr": "čitati", "hu": "olvas", "id": "baca", "it": "leggere", "ja": "読む", "kk": "to read", "km": "to read", "ko": "읽다", "ku": "روخوانی کردن, خواندن, مطالعه کردن", "lo": "to read", "mg": "mamaky teny", "ms": "baca,kata akar", "my": "to read", "pl": "czytać", "ro": "lire", "ru": "читать", "sk": "čítať", "sl": "brati", "sv": "läsa", "sw": "lire", "th": "อ่าน", "tok": "to read", "tr": "okumak", "uk": "читати", "ur": "پڑھنا", "vi": "đọc", "yo": "to read", "zh-tw": "讀", "zh": "读,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 124,
		},
		{
			slug: "vortaro-radiko-lern", typ: "vocab",
			content: map[string]interface{}{
				"word": "lern",
				"definition": "to learn",
				"definitions": map[string]interface{}{"en": "to learn", "nl": "leren", "de": "lernen", "fr": "apprendre", "es": "aprender", "pt": "aprender", "ar": "تعلم", "be": "изучать", "ca": "aprendre", "cs": "učit se", "da": "lære", "el": "το να μελετώ, το να μαθαίνω", "fa": "یاد گرفتن, آموختن", "frp": "apprendre", "ga": "foghlaim", "he": "ללמוד", "hi": "to learn", "hr": "učiti", "hu": "tanul", "id": "belajar", "it": "imparare", "ja": "学ぶ", "kk": "to learn", "km": "to learn", "ko": "배우다", "ku": "یاد گرفتن, آموختن", "lo": "to learn", "mg": "mianatra", "ms": "belajar,kata akar", "my": "to learn", "pl": "uczyć się", "ro": "apprendre", "ru": "изучать", "sk": "učiť sa", "sl": "učiti se", "sv": "lära sig", "sw": "apprendre", "th": "เรียน", "tok": "to learn", "tr": "öğrenmek", "uk": "вчити", "ur": "سیکھنا", "vi": "học", "yo": "to learn", "zh-tw": "學", "zh": "学,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 125,
		},
		{
			slug: "vortaro-radiko-leter", typ: "vocab",
			content: map[string]interface{}{
				"word": "leter",
				"definition": "letter",
				"definitions": map[string]interface{}{"en": "letter", "nl": "brief", "de": "Brief", "fr": "lettre", "es": "carta", "pt": "carta", "ar": "حرف", "be": "письмо", "ca": "carta", "cs": "dopis", "da": "brev", "el": "επιστολή", "fa": "نامه", "frp": "lettre", "ga": "litir", "he": "מכתב", "hi": "letter", "hr": "pismo", "hu": "levél", "id": "surat", "it": "lettera", "ja": "手紙", "kk": "letter", "km": "letter", "ko": "편지", "ku": "نامه", "lo": "letter", "mg": "taratasy", "ms": "surat,kata akar", "my": "letter", "pl": "list", "ro": "lettre", "ru": "письмо", "sk": "list", "sl": "pismo", "sv": "brev", "sw": "lettre", "th": "จดหมาย", "tok": "letter", "tr": "mektup", "uk": "лист", "ur": "letter", "vi": "bức thư", "yo": "letter", "zh-tw": "書信", "zh": "书信,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 126,
		},
		{
			slug: "vortaro-radiko-liber", typ: "vocab",
			content: map[string]interface{}{
				"word": "liber",
				"definition": "free",
				"definitions": map[string]interface{}{"en": "free", "nl": "vrij", "de": "frei", "fr": "libre", "es": "libre", "pt": "livre, liberdade", "ar": "حرية،حر", "be": "свободный", "ca": "lliure", "cs": "volný", "da": "gratis", "el": "ελεύθερος-η-ο", "fa": "آزاد", "frp": "libre", "ga": "saor", "he": "חופש", "hi": "free", "hr": "slobodan", "hu": "szabad", "id": "gratis, bebas", "it": "libero", "ja": "自由", "kk": "free", "km": "free", "ko": "자유로운", "ku": "آزاد", "lo": "free", "mg": "afaka , tsy voatery", "ms": "bebas,kata akar", "my": "free", "pl": "wolny", "ro": "libre", "ru": "свободный", "sk": "voľný", "sl": "prost", "sv": "fri, ledig", "sw": "libre", "th": "อิสระ", "tok": "free", "tr": "serbest", "uk": "вільний", "ur": "آزاد", "vi": "tự do, thoải mái", "yo": "free", "zh-tw": "自由", "zh": "自由,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 127,
		},
		{
			slug: "vortaro-radiko-libr", typ: "vocab",
			content: map[string]interface{}{
				"word": "libr",
				"definition": "book",
				"definitions": map[string]interface{}{"en": "book", "nl": "boek", "de": "Buch", "fr": "livre", "es": "libro", "pt": "livro", "ar": "كتاب", "be": "книга", "ca": "llibre", "cs": "kniha", "da": "bog", "el": "βιβλίο", "fa": "کتاب", "frp": "livre", "ga": "leabhar", "he": "ספר", "hi": "book", "hr": "knjiga", "hu": "könyv", "id": "buku", "it": "libro", "ja": "本", "kk": "кітап", "km": "book", "ko": "책", "ku": "کتاب", "lo": "book", "mg": "boky", "ms": "buku,kata akar", "my": "book", "pl": "książka", "ro": "livre", "ru": "книга", "sk": "kniha", "sl": "knjiga", "sv": "bok", "sw": "livre", "th": "หนังสือ", "tok": "book", "tr": "kitap", "uk": "книжка", "ur": "کتاب", "vi": "quyển sách", "yo": "book", "zh-tw": "書", "zh": "书,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 128,
		},
		{
			slug: "vortaro-radiko-lok", typ: "vocab",
			content: map[string]interface{}{
				"word": "lok",
				"definition": "place",
				"definitions": map[string]interface{}{"en": "place", "nl": "plaats", "de": "Ort", "fr": "placer", "es": "lugar", "pt": "lugar", "ar": "مكان", "be": "место", "ca": "lloc", "cs": "místo", "da": "sted", "el": "τόπος", "fa": "مکان", "frp": "placer", "ga": "áit", "he": "מקום", "hi": "place", "hr": "mjesto", "hu": "hely", "id": "tempat", "it": "luogo", "ja": "場所", "kk": "place", "km": "place", "ko": "장소", "ku": "مکان", "lo": "place", "mg": "mametraka", "ms": "tempat,kata akar", "my": "place", "pl": "miejsce", "ro": "placer", "ru": "место", "sk": "miesto", "sl": "prostor", "sv": "plats", "sw": "placer", "th": "สถานที่", "tok": "place", "tr": "yer, lokasyon", "uk": "місце", "ur": "جگہ", "vi": "nơi chốn", "yo": "place", "zh-tw": "地點", "zh": "地点,词根,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 129,
		},
		{
			slug: "vortaro-radiko-long", typ: "vocab",
			content: map[string]interface{}{
				"word": "long",
				"definition": "long",
				"definitions": map[string]interface{}{"en": "long", "nl": "lang", "de": "lange", "fr": "long", "es": "largo/a", "pt": "longo, longa", "ar": "طويل", "be": "длинный", "ca": "llarg/a (distància o temps)", "cs": "dlouhý", "da": "lang", "el": "μακρύς-ιά-ύ", "fa": "طولانی", "frp": "long", "ga": "fada", "he": "ארוך", "hi": "long", "hr": "dugačak", "hu": "hosszú", "id": "panjang", "it": "lungo", "ja": "長い", "kk": "ұзақ; ұзын", "km": "long", "ko": "긴, 오래된", "ku": "طولانی", "lo": "long", "mg": "lava", "ms": "panjang,kata akar", "my": "long", "pl": "długi", "ro": "long", "ru": "длинный", "sk": "dlhý", "sl": "dolg", "sv": "lång", "sw": "long", "th": "ยาว", "tok": "long", "tr": "uzun", "uk": "довгий", "ur": "لمبا، لمبی", "vi": "lâu, dài", "yo": "long", "zh-tw": "長", "zh": "长,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 130,
		},
		{
			slug: "vortaro-radiko-lud", typ: "vocab",
			content: map[string]interface{}{
				"word": "lud",
				"definition": "to play",
				"definitions": map[string]interface{}{"en": "to play", "nl": "spelen", "de": "spielen", "fr": "jouer", "es": "jugar", "pt": "jogar, brincar, tocar, representar", "ar": "لعب", "be": "играть", "ca": "jugar", "cs": "hrát", "da": "lege", "el": "το να παίζω", "fa": "بازی کردن, نواختن موسیقی", "frp": "jouer", "ga": "súgradh", "he": "לשחק, לנגן", "hi": "to play", "hr": "igrati", "hu": "játszik", "id": "main", "it": "giocare", "ja": "遊ぶ", "kk": "to play", "km": "to play", "ko": "놀다, 장난하다, 역할하다", "ku": "بازی کردن, نواختن موسیقی", "lo": "to play", "mg": "milalao", "ms": "main,kata akar", "my": "to play", "pl": "grać, bawić się", "ro": "jouer", "ru": "играть", "sk": "hrať", "sl": "igrati", "sv": "spela, leka", "sw": "jouer", "th": "เล่น", "tok": "to play", "tr": "oynamak", "uk": "грати", "ur": "کھیلنا", "vi": "chơi đùa", "yo": "to play", "zh-tw": "玩", "zh": "玩,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 131,
		},
		{
			slug: "vortaro-radiko-lum", typ: "vocab",
			content: map[string]interface{}{
				"word": "lum",
				"definition": "light",
				"definitions": map[string]interface{}{"en": "light", "nl": "licht", "de": "Licht", "fr": "lumière", "es": "luz", "pt": "luz, iluminar", "ar": "ضوء", "be": "свет", "ca": "llum", "cs": "světlo", "da": "let", "el": "φως", "fa": "نور", "frp": "lumière", "ga": "solas", "he": "אור", "hi": "light", "hr": "svjetlo", "hu": "fény", "id": "cahaya", "it": "luce", "ja": "光", "kk": "light", "km": "light", "ko": "빛", "ku": "نور", "lo": "light", "mg": "fahazavana", "ms": "cahaya,kata akar", "my": "light", "pl": "światło", "ro": "lumière", "ru": "свет", "sk": "svetlo", "sl": "svetloba", "sv": "ljus", "sw": "lumière", "th": "แสงไฟ", "tok": "light", "tr": "parlak", "uk": "світло, сяйво, світило", "ur": "روشنی", "vi": "ánh sáng", "yo": "light", "zh-tw": "光", "zh": "亮,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 132,
		},
		{
			slug: "vortaro-radiko-man", typ: "vocab",
			content: map[string]interface{}{
				"word": "man",
				"definition": "hand",
				"definitions": map[string]interface{}{"en": "hand", "nl": "hand", "de": "Hand", "fr": "main", "es": "mano", "pt": "mão", "ar": "يد", "be": "рука", "ca": "mà", "cs": "ruka", "da": "hånd", "el": "χέρι", "fa": "دست", "frp": "main", "ga": "lámh", "he": "יד", "hi": "hand", "hr": "ruka", "hu": "kéz", "id": "tangan", "it": "mano", "ja": "手", "kk": "қол", "km": "hand", "ko": "손", "ku": "دست", "lo": "hand", "mg": "tànana", "ms": "tangan,kata akar", "my": "hand", "pl": "ręka", "ro": "main", "ru": "рука", "sk": "ruka", "sl": "roka", "sv": "hand", "sw": "main", "th": "มือ", "tok": "hand", "tr": "el", "uk": "рука", "ur": "ہاتھ", "vi": "bàn tay", "yo": "hand", "zh-tw": "手", "zh": "手,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 133,
		},
		{
			slug: "vortaro-radiko-mank", typ: "vocab",
			content: map[string]interface{}{
				"word": "mank",
				"definition": "be lacking",
				"definitions": map[string]interface{}{"en": "be lacking", "nl": "ontbreken", "de": "fehlen", "fr": "manquer", "es": "faltar", "pt": "faltar", "ar": "مفتقر", "be": "не хватать", "ca": "faltar, mancar", "cs": "chybět", "da": "mangle", "el": "το να λείπω", "fa": "کم بودن, نبودن", "frp": "manquer", "ga": "easpa", "he": "להיות חסר", "hi": "be lacking", "hr": "nedostajati", "hu": "hiány", "id": "kurang", "it": "mancare", "ja": "欠けている", "kk": "be lacking", "km": "be lacking", "ko": "결핍하다, 부족하다, 모자라다", "ku": "کم بودن, نبودن", "lo": "be lacking", "mg": "diso", "ms": "mengurangkan sesuatu,kata akar", "my": "be lacking", "pl": "brakować", "ro": "manquer", "ru": "не хватать", "sk": "chýbať", "sl": "manjkati", "sv": "saknas, fattas", "sw": "manquer", "th": "ขาด, ไม่มี", "tok": "be lacking", "tr": "eksik olmak", "uk": "бракувати, не вистачати, бути відсутнім", "ur": "be lacking", "vi": "thiếu sót", "yo": "be lacking", "zh-tw": "欠缺", "zh": "欠缺,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 134,
		},
		{
			slug: "vortaro-radiko-mangx", typ: "vocab",
			content: map[string]interface{}{
				"word": "manĝ",
				"definition": "to eat",
				"definitions": map[string]interface{}{"en": "to eat", "nl": "eten", "de": "essen", "fr": "manger", "es": "comer", "pt": "comer", "ar": "أكل", "be": "есть", "ca": "menjar", "cs": "jíst", "da": "spise", "el": "το να τρώγω", "fa": "خوردن", "frp": "manger", "ga": "ith", "he": "לאכול", "hi": "to eat", "hr": "jesti", "hu": "eszik", "id": "makan", "it": "mangiare", "ja": "食べる", "kk": "to eat", "km": "to eat", "ko": "먹다", "ku": "خوردن", "lo": "to eat", "mg": "mihinana", "ms": "makan,kata akar", "my": "to eat", "pl": "jeść", "ro": "manger", "ru": "есть", "sk": "jesť", "sl": "jesti", "sv": "äta", "sw": "manger", "th": "กิน", "tok": "to eat", "tr": "yemek", "uk": "їсти", "ur": "کھانا کھانا", "vi": "ăn", "yo": "to eat", "zh-tw": "吃", "zh": "吃,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 135,
		},
		{
			slug: "vortaro-radiko-mar", typ: "vocab",
			content: map[string]interface{}{
				"word": "mar",
				"definition": "sea",
				"definitions": map[string]interface{}{"en": "sea", "nl": "zee", "de": "Meer", "fr": "mer", "es": "mar", "pt": "mar", "ar": "بحر", "be": "море", "ca": "mar", "cs": "moře", "da": "hav", "el": "θάλασσα", "fa": "دریا", "frp": "mer", "ga": "muir", "he": "ים", "hi": "sea", "hr": "more", "hu": "tenger", "id": "laut", "it": "mare", "ja": "海", "kk": "теңіз", "km": "សមុទ្រ", "ko": "바다", "ku": "دریا", "lo": "sea", "mg": "rano masina", "ms": "laut,kata akar", "my": "sea", "pl": "morze", "ro": "mer", "ru": "море", "sk": "more", "sl": "morje", "sv": "hav", "sw": "mer", "th": "ทะเล", "tok": "sea", "tr": "deniz", "uk": "море", "ur": "سمندر", "vi": "biển cả", "yo": "sea", "zh-tw": "海", "zh": "海,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 136,
		},
		{
			slug: "vortaro-radiko-maten", typ: "vocab",
			content: map[string]interface{}{
				"word": "maten",
				"definition": "morning",
				"definitions": map[string]interface{}{"en": "morning", "nl": "ochtend, morgen (begin van dag)", "de": "Morgen", "fr": "matin", "es": "mañana", "pt": "manhã", "ar": "صباح", "be": "утро", "ca": "matí", "cs": "ráno", "da": "morgen", "el": "πρωί", "fa": "صبح", "frp": "matin", "ga": "maidin", "he": "בוקר", "hi": "morning", "hr": "jutro", "hu": "reggel", "id": "pagi", "it": "mattina", "ja": "朝", "kk": "таң; таңертең", "km": "ព្រឹក", "ko": "아침", "ku": "صبح", "lo": "morning", "mg": "maraina", "ms": "pagi,kata akar", "my": "morning", "pl": "rano", "ro": "matin", "ru": "утро", "sk": "ráno", "sl": "jutro", "sv": "morgon", "sw": "matin", "th": "เช้า", "tok": "morning", "tr": "sabah", "uk": "ранок", "ur": "صبح", "vi": "buổi sáng", "yo": "morning", "zh-tw": "早上", "zh": "早上,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 137,
		},
		{
			slug: "vortaro-radiko-memor", typ: "vocab",
			content: map[string]interface{}{
				"word": "memor",
				"definition": "to remember",
				"definitions": map[string]interface{}{"en": "to remember", "nl": "zich herinneren", "de": "sich erinnern", "fr": "se souvenir", "es": "recordar", "pt": "lembrar, memória", "ar": "تذكر", "be": "запомнить", "ca": "recordar", "cs": "pamamtovat", "da": "huske", "el": "το να θυμάμαι", "fa": "به یاد آوردن", "frp": "se souvenir", "ga": "cuimhne", "he": "לזכור", "hi": "to remember", "hr": "sjećati se", "hu": "emlék", "id": "ingat", "it": "ricordare", "ja": "覚えている", "kk": "to remember", "km": "to remember", "ko": "기억하다", "ku": "به یاد آوردن", "lo": "to remember", "mg": "mahatsiaro", "ms": "memori,kata akar", "my": "to remember", "pl": "pamiętać", "ro": "se souvenir", "ru": "запомнить", "sk": "pamätať", "sl": "spomniti se", "sv": "minnas", "sw": "se souvenir", "th": "จำได้", "tok": "to remember", "tr": "hatırlamak", "uk": "пам'ятати", "ur": "یاد کرنا", "vi": "nhớ", "yo": "to remember", "zh-tw": "記得", "zh": "记得,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 138,
		},
		{
			slug: "vortaro-radiko-met", typ: "vocab",
			content: map[string]interface{}{
				"word": "met",
				"definition": "to put",
				"definitions": map[string]interface{}{"en": "to put", "nl": "zetten, plaatsen, leggen", "de": "setzen, stellen, legen", "fr": "mettre", "es": "poner, meter", "pt": "colocar, pôr", "ar": "وضع", "be": "положить", "ca": "posar, ficar", "cs": "položit", "da": "lægge", "el": "το να βάζω", "fa": "قرار دادن", "frp": "mettre", "ga": "cuir", "he": "לשים", "hi": "to put", "hr": "staviti", "hu": "tesz", "id": "taruh, simpan", "it": "mettere", "ja": "置く", "kk": "to put", "km": "to put", "ko": "두다", "ku": "قرار دادن", "lo": "to put", "mg": "mametraka", "ms": "letak,kata akar", "my": "to put", "pl": "kłaść", "ro": "mettre", "ru": "положить", "sk": "položiť", "sl": "položiti", "sv": "sätta, ställa, lägga", "sw": "mettre", "th": "วาง", "tok": "to put", "tr": "koymak", "uk": "класти", "ur": "to put", "vi": "để, đặt", "yo": "to put", "zh-tw": "放置", "zh": "放在,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 139,
		},
		{
			slug: "vortaro-radiko-mez", typ: "vocab",
			content: map[string]interface{}{
				"word": "mez",
				"definition": "middle",
				"definitions": map[string]interface{}{"en": "middle", "nl": "midden", "de": "Mitte", "fr": "milieu", "es": "medio/a", "pt": "meio", "ar": "وسط", "be": "середина", "ca": "mig (ubicació)", "cs": "střed", "da": "midte", "el": "μέση", "fa": "میان, میانه, وسط", "frp": "milieu", "ga": "lár", "he": "אמצע", "hi": "middle", "hr": "sredina", "hu": "közép", "id": "tengah", "it": "mezzo", "ja": "中央", "kk": "middle", "km": "middle", "ko": "중간", "ku": "میان, میانه, وسط", "lo": "middle", "mg": "ampovoany", "ms": "tengah,kata akar", "my": "middle", "pl": "środek", "ro": "milieu", "ru": "середина", "sk": "stred", "sl": "sredina", "sv": "mitt", "sw": "milieu", "th": "กลาง", "tok": "middle", "tr": "orta", "uk": "середній", "ur": "middle", "vi": "chính giữa", "yo": "middle", "zh-tw": "中間", "zh": "中间,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 140,
		},
		{
			slug: "vortaro-radiko-milit", typ: "vocab",
			content: map[string]interface{}{
				"word": "milit",
				"definition": "war",
				"definitions": map[string]interface{}{"en": "war", "nl": "oorlog", "de": "Krieg", "fr": "guerre", "es": "guerra", "pt": "guerra, guerrear", "ar": "حرب", "be": "война", "ca": "guerra", "cs": "válka", "da": "krig", "el": "το να πολεμώ", "fa": "جنگ", "frp": "guerre", "ga": "cogadh", "he": "מלחמה", "hi": "war", "hr": "rat", "hu": "háború", "id": "perang", "it": "guerra", "ja": "戦争", "kk": "соғыс", "km": "war", "ko": "전쟁", "ku": "جنگ", "lo": "war", "mg": "ady", "ms": "perang,kata akar", "my": "war", "pl": "wojna", "ro": "guerre", "ru": "война", "sk": "vojna", "sl": "vojna", "sv": "krig", "sw": "guerre", "th": "สงคราม", "tok": "war", "tr": "savaş", "uk": "війна", "ur": "جنگ", "vi": "cuộc chiến, chiến tranh", "yo": "war", "zh-tw": "戰爭", "zh": "战争,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 141,
		},
		{
			slug: "vortaro-radiko-minut", typ: "vocab",
			content: map[string]interface{}{
				"word": "minut",
				"definition": "minute",
				"definitions": map[string]interface{}{"en": "minute", "nl": "minuut", "de": "Minute", "fr": "minute", "es": "minuto", "pt": "minuto", "ar": "دقيقة", "be": "минута", "ca": "minut", "cs": "minuta", "da": "minut", "el": "λεπτό", "fa": "دقیقه", "frp": "minute", "ga": "nóiméad", "he": "דקה", "hi": "minute", "hr": "minuta", "hu": "perc", "id": "menit", "it": "minuto", "ja": "分", "kk": "minute", "km": "minute", "ko": "분", "ku": "دقیقه", "lo": "minute", "mg": "minitra", "ms": "minit,kata akar", "my": "minute", "pl": "minuta", "ro": "minute", "ru": "минута", "sk": "minúta", "sl": "minuta", "sv": "minut", "sw": "minute", "th": "นาที", "tok": "minute", "tr": "dakika", "uk": "хвилина", "ur": "دقیقہ، منٹ", "vi": "phút", "yo": "minute", "zh-tw": "分鐘", "zh": "分钟,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 142,
		},
		{
			slug: "vortaro-radiko-mir", typ: "vocab",
			content: map[string]interface{}{
				"word": "mir",
				"definition": "to wonder",
				"definitions": map[string]interface{}{"en": "to wonder", "nl": "verwonderd zijn", "de": "sich wundern", "fr": "s'étonner", "es": "maravillarse", "pt": "admirar-se", "ar": "تعجب", "be": "удивляться", "ca": "meravellar-se, admirar-se, preguntar-se", "cs": "divit se", "da": "spekulere", "el": "το να απορώ", "fa": "شگفت‌زده شدن", "frp": "s'étonner", "ga": "iontas", "he": "להתפלא", "hi": "to wonder", "hr": "čuditi se", "hu": "csodálkozik", "id": "bayang", "it": "meravigliarsi", "ja": "驚嘆する", "kk": "to wonder", "km": "to wonder", "ko": "놀라다", "ku": "شگفت‌زده شدن", "lo": "to wonder", "mg": "gaga , taitra", "ms": "hebat,kata akar", "my": "to wonder", "pl": "dziwić się", "ro": "s'étonner", "ru": "удивляться", "sk": "diviť sa", "sl": "čuditi", "sv": "förvånas, förundras", "sw": "s'étonner", "th": "ประหลาดใจ", "tok": "to wonder", "tr": "hayret etmek", "uk": "дивуватися", "ur": "حیران ہونا", "vi": "tự hỏi", "yo": "to wonder", "zh-tw": "奇跡", "zh": "奇迹,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 143,
		},
		{
			slug: "vortaro-radiko-moment", typ: "vocab",
			content: map[string]interface{}{
				"word": "moment",
				"definition": "moment",
				"definitions": map[string]interface{}{"en": "moment", "nl": "moment (ogenblik)", "de": "Moment", "fr": "moment", "es": "momento", "pt": "momento", "ar": "لحظة", "be": "мгновение, момент", "ca": "moment", "cs": "moment", "da": "øjeblik", "el": "στιγμή", "fa": "لحظه", "frp": "moment", "ga": "nóiméad", "he": "רגע", "hi": "moment", "hr": "trenutak", "hu": "pillanat", "id": "momen", "it": "momento", "ja": "瞬間", "kk": "moment", "km": "moment", "ko": "순간", "ku": "لحظه", "lo": "moment", "mg": "fotoana kely , kelikely", "ms": "masa,kata akar", "my": "moment", "pl": "moment", "ro": "moment", "ru": "мгновение, момент", "sk": "moment", "sl": "trenutek", "sv": "ögonblick", "sw": "moment", "th": "ชั่วขณะ", "tok": "moment", "tr": "an", "uk": "мить", "ur": "لمحہ", "vi": "khoảnh khắc", "yo": "moment", "zh-tw": "片刻", "zh": "时刻,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 144,
		},
		{
			slug: "vortaro-radiko-mon", typ: "vocab",
			content: map[string]interface{}{
				"word": "mon",
				"definition": "money",
				"definitions": map[string]interface{}{"en": "money", "nl": "geld", "de": "Geld", "fr": "argent, monnaie", "es": "dinero", "pt": "dinheiro", "ar": "فلوس, مال", "be": "деньги", "ca": "diners", "cs": "peníze", "da": "penge", "el": "χρήμα", "fa": "پول", "frp": "argent", "ga": "airgead", "he": "כסף", "hi": "money", "hr": "novac", "hu": "pénz", "id": "uang", "it": "donera", "ja": "お金", "kk": "ақша", "km": "ប្រាក់", "ko": "돈", "ku": "پول", "lo": "money", "mg": "vola, vola madinika", "ms": "duit,kata akar", "my": "money", "pl": "pieniądze", "ro": "argent, monnaie", "ru": "деньги", "sk": "peniaze", "sl": "denar", "sv": "pengar", "sw": "argent, monnaie", "th": "เงิน", "tok": "money", "tr": "para", "uk": "гроші", "ur": "رقم", "vi": "tiền bạc", "yo": "money", "zh-tw": "錢", "zh": "钱,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 145,
		},
		{
			slug: "vortaro-radiko-mont", typ: "vocab",
			content: map[string]interface{}{
				"word": "mont",
				"definition": "mountain",
				"definitions": map[string]interface{}{"en": "mountain", "nl": "berg", "de": "Berg", "fr": "montagne", "es": "montaña", "pt": "montanha", "ar": "جبل", "be": "гора", "ca": "muntanya", "cs": "hora", "da": "bjerg", "el": "βουνό", "fa": "کوه", "frp": "montagne", "ga": "sliabh", "he": "הר", "hi": "mountain", "hr": "planina", "hu": "hegy", "id": "gunung", "it": "montagna", "ja": "山", "kk": "тау", "km": "ភ្នំ", "ko": "산", "ku": "کوه", "lo": "mountain", "mg": "tendrombohitra", "ms": "gunung,kata akar", "my": "mountain", "pl": "góra", "ro": "montagne", "ru": "гора", "sk": "hora", "sl": "gora", "sv": "berg", "sw": "montagne", "th": "ภูเขา", "tok": "mountain", "tr": "dağ", "uk": "гора", "ur": "پہاڑ", "vi": "ngọn núi", "yo": "mountain", "zh-tw": "山", "zh": "山,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 146,
		},
		{
			slug: "vortaro-radiko-montr", typ: "vocab",
			content: map[string]interface{}{
				"word": "montr",
				"definition": "to show",
				"definitions": map[string]interface{}{"en": "to show", "nl": "tonen", "de": "zeigen", "fr": "montrer", "es": "mostrar, enseñar", "pt": "mostrar", "ar": "عرض", "be": "показать", "ca": "mostrar, ensenyar", "cs": "ukázat", "da": "vise", "el": "το να επιδεικνύω", "fa": "نشان دادن", "frp": "montrer", "ga": "taispeáin", "he": "להראות", "hi": "to show", "hr": "pokazati", "hu": "mutat", "id": "memperlihatkan", "it": "mostrare", "ja": "見せる", "kk": "to show", "km": "to show", "ko": "보여주다", "ku": "نشان دادن", "lo": "to show", "mg": "mampiseho", "ms": "tunjuk,kata akar", "my": "to show", "pl": "pokazać", "ro": "montrer", "ru": "показать", "sk": "ukázať", "sl": "kazati", "sv": "visa, peka ut", "sw": "montrer", "th": "แสดง", "tok": "to show", "tr": "göstermek", "uk": "показувати", "ur": "دکھانا", "vi": "cho xem, cho thấy, tỏ ra", "yo": "to show", "zh-tw": "展示", "zh": "展示,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 147,
		},
		{
			slug: "vortaro-radiko-mort", typ: "vocab",
			content: map[string]interface{}{
				"word": "mort",
				"definition": "to die",
				"definitions": map[string]interface{}{"en": "to die", "nl": "sterven, dood", "de": "sterben", "fr": "mourir", "es": "morir", "pt": "morrer, morte", "ar": "مات", "be": "умереть", "ca": "morir", "cs": "zemřít", "da": "dø", "el": "το να πεθαίνω", "fa": "مردن", "frp": "mourir", "ga": "bás", "he": "למות", "hi": "to die", "hr": "umrijeti", "hu": "halál", "id": "mati", "it": "morire", "ja": "死ぬ", "kk": "to die", "km": "to die", "ko": "죽다", "ku": "مردن", "lo": "to die", "mg": "maty , miala aina", "ms": "mati,kata akar", "my": "to die", "pl": "umierać", "ro": "mourir", "ru": "умереть", "sk": "zomrieť", "sl": "umreti", "sv": "dö", "sw": "mourir", "th": "ตาย", "tok": "to die", "tr": "ölmek", "uk": "смерть", "ur": "مرنا", "vi": "chết đi", "yo": "to die", "zh-tw": "死", "zh": "死,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 148,
		},
		{
			slug: "vortaro-radiko-mov", typ: "vocab",
			content: map[string]interface{}{
				"word": "mov",
				"definition": "to move",
				"definitions": map[string]interface{}{"en": "to move", "nl": "bewegen", "de": "bewegen", "fr": "bouger", "es": "mover", "pt": "mover, movimento", "ar": "يتحرك", "be": "двигать", "ca": "moure", "cs": "pohyb", "da": "bevæge, flytte", "el": "το να κινώ", "fa": "حرکت دادن", "frp": "bouger", "ga": "bog", "he": "לזוז", "hi": "to move", "hr": "pomaknuti", "hu": "mozdít", "id": "bergerak, pindah", "it": "muovere", "ja": "動かす", "kk": "to move", "km": "to move", "ko": "움직이다", "ku": "حرکت دادن", "lo": "to move", "mg": "mihetsika", "ms": "gerak,kata akar", "my": "to move", "pl": "ruszać", "ro": "bouger", "ru": "двигать", "sk": "pohyb", "sl": "gibati", "sv": "(för)flytta", "sw": "bouger", "th": "เคลื่อนที่", "tok": "to move", "tr": "hareket etmek", "uk": "рухати, пересувати, приводити в рух, ворушити", "ur": "حرکت کرنا", "vi": "di chuyển, rung động", "yo": "to move", "zh-tw": "移動", "zh": "移动,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 149,
		},
		{
			slug: "vortaro-radiko-mult", typ: "vocab",
			content: map[string]interface{}{
				"word": "mult",
				"definition": "many",
				"definitions": map[string]interface{}{"en": "many", "nl": "veel", "de": "viel", "fr": "beaucoup", "es": "mucho/a", "pt": "muitos, muitas", "ar": "كثير", "be": "много", "ca": "molt/a", "cs": "mnoho", "da": "mange", "el": "πολύς-λή-ύ", "fa": "زیاد", "frp": "beaucoup", "ga": "an-chuid", "he": "הרבה", "hi": "many", "hr": "mnogi", "hu": "sok", "id": "banyak", "it": "molto", "ja": "たくさんの", "kk": "many", "km": "មនុស្សជាច្រើន", "ko": "많은", "ku": "زیاد", "lo": "many", "mg": "betsaka", "ms": "ramai,kata akar", "my": "many", "pl": "wiele", "ro": "beaucoup", "ru": "много", "sk": "mnoho, veľa", "sl": "več", "sv": "många", "sw": "beaucoup", "th": "มาก", "tok": "many", "tr": "çok", "uk": "багато", "ur": "کئی", "vi": "nhiều", "yo": "many", "zh-tw": "多", "zh": "多,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 150,
		},
		{
			slug: "vortaro-radiko-muzik", typ: "vocab",
			content: map[string]interface{}{
				"word": "muzik",
				"definition": "music",
				"definitions": map[string]interface{}{"en": "music", "nl": "muziek", "de": "Musik", "fr": "musique", "es": "música", "pt": "música, tocar música", "ar": "موسيقى", "be": "музыка", "ca": "música", "cs": "hudba", "da": "musik", "el": "μουσική", "fa": "موسیقی", "frp": "musique", "ga": "ceol", "he": "מוזיקה", "hi": "music", "hr": "glazba", "hu": "zene", "id": "musik", "it": "musica", "ja": "音楽", "kk": "music", "km": "តន្ត្រី", "ko": "음악", "ku": "موسیقی", "lo": "music", "mg": "mozika", "ms": "muzik,kata akar", "my": "music", "pl": "muzyka", "ro": "musique", "ru": "музыка", "sk": "hudba", "sl": "glasba", "sv": "musik", "sw": "musique", "th": "ดนตรี", "tok": "music", "tr": "müzik", "uk": "музика", "ur": "موسیقی", "vi": "âm nhạc", "yo": "music", "zh-tw": "音樂", "zh": "音乐,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 151,
		},
		{
			slug: "vortaro-radiko-nask", typ: "vocab",
			content: map[string]interface{}{
				"word": "nask",
				"definition": "to give birth",
				"definitions": map[string]interface{}{"en": "to give birth", "nl": "baren", "de": "gebären", "fr": "donner naissance", "es": "parir", "pt": "dar à luz, parir", "ar": "يلد", "be": "родить", "ca": "engendrar", "cs": "rodit", "da": "føde", "el": "το να γεννώ", "fa": "زاییدن, به دنیا آوردن", "frp": "donner naissance", "ga": "saolaigh", "he": "ללדת", "hi": "to give birth", "hr": "roditi", "hu": "szül", "id": "melahirkan", "it": "partorire", "ja": "産む", "kk": "to give birth", "km": "to give birth", "ko": "낳다", "ku": "زاییدن, به دنیا آوردن", "lo": "to give birth", "mg": "miteraka", "ms": "melahirkan,kata akar", "my": "to give birth", "pl": "rodzić", "ro": "donner naissance", "ru": "родить", "sk": "rodiť", "sl": "roditi", "sv": "föda (barn)", "sw": "donner naissance", "th": "ให้กำเนิด", "tok": "to give birth", "tr": "doğurmak", "uk": "народжувати", "ur": "بچہ پیدا کرنا", "vi": "sinh ra", "yo": "to give birth", "zh-tw": "生", "zh": "生,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 152,
		},
		{
			slug: "vortaro-radiko-natur", typ: "vocab",
			content: map[string]interface{}{
				"word": "natur",
				"definition": "nature",
				"definitions": map[string]interface{}{"en": "nature", "nl": "natuur", "de": "Natur", "fr": "nature", "es": "naturaleza", "pt": "natureza", "ar": "طبيعة", "be": "природа", "ca": "natura", "cs": "příroda", "da": "natur", "el": "φύση", "fa": "طبیعت", "frp": "nature", "ga": "nádúr, dúlra", "he": "טבע", "hi": "nature", "hr": "priroda", "hu": "természet", "id": "alam", "it": "natura", "ja": "自然", "kk": "nature", "km": "nature", "ko": "자연", "ku": "طبیعت", "lo": "nature", "mg": "natiora ,toe-javatra", "ms": "alam semula jadi,kata akar", "my": "nature", "pl": "natura", "ro": "nature", "ru": "природа", "sk": "príroda", "sl": "narava", "sv": "natur", "sw": "nature", "th": "ธรรมชาติ", "tok": "nature", "tr": "doğa", "uk": "природа", "ur": "فطرت", "vi": "thiên nhiên", "yo": "nature", "zh-tw": "自然", "zh": "自然,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 153,
		},
		{
			slug: "vortaro-radiko-neces", typ: "vocab",
			content: map[string]interface{}{
				"word": "neces",
				"definition": "necessary",
				"definitions": map[string]interface{}{"en": "necessary", "nl": "nodig", "de": "notwendig", "fr": "avoir besoin", "es": "necesario/a", "pt": "precisar", "ar": "حاجة", "be": "необходимый, нужный", "ca": "necessari/a", "cs": "potřebovat", "da": "behøve, have brug for", "el": "αναγκαίος-α-ο", "fa": "ضروری", "frp": "avoir besoin", "ga": "riachtanach", "he": "להיות צריך", "hi": "necessary", "hr": "trebati", "hu": "szükséges", "id": "perlu, butuh", "it": "necessitare", "ja": "必要である", "kk": "to need", "km": "to need", "ko": "필요한", "ku": "ضروری", "lo": "to need", "mg": "mila", "ms": "perlu,kata akar", "my": "to need", "pl": "potrzebować", "ro": "avoir besoin", "ru": "необходимый, нужный", "sk": "potrebovať", "sl": "potrebno", "sv": "nödvändig", "sw": "avoir besoin", "th": "จำเป็น", "tok": "necessary", "tr": "ihtiyaç duymak", "uk": "необхідний, потрібний", "ur": "ضرورت ہونا", "vi": "cần", "yo": "necessary", "zh-tw": "需要", "zh": "需要,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 154,
		},
		{
			slug: "vortaro-radiko-nokt", typ: "vocab",
			content: map[string]interface{}{
				"word": "nokt",
				"definition": "night",
				"definitions": map[string]interface{}{"en": "night", "nl": "nacht", "de": "Nacht", "fr": "nuit", "es": "noche", "pt": "noite", "ar": "ليل", "be": "ночь", "ca": "nit", "cs": "noc", "da": "nat", "el": "νύχτα", "fa": "شب", "frp": "nuit", "ga": "oíche", "he": "לילה", "hi": "night", "hr": "noć", "hu": "éjjel", "id": "malam", "it": "notte", "ja": "夜", "kk": "түн", "km": "យប់", "ko": "밤", "ku": "شب", "lo": "night", "mg": "alina", "ms": "malam,kata akar", "my": "night", "pl": "noc", "ro": "nuit", "ru": "ночь", "sk": "noc", "sl": "noč", "sv": "natt", "sw": "nuit", "th": "กลางคืน", "tok": "night", "tr": "gece", "uk": "ніч", "ur": "رات", "vi": "đêm khuya", "yo": "night", "zh-tw": "夜晚", "zh": "夜晚,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 155,
		},
		{
			slug: "vortaro-radiko-nom", typ: "vocab",
			content: map[string]interface{}{
				"word": "nom",
				"definition": "name",
				"definitions": map[string]interface{}{"en": "name", "nl": "naam", "de": "Name", "fr": "nom", "es": "nombre", "pt": "nome, nomear", "ar": "اسم", "be": "имя", "ca": "nom", "cs": "jméno", "da": "navn", "el": "όνομα", "fa": "نام", "frp": "nom", "ga": "ainm", "he": "שם", "hi": "name", "hr": "ime", "hu": "név", "id": "nama", "it": "nome", "ja": "名前", "kk": "name", "km": "ឈ្មោះ", "ko": "이름", "ku": "نام", "lo": "name", "mg": "anarana", "ms": "nama,kata akar", "my": "name", "pl": "imię", "ro": "nom", "ru": "имя", "sk": "meno", "sl": "ime", "sv": "namn", "sw": "nom", "th": "ชื่อ, ตั้งชื่อ", "tok": "name", "tr": "isim", "uk": "ім'я", "ur": "نام", "vi": "tên", "yo": "name", "zh-tw": "名字", "zh": "名字,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 156,
		},
		{
			slug: "vortaro-radiko-not", typ: "vocab",
			content: map[string]interface{}{
				"word": "not",
				"definition": "note",
				"definitions": map[string]interface{}{"en": "note", "nl": "noteren, nota", "de": "Note, Notiz", "fr": "note", "es": "nota", "pt": "nota, anotar", "ar": "دوَنَ", "be": "заметка", "ca": "nota", "cs": "známka, poznámka", "da": "note", "el": "σημείωση", "fa": "نمره, یادداشت, نُت (موسیقی)", "frp": "note", "ga": "nóta", "he": "פתק, ציון", "hi": "note", "hr": "bilješka, ocjena", "hu": "jegy", "id": "catat", "it": "nota, appunto", "ja": "メモ", "kk": "note", "km": "note", "ko": "각주, 노트 필기", "ku": "نمره, یادداشت, نُت (موسیقی)", "lo": "note", "mg": "marika , fivoasana", "ms": "nota,kata akar", "my": "note", "pl": "notować", "ro": "note", "ru": "заметка", "sk": "známka (na vysvedčení), poznámka, nota, nóta", "sl": "opomba", "sv": "anteckning, betyg", "sw": "note", "th": "จดบันทึก, ตัวโน้ต", "tok": "note", "tr": "not", "uk": "замітка, нотатка, запис", "ur": "note", "vi": "ghi chú, điểm số", "yo": "note", "zh-tw": "記錄", "zh": "记录,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 157,
		},
		{
			slug: "vortaro-radiko-nov", typ: "vocab",
			content: map[string]interface{}{
				"word": "nov",
				"definition": "new",
				"definitions": map[string]interface{}{"en": "new", "nl": "nieuw", "de": "neu", "fr": "nouveau", "es": "nuevo", "pt": "novo, nova", "ar": "جديد", "be": "новый", "ca": "nou/nova", "cs": "nová", "da": "ny", "el": "νέος-α-ο", "fa": "نو", "frp": "nouveau", "ga": "nua", "he": "חדש", "hi": "new", "hr": "nov", "hu": "új", "id": "baru", "it": "nuovo", "ja": "新しい", "kk": "жаңа", "km": "ថ្មី", "ko": "새로운", "ku": "نو", "lo": "new", "mg": "vaovao , vao", "ms": "baru,kata akar", "my": "new", "pl": "nowy", "ro": "nouveau", "ru": "новый", "sk": "nový", "sl": "nov", "sv": "ny", "sw": "nouveau", "th": "ใหม่", "tok": "new", "tr": "yeni", "uk": "новий", "ur": "نیا، نئی", "vi": "mới", "yo": "new", "zh-tw": "新", "zh": "新,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 158,
		},
		{
			slug: "vortaro-radiko-odor", typ: "vocab",
			content: map[string]interface{}{
				"word": "odor",
				"definition": "smell",
				"definitions": map[string]interface{}{"en": "smell", "nl": "geur", "de": "Duft", "fr": "sentir", "es": "oler", "pt": "cheiro, exalar cheiro", "ar": "إحساس", "be": "пахнуть", "ca": "fer olor", "cs": "vůně", "da": "lugt", "el": "μυρωδιά", "fa": "بو", "frp": "sentir", "ga": "boladh", "he": "ריח", "hi": "smell", "hr": "miris", "hu": "szag", "id": "bau, cium", "it": "odore", "ja": "匂い", "kk": "smell", "km": "smell", "ko": "냄새나다", "ku": "بو", "lo": "smell", "mg": "mahatsapa fofona", "ms": "bau,kata akar", "my": "smell", "pl": "zapach", "ro": "sentir", "ru": "пахнуть", "sk": "vôňa", "sl": "vonj", "sv": "lukt", "sw": "sentir", "th": "ส่งกลิ่น, กลิ่น", "tok": "smell", "tr": "koku", "uk": "пахнути", "ur": "بو", "vi": "bốc mùi", "yo": "smell", "zh-tw": "味道", "zh": "味道,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 159,
		},
		{
			slug: "vortaro-radiko-oft", typ: "vocab",
			content: map[string]interface{}{
				"word": "oft",
				"definition": "often",
				"definitions": map[string]interface{}{"en": "often", "nl": "vaak, dikwijls", "de": "oft", "fr": "souvent", "es": "a menudo", "pt": "frequentemente", "ar": "غالبا", "be": "часто", "ca": "freqüent, sovint", "cs": "často", "da": "ofte", "el": "συχνός-ή-ό", "fa": "غالبا", "frp": "souvent", "ga": "minic", "he": "לעתים קרובות", "hi": "often", "hr": "često", "hu": "gyakran", "id": "sering", "it": "spesso", "ja": "度々", "kk": "often", "km": "often", "ko": "자주", "ku": "غالبا", "lo": "often", "mg": "matetika", "ms": "selalu,kata akar", "my": "often", "pl": "częsty", "ro": "souvent", "ru": "часто", "sk": "často", "sl": "pogosto", "sv": "ofta", "sw": "souvent", "th": "บ่อย", "tok": "often", "tr": "sıkça", "uk": "часто", "ur": "اکثر", "vi": "thường xuyên", "yo": "often", "zh-tw": "常", "zh": "经常,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 160,
		},
		{
			slug: "vortaro-radiko-okaz", typ: "vocab",
			content: map[string]interface{}{
				"word": "okaz",
				"definition": "to happen",
				"definitions": map[string]interface{}{"en": "to happen", "nl": "gebeuren, geschieden", "de": "geschehen", "fr": "avoir lieu", "es": "ocurrir, pasar", "pt": "acontecer, ocasião", "ar": "حدث", "be": "случиться, произойти", "ca": "succeir, esdevenir, passar, tenir lloc", "cs": "stát se", "da": "ske", "el": "το να συμβαίνω", "fa": "رخ دادن", "frp": "avoir lieu", "ga": "tarlaigh", "he": "לקרות", "hi": "to happen", "hr": "dogoditi se", "hu": "alkalom, történik", "id": "terjadi", "it": "accadere, succedere", "ja": "起きる", "kk": "to happen", "km": "to happen", "ko": "일어나다", "ku": "رخ دادن", "lo": "to happen", "mg": "ao , maka toerana", "ms": "laku,kata akar", "my": "to happen", "pl": "zdarzyć się", "ro": "avoir lieu", "ru": "случиться, произойти", "sk": "stať sa", "sl": "zgoditi se", "sv": "hända, inträffa", "sw": "avoir lieu", "th": "เกิดเหตุการณ์", "tok": "to happen", "tr": "olmak, vuku bulmak", "uk": "траплятися, ставатися, відбуватися, проходити", "ur": "واقع ہونا", "vi": "xảy ra", "yo": "to happen", "zh-tw": "發生", "zh": "发生,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 161,
		},
		{
			slug: "vortaro-radiko-okul", typ: "vocab",
			content: map[string]interface{}{
				"word": "okul",
				"definition": "eye",
				"definitions": map[string]interface{}{"en": "eye", "nl": "oog", "de": "Auge", "fr": "oeil", "es": "ojo", "pt": "olho", "ar": "عين", "be": "око, глаз", "ca": "ull", "cs": "oko", "da": "øje", "el": "μάτι", "fa": "چشم", "frp": "oeil", "ga": "súil", "he": "עין", "hi": "eye", "hr": "oko", "hu": "szem", "id": "mata", "it": "occhio", "ja": "eye", "kk": "көз", "km": "eye", "ko": "눈", "ku": "چشم", "lo": "eye", "mg": "maso", "ms": "mata,kata akar", "my": "eye", "pl": "oko", "ro": "oeil", "ru": "око, глаз", "sk": "oko", "sl": "oko", "sv": "öga", "sw": "oeil", "th": "ดวงตา", "tok": "eye", "tr": "göz", "uk": "око", "ur": "آنکھ", "vi": "con mắt", "yo": "eye", "zh-tw": "眼睛", "zh": "眼睛,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 162,
		},
		{
			slug: "vortaro-radiko-onkl", typ: "vocab",
			content: map[string]interface{}{
				"word": "onkl",
				"definition": "uncle",
				"definitions": map[string]interface{}{"en": "uncle", "nl": "oom", "de": "Onkel", "fr": "oncle", "es": "tío", "pt": "tio", "ar": "عم, خال", "be": "дядя", "ca": "oncle", "cs": "strýc", "da": "onkel", "el": "θείος", "fa": "عمو, دایی", "frp": "oncle", "ga": "uncail", "he": "דוד", "hi": "uncle", "hr": "stric, ujak", "hu": "nagybácsi", "id": "paman", "it": "zio", "ja": "叔父", "kk": "ағай", "km": "ពូ", "ko": "삼촌", "ku": "عمو, دایی", "lo": "uncle", "mg": "dadatoa , rahalahin-dray", "ms": "pakcik,kata akar", "my": "uncle", "pl": "wujek", "ro": "oncle", "ru": "дядя", "sk": "strýko", "sl": "stric", "sv": "farbror, morbror", "sw": "oncle", "th": "ลุง, อา, น้า (ชาย)", "tok": "uncle", "tr": "amca", "uk": "дядько", "ur": "چچا", "vi": "cậu, chú, bác", "yo": "uncle", "zh-tw": "叔伯", "zh": "叔叔、伯伯,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 163,
		},
		{
			slug: "vortaro-radiko-opini", typ: "vocab",
			content: map[string]interface{}{
				"word": "opini",
				"definition": "to opinionate, to think",
				"definitions": map[string]interface{}{"en": "to opinionate, to think", "nl": "menen", "de": "meinen", "fr": "opinion", "es": "opinar", "pt": "opinião, opinar", "ar": "رأي", "be": "мнение", "ca": "opinar", "cs": "názor", "da": "holdning", "el": "το να νομίζω", "fa": "به چیزی یا کسی عقیده داشتن", "frp": "opinion", "ga": "tuairim, smaoinigh", "he": "דעה", "hi": "to opinionate, to think", "hr": "smatrati", "hu": "vélemény", "id": "opini, pendapat", "it": "opinione", "ja": "意見", "kk": "opinion", "km": "opinion", "ko": "의견", "ku": "به چیزی یا کسی عقیده داشتن", "lo": "opinion", "mg": "hevitra , fiheverina", "ms": "pendapat,kata akar", "my": "opinion", "pl": "opinia", "ro": "opinion", "ru": "мнение", "sk": "názor", "sl": "mnenje", "sv": "åsikt", "sw": "opinion", "th": "ความเห็น", "tok": "to opinionate, to think", "tr": "fikir", "uk": "думати, гадати, вважати, мати думку", "ur": "opinion", "vi": "ý kiến, quan điểm", "yo": "to opinionate, to think", "zh-tw": "意見", "zh": "意见,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 164,
		},
		{
			slug: "vortaro-radiko-ord", typ: "vocab",
			content: map[string]interface{}{
				"word": "ord",
				"definition": "to order",
				"definitions": map[string]interface{}{"en": "to order", "nl": "orde", "de": "Ordnung", "fr": "commander", "es": "ordenar", "pt": "ordem", "ar": "نظام, تمام", "be": "порядок", "ca": "ordre, endreç", "cs": "řádný, pořádný", "da": "bestille", "el": "τάξη", "fa": "ترتیب", "frp": "commander", "ga": "ordaigh", "he": "סדר", "hi": "to order", "hr": "red", "hu": "rend", "id": "suruh, perintah", "it": "ordine", "ja": "秩序", "kk": "to order", "km": "to order", "ko": "질서, 순서, 정돈", "ku": "ترتیب", "lo": "to order", "mg": "mandidy , mifehy , manafatra", "ms": "pesan,kata akar", "my": "to order", "pl": "porządkować", "ro": "commander", "ru": "порядок", "sk": "riadny, usporiadaný", "sl": "red", "sv": "ordning", "sw": "commander", "th": "สั่ง", "tok": "to order", "tr": "emretmek, sipariş vermek", "uk": "порядок, лад", "ur": "to order", "vi": "ra lệnh, chỉ dẫn, sắp xếp", "yo": "to order", "zh-tw": "秩序", "zh": "秩序,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 165,
		},
		{
			slug: "vortaro-radiko-orel", typ: "vocab",
			content: map[string]interface{}{
				"word": "orel",
				"definition": "ear",
				"definitions": map[string]interface{}{"en": "ear", "nl": "oor", "de": "Ohr", "fr": "oreille", "es": "oreja", "pt": "orelha", "ar": "إذن", "be": "ухо", "ca": "orella", "cs": "ucho", "da": "øre", "el": "αφτί", "fa": "گوش", "frp": "oreille", "ga": "cluas", "he": "אוזן", "hi": "ear", "hr": "uho", "hu": "fül", "id": "telinga, kuping", "it": "orecchio", "ja": "耳", "kk": "құлақ", "km": "ear", "ko": "귀", "ku": "گوش", "lo": "ear", "mg": "sofina", "ms": "telinga,kata akar", "my": "ear", "pl": "ucho", "ro": "oreille", "ru": "ухо", "sk": "ucho", "sl": "uho", "sv": "öra", "sw": "oreille", "th": "หู", "tok": "ear", "tr": "kulak", "uk": "вухо", "ur": "کان", "vi": "tai", "yo": "ear", "zh-tw": "耳朵", "zh": "耳朵,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 166,
		},
		{
			slug: "vortaro-radiko-pac", typ: "vocab",
			content: map[string]interface{}{
				"word": "pac",
				"definition": "peace",
				"definitions": map[string]interface{}{"en": "peace", "nl": "vrede", "de": "Friede", "fr": "paix", "es": "paz", "pt": "paz", "ar": "سلام", "be": "мир", "ca": "pau", "cs": "mír", "da": "sted", "el": "ειρήνη", "fa": "صلح", "frp": "paix", "ga": "síocháin", "he": "חתיכה", "hi": "peace", "hr": "mir", "hu": "béke", "id": "damai", "it": "pace", "ja": "平和", "kk": "peace", "km": "សន្តិភាព", "ko": "평화", "ku": "صلح", "lo": "peace", "mg": "fifanarahana", "ms": "aman,kata akar", "my": "peace", "pl": "pokój (brak wojny)", "ro": "paix", "ru": "мир", "sk": "mier", "sl": "mir", "sv": "fred", "sw": "paix", "th": "สันติ", "tok": "peace", "tr": "barış", "uk": "мир", "ur": "امن", "vi": "hoà bình", "yo": "peace", "zh-tw": "和平", "zh": "和平,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 167,
		},
		{
			slug: "vortaro-radiko-pag", typ: "vocab",
			content: map[string]interface{}{
				"word": "pag",
				"definition": "to pay",
				"definitions": map[string]interface{}{"en": "to pay", "nl": "betalen", "de": "zahlen", "fr": "payer", "es": "pagar", "pt": "pagar, pagamento", "ar": "دفع", "be": "платить", "ca": "pagar", "cs": "platit", "da": "betale", "el": "το να πληρώνω", "fa": "پرداخت کردن", "frp": "payer", "ga": "íoc", "he": "לשלם", "hi": "to pay", "hr": "platiti", "hu": "fizet", "id": "bayar", "it": "pagare", "ja": "支払う", "kk": "to pay", "km": "to pay", "ko": "지불하다", "ku": "پرداخت کردن", "lo": "to pay", "mg": "mandoa", "ms": "bayar,kata akar", "my": "to pay", "pl": "płacić", "ro": "payer", "ru": "платить", "sk": "platiť", "sl": "plačati", "sv": "betala", "sw": "payer", "th": "จ่าย", "tok": "to pay", "tr": "ödemek", "uk": "платити", "ur": "ادا کرنا", "vi": "trả tiền", "yo": "to pay", "zh-tw": "付款", "zh": "付（款）,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 168,
		},
		{
			slug: "vortaro-radiko-pan", typ: "vocab",
			content: map[string]interface{}{
				"word": "pan",
				"definition": "bread",
				"definitions": map[string]interface{}{"en": "bread", "nl": "brood", "de": "Brot", "fr": "pain", "es": "pan", "pt": "pão", "ar": "خبز", "be": "хлеб", "ca": "pa", "cs": "chléb", "da": "brød", "el": "ψωμί", "fa": "نان", "frp": "pain", "ga": "arán", "he": "לחם", "hi": "bread", "hr": "kruh", "hu": "kenyér", "id": "roti", "it": "pane", "ja": "パン", "kk": "bread", "km": "នំបុ័ង", "ko": "빵", "ku": "نان", "lo": "bread", "mg": "mofo", "ms": "roti,kata akar", "my": "bread", "pl": "chleb", "ro": "pain", "ru": "хлеб", "sk": "chlieb", "sl": "kruh", "sv": "bröd", "sw": "pain", "th": "ขนมปัง", "tok": "bread", "tr": "ekmek", "uk": "хліб", "ur": "روٹی", "vi": "bánh mì", "yo": "bread", "zh-tw": "麵包", "zh": "面包,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 169,
		},
		{
			slug: "vortaro-radiko-paper", typ: "vocab",
			content: map[string]interface{}{
				"word": "paper",
				"definition": "paper",
				"definitions": map[string]interface{}{"en": "paper", "nl": "papier", "de": "Papier", "fr": "papier", "es": "papel", "pt": "papel", "ar": "ورق", "be": "бумага", "ca": "paper", "cs": "papír", "da": "papir", "el": "χαρτί", "fa": "کاغذ", "frp": "papier", "ga": "páipéar", "he": "נייר", "hi": "paper", "hr": "papir", "hu": "papír", "id": "kertas", "it": "carta", "ja": "紙", "kk": "paper", "km": "ក្រដាស", "ko": "종이", "ku": "کاغذ", "lo": "paper", "mg": "taratasy", "ms": "kertas,kata akar", "my": "paper", "pl": "papier", "ro": "papier", "ru": "бумага", "sk": "papier", "sl": "papir", "sv": "papper", "sw": "papier", "th": "กระดาษ", "tok": "paper", "tr": "kağıt", "uk": "папір", "ur": "کاغذ", "vi": "tờ giấy", "yo": "paper", "zh-tw": "紙張", "zh": "纸张,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 170,
		},
		{
			slug: "vortaro-radiko-pardon", typ: "vocab",
			content: map[string]interface{}{
				"word": "pardon",
				"definition": "to forgive",
				"definitions": map[string]interface{}{"en": "to forgive", "nl": "vergeven", "de": "verzeihen", "fr": "pardonner", "es": "perdonar", "pt": "perdoar, perdão", "ar": "غفر", "be": "простить", "ca": "perdonar", "cs": "odpustit", "da": "tilgive, undskylde", "el": "το να συγχωρώ", "fa": "بخشیدن, آمرزیدن", "frp": "pardonner", "ga": "maith", "he": "לסלוח", "hi": "to forgive", "hr": "oprostiti", "hu": "bocsánat", "id": "memaafkan", "it": "perdonare", "ja": "許す", "kk": "to forgive", "km": "to forgive", "ko": "용서하다", "ku": "بخشیدن, آمرزیدن", "lo": "to forgive", "mg": "mamela", "ms": "maaf,kata akar", "my": "to forgive", "pl": "przebaczyć", "ro": "pardonner", "ru": "простить", "sk": "odpustiť", "sl": "oprostiti", "sv": "förlåta", "sw": "pardonner", "th": "ขอโทษ", "tok": "to forgive", "tr": "bağışlamak", "uk": "прощати", "ur": "معاف کرنا", "vi": "tha thứ", "yo": "to forgive", "zh-tw": "饒恕, 原諒", "zh": "饶恕,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 171,
		},
		{
			slug: "vortaro-radiko-parol", typ: "vocab",
			content: map[string]interface{}{
				"word": "parol",
				"definition": "to speak",
				"definitions": map[string]interface{}{"en": "to speak", "nl": "spreken", "de": "sprechen", "fr": "parler", "es": "hablar", "pt": "falar, fala", "ar": "يتحدث", "be": "говорить", "ca": "parlar", "cs": "mluvit", "da": "tale", "el": "το να μιλώ", "fa": "صحبت کردن", "frp": "parler", "ga": "labhair", "he": "לדבר", "hi": "to speak", "hr": "govoriti", "hu": "beszél", "id": "bicara", "it": "parlare", "ja": "話す", "kk": "to speak", "km": "to speak", "ko": "말하다", "ku": "صحبت کردن", "lo": "to speak", "mg": "miteny", "ms": "ucap,kata akar", "my": "to speak", "pl": "mówić", "ro": "parler", "ru": "говорить", "sk": "hovoriť", "sl": "govoriti", "sv": "tala, prata", "sw": "parler", "th": "พูด", "tok": "to speak", "tr": "konuşmak", "uk": "говорити", "ur": "بولنا", "vi": "nói chuyện", "yo": "to speak", "zh-tw": "說", "zh": "说,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 172,
		},
		{
			slug: "vortaro-radiko-part", typ: "vocab",
			content: map[string]interface{}{
				"word": "part",
				"definition": "part",
				"definitions": map[string]interface{}{"en": "part", "nl": "deel", "de": "Teil", "fr": "partir", "es": "parte", "pt": "parte", "ar": "يرحل", "be": "часть", "ca": "part", "cs": "část", "da": "part, del", "el": "μέρος", "fa": "قسمت", "frp": "partir", "ga": "cuid", "he": "חלק", "hi": "part", "hr": "dio", "hu": "rész", "id": "bagian", "it": "parte", "ja": "部分", "kk": "part", "km": "part", "ko": "부분", "ku": "قسمت", "lo": "part", "mg": "mandeha", "ms": "bahagian,kata akar", "my": "part", "pl": "część", "ro": "partir", "ru": "часть", "sk": "časť", "sl": "del", "sv": "del", "sw": "partir", "th": "ส่วน", "tok": "part", "tr": "parça", "uk": "частина, частка", "ur": "حصہ", "vi": "phần", "yo": "part", "zh-tw": "部分", "zh": "部分,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 173,
		},
		{
			slug: "vortaro-radiko-pas", typ: "vocab",
			content: map[string]interface{}{
				"word": "pas",
				"definition": "to pass",
				"definitions": map[string]interface{}{"en": "to pass", "nl": "voorbijgaan", "de": "vorbei gehen", "fr": "passer", "es": "pasar", "pt": "passar", "ar": "مرر", "be": "пройти", "ca": "passar", "cs": "míjet", "da": "lade", "el": "το να περνώ", "fa": "گذشتن", "frp": "passer", "ga": "dul thart", "he": "לעבור", "hi": "to pass", "hr": "proći", "hu": "múlik", "id": "lewat", "it": "passare", "ja": "通る", "kk": "to pass", "km": "to pass", "ko": "지나가다, 통과하다", "ku": "گذشتن", "lo": "to pass", "mg": "mandalo", "ms": "lulus,kata akar", "my": "to pass", "pl": "mijać", "ro": "passer", "ru": "пройти", "sk": "prebiehať, uplynúť", "sl": "miniti", "sv": "passera", "sw": "passer", "th": "ผ่าน", "tok": "to pass", "tr": "geçmek", "uk": "проходити, переходити, минати, спливати", "ur": "گذرنا", "vi": "đi qua, trôi qua", "yo": "to pass", "zh-tw": "通過", "zh": "通过,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 174,
		},
		{
			slug: "vortaro-radiko-patr", typ: "vocab",
			content: map[string]interface{}{
				"word": "patr",
				"definition": "father",
				"definitions": map[string]interface{}{"en": "father", "nl": "vader", "de": "Vater", "fr": "père", "es": "padre", "pt": "pai", "ar": "الأب", "be": "отец", "ca": "pare", "cs": "otec", "da": "far", "el": "πατέρας", "fa": "پدر", "frp": "père", "ga": "athair", "he": "אב", "hi": "father", "hr": "otac", "hu": "apa", "id": "ayah", "it": "padre", "ja": "父親", "kk": "father", "km": "ឪពុក", "ko": "아버지", "ku": "پدر", "lo": "father", "mg": "ray", "ms": "ayah,kata akar", "my": "father", "pl": "ojciec", "ro": "père", "ru": "отец", "sk": "otec", "sl": "oče", "sv": "fader, far", "sw": "père", "th": "พ่อ", "tok": "father", "tr": "baba", "uk": "батько", "ur": "باپ", "vi": "cha", "yo": "father", "zh-tw": "父親", "zh": "父亲,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 175,
		},
		{
			slug: "vortaro-radiko-pasx", typ: "vocab",
			content: map[string]interface{}{
				"word": "paŝ",
				"definition": "step",
				"definitions": map[string]interface{}{"en": "step", "nl": "schrijden, stappen", "de": "schreiten", "fr": "pas", "es": "paso", "pt": "passo, dar passos", "ar": "خطوة", "be": "шаг", "ca": "caminar, pas, passa", "cs": "krok", "da": "skridt", "el": "το να βηματίζω", "fa": "قدم گذاشتن, گام نهادن", "frp": "pas", "ga": "céim", "he": "צעד", "hi": "step", "hr": "korak", "hu": "lép", "id": "langkah", "it": "passo", "ja": "歩み", "kk": "step", "km": "step", "ko": "걷다, 걸음을 옮기다", "ku": "قدم گذاشتن, گام نهادن", "lo": "step", "mg": "dia , dingana", "ms": "langkah,kata akar", "my": "step", "pl": "krok", "ro": "pas", "ru": "шаг", "sk": "krok", "sl": "korak", "sv": "kliva, gå", "sw": "pas", "th": "ก้าว", "tok": "step", "tr": "adım, adımlamak", "uk": "ступати, крокувати, ходити", "ur": "قدم", "vi": "bước đi", "yo": "step", "zh-tw": "步伐", "zh": "步伐,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 176,
		},
		{
			slug: "vortaro-radiko-pens", typ: "vocab",
			content: map[string]interface{}{
				"word": "pens",
				"definition": "to think",
				"definitions": map[string]interface{}{"en": "to think", "nl": "denken", "de": "denken", "fr": "penser", "es": "pensar", "pt": "pensar, pensamento", "ar": "يفكر", "be": "думать", "ca": "pensar", "cs": "myslet", "da": "tænke", "el": "το να σκέφτομαι", "fa": "فکر کردن", "frp": "penser", "ga": "smaoinigh", "he": "לחשוב", "hi": "to think", "hr": "misao, misliti", "hu": "gondol", "id": "pikir", "it": "pensare", "ja": "考える", "kk": "to think", "km": "to think", "ko": "생각하다", "ku": "فکر کردن", "lo": "to think", "mg": "mihevitra", "ms": "fikir,kata akar", "my": "to think", "pl": "myśleć", "ro": "penser", "ru": "думать", "sk": "myslieť", "sl": "misliti", "sv": "tänka, tycka", "sw": "penser", "th": "คิด", "tok": "to think", "tr": "düşünmek", "uk": "думати", "ur": "سوچنا", "vi": "suy nghĩ", "yo": "to think", "zh-tw": "想", "zh": "想,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 177,
		},
		{
			slug: "vortaro-radiko-perd", typ: "vocab",
			content: map[string]interface{}{
				"word": "perd",
				"definition": "to loose",
				"definitions": map[string]interface{}{"en": "to loose", "nl": "verliezen", "de": "verlieren", "fr": "perdre", "es": "perder", "pt": "perder", "ar": "فقد", "be": "потерять", "ca": "perdre", "cs": "ztratit", "da": "miste", "el": "το να χάνω", "fa": "از دست دادن", "frp": "perdre", "ga": "caill", "he": "לאבד", "hi": "to loose", "hr": "izgubiti", "hu": "veszt", "id": "kehilangan", "it": "perdere", "ja": "失う", "kk": "to loose", "km": "to loose", "ko": "잃어버리다", "ku": "از دست دادن", "lo": "to loose", "mg": "manary", "ms": "hilang,kata akar", "my": "to loose", "pl": "zgubić", "ro": "perdre", "ru": "потерять", "sk": "stratiť", "sl": "izgubiti", "sv": "tappa, mista, förlora", "sw": "perdre", "th": "หาย", "tok": "to loose", "tr": "kaybetmek", "uk": "губити, втрачати", "ur": "کھو دینا", "vi": "làm rơi", "yo": "to loose", "zh-tw": "失去", "zh": "失去,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 178,
		},
		{
			slug: "vortaro-radiko-person", typ: "vocab",
			content: map[string]interface{}{
				"word": "person",
				"definition": "person",
				"definitions": map[string]interface{}{"en": "person", "nl": "persoon", "de": "Person", "fr": "personne", "es": "persona", "pt": "pessoa", "ar": "شخص", "be": "персона, лицо (особь), личность", "ca": "persona, personatge", "cs": "osoba", "da": "person", "el": "πρόσωπο", "fa": "شخص", "frp": "personne", "ga": "duine", "he": "אדם", "hi": "person", "hr": "osoba", "hu": "személy", "id": "orang", "it": "persona", "ja": "人物", "kk": "person", "km": "person", "ko": "사람", "ku": "شخص", "lo": "person", "mg": "olona", "ms": "orang,kata akar", "my": "person", "pl": "osoba", "ro": "personne", "ru": "персона, лицо (особь), личность", "sk": "osoba", "sl": "oseba", "sv": "person", "sw": "personne", "th": "บุคคล, คน", "tok": "person", "tr": "kişi", "uk": "особа", "ur": "سخص", "vi": "con người", "yo": "person", "zh-tw": "人", "zh": "人,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 179,
		},
		{
			slug: "vortaro-radiko-pet", typ: "vocab",
			content: map[string]interface{}{
				"word": "pet",
				"definition": "to request",
				"definitions": map[string]interface{}{"en": "to request", "nl": "verzoeken, vragen", "de": "bitten", "fr": "demander", "es": "pedir", "pt": "pedir", "ar": "تطلب", "be": "просить", "ca": "demanar", "cs": "žádat", "da": "anmode", "el": "το να ζητώ", "fa": "درخواست کردن, فراخواندن", "frp": "demander", "ga": "iarr", "he": "לבקש", "hi": "to request", "hr": "moliti", "hu": "kér", "id": "meminta", "it": "chiedere", "ja": "請う", "kk": "to request", "km": "to request", "ko": "요청하다", "ku": "درخواست کردن, فراخواندن", "lo": "to request", "mg": "mangataka", "ms": "minta,kata akar", "my": "to request", "pl": "prosić", "ro": "demander", "ru": "просить", "sk": "prosiť, žiadať", "sl": "prositi", "sv": "be (om något)", "sw": "demander", "th": "ขอ, ขอร้อง", "tok": "to request", "tr": "rica etmek, talep etmek", "uk": "просити", "ur": "درخواست کرنا", "vi": "xin, cầu mong", "yo": "to request", "zh-tw": "要求", "zh": "要求,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 180,
		},
		{
			slug: "vortaro-radiko-pez", typ: "vocab",
			content: map[string]interface{}{
				"word": "pez",
				"definition": "heavy",
				"definitions": map[string]interface{}{"en": "heavy", "nl": "zwaar", "de": "schwer", "fr": "lourd", "es": "pesar", "pt": "pesado, ter peso", "ar": "ثقيل", "be": "тяжёлый", "ca": "pesar (tenir pes)", "cs": "těžké", "da": "tung", "el": "βαρύς-ιά-ύ", "fa": "سنگین", "frp": "lourd", "ga": "trom", "he": "כבד", "hi": "heavy", "hr": "težak", "hu": "nyom", "id": "berat", "it": "pesante", "ja": "重い", "kk": "heavy", "km": "ធ្ងន់", "ko": "무거운", "ku": "سنگین", "lo": "heavy", "mg": "mavesatra", "ms": "berat,kata akar", "my": "heavy", "pl": "ciężki", "ro": "lourd", "ru": "тяжёлый", "sk": "ťažký", "sl": "težko", "sv": "tung", "sw": "lourd", "th": "หนัก", "tok": "heavy", "tr": "ağır", "uk": "важити", "ur": "بھاری", "vi": "nặng nề", "yo": "heavy", "zh-tw": "重", "zh": "重,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 181,
		},
		{
			slug: "vortaro-radiko-pied", typ: "vocab",
			content: map[string]interface{}{
				"word": "pied",
				"definition": "foot",
				"definitions": map[string]interface{}{"en": "foot", "nl": "voet", "de": "Fuß", "fr": "pied", "es": "pie", "pt": "pé", "ar": "قدم", "be": "нога, стопа, лапа", "ca": "peu", "cs": "noha", "da": "fod", "el": "πόδι", "fa": "پا", "frp": "pied", "ga": "cos", "he": "רגל", "hi": "foot", "hr": "stopalo", "hu": "láb", "id": "kaki", "it": "piede", "ja": "足", "kk": "foot", "km": "foot", "ko": "발", "ku": "پا", "lo": "foot", "mg": "tongotra", "ms": "kaki,kata akar", "my": "foot", "pl": "piedo", "ro": "pied", "ru": "нога, стопа, лапа", "sk": "noha", "sl": "noga, stopalo", "sv": "fot", "sw": "pied", "th": "เท้า", "tok": "foot", "tr": "ayak", "uk": "нога, ступня", "ur": "پاؤں", "vi": "chân", "yo": "foot", "zh-tw": "腳", "zh": "脚,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 182,
		},
		{
			slug: "vortaro-radiko-plac", typ: "vocab",
			content: map[string]interface{}{
				"word": "plac",
				"definition": "place",
				"definitions": map[string]interface{}{"en": "place", "nl": "plein, plaats", "de": "Platz", "fr": "place", "es": "plaza", "pt": "praça", "ar": "مكان", "be": "площадь, площадка", "ca": "plaça", "cs": "náměstí", "da": "plads", "el": "πλατεία", "fa": "محوطه, میدان", "frp": "place", "ga": "áit", "he": "מקום", "hi": "place", "hr": "trg", "hu": "tér", "id": "tempat", "it": "piazza", "ja": "場所", "kk": "place", "km": "place", "ko": "장소", "ku": "محوطه, میدان", "lo": "place", "mg": "toerana", "ms": "tempat,kata akar", "my": "place", "pl": "plac", "ro": "place", "ru": "площадь, площадка", "sk": "námestie", "sl": "trg, ploščad", "sv": "torg", "sw": "place", "th": "สถานที่", "tok": "place", "tr": "yer", "uk": "майдан, площа", "ur": "جگہ", "vi": "nơi chốn", "yo": "place", "zh-tw": "廣場", "zh": "广场,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 183,
		},
		{
			slug: "vortaro-radiko-plan", typ: "vocab",
			content: map[string]interface{}{
				"word": "plan",
				"definition": "to plan",
				"definitions": map[string]interface{}{"en": "to plan", "nl": "plan", "de": "Plan", "fr": "planifier", "es": "plan", "pt": "planejar", "ar": "خطة", "be": "план", "ca": "pla", "cs": "plánovat", "da": "planlægge", "el": "σχέδιο", "fa": "طرح, برنامه", "frp": "planifier", "ga": "pleanáil", "he": "לתכנן", "hi": "to plan", "hr": "planirati", "hu": "terv", "id": "rencana", "it": "progettare", "ja": "計画する", "kk": "to plan", "km": "to plan", "ko": "계획하다", "ku": "طرح, برنامه", "lo": "to plan", "mg": "mandrafitra", "ms": "rancang,kata akar", "my": "to plan", "pl": "plan", "ro": "planifier", "ru": "план", "sk": "plánovať", "sl": "planirati", "sv": "plan", "sw": "planifier", "th": "วางแผน", "tok": "to plan", "tr": "planlamak", "uk": "плянувати", "ur": "منصوبہ بنانا", "vi": "lên kế hoạch, dự định", "yo": "to plan", "zh-tw": "計畫", "zh": "计划,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 184,
		},
		{
			slug: "vortaro-radiko-placx", typ: "vocab",
			content: map[string]interface{}{
				"word": "plaĉ",
				"definition": "to be pleasing",
				"definitions": map[string]interface{}{"en": "to be pleasing", "nl": "bevallen", "de": "gefallen", "fr": "plaire", "es": "agradar, gustar", "pt": "ser agradável", "ar": "يعجب", "be": "нравиться", "ca": "agradar", "cs": "líbit se", "da": "tilfredsstille", "el": "το να αρέσω", "fa": "خوشایند بودن", "frp": "plaire", "ga": "taitnigh", "he": "למצוא חן", "hi": "to be pleasing", "hr": "svidjeti se", "hu": "tetszik", "id": "menyenangkan", "it": "piacere", "ja": "好ましい", "kk": "to be pleasing", "km": "to be pleasing", "ko": "기쁘게하다, 마음에 들게 하다", "ku": "خوشایند بودن", "lo": "to be pleasing", "mg": "ankasitrahana", "ms": "merasa gembira,kata akar", "my": "to be pleasing", "pl": "podobać się", "ro": "plaire", "ru": "нравиться", "sk": "páčiť sa", "sl": "ugajati", "sv": "behaga", "sw": "plaire", "th": "เป็นที่พอใจ, ต้องใจ", "tok": "to be pleasing", "tr": "memnu etmek", "uk": "подобатися", "ur": "خوش ہونا", "vi": "vừa ý", "yo": "to be pleasing", "zh-tw": "討喜歡", "zh": "令人愉快,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 185,
		},
		{
			slug: "vortaro-radiko-plen", typ: "vocab",
			content: map[string]interface{}{
				"word": "plen",
				"definition": "full",
				"definitions": map[string]interface{}{"en": "full", "nl": "vol", "de": "voll", "fr": "plein", "es": "lleno/a", "pt": "cheio, cheia", "ar": "كامل", "be": "полный", "ca": "ple/plena", "cs": "plný", "da": "fuld", "el": "γεμάτος-η-ο", "fa": "پُر", "frp": "plein", "ga": "lán", "he": "מלא", "hi": "full", "hr": "pun", "hu": "tele", "id": "penuh", "it": "pieno", "ja": "いっぱいの", "kk": "full", "km": "full", "ko": "가득찬", "ku": "پُر", "lo": "full", "mg": "feno", "ms": "penuh,kata akar", "my": "full", "pl": "pełny", "ro": "plein", "ru": "полный", "sk": "plný", "sl": "poln", "sv": "full, fullständig", "sw": "plein", "th": "เต็ม", "tok": "full", "tr": "dolu, full", "uk": "повний", "ur": "full", "vi": "đầy đủ", "yo": "full", "zh-tw": "滿", "zh": "满,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 186,
		},
		{
			slug: "vortaro-radiko-polic", typ: "vocab",
			content: map[string]interface{}{
				"word": "polic",
				"definition": "police",
				"definitions": map[string]interface{}{"en": "police", "nl": "politie", "de": "Polizei", "fr": "police", "es": "policía", "pt": "polícia", "ar": "شرطة", "be": "полиция", "ca": "policía", "cs": "policie", "da": "politi", "el": "αστυνομία", "fa": "پلیس", "frp": "police", "ga": "póilíní", "he": "משטרה", "hi": "police", "hr": "policija", "hu": "rendőrség", "id": "polisi", "it": "polizia", "ja": "警察官", "kk": "police", "km": "police", "ko": "경찰", "ku": "پلیس", "lo": "police", "mg": "polisy", "ms": "polis,kata akar", "my": "police", "pl": "policja", "ro": "police", "ru": "полиция", "sk": "polícia", "sl": "policija", "sv": "polis", "sw": "police", "th": "ตำรวจ", "tok": "police", "tr": "polis", "uk": "поліція", "ur": "پولیس", "vi": "cảnh sát, khống chế", "yo": "police", "zh-tw": "警察", "zh": "警察,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 187,
		},
		{
			slug: "vortaro-radiko-pord", typ: "vocab",
			content: map[string]interface{}{
				"word": "pord",
				"definition": "door",
				"definitions": map[string]interface{}{"en": "door", "nl": "deur", "de": "Tür", "fr": "porte", "es": "puerta", "pt": "porta", "ar": "باب", "be": "дверь", "ca": "porta", "cs": "dveře", "da": "dør", "el": "πόρτα", "fa": "در", "frp": "porte", "ga": "doras", "he": "דלת", "hi": "door", "hr": "vrata", "hu": "ajtó", "id": "pintu", "it": "porta", "ja": "ドア", "kk": "есік", "km": "ទ្វារ", "ko": "문", "ku": "در", "lo": "door", "mg": "varavarana", "ms": "pintu,kata akar", "my": "door", "pl": "drzwi", "ro": "porte", "ru": "дверь", "sk": "dvere", "sl": "vrata", "sv": "dörr", "sw": "porte", "th": "ประตู", "tok": "door", "tr": "kapı", "uk": "двері", "ur": "دروازہ", "vi": "cánh cửa", "yo": "door", "zh-tw": "門", "zh": "门,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 188,
		},
		{
			slug: "vortaro-radiko-port", typ: "vocab",
			content: map[string]interface{}{
				"word": "port",
				"definition": "to carry",
				"definitions": map[string]interface{}{"en": "to carry", "nl": "dragen", "de": "tragen", "fr": "faire attention", "es": "llevar, traer", "pt": "carregar", "ar": "يحمل, يرتدي", "be": "нести, носить", "ca": "portar", "cs": "nést", "da": "bære", "el": "το να κρατώ", "fa": "حمل کردن", "frp": "faire attention", "ga": "iompair", "he": "לשאת, ללבוש", "hi": "to carry", "hr": "nositi", "hu": "visz, visel", "id": "bawa", "it": "portare", "ja": "携える", "kk": "to carry", "km": "to carry", "ko": "옮기다, 운반하다", "ku": "حمل کردن", "lo": "to carry", "mg": "mitandrina", "ms": "angkat,kata akar", "my": "to carry", "pl": "nosić", "ro": "faire attention", "ru": "нести, носить", "sk": "niesť", "sl": "nositi", "sv": "bära", "sw": "faire attention", "th": "ขน, นำไปด้วย", "tok": "to carry", "tr": "taşımak, giyior olmak", "uk": "нести, носити", "ur": "to carry", "vi": "mang, đem", "yo": "to carry", "zh-tw": "携帶", "zh": "带,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 189,
		},
		{
			slug: "vortaro-radiko-pov", typ: "vocab",
			content: map[string]interface{}{
				"word": "pov",
				"definition": "can",
				"definitions": map[string]interface{}{"en": "can", "nl": "kunnen", "de": "können", "fr": "pouvoir", "es": "poder", "pt": "ser capaz de, poder", "ar": "يستطيع", "be": "мочь", "ca": "poder", "cs": "moci", "da": "kunne", "el": "το να μπορώ", "fa": "توانستن", "frp": "pouvoir", "ga": "is féidir", "he": "יכול", "hi": "can", "hr": "moći", "hu": "tud, képes", "id": "bisa", "it": "potere", "ja": "できる", "kk": "can", "km": "can", "ko": "할 수 있다", "ku": "توانستن", "lo": "can", "mg": "fahefana", "ms": "boleh,kata akar", "my": "can", "pl": "móc", "ro": "pouvoir", "ru": "мочь", "sk": "môcť, dokázať", "sl": "moči", "sv": "kunna", "sw": "pouvoir", "th": "สามารถ", "tok": "can", "tr": "yapabilmek, edebilmek, -bilmek", "uk": "могти", "ur": "سکنا", "vi": "có thể", "yo": "can", "zh-tw": "可以", "zh": "可以,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 190,
		},
		{
			slug: "vortaro-radiko-pren", typ: "vocab",
			content: map[string]interface{}{
				"word": "pren",
				"definition": "to take",
				"definitions": map[string]interface{}{"en": "to take", "nl": "nemen", "de": "nehmen", "fr": "prendre", "es": "coger", "pt": "pegar", "ar": "أخذ", "be": "взять", "ca": "prendre", "cs": "vzít", "da": "tage", "el": "το να παίρνω", "fa": "گرفتن", "frp": "prendre", "ga": "tóg", "he": "לקחת", "hi": "to take", "hr": "uzeti", "hu": "elvesz, megfog", "id": "ambil", "it": "prendere", "ja": "取る", "kk": "to take", "km": "to take", "ko": "취하다, 손에 잡다", "ku": "گرفتن", "lo": "to take", "mg": "maka , mandray", "ms": "ambil,kata akar", "my": "to take", "pl": "brać", "ro": "prendre", "ru": "взять", "sk": "vziať", "sl": "vzeti", "sv": "ta", "sw": "prendre", "th": "เอามา", "tok": "to take", "tr": "almak", "uk": "брати", "ur": "to take", "vi": "lấy, cầm, nắm", "yo": "to take", "zh-tw": "拿", "zh": "拿,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 191,
		},
		{
			slug: "vortaro-radiko-pret", typ: "vocab",
			content: map[string]interface{}{
				"word": "pret",
				"definition": "ready",
				"definitions": map[string]interface{}{"en": "ready", "nl": "gereed, bereid, klaar", "de": "bereit", "fr": "prêt", "es": "listo, preparado", "pt": "pronto, pronta", "ar": "جاهز, مستعد", "be": "готовый", "ca": "llest, a punt, preparat/da", "cs": "připraveno", "da": "være klar", "el": "έτοιμος-η-ο", "fa": "آماده", "frp": "prêt", "ga": "ullamh", "he": "מוכן", "hi": "ready", "hr": "spreman", "hu": "kész", "id": "siap", "it": "pronto", "ja": "用意のできた", "kk": "ready", "km": "ready", "ko": "준비된", "ku": "آماده", "lo": "ready", "mg": "vonona", "ms": "sedia,kata akar", "my": "ready", "pl": "gotowy", "ro": "prêt", "ru": "готовый", "sk": "pripravený", "sl": "pripravljen", "sv": "färdig, redo", "sw": "prêt", "th": "พร้อม", "tok": "ready", "tr": "hazır olmak", "uk": "готовий", "ur": "تیار", "vi": "sẵn sàng", "yo": "ready", "zh-tw": "備好", "zh": "备好,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 192,
		},
		{
			slug: "vortaro-radiko-prezent", typ: "vocab",
			content: map[string]interface{}{
				"word": "prezent",
				"definition": "to present",
				"definitions": map[string]interface{}{"en": "to present", "nl": "voorstellen", "de": "vorstellen", "fr": "présenter", "es": "presentar", "pt": "apresentar, apresentação", "ar": "يقدم", "be": "представить", "ca": "presentar", "cs": "představit", "da": "præsentere", "el": "το να παρουσιάζω", "fa": "معرفی کردن", "frp": "présenter", "ga": "bronn", "he": "להציג", "hi": "to present", "hr": "predstaviti", "hu": "bemutat", "id": "menghadirkan", "it": "presentare", "ja": "提示する", "kk": "to present", "km": "to present", "ko": "증정하다, 제출하다, 소개하다", "ku": "معرفی کردن", "lo": "to present", "mg": "manolotra", "ms": "mempekenalkan,kata akar", "my": "to present", "pl": "przedstawiać", "ro": "présenter", "ru": "представить", "sk": "predstaviť, predviesť, navrhnúť", "sl": "pokazati", "sv": "presentera", "sw": "présenter", "th": "นำเสอน", "tok": "to present", "tr": "sunmak", "uk": "презентувати", "ur": "پیش کرنا", "vi": "biểu thị, trình bày", "yo": "to present", "zh-tw": "呈現", "zh": "呈现,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 193,
		},
		{
			slug: "vortaro-radiko-problem", typ: "vocab",
			content: map[string]interface{}{
				"word": "problem",
				"definition": "problem",
				"definitions": map[string]interface{}{"en": "problem", "nl": "probleem", "de": "Problem", "fr": "problème", "es": "problema", "pt": "problema", "ar": "مشكلة", "be": "проблема", "ca": "problema", "cs": "problém", "da": "problem", "el": "πρόβλημα", "fa": "مشکل, مسئله", "frp": "problème", "ga": "fadhb", "he": "בעיה", "hi": "problem", "hr": "problem", "hu": "probléma", "id": "masalah", "it": "problema", "ja": "問題", "kk": "problem", "km": "បញ្ហា", "ko": "문제", "ku": "مشکل, مسئله", "lo": "problem", "mg": "olana", "ms": "masalah,kata akar", "my": "problem", "pl": "problem", "ro": "problème", "ru": "проблема", "sk": "problém", "sl": "problem", "sv": "problem", "sw": "problème", "th": "ปัญหา", "tok": "problem", "tr": "problem", "uk": "проблема", "ur": "مسئلہ", "vi": "vấn đề", "yo": "problem", "zh-tw": "問題, 困難", "zh": "问题，困难,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 194,
		},
		{
			slug: "vortaro-radiko-proksim", typ: "vocab",
			content: map[string]interface{}{
				"word": "proksim",
				"definition": "near",
				"definitions": map[string]interface{}{"en": "near", "nl": "dicht(bij)", "de": "nahe", "fr": "à côté", "es": "cercano/a", "pt": "perto", "ar": "بجانب", "be": "близкий", "ca": "proper/a, pròxim/a", "cs": "blízko", "da": "nær", "el": "κοντινός-ή-ό", "fa": "نزدیک", "frp": "à côté", "ga": "in aice", "he": "קרוב", "hi": "near", "hr": "blizu", "hu": "közel", "id": "dekat", "it": "vicino", "ja": "近い", "kk": "near", "km": "នៅក្បែរ", "ko": "가까운", "ku": "نزدیک", "lo": "near", "mg": "anila , ankila", "ms": "dekat,kata akar", "my": "near", "pl": "blisko", "ro": "à côté", "ru": "близкий", "sk": "blízky", "sl": "blizu", "sv": "nära", "sw": "à côté", "th": "ใกล้", "tok": "near", "tr": "yakın", "uk": "близький, ближній", "ur": "نزدیک", "vi": "gần", "yo": "near", "zh-tw": "近", "zh": "近,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 195,
		},
		{
			slug: "vortaro-radiko-promen", typ: "vocab",
			content: map[string]interface{}{
				"word": "promen",
				"definition": "to stroll",
				"definitions": map[string]interface{}{"en": "to stroll", "nl": "wandelen", "de": "spazierengehen", "fr": "se promener", "es": "pasear", "pt": "passear, passeio", "ar": "تنزه, تجول", "be": "гулять, прогуливаться", "ca": "passejar", "cs": "procházet se", "da": "slentre", "el": "το να περπατώ", "fa": "برای تفریح یا سلامتی گشتن", "frp": "se promener", "ga": "spaisteoireacht", "he": "לטייל", "hi": "to stroll", "hr": "šetati", "hu": "sétál", "id": "jalan-jalan", "it": "passeggiare", "ja": "散歩をする", "kk": "to stroll", "km": "to stroll", "ko": "산보하다", "ku": "برای تفریح یا سلامتی گشتن", "lo": "to stroll", "mg": "mitsangatsangana", "ms": "bersiar-siar,kata akar", "my": "to stroll", "pl": "spacerować", "ro": "se promener", "ru": "гулять, прогуливаться", "sk": "prechádzať sa", "sl": "sprehajati se", "sv": "promenera", "sw": "se promener", "th": "เดินเล่น", "tok": "to stroll", "tr": "dolaşmak", "uk": "гуляти, прогулюватися", "ur": "to stroll", "vi": "đi dạo, tản bộ", "yo": "to stroll", "zh-tw": "散步", "zh": "散步,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 196,
		},
		{
			slug: "vortaro-radiko-propon", typ: "vocab",
			content: map[string]interface{}{
				"word": "propon",
				"definition": "to propose",
				"definitions": map[string]interface{}{"en": "to propose", "nl": "voorstellen", "de": "vorschlagen", "fr": "proposer", "es": "proponer", "pt": "propor, proposta", "ar": "عرض", "be": "предложить", "ca": "proposar", "cs": "nabídka", "da": "foreslå", "el": "το να προτείνω", "fa": "پیشنهاد دادن", "frp": "proposer", "ga": "mol", "he": "להציע", "hi": "to propose", "hr": "predložiti", "hu": "ajánl, javasol", "id": "usul", "it": "proporre", "ja": "提案する", "kk": "to propose", "km": "to propose", "ko": "제안하다", "ku": "پیشنهاد دادن", "lo": "to propose", "mg": "manolotra , mamosaka", "ms": "cadang,kata akar", "my": "to propose", "pl": "proponować", "ro": "proposer", "ru": "предложить", "sk": "ponuka", "sl": "predlagati", "sv": "föreslå", "sw": "proposer", "th": "เสนอ, ข้อเสนอ", "tok": "to propose", "tr": "önermek", "uk": "пропонувати", "ur": "to propose", "vi": "đề nghị", "yo": "to propose", "zh-tw": "建議", "zh": "建议,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 197,
		},
		{
			slug: "vortaro-radiko-prov", typ: "vocab",
			content: map[string]interface{}{
				"word": "prov",
				"definition": "to try",
				"definitions": map[string]interface{}{"en": "to try", "nl": "proberen", "de": "versuchen", "fr": "essayer", "es": "intentar", "pt": "tentar", "ar": "يحاول", "be": "пробовать", "ca": "intentar, provar", "cs": "zkusit", "da": "prøve", "el": "το να δοκιμάζω", "fa": "تلاش کردن, آزمودن", "frp": "essayer", "ga": "iarracht", "he": "לנסות", "hi": "to try", "hr": "pokušati", "hu": "próbál", "id": "coba", "it": "provare, tentare", "ja": "試みる", "kk": "to try", "km": "to try", "ko": "시도하다", "ku": "تلاش کردن, آزمودن", "lo": "to try", "mg": "mitsapa , manandrana", "ms": "cuba,kata akar", "my": "to try", "pl": "próbować", "ro": "essayer", "ru": "пробовать", "sk": "skúsiť", "sl": "poskusiti", "sv": "prova", "sw": "essayer", "th": "ลอง", "tok": "to try", "tr": "denemek", "uk": "пробувати", "ur": "کوشش کرنا", "vi": "thử, cố gắng", "yo": "to try", "zh-tw": "嘗試", "zh": "尝试,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 198,
		},
		{
			slug: "vortaro-radiko-rajt", typ: "vocab",
			content: map[string]interface{}{
				"word": "rajt",
				"definition": "to be allowed to",
				"definitions": map[string]interface{}{"en": "to be allowed to", "nl": "het recht hebben, mogen", "de": "das Recht haben, dürfen", "fr": "avoir le droit de", "es": "tener derecho (a)", "pt": "direito, ter direito de", "ar": "الحق في", "be": "иметь право", "ca": "poder, tenir dret (a)", "cs": "smět, mít právo", "da": "måtte", "el": "δικαίωμα", "fa": "حق داشتن", "frp": "avoir le droit de", "ga": "cead", "he": "בעל זכות", "hi": "to be allowed to", "hr": "smjeti", "hu": "szabad, jog", "id": "berhak", "it": "avere il permesso di, potere", "ja": "してもよい", "kk": "to be allowed to", "km": "to be allowed to", "ko": "권리가 있는", "ku": "حق داشتن", "lo": "to be allowed to", "mg": "manan-jo amin'ny", "ms": "hak,kata akar", "my": "to be allowed to", "pl": "mieć prawo", "ro": "avoir le droit de", "ru": "иметь право", "sk": "smieť, mať právo", "sl": "smeti", "sv": "få, ha rätt till", "sw": "avoir le droit de", "th": "มีสิทธิ์", "tok": "to be allowed to", "tr": "izni olmak", "uk": "право", "ur": "to be allowed to", "vi": "cho phép", "yo": "to be allowed to", "zh-tw": "權益", "zh": "权益,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 199,
		},
		{
			slug: "vortaro-radiko-rapid", typ: "vocab",
			content: map[string]interface{}{
				"word": "rapid",
				"definition": "fast",
				"definitions": map[string]interface{}{"en": "fast", "nl": "snel, rap", "de": "schnell", "fr": "rapide", "es": "rápido/a", "pt": "rápido, rápida", "ar": "سريع", "be": "быстрый", "ca": "ràpid/a", "cs": "rychle", "da": "hurtig", "el": "γρήγορος-η-ο", "fa": "سریع", "frp": "rapide", "ga": "tapa", "he": "מהיר", "hi": "fast", "hr": "brz", "hu": "gyors", "id": "cepat", "it": "veloce", "ja": "速い", "kk": "fast", "km": "fast", "ko": "빠른", "ku": "سریع", "lo": "fast", "mg": "malaky , faingana", "ms": "cepat,kata akar", "my": "fast", "pl": "szybki", "ro": "rapide", "ru": "быстрый", "sk": "rýchly", "sl": "hitro", "sv": "snabb", "sw": "rapide", "th": "เร็ว", "tok": "fast", "tr": "hızlı", "uk": "швидкий", "ur": "تیز", "vi": "nhanh", "yo": "fast", "zh-tw": "快", "zh": "快,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 200,
		},
		{
			slug: "vortaro-radiko-raport", typ: "vocab",
			content: map[string]interface{}{
				"word": "raport",
				"definition": "report",
				"definitions": map[string]interface{}{"en": "report", "nl": "verlaggeven, rapporteren", "de": "berichten", "fr": "rapporter", "es": "informar", "pt": "relatório, relatar", "ar": "تقرير", "be": "доложить", "ca": "informar", "cs": "zpráva", "da": "anmelde", "el": "το να αναφέρω", "fa": "گزارش دادن", "frp": "rapporter", "ga": "tuairisc", "he": "דיווח", "hi": "report", "hr": "izvijestiti", "hu": "jelent", "id": "lapor", "it": "rapporto, esposto", "ja": "報告", "kk": "report", "km": "report", "ko": "보고하다, 보도하다", "ku": "گزارش دادن", "lo": "report", "mg": "mamerina , mitondra", "ms": "lapor,kata akar", "my": "report", "pl": "raport", "ro": "rapporter", "ru": "доложить", "sk": "správa", "sl": "poročati", "sv": "rapportera", "sw": "rapporter", "th": "รายงาน", "tok": "report", "tr": "rapor vermek", "uk": "рапортувати", "ur": "رپورٹ", "vi": "báo cáo", "yo": "report", "zh-tw": "報告", "zh": "报告,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 201,
		},
		{
			slug: "vortaro-radiko-real", typ: "vocab",
			content: map[string]interface{}{
				"word": "real",
				"definition": "real",
				"definitions": map[string]interface{}{"en": "real", "nl": "werkelijk, reëel", "de": "wirklich", "fr": "réél", "es": "real", "pt": "real", "ar": "شئ حقيقي", "be": "настоящий", "ca": "real", "cs": "skutečný", "da": "rigtig", "el": "πραγματικός-ή-ό", "fa": "واقعی", "frp": "réél", "ga": "fíor-", "he": "אמיתי", "hi": "real", "hr": "stvaran", "hu": "valódi", "id": "asli", "it": "reale", "ja": "実際の", "kk": "real", "km": "real", "ko": "실제의", "ku": "واقعی", "lo": "real", "mg": "misy tokoa , marina", "ms": "benar,kata akar", "my": "real", "pl": "rzeczywisty", "ro": "réél", "ru": "настоящий", "sk": "skutočný", "sl": "resnično", "sv": "verklig", "sw": "réél", "th": "เกิดขึ้นจริง", "tok": "real", "tr": "gerçek", "uk": "реальний", "ur": "اصل", "vi": "thực tế, có thật", "yo": "real", "zh-tw": "真實", "zh": "真实,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 202,
		},
		{
			slug: "vortaro-radiko-rekt", typ: "vocab",
			content: map[string]interface{}{
				"word": "rekt",
				"definition": "direct",
				"definitions": map[string]interface{}{"en": "direct", "nl": "rechtstreeks, direct", "de": "direkt", "fr": "directe", "es": "directo/a", "pt": "reto, direto", "ar": "مباشرة", "be": "прямой", "ca": "directe/a", "cs": "přímý", "da": "direkte", "el": "ευθύς-εία-ές", "fa": "مستقیم", "frp": "directe", "ga": "díreach", "he": "ישיר", "hi": "direct", "hr": "direktan", "hu": "egyenes", "id": "langsung", "it": "diretto, dritto", "ja": "まっすぐな, 直接の", "kk": "direct", "km": "direct", "ko": "똑바른, 직접의, 솔직한", "ku": "مستقیم", "lo": "direct", "mg": "mahitsy", "ms": "terus,kata akar", "my": "direct", "pl": "bezpośredni", "ro": "directe", "ru": "прямой", "sk": "priamy", "sl": "smer", "sv": "rak, direkt", "sw": "directe", "th": "โดยตรง, ทางตรง", "tok": "direct", "tr": "direk", "uk": "прямий", "ur": "direct", "vi": "gửi đến", "yo": "direct", "zh-tw": "直接", "zh": "直接,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 203,
		},
		{
			slug: "vortaro-radiko-renkont", typ: "vocab",
			content: map[string]interface{}{
				"word": "renkont",
				"definition": "to meet",
				"definitions": map[string]interface{}{"en": "to meet", "nl": "ontmoeten", "de": "begegnen", "fr": "rencontrer", "es": "encontrarse (a)", "pt": "encontrar", "ar": "يقابل", "be": "встретить", "ca": "trobar-se (amb), topar", "cs": "potkat", "da": "møde", "el": "το να συναντώ", "fa": "ملاقات کردن", "frp": "rencontrer", "ga": "buail le", "he": "לפגוש", "hi": "to meet", "hr": "susresti", "hu": "találkozik", "id": "menemui", "it": "incontrare", "ja": "出会う", "kk": "to meet", "km": "to meet", "ko": "만나다", "ku": "ملاقات کردن", "lo": "to meet", "mg": "mifanena", "ms": "jumpa,kata akar", "my": "to meet", "pl": "spotykać", "ro": "rencontrer", "ru": "встретить", "sk": "stretnúť", "sl": "srečati", "sv": "möta, träffa", "sw": "rencontrer", "th": "พบปะ", "tok": "to meet", "tr": "tanışmak, karşılaşmak", "uk": "зустрічати", "ur": "ملنا", "vi": "gặp gỡ", "yo": "to meet", "zh-tw": "會見", "zh": "会见,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 204,
		},
		{
			slug: "vortaro-radiko-respekt", typ: "vocab",
			content: map[string]interface{}{
				"word": "respekt",
				"definition": "to respect",
				"definitions": map[string]interface{}{"en": "to respect", "nl": "achten, respecteren", "de": "achten", "fr": "respecter", "es": "respetar", "pt": "respeitar, respeito", "ar": "احترام", "be": "почтение, уважение", "ca": "respectar", "cs": "respektovat", "da": "respektere", "el": "το να σέβομαι", "fa": "احترام گذاشتن به", "frp": "respecter", "ga": "meas", "he": "לכבד", "hi": "to respect", "hr": "uvažavanje", "hu": "tisztel", "id": "menghargai", "it": "rispettare", "ja": "尊敬する", "kk": "to respect", "km": "to respect", "ko": "존경하다", "ku": "احترام گذاشتن به", "lo": "to respect", "mg": "mifanaja", "ms": "hormat,kata akar", "my": "to respect", "pl": "szanować", "ro": "respecter", "ru": "почтение, уважение", "sk": "rešpektovať", "sl": "spoštovati", "sv": "respektera", "sw": "respecter", "th": "เคารพ", "tok": "to respect", "tr": "saygı duymak", "uk": "поважати", "ur": "عزت کرنا", "vi": "tôn trọng", "yo": "to respect", "zh-tw": "尊敬", "zh": "尊敬,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 205,
		},
		{
			slug: "vortaro-radiko-respond", typ: "vocab",
			content: map[string]interface{}{
				"word": "respond",
				"definition": "to answer",
				"definitions": map[string]interface{}{"en": "to answer", "nl": "antwoorden, beantwoorden", "de": "antworten", "fr": "répondre", "es": "responder", "pt": "responder, resposta", "ar": "إجابة", "be": "ответить", "ca": "respondre", "cs": "odpovědět", "da": "besvare", "el": "το να απαντώ", "fa": "پاسخ دادن", "frp": "répondre", "ga": "freagra", "he": "לענות", "hi": "to answer", "hr": "odgovoriti", "hu": "válaszol", "id": "menjawab", "it": "rispondere", "ja": "答える", "kk": "to answer", "km": "to answer", "ko": "대답하다", "ku": "پاسخ دادن", "lo": "to answer", "mg": "mamaly", "ms": "jawab,kata akar", "my": "to answer", "pl": "odpowiadać", "ro": "répondre", "ru": "ответить", "sk": "odpovedať", "sl": "odgovoriti", "sv": "svara", "sw": "répondre", "th": "ตอบ", "tok": "to answer", "tr": "cevap vermek", "uk": "відповідати", "ur": "جواب دینا", "vi": "trả lời", "yo": "to answer", "zh-tw": "回復", "zh": "回复,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 206,
		},
		{
			slug: "vortaro-radiko-rest", typ: "vocab",
			content: map[string]interface{}{
				"word": "rest",
				"definition": "to remain",
				"definitions": map[string]interface{}{"en": "to remain", "nl": "blijven", "de": "bleiben", "fr": "rester", "es": "quedarse", "pt": "permanecer", "ar": "يبقي", "be": "находиться", "ca": "romandre, quedar-se", "cs": "zůstat", "da": "blive", "el": "το να μένω", "fa": "باقی ماندن, ماندن", "frp": "rester", "ga": "fan", "he": "להישאר", "hi": "to remain", "hr": "ostati", "hu": "marad", "id": "sisa", "it": "restare", "ja": "留まっている", "kk": "to remain", "km": "to remain", "ko": "남다", "ku": "باقی ماندن, ماندن", "lo": "to remain", "mg": "mijanona", "ms": "tinggal,kata akar", "my": "to remain", "pl": "zostawać", "ro": "rester", "ru": "находиться", "sk": "zostať", "sl": "ostati", "sv": "stanna kvar, förbli", "sw": "rester", "th": "พักอาศัย", "tok": "to remain", "tr": "geriye kalmak, arta kalmak", "uk": "залишатися, продовжувати знаходитися", "ur": "to remain", "vi": "còn sót, tàn tích", "yo": "to remain", "zh-tw": "停留", "zh": "停留,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 207,
		},
		{
			slug: "vortaro-radiko-ricev", typ: "vocab",
			content: map[string]interface{}{
				"word": "ricev",
				"definition": "to receive",
				"definitions": map[string]interface{}{"en": "to receive", "nl": "ontvangen, krijgen", "de": "bekommen", "fr": "recevoir", "es": "recibir", "pt": "receber", "ar": "تسلم, تلقى", "be": "получить", "ca": "rebre", "cs": "dostat", "da": "modtage", "el": "το να λαμβάνω", "fa": "دریافت کردن", "frp": "recevoir", "ga": "faigh", "he": "לקבל", "hi": "to receive", "hr": "dobiti", "hu": "kap", "id": "menerima", "it": "ricevere", "ja": "受け取る", "kk": "to receive", "km": "to receive", "ko": "받다", "ku": "دریافت کردن", "lo": "to receive", "mg": "mandray , mahazo", "ms": "menerima,kata akar", "my": "to receive", "pl": "otrzymywać", "ro": "recevoir", "ru": "получить", "sk": "dostať", "sl": "prejeti", "sv": "få", "sw": "recevoir", "th": "รับ", "tok": "to receive", "tr": "teslim almak", "uk": "отримувати", "ur": "وصول کرنا", "vi": "nhận được", "yo": "to receive", "zh-tw": "接獲, 收到", "zh": "接获，收到,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 208,
		},
		{
			slug: "vortaro-radiko-rid", typ: "vocab",
			content: map[string]interface{}{
				"word": "rid",
				"definition": "to laugh",
				"definitions": map[string]interface{}{"en": "to laugh", "nl": "lachen", "de": "lachen", "fr": "rire", "es": "reír", "pt": "rir", "ar": "ضحك", "be": "смеяться", "ca": "riure", "cs": "smát se", "da": "grine", "el": "το να γελώ", "fa": "خندیدن", "frp": "rire", "ga": "gáir", "he": "לצחוק", "hi": "to laugh", "hr": "smijati se", "hu": "nevet", "id": "tawa", "it": "ridere", "ja": "笑う", "kk": "to laugh", "km": "to laugh", "ko": "웃다", "ku": "خندیدن", "lo": "to laugh", "mg": "hehy", "ms": "ketawa,kata akar", "my": "to laugh", "pl": "śmiać się", "ro": "rire", "ru": "смеяться", "sk": "smiať sa", "sl": "smejati se", "sv": "skratta", "sw": "rire", "th": "หัวเราะ", "tok": "to laugh", "tr": "gülmek", "uk": "сміятись", "ur": "مسکرانا", "vi": "cười lên", "yo": "to laugh", "zh-tw": "笑", "zh": "笑,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 209,
		},
		{
			slug: "vortaro-radiko-rigard", typ: "vocab",
			content: map[string]interface{}{
				"word": "rigard",
				"definition": "to look at",
				"definitions": map[string]interface{}{"en": "to look at", "nl": "kijken, kijken naar, bekijken", "de": "schauen", "fr": "regarder", "es": "mirar", "pt": "olhar para", "ar": "شاهد", "be": "смотреть", "ca": "mirar", "cs": "dívat se", "da": "kigge på", "el": "το να κοιτάζω", "fa": "نگاه کردن به", "frp": "regarder", "ga": "féach ar", "he": "להסתכל", "hi": "to look at", "hr": "promatrati, gledati", "hu": "néz", "id": "memandang", "it": "guardare", "ja": "目を向ける", "kk": "to look at", "km": "to look at", "ko": "바라보다", "ku": "نگاه کردن به", "lo": "to look at", "mg": "mijery", "ms": "melihat,kata akar", "my": "to look at", "pl": "patrzeć", "ro": "regarder", "ru": "смотреть", "sk": "dívať sa", "sl": "gledati", "sv": "titta", "sw": "regarder", "th": "ดู", "tok": "to look at", "tr": "bakmak", "uk": "дивитись", "ur": "to look at", "vi": "ngắm nhìn", "yo": "to look at", "zh-tw": "看", "zh": "看,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 210,
		},
		{
			slug: "vortaro-radiko-ricx", typ: "vocab",
			content: map[string]interface{}{
				"word": "riĉ",
				"definition": "rich",
				"definitions": map[string]interface{}{"en": "rich", "nl": "rijk", "de": "reich", "fr": "riche", "es": "rico/a", "pt": "rico, rica", "ar": "غني", "be": "богатый", "ca": "ric/a", "cs": "bohatý", "da": "rig", "el": "πλούσιος-α-ο", "fa": "ثروتمند, غنی", "frp": "riche", "ga": "saibhir", "he": "עשיר", "hi": "rich", "hr": "bogat", "hu": "gazdag", "id": "kaya", "it": "ricco", "ja": "豊かな", "kk": "бай", "km": "សម្បូរបែប", "ko": "부유한", "ku": "ثروتمند, غنی", "lo": "rich", "mg": "manan-karena", "ms": "kaya,kata akar", "my": "rich", "pl": "boagty", "ro": "riche", "ru": "богатый", "sk": "bohatý", "sl": "bogat", "sv": "rik", "sw": "riche", "th": "รวย", "tok": "rich", "tr": "zengin", "uk": "багатий", "ur": "امیر", "vi": "giàu có", "yo": "rich", "zh-tw": "富", "zh": "富,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 211,
		},
		{
			slug: "vortaro-radiko-rol", typ: "vocab",
			content: map[string]interface{}{
				"word": "rol",
				"definition": "role",
				"definitions": map[string]interface{}{"en": "role", "nl": "rol", "de": "Rolle", "fr": "rôle", "es": "papel", "pt": "função, desempenhar papel de", "ar": "دور", "be": "роль", "ca": "paper, rol", "cs": "role", "da": "rolle", "el": "ρόλος", "fa": "نقش", "frp": "rôle", "ga": "ról", "he": "תפקיד", "hi": "role", "hr": "uloga", "hu": "szerep", "id": "peran", "it": "ruolo", "ja": "役割", "kk": "рөл", "km": "role", "ko": "역할", "ku": "نقش", "lo": "role", "mg": "anjarany , filaharana", "ms": "peranan,kata akar", "my": "role", "pl": "rola", "ro": "rôle", "ru": "роль", "sk": "rola", "sl": "vloga", "sv": "roll", "sw": "rôle", "th": "บทบาท, หน้าที่", "tok": "role", "tr": "rol", "uk": "роль", "ur": "role", "vi": "vai trò", "yo": "role", "zh-tw": "角色", "zh": "角色,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 212,
		},
		{
			slug: "vortaro-radiko-rugx", typ: "vocab",
			content: map[string]interface{}{
				"word": "ruĝ",
				"definition": "red",
				"definitions": map[string]interface{}{"en": "red", "nl": "rood", "de": "rot", "fr": "rouge", "es": "rojo/a", "pt": "vermelho", "ar": "أحمر", "be": "красный", "ca": "roig/roja", "cs": "červená", "da": "rød", "el": "κόκκινος-η-ο", "fa": "قرمز", "frp": "rouge", "ga": "dearg", "he": "אדום", "hi": "red", "hr": "crven", "hu": "piros", "id": "merah", "it": "rosso", "ja": "赤い", "kk": "қызыл", "km": "ក្រហម", "ko": "붉은", "ku": "قرمز", "lo": "red", "mg": "mena", "ms": "merah,kata akar", "my": "red", "pl": "czerwony", "ro": "rouge", "ru": "красный", "sk": "červený", "sl": "rdeče", "sv": "röd", "sw": "rouge", "th": "สีแดง", "tok": "red", "tr": "kırmızı", "uk": "червоний", "ur": "سرخ", "vi": "đỏ rực", "yo": "red", "zh-tw": "紅", "zh": "红,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 213,
		},
		{
			slug: "vortaro-radiko-salt", typ: "vocab",
			content: map[string]interface{}{
				"word": "salt",
				"definition": "to jump",
				"definitions": map[string]interface{}{"en": "to jump", "nl": "springen", "de": "springen", "fr": "sauter", "es": "saltar", "pt": "saltar, salto", "ar": "قفز", "be": "прыгать", "ca": "saltar", "cs": "skákat", "da": "hoppe", "el": "το να πηδώ", "fa": "پریدن", "frp": "sauter", "ga": "léim", "he": "לקפוץ", "hi": "to jump", "hr": "skočiti", "hu": "ugrik", "id": "lompat", "it": "saltare", "ja": "跳ぶ", "kk": "to jump", "km": "to jump", "ko": "점프하다", "ku": "پریدن", "lo": "to jump", "mg": "mitsambikina", "ms": "lompat,kata akar", "my": "to jump", "pl": "skakać", "ro": "sauter", "ru": "прыгать", "sk": "skákať", "sl": "skočiti", "sv": "hoppa", "sw": "sauter", "th": "กระโดด", "tok": "to jump", "tr": "zıplamak", "uk": "стрибати", "ur": "اچھلنا", "vi": "nhảy lên", "yo": "to jump", "zh-tw": "跳", "zh": "跳,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 214,
		},
		{
			slug: "vortaro-radiko-salut", typ: "vocab",
			content: map[string]interface{}{
				"word": "salut",
				"definition": "hello",
				"definitions": map[string]interface{}{"en": "hello", "nl": "groet", "de": "Gruß", "fr": "salut", "es": "saludo", "pt": "saudar, saudação", "ar": "سلام", "be": "приветствие, привет", "ca": "saludar", "cs": "zdravit", "da": "hej", "el": "το να χαιρετώ", "fa": "سلام", "frp": "salut", "ga": "beannaigh", "he": "לברך לשלום", "hi": "hello", "hr": "pozdrav", "hu": "köszön", "id": "sapa", "it": "saluto, ciao", "ja": "挨拶する", "kk": "сәлем", "km": "hello", "ko": "인사하다", "ku": "سلام", "lo": "hello", "mg": "famonjena , fiarabana", "ms": "salam hormat,kata akar", "my": "hello", "pl": "pozdrawiać", "ro": "salut", "ru": "приветствие, привет", "sk": "zdraviť", "sl": "zdravo", "sv": "hälsa", "sw": "salut", "th": "สวัสดี", "tok": "hello", "tr": "merhaba", "uk": "вітати", "ur": "سلام", "vi": "xin chào", "yo": "hello", "zh-tw": "敬禮, 問好", "zh": "敬礼、祝福,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 215,
		},
		{
			slug: "vortaro-radiko-sam", typ: "vocab",
			content: map[string]interface{}{
				"word": "sam",
				"definition": "same",
				"definitions": map[string]interface{}{"en": "same", "nl": "gelijk", "de": "gleich", "fr": "même", "es": "mismo/a", "pt": "mesmo, mesma", "ar": "نفسه", "be": "такой-же", "ca": "mateix/a", "cs": "stejně", "da": "samme", "el": "ίδιος-α-ο", "fa": "همان", "frp": "même", "ga": "céanna", "he": "אותו דבר", "hi": "same", "hr": "isto", "hu": "ugyanaz", "id": "sama", "it": "stesso", "ja": "同じ", "kk": "same", "km": "ដូចគ្នា", "ko": "같은", "ku": "همان", "lo": "same", "mg": "izay izany", "ms": "sama,kata akar", "my": "same", "pl": "taki sam", "ro": "même", "ru": "такой-же", "sk": "rovnaký", "sl": "isto", "sv": "samma, likadan", "sw": "même", "th": "เหมือนกัน", "tok": "same", "tr": "aynı", "uk": "той самий", "ur": "اہک جیسا", "vi": "giống, như nhau", "yo": "same", "zh-tw": "相同", "zh": "相同,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 216,
		},
		{
			slug: "vortaro-radiko-san", typ: "vocab",
			content: map[string]interface{}{
				"word": "san",
				"definition": "healthy",
				"definitions": map[string]interface{}{"en": "healthy", "nl": "gezond", "de": "gesund", "fr": "santé", "es": "sano/a", "pt": "saudável, saúde", "ar": "صحة", "be": "здоровье", "ca": "sà/sana", "cs": "zdravý", "da": "rask", "el": "υγιής-ής-ές", "fa": "سالم", "frp": "santé", "ga": "sláinte", "he": "בריא", "hi": "healthy", "hr": "zdrav", "hu": "egészséges", "id": "sehat", "it": "sano", "ja": "健康", "kk": "healthy", "km": "healthy", "ko": "건강한", "ku": "سالم", "lo": "healthy", "mg": "fahasalamana", "ms": "sihat,kata akar", "my": "healthy", "pl": "zdrowy", "ro": "santé", "ru": "здоровье", "sk": "zdravý", "sl": "zdrav", "sv": "frisk", "sw": "santé", "th": "สุขภาพดี", "tok": "healthy", "tr": "sağlıklı", "uk": "здоровий", "ur": "صحت مند", "vi": "khoẻ mạnh, lành mạnh", "yo": "healthy", "zh-tw": "健康", "zh": "健康,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 217,
		},
		{
			slug: "vortaro-radiko-sagx", typ: "vocab",
			content: map[string]interface{}{
				"word": "saĝ",
				"definition": "wise",
				"definitions": map[string]interface{}{"en": "wise", "nl": "snugger, wijs", "de": "klug, weise", "fr": "sage", "es": "sabio/a", "pt": "sensato, sensata", "ar": "حكيم", "be": "умный", "ca": "savi/sàvia", "cs": "moudře", "da": "vis", "el": "σοφός-ή-ό", "fa": "هوشمند, باهوش", "frp": "sage", "ga": "críonna", "he": "חכם", "hi": "wise", "hr": "mudar", "hu": "okos", "id": "bijak", "it": "saggio", "ja": "賢い", "kk": "wise", "km": "wise", "ko": "현명한, 지혜로운", "ku": "هوشمند, باهوش", "lo": "wise", "mg": "hendry", "ms": "bijak,kata akar", "my": "wise", "pl": "mądry", "ro": "sage", "ru": "умный", "sk": "múdry", "sl": "moder", "sv": "vis, klok", "sw": "sage", "th": "ฉลาด", "tok": "wise", "tr": "akıllı", "uk": "розумний", "ur": "عقل مند", "vi": "không ngoan, uyên bác", "yo": "wise", "zh-tw": "智慧", "zh": "智慧,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 218,
		},
		{
			slug: "vortaro-radiko-sci", typ: "vocab",
			content: map[string]interface{}{
				"word": "sci",
				"definition": "to know",
				"definitions": map[string]interface{}{"en": "to know", "nl": "weten", "de": "wissen", "fr": "savoir", "es": "saber", "pt": "saber, conhecimento", "ar": "علم", "be": "знать", "ca": "saber", "cs": "znát", "da": "vide", "el": "το να ξέρω", "fa": "دانستن", "frp": "savoir", "ga": "fios", "he": "לדעת", "hi": "to know", "hr": "znati", "hu": "tud", "id": "tahu", "it": "sapere", "ja": "知る", "kk": "to know", "km": "to know", "ko": "알다", "ku": "دانستن", "lo": "to know", "mg": "fahaizana", "ms": "tahu,kata akar", "my": "to know", "pl": "wiedzieć", "ro": "savoir", "ru": "знать", "sk": "vedieť, ovládať", "sl": "vedeti", "sv": "veta", "sw": "savoir", "th": "รู้", "tok": "to know", "tr": "bilmek", "uk": "знати", "ur": "جاننا", "vi": "biết được một điều, tri thức", "yo": "to know", "zh-tw": "知道", "zh": "知道、明了,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 219,
		},
		{
			slug: "vortaro-radiko-sek", typ: "vocab",
			content: map[string]interface{}{
				"word": "sek",
				"definition": "dry",
				"definitions": map[string]interface{}{"en": "dry", "nl": "droog", "de": "trocken", "fr": "sec", "es": "seco/a", "pt": "seco, seca", "ar": "جاف", "be": "сухой", "ca": "sec/a", "cs": "suchý", "da": "tør", "el": "ξηρός-ή-ό", "fa": "خشک بودن", "frp": "sec", "ga": "tirim", "he": "יבש", "hi": "dry", "hr": "suh", "hu": "száraz", "id": "kering", "it": "secco", "ja": "乾いた", "kk": "dry", "km": "dry", "ko": "건조한", "ku": "خشک بودن", "lo": "dry", "mg": "maina", "ms": "kering,kata akar", "my": "dry", "pl": "suchy", "ro": "sec", "ru": "сухой", "sk": "suchý", "sl": "suh", "sv": "torr", "sw": "sec", "th": "แห้ง", "tok": "dry", "tr": "kuru", "uk": "сухий", "ur": "خشک", "vi": "khô cạn, khô ráo", "yo": "dry", "zh-tw": "乾", "zh": "干,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 220,
		},
		{
			slug: "vortaro-radiko-sekret", typ: "vocab",
			content: map[string]interface{}{
				"word": "sekret",
				"definition": "secret",
				"definitions": map[string]interface{}{"en": "secret", "nl": "geheim", "de": "Geheimnis", "fr": "secret", "es": "secreto", "pt": "segredo, secreto, secreta", "ar": "سر", "be": "тайна", "ca": "secret", "cs": "tajemství", "da": "hemmelighed", "el": "μυστικός-ή-ό", "fa": "راز", "frp": "secret", "ga": "rún", "he": "סוד", "hi": "secret", "hr": "tajna", "hu": "titok", "id": "rahasia", "it": "segreto", "ja": "秘密の", "kk": "құпия", "km": "secret", "ko": "비밀", "ku": "راز", "lo": "secret", "mg": "tsiambaratelo", "ms": "rahsia,kata akar", "my": "secret", "pl": "sekret", "ro": "secret", "ru": "тайна", "sk": "tajomstvo", "sl": "skrivnost", "sv": "hemlighet", "sw": "secret", "th": "คงามลับ", "tok": "secret", "tr": "gizli", "uk": "секрет", "ur": "راز", "vi": "bí mật", "yo": "secret", "zh-tw": "秘密", "zh": "秘密,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 221,
		},
		{
			slug: "vortaro-radiko-sekv", typ: "vocab",
			content: map[string]interface{}{
				"word": "sekv",
				"definition": "to follow",
				"definitions": map[string]interface{}{"en": "to follow", "nl": "volgen", "de": "folgen", "fr": "suivre", "es": "seguir", "pt": "seguir", "ar": "تابع", "be": "следовать", "ca": "seguir", "cs": "následovat", "da": "følge", "el": "το να ακολουθώ", "fa": "دنبال کردن", "frp": "suivre", "ga": "lean", "he": "לעקוב", "hi": "to follow", "hr": "slijediti", "hu": "következő", "id": "mengikuti", "it": "seguire", "ja": "ついていく", "kk": "to follow", "km": "to follow", "ko": "뒤따르다, 잇따르다", "ku": "دنبال کردن", "lo": "to follow", "mg": "manaraka", "ms": "ikut,kata akar", "my": "to follow", "pl": "podążać", "ro": "suivre", "ru": "следовать", "sk": "nasledovať", "sl": "slediti", "sv": "följa", "sw": "suivre", "th": "ตาม", "tok": "to follow", "tr": "takip etmek", "uk": "іти за", "ur": "پیروی کرنا", "vi": "làm theo, đi theo", "yo": "to follow", "zh-tw": "跟隨", "zh": "跟随,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 222,
		},
		{
			slug: "vortaro-radiko-semajn", typ: "vocab",
			content: map[string]interface{}{
				"word": "semajn",
				"definition": "week",
				"definitions": map[string]interface{}{"en": "week", "nl": "week (7 dagen)", "de": "Woche", "fr": "semaine", "es": "semana", "pt": "semana", "ar": "أسبوع", "be": "неделя", "ca": "setmana", "cs": "týden", "da": "uge", "el": "εβδομάδα", "fa": "هفته", "frp": "semaine", "ga": "seachtain", "he": "שבוע", "hi": "week", "hr": "tjedan", "hu": "hét", "id": "minggu", "it": "settimana", "ja": "週", "kk": "апта", "km": "week", "ko": "주", "ku": "هفته", "lo": "week", "mg": "herinandro", "ms": "minggu,kata akar", "my": "week", "pl": "tydzień", "ro": "semaine", "ru": "неделя", "sk": "týždeň", "sl": "teden", "sv": "vecka", "sw": "semaine", "th": "สัปดาห์", "tok": "week", "tr": "hafta", "uk": "тиждень", "ur": "ہفتہ", "vi": "tuần lễ", "yo": "week", "zh-tw": "星期", "zh": "一周,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 223,
		},
		{
			slug: "vortaro-radiko-send", typ: "vocab",
			content: map[string]interface{}{
				"word": "send",
				"definition": "to send",
				"definitions": map[string]interface{}{"en": "to send", "nl": "zenden", "de": "schicken", "fr": "envoyer", "es": "enviar", "pt": "enviar", "ar": "إرسال", "be": "отправлять", "ca": "enviar", "cs": "poslat", "da": "sende", "el": "το να στέλνω", "fa": "فرستادن", "frp": "envoyer", "ga": "seol", "he": "לשלוח", "hi": "to send", "hr": "poslati", "hu": "küld", "id": "kirim", "it": "spedire, mandare", "ja": "送る", "kk": "to send", "km": "to send", "ko": "보내다", "ku": "فرستادن", "lo": "to send", "mg": "maniraka ,mandefa", "ms": "hantar,kata akar", "my": "to send", "pl": "wysyłać", "ro": "envoyer", "ru": "отправлять", "sk": "poslať", "sl": "poslati", "sv": "sända, skicka", "sw": "envoyer", "th": "ส่ง", "tok": "to send", "tr": "iletmek", "uk": "посилати", "ur": "بھیجنا", "vi": "gửi đến", "yo": "to send", "zh-tw": "送, 寄", "zh": "送，发送,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 224,
		},
		{
			slug: "vortaro-radiko-sercx", typ: "vocab",
			content: map[string]interface{}{
				"word": "serĉ",
				"definition": "to search",
				"definitions": map[string]interface{}{"en": "to search", "nl": "zoeken", "de": "suchen", "fr": "chercher", "es": "buscar", "pt": "procurar", "ar": "بحث", "be": "искать", "ca": "buscar, cercar", "cs": "hledat", "da": "søge", "el": "το να ψάχνω", "fa": "جُستن, دنبال چیزی یا کسی گشتن, جست‌وجو کردن", "frp": "chercher", "ga": "cuardaigh", "he": "לחפש", "hi": "to search", "hr": "tražiti", "hu": "keres", "id": "cari", "it": "cercare", "ja": "探す", "kk": "to search", "km": "to search", "ko": "찾다", "ku": "جُستن, دنبال چیزی یا کسی گشتن, جست‌وجو کردن", "lo": "to search", "mg": "mitady", "ms": "cari,kata akar", "my": "to search", "pl": "szukać", "ro": "chercher", "ru": "искать", "sk": "hľadať", "sl": "iskati", "sv": "söka, leta", "sw": "chercher", "th": "ค้นหา", "tok": "to search", "tr": "aramak", "uk": "шукати", "ur": "تلاش کرنا", "vi": "tìm kiếm", "yo": "to search", "zh-tw": "尋找", "zh": "寻找,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 225,
		},
		{
			slug: "vortaro-radiko-si", typ: "vocab",
			content: map[string]interface{}{
				"word": "si",
				"definition": "reflexive pronoun",
				"definitions": map[string]interface{}{"en": "reflexive pronoun", "nl": "zich", "de": "sein/ihr", "fr": "pronom réfléchi", "es": "pronombre reflexivo", "pt": "se (reflexivo)", "ar": "ضمير متعكس", "be": "себе, себя, свой", "ca": "pronom reflexiu", "cs": "zvratné zájmeno", "da": "sig", "el": "(αυτοπαθής αντωνυμία)", "fa": "خود (سوم‌شخص), خودش, خودشان", "frp": "pronom réfléchi", "ga": "forainm aisfhillteach", "he": "את עצמו", "hi": "reflexive pronoun", "hr": "(refleksivna zamjenica)", "hu": "visszaható névmás", "id": "(kata ganti refleksif)", "it": "(pronome riflessivo)", "ja": "三人称再帰代名詞", "kk": "reflexive pronoun", "km": "reflexive pronoun", "ko": "자기자신", "ku": "خود (سوم‌شخص), خودش, خودشان", "lo": "reflexive pronoun", "mg": "mpisolo toerana", "ms": "kata ganti refleksif,kata akar", "my": "reflexive pronoun", "pl": "siebie, się", "ro": "pronom réfléchi", "ru": "себе, себя, свой", "sk": "zvratné zámeno", "sl": "povratni zaimek", "sv": "sig", "sw": "pronom réfléchi", "th": "เขาเอง", "tok": "reflexive pronoun", "tr": "dönüşlü zamir (kendi, kendisini)", "uk": "себе", "ur": "reflexive pronoun", "vi": "đại từ phản thân", "yo": "reflexive pronoun", "zh-tw": "反身代詞", "zh": "反身代词,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 226,
		},
		{
			slug: "vortaro-radiko-sid", typ: "vocab",
			content: map[string]interface{}{
				"word": "sid",
				"definition": "to sit",
				"definitions": map[string]interface{}{"en": "to sit", "nl": "zitten", "de": "sitzen", "fr": "être assis", "es": "estar sentado", "pt": "sentar, estar sentado", "ar": "جلس", "be": "сидеть", "ca": "estar assegut, seure", "cs": "sedět", "da": "sidde", "el": "το να κάθομαι", "fa": "نشسته بودن, نشستن", "frp": "assoir", "ga": "suigh", "he": "לשבת", "hi": "to sit", "hr": "sjediti", "hu": "ül", "id": "duduk", "it": "sedersi", "ja": "座っている", "kk": "to sit", "km": "to sit", "ko": "앉다", "ku": "نشسته بودن, نشستن", "lo": "to sit", "mg": "mametraka", "ms": "duduk,kata akar", "my": "to sit", "pl": "siedzieć", "ro": "asseoir", "ru": "сидеть", "sk": "sedieť", "sl": "sedeti", "sv": "sitta", "sw": "asseoir", "th": "นั่ง", "tok": "to sit", "tr": "oturmak", "uk": "сидіти", "ur": "بیٹھنا", "vi": "ngồi", "yo": "to sit", "zh-tw": "坐", "zh": "坐,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 227,
		},
		{
			slug: "vortaro-radiko-silent", typ: "vocab",
			content: map[string]interface{}{
				"word": "silent",
				"definition": "silent",
				"definitions": map[string]interface{}{"en": "silent", "nl": "zwijgen", "de": "Schweigen", "fr": "silence", "es": "callar", "pt": "silencioso, silenciosa, fazer silêncio", "ar": "الصمت", "be": "тихий", "ca": "callar, estar en silenci", "cs": "tiše", "da": "stille", "el": "το να σιωπώ", "fa": "سکوت", "frp": "silence", "ga": "ciúin", "he": "שקט", "hi": "silent", "hr": "tih", "hu": "csend", "id": "senyap, diam", "it": "silenzioso", "ja": "声を出さない", "kk": "silent", "km": "silent", "ko": "조용한, 침묵의", "ku": "سکوت", "lo": "silent", "mg": "fanginana", "ms": "senyap,kata akar", "my": "silent", "pl": "milczeć", "ro": "silence", "ru": "тихий", "sk": "ticho", "sl": "molčati", "sv": "tyst", "sw": "silence", "th": "เงียบ", "tok": "silent", "tr": "sessiz", "uk": "мовчати", "ur": "خاموش رہنا", "vi": "tĩnh lặng", "yo": "silent", "zh-tw": "靜", "zh": "静,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 228,
		},
		{
			slug: "vortaro-radiko-simil", typ: "vocab",
			content: map[string]interface{}{
				"word": "simil",
				"definition": "similar",
				"definitions": map[string]interface{}{"en": "similar", "nl": "gelijkaardig, gelijkend", "de": "ähnlich", "fr": "identique", "es": "similar", "pt": "semelhante", "ar": "مطابق", "be": "похожий", "ca": "similar, semblant", "cs": "podobný", "da": "lignende", "el": "όμοιος-α-ο", "fa": "شبیه", "frp": "identique", "ga": "cosúil", "he": "דומה", "hi": "similar", "hr": "sličan", "hu": "hasonló", "id": "mirip", "it": "simile", "ja": "類似の", "kk": "similar", "km": "similar", "ko": "비슷한", "ku": "شبیه", "lo": "similar", "mg": "mitovy", "ms": "sama,kata akar", "my": "similar", "pl": "podobny", "ro": "identique", "ru": "похожий", "sk": "podobný", "sl": "podobno", "sv": "lik, liknande", "sw": "identique", "th": "คล้าย", "tok": "similar", "tr": "benzer", "uk": "схожий", "ur": "similar", "vi": "tương đồng", "yo": "similar", "zh-tw": "相似", "zh": "相似，相同,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 229,
		},
		{
			slug: "vortaro-radiko-sinjor", typ: "vocab",
			content: map[string]interface{}{
				"word": "sinjor",
				"definition": "gentleman, Mr.",
				"definitions": map[string]interface{}{"en": "gentleman, Mr.", "nl": "heer, mijnheer", "de": "Herr", "fr": "monsieur, M.", "es": "señor", "pt": "senhor", "ar": "سيد", "be": "господин, сеньор", "ca": "senyor", "cs": "gentleman, Pan.", "da": "herre, Hr", "el": "κύριος", "fa": "آقا", "frp": "monsieur, M.", "ga": "duine uasal, An tUas.", "he": "מר, אדון", "hi": "gentleman, Mr.", "hr": "gospodin", "hu": "úr", "id": "tuan, Tn.", "it": "signore", "ja": "紳士, さん", "kk": "мырза", "km": "gentleman, Mr.", "ko": "신사, 미스터", "ku": "آقا", "lo": "gentleman, Mr.", "mg": "-andriamatoa - Ra.", "ms": "tuan,kata akar, Encik,kata akar", "my": "gentleman, Mr.", "pl": "pan, P.", "ro": "monsieur, M.", "ru": "господин, сеньор", "sk": "pán", "sl": "gospod", "sv": "herre, herr", "sw": "monsieur, M.", "th": "สุภาพบุรุษ, นาย", "tok": "gentleman, Mr.", "tr": "beyefendi, sayın", "uk": "пан", "ur": "عزیز, جناب", "vi": "quý ngài, quý ngài", "yo": "gentleman, Mr.", "zh-tw": "紳士, 先生", "zh": "绅士,词根, 先生,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 230,
		},
		{
			slug: "vortaro-radiko-situaci", typ: "vocab",
			content: map[string]interface{}{
				"word": "situaci",
				"definition": "situation",
				"definitions": map[string]interface{}{"en": "situation", "nl": "toestand, situatie", "de": "Lage", "fr": "situation", "es": "situación", "pt": "situação", "ar": "حالة", "be": "положение, ситуация", "ca": "situació", "cs": "situace", "da": "situation", "el": "κατάσταση", "fa": "وضعیت", "frp": "situation", "ga": "staid", "he": "מצב", "hi": "situation", "hr": "situacija", "hu": "szituáció", "id": "situasi", "it": "situazione", "ja": "情勢, 立場", "kk": "sitation", "km": "sitation", "ko": "상황", "ku": "وضعیت", "lo": "sitation", "mg": "toerana , fitoerana", "ms": "keadaan,kata akar", "my": "sitation", "pl": "sytuacja", "ro": "situation", "ru": "положение, ситуация", "sk": "situácia", "sl": "situacija", "sv": "situation", "sw": "situation", "th": "สถานการณ์", "tok": "situation", "tr": "durum", "uk": "ситуація", "ur": "sitation", "vi": "hoàn cảnh", "yo": "situation", "zh-tw": "情況", "zh": "处境,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 231,
		},
		{
			slug: "vortaro-radiko-skrib", typ: "vocab",
			content: map[string]interface{}{
				"word": "skrib",
				"definition": "to write",
				"definitions": map[string]interface{}{"en": "to write", "nl": "schrijven", "de": "schreiben", "fr": "écrire", "es": "escribir", "pt": "escrever", "ar": "يكتب", "be": "писать", "ca": "escriure", "cs": "psát", "da": "skrive", "el": "το να γράφω", "fa": "نوشتن", "frp": "écrire", "ga": "scríobh", "he": "לכתוב", "hi": "to write", "hr": "pisati", "hu": "ír", "id": "tulis", "it": "scrivere", "ja": "書く", "kk": "to write", "km": "to write", "ko": "쓰다", "ku": "نوشتن", "lo": "to write", "mg": "manoratra", "ms": "tulis,kata akar", "my": "to write", "pl": "pisać", "ro": "écrire", "ru": "писать", "sk": "písať", "sl": "pisati", "sv": "skriva", "sw": "écrire", "th": "เขียน", "tok": "to write", "tr": "yazmak", "uk": "писати", "ur": "لکھنا", "vi": "viết", "yo": "to write", "zh-tw": "寫", "zh": "写,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 232,
		},
		{
			slug: "vortaro-radiko-sol", typ: "vocab",
			content: map[string]interface{}{
				"word": "sol",
				"definition": "alone",
				"definitions": map[string]interface{}{"en": "alone", "nl": "alleen", "de": "allein", "fr": "seul", "es": "solo/a", "pt": "sozinho, sozinha", "ar": "وحيد", "be": "одинокий", "ca": "sol/a, únic/a", "cs": "sám", "da": "alene", "el": "μόνος-η-ο", "fa": "تنها", "frp": "seul", "ga": "i d'aonar", "he": "לבד", "hi": "alone", "hr": "sam", "hu": "egyedül", "id": "sendiri", "it": "solo", "ja": "唯一の", "kk": "alone", "km": "alone", "ko": "홀로", "ku": "تنها", "lo": "alone", "mg": "irery , tokana", "ms": "bersendirian,kata akar", "my": "alone", "pl": "sam", "ro": "seul", "ru": "одинокий", "sk": "sám", "sl": "sam", "sv": "ensam", "sw": "seul", "th": "เดี่ยว, โดดเดี่ยว", "tok": "alone", "tr": "yalnız", "uk": "єдиний, сам, самотній", "ur": "اکیلا", "vi": "một mình", "yo": "alone", "zh-tw": "孤單", "zh": "孤单,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 233,
		},
		{
			slug: "vortaro-radiko-son", typ: "vocab",
			content: map[string]interface{}{
				"word": "son",
				"definition": "to sound",
				"definitions": map[string]interface{}{"en": "to sound", "nl": "klank", "de": "Klang", "fr": "son", "es": "sonar", "pt": "soar, som", "ar": "صوت", "be": "звук", "ca": "sonar", "cs": "znít", "da": "lyde", "el": "το να ηχώ", "fa": "صدا", "frp": "son", "ga": "fuaim", "he": "קול", "hi": "to sound", "hr": "zvuk", "hu": "hang", "id": "bunyi", "it": "suonare", "ja": "音", "kk": "to sound", "km": "to sound", "ko": "소리", "ku": "صدا", "lo": "to sound", "mg": "feo , faneno", "ms": "bunyi,kata akar", "my": "to sound", "pl": "brzmieć", "ro": "son", "ru": "звук", "sk": "znieť", "sl": "zveneti", "sv": "ljud", "sw": "son", "th": "ส่งเสียง", "tok": "to sound", "tr": "ses çıkarmak?", "uk": "звук", "ur": "to sound", "vi": "lên tiếng", "yo": "to sound", "zh-tw": "聲音", "zh": "声音,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 234,
		},
		{
			slug: "vortaro-radiko-sovagx", typ: "vocab",
			content: map[string]interface{}{
				"word": "sovaĝ",
				"definition": "wild",
				"definitions": map[string]interface{}{"en": "wild", "nl": "wild", "de": "wild", "fr": "sauvage", "es": "salvaje", "pt": "selvagem", "ar": "بري", "be": "дикий", "ca": "salvatge", "cs": "divoký", "da": "vild", "el": "άγριος-α-ο", "fa": "وحشی", "frp": "sauvage", "ga": "fiáin", "he": "פראי", "hi": "wild", "hr": "divlji", "hu": "vad", "id": "liar", "it": "selvaggio, feroce", "ja": "野生の", "kk": "wild", "km": "wild", "ko": "야생의", "ku": "وحشی", "lo": "wild", "mg": "dia", "ms": "liar,kata akar", "my": "wild", "pl": "dziki", "ro": "sauvage", "ru": "дикий", "sk": "divoký", "sl": "divji", "sv": "vild", "sw": "sauvage", "th": "ป่าเถื่อน", "tok": "wild", "tr": "vahşi, yabani", "uk": "дикий", "ur": "wild", "vi": "hoang dã", "yo": "wild", "zh-tw": "野蠻, 野生", "zh": "野外,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 235,
		},
		{
			slug: "vortaro-radiko-sport", typ: "vocab",
			content: map[string]interface{}{
				"word": "sport",
				"definition": "sport",
				"definitions": map[string]interface{}{"en": "sport", "nl": "sport", "de": "Sport", "fr": "sport", "es": "deporte", "pt": "esporte", "ar": "رياضة", "be": "спорт", "ca": "esport", "cs": "sport", "da": "sport", "el": "σπορ", "fa": "ورزش", "frp": "sport", "ga": "spórt", "he": "ספורט", "hi": "sport", "hr": "sport", "hu": "sport", "id": "olahraga", "it": "sport", "ja": "スポーツ", "kk": "sport", "km": "sport", "ko": "스포츠", "ku": "ورزش", "lo": "sport", "mg": "fanatanjahan-tena", "ms": "sukan,kata akar", "my": "sport", "pl": "sport", "ro": "sport", "ru": "спорт", "sk": "šport", "sl": "šport", "sv": "sport", "sw": "sport", "th": "กีฬา", "tok": "sport", "tr": "spor", "uk": "спорт", "ur": "کھیل", "vi": "thể thao", "yo": "sport", "zh-tw": "運動", "zh": "运动,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 236,
		},
		{
			slug: "vortaro-radiko-star", typ: "vocab",
			content: map[string]interface{}{
				"word": "star",
				"definition": "to stand",
				"definitions": map[string]interface{}{"en": "to stand", "nl": "staan", "de": "stehen", "fr": "être debout", "es": "permanecer de pie", "pt": "estar de pé", "ar": "قائم", "be": "стоять", "ca": "estar dret, dempeus", "cs": "stát", "da": "stå", "el": "το να στέκομαι", "fa": "ایستادن", "frp": "être debout", "ga": "seas", "he": "לעמוד", "hi": "to stand", "hr": "stajati", "hu": "áll", "id": "berdiri", "it": "stare in piedi", "ja": "立っている", "kk": "to stand", "km": "to stand", "ko": "일어서다", "ku": "ایستادن", "lo": "to stand", "mg": "mijoro", "ms": "berdiri,kata akar", "my": "to stand", "pl": "stać", "ro": "être debout", "ru": "стоять", "sk": "stáť", "sl": "stati", "sv": "stå", "sw": "être debout", "th": "ยืน", "tok": "to stand", "tr": "ayakta durmak, dikilmek", "uk": "стояти", "ur": "کھڑے ہونا", "vi": "đứng", "yo": "to stand", "zh-tw": "站立", "zh": "站立,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 237,
		},
		{
			slug: "vortaro-radiko-strat", typ: "vocab",
			content: map[string]interface{}{
				"word": "strat",
				"definition": "street",
				"definitions": map[string]interface{}{"en": "street", "nl": "straat", "de": "Straße", "fr": "rue", "es": "calle", "pt": "rua", "ar": "شارع", "be": "улица", "ca": "carrer", "cs": "ulice", "da": "gade", "el": "δρόμος", "fa": "خیابان", "frp": "rue", "ga": "sráid", "he": "רחוב", "hi": "street", "hr": "ulica", "hu": "utca", "id": "jalan", "it": "strada", "ja": "街路", "kk": "street", "km": "តាមផ្លូវ", "ko": "거리", "ku": "خیابان", "lo": "street", "mg": "arabe", "ms": "jalan,kata akar", "my": "street", "pl": "ulica", "ro": "rue", "ru": "улица", "sk": "ulica", "sl": "cesta", "sv": "gata", "sw": "rue", "th": "ถนน", "tok": "street", "tr": "sokak", "uk": "вулиця", "ur": "گلی", "vi": "đường phố", "yo": "street", "zh-tw": "街道", "zh": "街道,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 238,
		},
		{
			slug: "vortaro-radiko-subit", typ: "vocab",
			content: map[string]interface{}{
				"word": "subit",
				"definition": "suddenly",
				"definitions": map[string]interface{}{"en": "suddenly", "nl": "plots, eensklaps, ineens, plotseling", "de": "plötzlich", "fr": "subitement", "es": "súbito", "pt": "de repente", "ar": "فجأة", "be": "внезапно, неожиданно", "ca": "sobtat, de sobte", "cs": "náhle", "da": "pludselig", "el": "ξαφνικά", "fa": "ناگهانی", "frp": "subitement", "ga": "go tobann", "he": "פתאום", "hi": "suddenly", "hr": "iznenada", "hu": "hirtelen", "id": "tiba-tiba", "it": "improvviso", "ja": "突然", "kk": "suddenly", "km": "suddenly", "ko": "갑자기", "ku": "ناگهانی", "lo": "suddenly", "mg": "tampoka", "ms": "tiba-tiba,kata akar", "my": "suddenly", "pl": "nagle", "ro": "subitement", "ru": "внезапно, неожиданно", "sk": "náhly", "sl": "nenadoma", "sv": "plötslig", "sw": "subitement", "th": "ในทันใด", "tok": "suddenly", "tr": "aniden", "uk": "раптовий", "ur": "اچانک", "vi": "đột ngột, đột nhiên", "yo": "suddenly", "zh-tw": "突然", "zh": "突然间,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 239,
		},
		{
			slug: "vortaro-radiko-suficx", typ: "vocab",
			content: map[string]interface{}{
				"word": "sufiĉ",
				"definition": "sufficient",
				"definitions": map[string]interface{}{"en": "sufficient", "nl": "volstaan, genoeg", "de": "genügend", "fr": "suffisant", "es": "suficiente, bastantes", "pt": "suficiente", "ar": "كاف", "be": "достаточно", "ca": "suficient, bastant, prou", "cs": "dostatečný", "da": "nok", "el": "αρκετό-ή-ό", "fa": "کافی", "frp": "suffisant", "ga": "leor", "he": "מספיק", "hi": "sufficient", "hr": "dovoljno", "hu": "elég", "id": "cukup", "it": "sufficiente", "ja": "十分な", "kk": "sufficient", "km": "sufficient", "ko": "충분한", "ku": "کافی", "lo": "sufficient", "mg": "ampy", "ms": "cukup,kata akar", "my": "sufficient", "pl": "wystarczająco", "ro": "suffisant", "ru": "достаточно", "sk": "dostatočný", "sl": "dovolj", "sv": "tillräcklig", "sw": "suffisant", "th": "พอ, เพียงพอ", "tok": "sufficient", "tr": "yeterli", "uk": "достатній", "ur": "کافی", "vi": "đủ", "yo": "sufficient", "zh-tw": "充足", "zh": "充足,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 240,
		},
		{
			slug: "vortaro-radiko-sukces", typ: "vocab",
			content: map[string]interface{}{
				"word": "sukces",
				"definition": "success",
				"definitions": map[string]interface{}{"en": "success", "nl": "slagen", "de": "Erfolg haben", "fr": "succès", "es": "tener éxito", "pt": "sucesso, conseguir", "ar": "نجاح", "be": "успех, удача", "ca": "tenir èxit", "cs": "úspěch", "da": "succes", "el": "επιτυχία", "fa": "موفقیت", "frp": "succès", "ga": "éirigh le", "he": "הצלחה", "hi": "success", "hr": "uspjeh", "hu": "siker", "id": "sukses", "it": "successo", "ja": "成功する", "kk": "табыс", "km": "success", "ko": "성공", "ku": "موفقیت", "lo": "success", "mg": "fahombiasana", "ms": "berjaya,kata akar", "my": "success", "pl": "sukces", "ro": "succès", "ru": "успех, удача", "sk": "úspech", "sl": "uspeh", "sv": "lyckas", "sw": "succès", "th": "สำเร็จ", "tok": "success", "tr": "başarı", "uk": "успіх", "ur": "کامیابی", "vi": "thành công", "yo": "success", "zh-tw": "成功", "zh": "成功,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 241,
		},
		{
			slug: "vortaro-radiko-sun", typ: "vocab",
			content: map[string]interface{}{
				"word": "sun",
				"definition": "sun",
				"definitions": map[string]interface{}{"en": "sun", "nl": "zon", "de": "Sonne", "fr": "soleil", "es": "sol", "pt": "sol", "ar": "شمس", "be": "солнце", "ca": "sol (astre)", "cs": "slunce", "da": "sol", "el": "ήλιος", "fa": "خورشید", "frp": "soleil", "ga": "grian", "he": "שמש", "hi": "sun", "hr": "sunce", "hu": "nap", "id": "matahari", "it": "sole", "ja": "太陽", "kk": "күн", "km": "ព្រះអាទិត្យ", "ko": "태양", "ku": "خورشید", "lo": "sun", "mg": "masoandro", "ms": "matahari,kata akar", "my": "sun", "pl": "słońce", "ro": "soleil", "ru": "солнце", "sk": "slnko", "sl": "sonce", "sv": "sol", "sw": "soleil", "th": "ดวงอาทิตย์", "tok": "sun", "tr": "güneş", "uk": "сонце", "ur": "سورج", "vi": "mặt trời", "yo": "sun", "zh-tw": "太陽", "zh": "太阳,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 242,
		},
		{
			slug: "vortaro-radiko-supr", typ: "vocab",
			content: map[string]interface{}{
				"word": "supr",
				"definition": "above",
				"definitions": map[string]interface{}{"en": "above", "nl": "boven", "de": "oben", "fr": "au dessus", "es": "arriba", "pt": "acima", "ar": "فوق", "be": "наверху", "ca": "(a) dalt", "cs": "nad", "da": "over", "el": "πάνω από", "fa": "بالا بودن", "frp": "au dessus", "ga": "os cionn", "he": "מעל", "hi": "above", "hr": "gornji, gore", "hu": "felső", "id": "atas", "it": "sopra", "ja": "上の", "kk": "above", "km": "above", "ko": "위의", "ku": "بالا بودن", "lo": "above", "mg": "ambony", "ms": "di atas ,kata akar", "my": "above", "pl": "nad", "ro": "au dessus", "ru": "наверху", "sk": "horný", "sl": "zgoraj", "sv": "ovanför", "sw": "au dessus", "th": "ด้านบน", "tok": "above", "tr": "yukarı", "uk": "верхній", "ur": "اوپر", "vi": "ở trên", "yo": "above", "zh-tw": "上面", "zh": "上面,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 243,
		},
		{
			slug: "vortaro-radiko-tabl", typ: "vocab",
			content: map[string]interface{}{
				"word": "tabl",
				"definition": "table",
				"definitions": map[string]interface{}{"en": "table", "nl": "tafel", "de": "Tisch", "fr": "table", "es": "mesa", "pt": "mesa", "ar": "طاولة", "be": "стол", "ca": "taula", "cs": "stůl", "da": "bord", "el": "τραπέζι", "fa": "میز", "frp": "table", "ga": "bord", "he": "שולחן", "hi": "table", "hr": "stol", "hu": "asztal", "id": "meja", "it": "tavolo", "ja": "テーブル", "kk": "table", "km": "តារាង", "ko": "테이블, 탁자", "ku": "میز", "lo": "table", "mg": "latabatra", "ms": "meja,kata akar", "my": "table", "pl": "stół", "ro": "table", "ru": "стол", "sk": "stôl", "sl": "miza", "sv": "bord", "sw": "table", "th": "โต๊ะ", "tok": "table", "tr": "masa", "uk": "стіл", "ur": "میز", "vi": "cái bàn", "yo": "table", "zh-tw": "桌子", "zh": "桌子,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 244,
		},
		{
			slug: "vortaro-radiko-tag", typ: "vocab",
			content: map[string]interface{}{
				"word": "tag",
				"definition": "day",
				"definitions": map[string]interface{}{"en": "day", "nl": "dag", "de": "Tag", "fr": "jour", "es": "día", "pt": "dia", "ar": "يوم", "be": "день", "ca": "dia", "cs": "den", "da": "dag", "el": "ημέρα", "fa": "روز", "frp": "jour", "ga": "lá", "he": "יום", "hi": "day", "hr": "dan", "hu": "nap", "id": "hari", "it": "giorno", "ja": "日", "kk": "күн", "km": "day", "ko": "날", "ku": "روز", "lo": "day", "mg": "andro", "ms": "hari,kata akar", "my": "day", "pl": "dzień", "ro": "jour", "ru": "день", "sk": "deň", "sl": "dan", "sv": "dag", "sw": "jour", "th": "วัน", "tok": "day", "tr": "gün", "uk": "день", "ur": "دن", "vi": "ngày", "yo": "day", "zh-tw": "天", "zh": "天,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 245,
		},
		{
			slug: "vortaro-radiko-te", typ: "vocab",
			content: map[string]interface{}{
				"word": "te",
				"definition": "tea",
				"definitions": map[string]interface{}{"en": "tea", "nl": "thee", "de": "Tee", "fr": "thé", "es": "té", "pt": "chá", "ar": "شاي", "be": "чай", "ca": "te (planta)", "cs": "čaj", "da": "te", "el": "τσάι", "fa": "چای", "frp": "thé", "ga": "tae", "he": "תה", "hi": "tea", "hr": "čaj", "hu": "tea", "id": "teh", "it": "tè", "ja": "茶", "kk": "шай", "km": "តែ", "ko": "차", "ku": "چای", "lo": "tea", "mg": "dité", "ms": "teh,kata akar", "my": "tea", "pl": "herbata", "ro": "thé", "ru": "чай", "sk": "čaj", "sl": "čaj", "sv": "te", "sw": "thé", "th": "ชา", "tok": "tea", "tr": "çay", "uk": "чай", "ur": "چائے", "vi": "trà", "yo": "tea", "zh-tw": "茶", "zh": "茶,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 246,
		},
		{
			slug: "vortaro-radiko-telefon", typ: "vocab",
			content: map[string]interface{}{
				"word": "telefon",
				"definition": "phone",
				"definitions": map[string]interface{}{"en": "phone", "nl": "telefoon", "de": "Telefon", "fr": "téléphone", "es": "teléfono", "pt": "telefone, telefonar", "ar": "هاتف", "be": "телефон", "ca": "telèfon", "cs": "telefon", "da": "telefon", "el": "τηλέφωνο", "fa": "تلفن", "frp": "téléphone", "ga": "fón", "he": "טלפון", "hi": "phone", "hr": "telefon", "hu": "telefon", "id": "telepon", "it": "telefono", "ja": "電話", "kk": "phone", "km": "ទូរស័ព្ទ", "ko": "전화", "ku": "تلفن", "lo": "phone", "mg": "telefaonina", "ms": "telefon,kata akar", "my": "phone", "pl": "telefon", "ro": "téléphone", "ru": "телефон", "sk": "telefón", "sl": "telefon", "sv": "telefon", "sw": "téléphone", "th": "โทรศัพท์", "tok": "phone", "tr": "telefon", "uk": "телефон", "ur": "فون", "vi": "điện thoại", "yo": "phone", "zh-tw": "電話", "zh": "电话,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 247,
		},
		{
			slug: "vortaro-radiko-temp", typ: "vocab",
			content: map[string]interface{}{
				"word": "temp",
				"definition": "time",
				"definitions": map[string]interface{}{"en": "time", "nl": "tijd", "de": "Zeit", "fr": "temps", "es": "tiempo", "pt": "tempo", "ar": "وقت", "be": "время", "ca": "temps", "cs": "čas", "da": "tid", "el": "χρόνος", "fa": "زمان", "frp": "temps", "ga": "am", "he": "זמן", "hi": "time", "hr": "vrijeme", "hu": "idő", "id": "waktu", "it": "tempo", "ja": "時間", "kk": "уақыт", "km": "time", "ko": "시간", "ku": "زمان", "lo": "time", "mg": "andro", "ms": "masa,kata akar", "my": "time", "pl": "czas", "ro": "temps", "ru": "время", "sk": "čas", "sl": "čas", "sv": "tid", "sw": "temps", "th": "เวลา", "tok": "time", "tr": "zaman", "uk": "час", "ur": "وقت", "vi": "thời gian", "yo": "time", "zh-tw": "時間", "zh": "时间,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 248,
		},
		{
			slug: "vortaro-radiko-temperatur", typ: "vocab",
			content: map[string]interface{}{
				"word": "temperatur",
				"definition": "temperature",
				"definitions": map[string]interface{}{"en": "temperature", "nl": "temperatuur", "de": "Temperatur", "fr": "température", "es": "temperatura", "pt": "temperatura", "ar": "درجة الحرارة", "be": "температура", "ca": "temperatura", "cs": "teplota", "da": "temperatur", "el": "θερμοκρασία", "fa": "تب, دما", "frp": "température", "ga": "teocht", "he": "טמפרטורה", "hi": "temperature", "hr": "temperatura", "hu": "hőmérséklet", "id": "temperatur", "it": "temperatura", "ja": "温度", "kk": "temperature", "km": "temperature", "ko": "온도", "ku": "تب, دما", "lo": "temperature", "mg": "toetrandro , hafanana", "ms": "suhu,kata akar", "my": "temperature", "pl": "temperatura", "ro": "température", "ru": "температура", "sk": "teplota", "sl": "temperatura", "sv": "temperatur", "sw": "température", "th": "อุณหภูมิ", "tok": "temperature", "tr": "sıcaklık", "uk": "температура", "ur": "درجہ حرارت", "vi": "nhiệt độ", "yo": "temperature", "zh-tw": "溫度", "zh": "温度,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 249,
		},
		{
			slug: "vortaro-radiko-ten", typ: "vocab",
			content: map[string]interface{}{
				"word": "ten",
				"definition": "to hold",
				"definitions": map[string]interface{}{"en": "to hold", "nl": "houden", "de": "halten", "fr": "tenir", "es": "sostener", "pt": "segurar", "ar": "عَقدَ", "be": "держать", "ca": "sostenir", "cs": "držet", "da": "hold", "el": "το να κρατώ", "fa": "نگه داشتن", "frp": "tenir", "ga": "beir ar, greim", "he": "החזיק", "hi": "hold", "hr": "držati", "hu": "tart", "id": "pegang", "it": "tenere", "ja": "支えている", "kk": "hold", "km": "hold", "ko": "손에 쥐다", "ku": "نگه داشتن", "lo": "hold", "mg": "mitana , mihazona", "ms": "pegang,kata akar", "my": "hold", "pl": "trzymać", "ro": "tenir", "ru": "держать", "sk": "držať", "sl": "držati", "sv": "hålla", "sw": "tenir", "th": "ถือ", "tok": "to hold", "tr": "tutmak", "uk": "тримати", "ur": "پکرنا", "vi": "nắm giữ", "yo": "to hold", "zh-tw": "持", "zh": "拿着,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 250,
		},
		{
			slug: "vortaro-radiko-teror", typ: "vocab",
			content: map[string]interface{}{
				"word": "teror",
				"definition": "terror",
				"definitions": map[string]interface{}{"en": "terror", "nl": "terreur", "de": "Terror", "fr": "terreur", "es": "terror", "pt": "terror", "ar": "إرهاب, رعب", "be": "террор", "ca": "terror", "cs": "teror", "da": "terror", "el": "τρόμος", "fa": "اقدام وحشت‌برانگیز, ترور", "frp": "terreur", "ga": "sceimhle", "he": "אימה", "hi": "terror", "hr": "teror", "hu": "terror", "id": "teror", "it": "terrore", "ja": "テロル", "kk": "terror", "km": "terror", "ko": "테러", "ku": "اقدام وحشت‌برانگیز, ترور", "lo": "terror", "mg": "fahatsiravina ,tahotra mafy", "ms": "keganasan,kata akar", "my": "terror", "pl": "terror", "ro": "terreur", "ru": "террор", "sk": "teror", "sl": "teror", "sv": "terror", "sw": "terreur", "th": "ก่อการร้าย", "tok": "terror", "tr": "terör, terörie etmek", "uk": "терор", "ur": "دہشت", "vi": "kinh hãi, khiếp sợ", "yo": "terror", "zh-tw": "恐怖", "zh": "恐怖,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 251,
		},
		{
			slug: "vortaro-radiko-tim", typ: "vocab",
			content: map[string]interface{}{
				"word": "tim",
				"definition": "to fear",
				"definitions": map[string]interface{}{"en": "to fear", "nl": "vrezen, bang zijn", "de": "fürchten", "fr": "crainte", "es": "temer, tener miedo", "pt": "medo, ter medo", "ar": "خوف", "be": "страх", "ca": "témer, tenir por", "cs": "strach", "da": "frygt", "el": "το να φοβάμαι", "fa": "ترسیدن", "frp": "crainte", "ga": "eagla", "he": "פחד", "hi": "fear", "hr": "bojati se", "hu": "fél", "id": "takut", "it": "paura", "ja": "恐怖", "kk": "fear", "km": "fear", "ko": "두려운", "ku": "ترسیدن", "lo": "fear", "mg": "tahotra", "ms": "takut,kata akar", "my": "fear", "pl": "strach", "ro": "crainte", "ru": "страх", "sk": "strach", "sl": "strah", "sv": "rädsla", "sw": "crainte", "th": "กลัว", "tok": "to fear", "tr": "korku", "uk": "боятись", "ur": "خوف", "vi": "sợ hãi", "yo": "to fear", "zh-tw": "怕", "zh": "害怕,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 252,
		},
		{
			slug: "vortaro-radiko-ton", typ: "vocab",
			content: map[string]interface{}{
				"word": "ton",
				"definition": "tone",
				"definitions": map[string]interface{}{"en": "tone", "nl": "toon", "de": "Ton", "fr": "ton", "es": "nota", "pt": "tom", "ar": "نغمة", "be": "тон", "ca": "to", "cs": "tón", "da": "tone", "el": "ηχητικός τόνος", "fa": "لحن", "frp": "ton", "ga": "tuin", "he": "טון, גוון", "hi": "tone", "hr": "ton", "hu": "hangnem", "id": "nada", "it": "tono", "ja": "音の調子", "kk": "tone", "km": "tone", "ko": "톤, 성조", "ku": "لحن", "lo": "tone", "mg": "feo", "ms": "nada,kata akar", "my": "tone", "pl": "ton", "ro": "ton", "ru": "тон", "sk": "tón", "sl": "ton", "sv": "ton", "sw": "ton", "th": "น้ำเสียง", "tok": "tone", "tr": "ton", "uk": "тон, лад, звук", "ur": "tone", "vi": "giọng", "yo": "tone", "zh-tw": "音調", "zh": "音,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 253,
		},
		{
			slug: "vortaro-radiko-trankvil", typ: "vocab",
			content: map[string]interface{}{
				"word": "trankvil",
				"definition": "calm",
				"definitions": map[string]interface{}{"en": "calm", "nl": "rustig", "de": "ruhig", "fr": "calme", "es": "tranquilo/a", "pt": "calmo, calma", "ar": "هدوء", "be": "спокойный", "ca": "tranquil/tranquil·la", "cs": "klidný", "da": "rolig", "el": "ήσυχος-η-ο", "fa": "آرام, دارای آرام‌وقرار, دارای آرامش", "frp": "calme", "ga": "suaimhneas", "he": "רגוע", "hi": "calm", "hr": "miran", "hu": "nyugodt", "id": "tenang", "it": "tranquillo, calmo", "ja": "安心した, 平穏な", "kk": "calm", "km": "calm", "ko": "고요한", "ku": "آرام, دارای آرام‌وقرار, دارای آرامش", "lo": "calm", "mg": "tony , maotona ; mandry", "ms": "tenang,kata akar", "my": "calm", "pl": "spokojny", "ro": "calme", "ru": "спокойный", "sk": "pokojný", "sl": "miren", "sv": "lugn", "sw": "calme", "th": "ใจเย็น", "tok": "calm", "tr": "sakin", "uk": "спокійний", "ur": "calm", "vi": "bình tĩnh, yên bình, lặng lẽ", "yo": "calm", "zh-tw": "安", "zh": "安静,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 254,
		},
		{
			slug: "vortaro-radiko-trink", typ: "vocab",
			content: map[string]interface{}{
				"word": "trink",
				"definition": "to drink",
				"definitions": map[string]interface{}{"en": "to drink", "nl": "drinken", "de": "trinken", "fr": "boire", "es": "beber", "pt": "beber", "ar": "شراب", "be": "пить", "ca": "beure", "cs": "pít", "da": "drikke", "el": "το να πίνω", "fa": "نوشیدن", "frp": "boire", "ga": "ól", "he": "לשתות", "hi": "to drink", "hr": "piti", "hu": "iszik", "id": "minum", "it": "bere", "ja": "飲む", "kk": "to drink", "km": "to drink", "ko": "마시다", "ku": "نوشیدن", "lo": "to drink", "mg": "misotro", "ms": "minum,kata akar", "my": "to drink", "pl": "pić", "ro": "boire", "ru": "пить", "sk": "piť", "sl": "piti", "sv": "dricka", "sw": "boire", "th": "ดื่ม", "tok": "to drink", "tr": "içmek", "uk": "пити (воду)", "ur": "پینا", "vi": "uống", "yo": "to drink", "zh-tw": "飲, 喝", "zh": "饮，喝,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 255,
		},
		{
			slug: "vortaro-radiko-trov", typ: "vocab",
			content: map[string]interface{}{
				"word": "trov",
				"definition": "to find",
				"definitions": map[string]interface{}{"en": "to find", "nl": "vinden", "de": "finden", "fr": "trouver", "es": "encontrar", "pt": "encontrar", "ar": "يجد", "be": "найти, находить", "ca": "trobar", "cs": "najít", "da": "finde", "el": "το να βρίσκω", "fa": "پیدا کردن", "frp": "trouver", "ga": "faigh", "he": "למצוא", "hi": "to find", "hr": "pronaći", "hu": "talál", "id": "menemukan", "it": "trovare", "ja": "見つける", "kk": "to find", "km": "to find", "ko": "발견하다", "ku": "پیدا کردن", "lo": "to find", "mg": "mahita", "ms": "cari,kata akar", "my": "to find", "pl": "znaleźć", "ro": "trouver", "ru": "найти, находить", "sk": "nájsť", "sl": "najti", "sv": "hitta", "sw": "trouver", "th": "พบ", "tok": "to find", "tr": "bulmak", "uk": "знаходити", "ur": "ڈھونڈنا", "vi": "đi tìm", "yo": "to find", "zh-tw": "找到", "zh": "寻找,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 256,
		},
		{
			slug: "vortaro-radiko-turk", typ: "vocab",
			content: map[string]interface{}{
				"word": "turk",
				"definition": "Turkish",
				"definitions": map[string]interface{}{"en": "Turkish", "nl": "Turk", "de": "Türke", "fr": "turc", "es": "turco/a", "pt": "turco", "ar": "تركي", "be": "турецкий", "ca": "turc/a", "cs": "Turek", "da": "tyrkisk", "el": "τούρκικο", "fa": "ترک", "frp": "turc", "ga": "Turcach", "he": "תורכי", "hi": "Turkish", "hr": "turski", "hu": "török", "id": "Turki", "it": "Turco", "ja": "トルコ人", "kk": "Turkish", "km": "Turkish", "ko": "터키", "ku": "ترک", "lo": "Turkish", "mg": "torka", "ms": "Turki,kata akar", "my": "Turkish", "pl": "turecki", "ro": "turc", "ru": "турецкий", "sk": "Turek", "sl": "turško", "sv": "turkisk", "sw": "turc", "th": "ชาวตุรกี", "tok": "Turkish", "tr": "Türkçe", "uk": "турок", "ur": "ترکی", "vi": "Thổ Nhĩ Kỳ", "yo": "Turkish", "zh-tw": "土耳其", "zh": "土耳其,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 257,
		},
		{
			slug: "vortaro-radiko-tusx", typ: "vocab",
			content: map[string]interface{}{
				"word": "tuŝ",
				"definition": "to touch",
				"definitions": map[string]interface{}{"en": "to touch", "nl": "aanraken", "de": "berühren", "fr": "toucher", "es": "tocar", "pt": "tocar", "ar": "لمس", "be": "трогать", "ca": "tocar", "cs": "dotýkat se", "da": "røre", "el": "το να αγγίζω", "fa": "لمس کردن", "frp": "toucher", "ga": "bain do", "he": "לגעת", "hi": "to touch", "hr": "dirati", "hu": "érint", "id": "memegang", "it": "toccare", "ja": "触れる", "kk": "to touch", "km": "to touch", "ko": "만지다", "ku": "لمس کردن", "lo": "to touch", "mg": "mikasika", "ms": "sentuh,kata akar", "my": "to touch", "pl": "dotykać", "ro": "toucher", "ru": "трогать", "sk": "dotýkať sa", "sl": "dotakniti se", "sv": "röra, vidröra, beröra", "sw": "toucher", "th": "แตะ, สัมผัส", "tok": "to touch", "tr": "dokunmak", "uk": "торкатись", "ur": "چھونا", "vi": "đụng chạm", "yo": "to touch", "zh-tw": "觸摸", "zh": "触摸,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 258,
		},
		{
			slug: "vortaro-radiko-urb", typ: "vocab",
			content: map[string]interface{}{
				"word": "urb",
				"definition": "city",
				"definitions": map[string]interface{}{"en": "city", "nl": "stad", "de": "Stadt", "fr": "ville", "es": "ciudad", "pt": "cidade", "ar": "مدينة", "be": "город", "ca": "ciutat", "cs": "město", "da": "by", "el": "πόλη", "fa": "شهر", "frp": "ville", "ga": "cathair", "he": "עיר", "hi": "city", "hr": "grad", "hu": "város", "id": "kota", "it": "città", "ja": "都市", "kk": "қала", "km": "ទីក្រុង", "ko": "도시", "ku": "شهر", "lo": "city", "mg": "tanàna lehibe", "ms": "bandaraya,kata akar", "my": "city", "pl": "miasto", "ro": "ville", "ru": "город", "sk": "mesto", "sl": "mesto", "sv": "stad", "sw": "ville", "th": "เมือง", "tok": "city", "tr": "şehir", "uk": "місто", "ur": "شہر", "vi": "thành phố", "yo": "city", "zh-tw": "城市", "zh": "城市,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 259,
		},
		{
			slug: "vortaro-radiko-util", typ: "vocab",
			content: map[string]interface{}{
				"word": "util",
				"definition": "useful",
				"definitions": map[string]interface{}{"en": "useful", "nl": "nuttig", "de": "nützlich", "fr": "utile", "es": "útil", "pt": "útil", "ar": "مفيد", "be": "полезный", "ca": "útil", "cs": "užitečný", "da": "brugbar", "el": "χρήσιμος-η-ο", "fa": "کارا, مفید", "frp": "utile", "ga": "úsáideach", "he": "שימושי", "hi": "useful", "hr": "koristan", "hu": "hasznos", "id": "berguna", "it": "utile", "ja": "役に立つ", "kk": "useful", "km": "useful", "ko": "유용한", "ku": "کارا, مفید", "lo": "useful", "mg": "mahasoa", "ms": "kegunaan,kata akar", "my": "useful", "pl": "użyteczny", "ro": "utile", "ru": "полезный", "sk": "užitočný", "sl": "koristno", "sv": "nyttig, användbar", "sw": "utile", "th": "มีประโยชน์", "tok": "useful", "tr": "kullanışlı, faydalı", "uk": "корисний -придатний", "ur": "مفید", "vi": "hữu ích", "yo": "useful", "zh-tw": "用處", "zh": "用处,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 260,
		},
		{
			slug: "vortaro-radiko-uz", typ: "vocab",
			content: map[string]interface{}{
				"word": "uz",
				"definition": "to use",
				"definitions": map[string]interface{}{"en": "to use", "nl": "gebruiken", "de": "gebrauchen", "fr": "utiliser", "es": "usar", "pt": "usar", "ar": "استعمال", "be": "использовать", "ca": "usar, emprar", "cs": "použít", "da": "bruge", "el": "το να χρησιμοποιώ", "fa": "استفاده کردن", "frp": "utiliser", "ga": "úsáid", "he": "להשתמש", "hi": "to use", "hr": "koristiti", "hu": "használ", "id": "pakai, guna", "it": "usare", "ja": "使う", "kk": "to use", "km": "to use", "ko": "사용하다", "ku": "استفاده کردن", "lo": "to use", "mg": "mampiasa , manao zavatra mahasoa", "ms": "guna,kata akar", "my": "to use", "pl": "używać", "ro": "utiliser", "ru": "использовать", "sk": "použiť", "sl": "uporabiti", "sv": "använda", "sw": "utiliser", "th": "ใช้", "tok": "to use", "tr": "kullanmak", "uk": "користуватися, використовувати", "ur": "استعمال کرنا", "vi": "sử dụng", "yo": "to use", "zh-tw": "使用", "zh": "使用,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 261,
		},
		{
			slug: "vortaro-radiko-vagon", typ: "vocab",
			content: map[string]interface{}{
				"word": "vagon",
				"definition": "coach",
				"definitions": map[string]interface{}{"en": "coach", "nl": "wagon", "de": "Waggon", "fr": "wagon", "es": "vagón", "pt": "vagão", "ar": "عربة", "be": "вагон", "ca": "vagó", "cs": "vagon, vůz", "da": "vogn", "el": "βαγόνι", "fa": "واگن", "frp": "wagon", "ga": "cóiste", "he": "קרון", "hi": "coach", "hr": "vagon", "hu": "vagon", "id": "gerbong, kereta", "it": "vagone, carrozza", "ja": "車両", "kk": "coach", "km": "coach", "ko": "차량", "ku": "واگن", "lo": "coach", "mg": "kalesy ny lalam-by", "ms": "gerabak,kata akar", "my": "coach", "pl": "wagon", "ro": "wagon", "ru": "вагон", "sk": "vagón, vozeň", "sl": "vagon", "sv": "järnvägsvagn", "sw": "wagon", "th": "ตู้รถไฟ", "tok": "coach", "tr": "vagon", "uk": "вагон", "ur": "coach", "vi": "xe ngựa bốn bánh, xe đò", "yo": "coach", "zh-tw": "車廂", "zh": "车厢,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 262,
		},
		{
			slug: "vortaro-radiko-varm", typ: "vocab",
			content: map[string]interface{}{
				"word": "varm",
				"definition": "warm",
				"definitions": map[string]interface{}{"en": "warm", "nl": "warm", "de": "warm", "fr": "chaud", "es": "caliente", "pt": "quente", "ar": "حار", "be": "тёплый", "ca": "calent/a", "cs": "teplý", "da": "varm", "el": "ζεστός-ή-ό", "fa": "گرم", "frp": "chaud", "ga": "teolaí", "he": "חם", "hi": "warm", "hr": "toplo", "hu": "meleg", "id": "panas", "it": "caldo", "ja": "暖かい", "kk": "warm", "km": "warm", "ko": "따뜻한, 더운, 뜨거운", "ku": "گرم", "lo": "warm", "mg": "mafana", "ms": "panas,kata akar", "my": "warm", "pl": "ciepły", "ro": "chaud", "ru": "тёплый", "sk": "teplý", "sl": "toplo", "sv": "varm", "sw": "chaud", "th": "อุ่น", "tok": "warm", "tr": "sıcak", "uk": "жаркий", "ur": "گرم", "vi": "nóng", "yo": "warm", "zh-tw": "溫熱", "zh": "热,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 263,
		},
		{
			slug: "vortaro-radiko-ven", typ: "vocab",
			content: map[string]interface{}{
				"word": "ven",
				"definition": "come",
				"definitions": map[string]interface{}{"en": "come", "nl": "komen", "de": "kommen", "fr": "venir", "es": "venir", "pt": "vir", "ar": "جاء", "be": "приходить", "ca": "venir", "cs": "přijít", "da": "komme", "el": "έρχομαι", "fa": "آمدن", "frp": "venir", "ga": "tar", "he": "בא", "hi": "come", "hr": "doći", "hu": "jön", "id": "datang", "it": "venire", "ja": "来る", "kk": "come", "km": "មក", "ko": "오다", "ku": "آمدن", "lo": "come", "mg": "avy , tonga", "ms": "mari,kata akar", "my": "come", "pl": "przyjść", "ro": "venir", "ru": "приходить", "sk": "prísť", "sl": "priti", "sv": "komma", "sw": "venir", "th": "มา", "tok": "come", "tr": "gelmek", "uk": "приходити", "ur": "آنا", "vi": "đi đến", "yo": "come", "zh-tw": "來", "zh": "来,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 264,
		},
		{
			slug: "vortaro-radiko-vend", typ: "vocab",
			content: map[string]interface{}{
				"word": "vend",
				"definition": "to sell",
				"definitions": map[string]interface{}{"en": "to sell", "nl": "verkopen", "de": "verkaufen", "fr": "vendre", "es": "vender", "pt": "vender", "ar": "بيع", "be": "продавать", "ca": "vendre", "cs": "prodat", "da": "sælge", "el": "το να πουλώ", "fa": "فروختن", "frp": "vendre", "ga": "díol", "he": "למכור", "hi": "to sell", "hr": "prodati", "hu": "árul", "id": "jual", "it": "vendere", "ja": "売る", "kk": "to sell", "km": "to sell", "ko": "팔다", "ku": "فروختن", "lo": "to sell", "mg": "mivarotra", "ms": "jual,kata akar", "my": "to sell", "pl": "sprzedawać", "ro": "vendre", "ru": "продавать", "sk": "predať", "sl": "prodati", "sv": "sälja", "sw": "vendre", "th": "ขาย", "tok": "to sell", "tr": "satmak", "uk": "продавати", "ur": "بیچنا", "vi": "buôn bán", "yo": "to sell", "zh-tw": "賣", "zh": "卖,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 265,
		},
		{
			slug: "vortaro-radiko-veneci", typ: "vocab",
			content: map[string]interface{}{
				"word": "Veneci",
				"definition": "Venice",
				"definitions": map[string]interface{}{"en": "Venice", "nl": "Venetië", "de": "Venedig", "fr": "Venise", "es": "Venecia", "ar": "مدينة البندقية", "be": "Венеция", "ca": "Venècia", "cs": "Venécie", "da": "Venedig", "el": "Βενετία", "fa": "ونیز", "frp": "Venise", "ga": "Veinéis", "he": "ונציה", "hi": "Venice", "hr": "Venecija", "hu": "Velence", "id": "Venezia", "it": "Veneiza", "ja": "ヴェネチア", "kk": "Венеция", "km": "Venice", "ko": "베니스", "ku": "ونیز", "lo": "Venice", "mg": "Venise", "ms": "Venice,kata akar", "my": "Venice", "pl": "Wenecja", "ro": "Venise", "ru": "Венеция", "sk": "Benátky", "sl": "Benetke", "sv": "Venedig", "sw": "Venise", "th": "เวนิส", "tok": "Venice", "tr": "Venedik", "uk": "Венеція", "ur": "وینس", "vi": "thành phố Vơ-ni-dơ", "yo": "Venice", "zh-tw": "威尼斯", "zh": "威尼斯,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 266,
		},
		{
			slug: "vortaro-radiko-ver", typ: "vocab",
			content: map[string]interface{}{
				"word": "ver",
				"definition": "true",
				"definitions": map[string]interface{}{"en": "true", "nl": "waar, echt, waarlijk", "de": "wahr", "fr": "vrai", "es": "verdad", "pt": "verdade, verdadeiro, verdadeira", "ar": "حقيقي", "be": "правда, истина", "ca": "veritable, veritat", "cs": "pravda", "da": "sand", "el": "αλήθεια", "fa": "حقیقی", "frp": "vrai", "ga": "fíor", "he": "אמת", "hi": "true", "hr": "istina", "hu": "igaz", "id": "benar, sejati", "it": "vero", "ja": "真実", "kk": "true", "km": "true", "ko": "진실한", "ku": "حقیقی", "lo": "true", "mg": "marina", "ms": "benar,kata akar", "my": "true", "pl": "prawda", "ro": "vrai", "ru": "правда, истина", "sk": "pravda", "sl": "resnica", "sv": "sann, verklig", "sw": "vrai", "th": "จริง", "tok": "true", "tr": "doğru, gerçek", "uk": "правда", "ur": "سچا", "vi": "thật sự, đúng là", "yo": "true", "zh-tw": "真", "zh": "真,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 267,
		},
		{
			slug: "vortaro-radiko-vesper", typ: "vocab",
			content: map[string]interface{}{
				"word": "vesper",
				"definition": "evening",
				"definitions": map[string]interface{}{"en": "evening", "nl": "avond", "de": "Abend", "fr": "soir", "es": "tarde", "pt": "tarde, noite", "ar": "مساء", "be": "вечер", "ca": "vespre", "cs": "večer", "da": "aften", "el": "βράδυ", "fa": "سرشب", "frp": "soir", "ga": "tráthnóna", "he": "ערב", "hi": "evening", "hr": "večer", "hu": "este", "id": "malam", "it": "sera", "ja": "夕方", "kk": "evening", "km": "ល្ងាច", "ko": "저녁", "ku": "سرشب", "lo": "evening", "mg": "hariva", "ms": "senja,kata akar", "my": "evening", "pl": "wieczór", "ro": "soir", "ru": "вечер", "sk": "večer", "sl": "večer", "sv": "kväll", "sw": "soir", "th": "ตอนเย็น", "tok": "evening", "tr": "akşam", "uk": "вечір", "ur": "شام", "vi": "buổi tối", "yo": "evening", "zh-tw": "傍晚, 晚上", "zh": "傍晚、晚上,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 268,
		},
		{
			slug: "vortaro-radiko-vest", typ: "vocab",
			content: map[string]interface{}{
				"word": "vest",
				"definition": "to dress",
				"definitions": map[string]interface{}{"en": "to dress", "nl": "kleden", "de": "kleiden", "fr": "habiller", "es": "vestir", "pt": "vestir, roupa", "ar": "يرتدي", "be": "одежда", "ca": "vestir", "cs": "obléct se", "da": "have på", "el": "το να ντύνω", "fa": "پوشاندن", "frp": "habiller", "ga": "gléas", "he": "בגד, לבוש", "hi": "to dress", "hr": "odjenuti", "hu": "ruha", "id": "berpakaian", "it": "indossare", "ja": "服を着せる", "kk": "to dress", "km": "to dress", "ko": "옷", "ku": "پوشاندن", "lo": "to dress", "mg": "mampiakanjo", "ms": "memakai,kata akar", "my": "to dress", "pl": "ubrać się", "ro": "habiller", "ru": "одежда", "sk": "obliecť, šatiť", "sl": "obleči", "sv": "klä", "sw": "habiller", "th": "สวมเสื้อ", "tok": "to dress", "tr": "giyinmek", "uk": "одяг", "ur": "پہننا", "vi": "ăn mặc", "yo": "to dress", "zh-tw": "穿衣", "zh": "穿 (衣服),词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 269,
		},
		{
			slug: "vortaro-radiko-vetur", typ: "vocab",
			content: map[string]interface{}{
				"word": "vetur",
				"definition": "to ride",
				"definitions": map[string]interface{}{"en": "to ride", "nl": "rijden (auto)", "de": "fahren", "fr": "conduire", "es": "ir (con medio de transporte)", "pt": "viajar, ir de veículo", "ar": "قيادة", "be": "ехать, путешествовать", "ca": "anar (amb vehicle)", "cs": "cestovat", "da": "rejse", "el": "το να εποχούμαι, το να ταξιδεύω με μεταφόρικό μέσο", "fa": "با وسیله‌ی نقلیه پیمودن", "frp": "conduire", "ga": "taisteal", "he": "לנסוע", "hi": "to ride", "hr": "voziti se", "hu": "utazik, közlekedik", "id": "pergi, perjalanan", "it": "viaggiare (con un veicolo)", "ja": "（乗り物で）行く, （乗り物で）旅行をする", "kk": "to travel", "km": "to travel", "ko": "타고 이동하다", "ku": "با وسیله‌ی نقلیه پیمودن", "lo": "to travel", "mg": "mitondra", "ms": "melancong,kata akar", "my": "to travel", "pl": "jechać", "ro": "conduire", "ru": "ехать, путешествовать", "sk": "cestovať", "sl": "potovati", "sv": "åka (med fordon)", "sw": "conduire", "th": "เดินทาง", "tok": "to ride", "tr": "arabayla gitmek", "uk": "їхати", "ur": "سفر کرنا", "vi": "chạy xe", "yo": "to ride", "zh-tw": "乘行, 駕行", "zh": "乘行、行,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 270,
		},
		{
			slug: "vortaro-radiko-vid", typ: "vocab",
			content: map[string]interface{}{
				"word": "vid",
				"definition": "to see",
				"definitions": map[string]interface{}{"en": "to see", "nl": "zien", "de": "sehen", "fr": "voir", "es": "ver", "pt": "ver, vista", "ar": "يري", "be": "видеть", "ca": "veure", "cs": "vidět", "da": "se", "el": "το να βλέπω", "fa": "دیدن", "frp": "voir", "ga": "feic", "he": "לראות", "hi": "to see", "hr": "vidjeti", "hu": "lát", "id": "lihat", "it": "vedere", "ja": "見えている", "kk": "to see", "km": "to see", "ko": "보다", "ku": "دیدن", "lo": "to see", "mg": "mijery , mizaha", "ms": "lihat,kata akar", "my": "to see", "pl": "widzieć", "ro": "voir", "ru": "видеть", "sk": "vidieť", "sl": "videti", "sv": "se", "sw": "voir", "th": "เห็น, พบ", "tok": "to see", "tr": "görmek", "uk": "бачити", "ur": "دیکھنا", "vi": "thấy", "yo": "to see", "zh-tw": "看", "zh": "看,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 271,
		},
		{
			slug: "vortaro-radiko-vin", typ: "vocab",
			content: map[string]interface{}{
				"word": "vin",
				"definition": "wine",
				"definitions": map[string]interface{}{"en": "wine", "nl": "wijn", "de": "Wein", "fr": "vin", "es": "vino", "pt": "vinho", "ar": "خمر", "be": "вино", "ca": "vi", "cs": "víno", "da": "vin", "el": "κρασί", "fa": "شراب", "frp": "vin", "ga": "fíon", "he": "יין", "hi": "wine", "hr": "vino", "hu": "bor", "id": "anggur (minuman)", "it": "vino", "ja": "ワイン", "kk": "шарап", "km": "ស្រា", "ko": "와인", "ku": "شراب", "lo": "wine", "mg": "divay", "ms": "wain,kata akar", "my": "wine", "pl": "wino", "ro": "vin", "ru": "вино", "sk": "víno", "sl": "vino", "sv": "vin", "sw": "vin", "th": "ไวน์", "tok": "wine", "tr": "şarap", "uk": "вино", "ur": "wine", "vi": "rượu vang", "yo": "wine", "zh-tw": "葡萄酒", "zh": "葡萄酒,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 272,
		},
		{
			slug: "vortaro-radiko-vir", typ: "vocab",
			content: map[string]interface{}{
				"word": "vir",
				"definition": "man",
				"definitions": map[string]interface{}{"en": "man", "nl": "man", "de": "Mann", "fr": "homme", "es": "hombre", "pt": "homem, masculino", "ar": "رجل", "be": "мужчина", "ca": "home, mascle", "cs": "muž", "da": "mand", "el": "άντρας", "fa": "مرد", "frp": "homme", "ga": "fear", "he": "גבר", "hi": "man", "hr": "muškarac", "hu": "ember", "id": "pria", "it": "uomo", "ja": "男", "kk": "man", "km": "man", "ko": "남자", "ku": "مرد", "lo": "man", "mg": "olona , lehilahy", "ms": "lelaki,kata akar", "my": "man", "pl": "mężczyzna", "ro": "homme", "ru": "мужчина", "sk": "muž", "sl": "moški", "sv": "man", "sw": "homme", "th": "ผู้ชาย", "tok": "man", "tr": "adam", "uk": "чоловік", "ur": "آدمی", "vi": "đàn ông", "yo": "man", "zh-tw": "男人", "zh": "男人,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 273,
		},
		{
			slug: "vortaro-radiko-viv", typ: "vocab",
			content: map[string]interface{}{
				"word": "viv",
				"definition": "to live",
				"definitions": map[string]interface{}{"en": "to live", "nl": "leven", "de": "leben", "fr": "vivre", "es": "vivir", "pt": "viver, vida", "ar": "يعيش", "be": "жить", "ca": "viure", "cs": "žít", "da": "leve", "el": "το να ζω", "fa": "زنده بودن", "frp": "vivre", "ga": "cónaigh, beo", "he": "לחיות", "hi": "to live", "hr": "živjeti", "hu": "él", "id": "hidup", "it": "vivere", "ja": "生きる", "kk": "to live", "km": "to live", "ko": "살다", "ku": "زنده بودن", "lo": "to live", "mg": "miaina , velona", "ms": "hidup,kata akar", "my": "to live", "pl": "żyć", "ro": "vivre", "ru": "жить", "sk": "život", "sl": "živeti", "sv": "leva", "sw": "vivre", "th": "มีชีวิต", "tok": "to live", "tr": "yaşamak", "uk": "життя", "ur": "جینا", "vi": "sống, tồn tại", "yo": "to live", "zh-tw": "活, 生活", "zh": "活、居住,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 274,
		},
		{
			slug: "vortaro-radiko-vizagx", typ: "vocab",
			content: map[string]interface{}{
				"word": "vizaĝ",
				"definition": "face",
				"definitions": map[string]interface{}{"en": "face", "nl": "(aan)gezicht", "de": "Gesicht", "fr": "visage", "es": "cara", "pt": "cara, face", "ar": "وجه", "be": "лик, лицо", "ca": "cara", "cs": "obličej", "da": "ansigt", "el": "πρόσωπο", "fa": "چهره", "frp": "visage", "ga": "aghaidh", "he": "פנים", "hi": "face", "hr": "lice", "hu": "arc", "id": "wajah", "it": "faccia", "ja": "顔", "kk": "face", "km": "face", "ko": "얼굴", "ku": "چهره", "lo": "face", "mg": "tarehy, tava", "ms": "muka,kata akar", "my": "face", "pl": "twarz", "ro": "visage", "ru": "лик, лицо", "sk": "tvár", "sl": "obraz", "sv": "ansikte", "sw": "visage", "th": "ใบหน้า", "tok": "face", "tr": "yüz", "uk": "обличчя", "ur": "چہرہ", "vi": "khuôn mặt", "yo": "face", "zh-tw": "臉", "zh": "脸,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 275,
		},
		{
			slug: "vortaro-radiko-vizit", typ: "vocab",
			content: map[string]interface{}{
				"word": "vizit",
				"definition": "to visit",
				"definitions": map[string]interface{}{"en": "to visit", "nl": "bezoeken", "de": "besuchen", "fr": "visiter", "es": "visitar", "pt": "visitar, visita", "ar": "زيارة", "be": "посетить", "ca": "visitar", "cs": "navštívit", "da": "besøge", "el": "το να επισκέπτομαι", "fa": "بازدید کردن", "frp": "visiter", "ga": "cuairt", "he": "לבקר", "hi": "to visit", "hr": "posjetiti", "hu": "látogat", "id": "kunjung", "it": "visitare", "ja": "訪れる", "kk": "to visit", "km": "to visit", "ko": "방문하다", "ku": "بازدید کردن", "lo": "to visit", "mg": "mamangy , mitsidika", "ms": "lawat,kata akar", "my": "to visit", "pl": "odwiedzać", "ro": "visiter", "ru": "посетить", "sk": "návšteva", "sl": "obiskati", "sv": "besöka", "sw": "visiter", "th": "มาเยี่ยม, มาเยือน, เยี่ยมชม", "tok": "to visit", "tr": "ziyaret etmek", "uk": "відвідувати", "ur": "زیارت کرنا", "vi": "viếng thăm", "yo": "to visit", "zh-tw": "參觀, 拜訪", "zh": "参观、拜访,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 276,
		},
		{
			slug: "vortaro-radiko-voj", typ: "vocab",
			content: map[string]interface{}{
				"word": "voj",
				"definition": "way",
				"definitions": map[string]interface{}{"en": "way", "nl": "weg, baan", "de": "Weg", "fr": "voie", "es": "camino", "pt": "caminho", "ar": "طريق", "be": "путь", "ca": "camí", "cs": "cesta", "da": "vej", "el": "δρόμος", "fa": "راه, جاده, مسیر", "frp": "voie", "ga": "slí", "he": "דרך", "hi": "way", "hr": "put", "hu": "út", "id": "jalan", "it": "via", "ja": "道", "kk": "жол", "km": "way", "ko": "길", "ku": "راه, جاده, مسیر", "lo": "way", "mg": "làlana , arabe", "ms": "jalan,kata akar", "my": "way", "pl": "droga", "ro": "voie", "ru": "путь", "sk": "cesta", "sl": "pot", "sv": "väg", "sw": "voie", "th": "ทาง", "tok": "way", "tr": "yol", "uk": "дорога", "ur": "راستہ", "vi": "đường lối", "yo": "way", "zh-tw": "路", "zh": "路,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 277,
		},
		{
			slug: "vortaro-radiko-vojagx", typ: "vocab",
			content: map[string]interface{}{
				"word": "vojaĝ",
				"definition": "to travel",
				"definitions": map[string]interface{}{"en": "to travel", "nl": "reizen", "de": "reisen", "fr": "voyage", "es": "viajar", "pt": "viagem, viajar", "ar": "سفر, رحلة", "be": "путешествие", "ca": "viatjar", "cs": "cestovat, cestování", "da": "rejse", "el": "το να ταξιδεύω", "fa": "سفر کردن", "frp": "voyage", "ga": "taisteal", "he": "מסע", "hi": "to travel", "hr": "putovati", "hu": "utazik", "id": "perjalanan", "it": "viaggio", "ja": "旅行をする", "kk": "сапар", "km": "journey", "ko": "여행", "ku": "سفر کردن", "lo": "journey", "mg": "fandehanana , dia", "ms": "perjalanan,kata akar", "my": "journey", "pl": "journey", "ro": "voyage", "ru": "путешествие", "sk": "cestovať, cestovanie", "sl": "potovanje", "sv": "resa", "sw": "voyage", "th": "ท่องเที่ยว", "tok": "to travel", "tr": "seyahat", "uk": "подорожувати, мандрувати, їздити", "ur": "سفر", "vi": "đi du lịch", "yo": "to travel", "zh-tw": "旅行", "zh": "旅行,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 278,
		},
		{
			slug: "vortaro-radiko-vok", typ: "vocab",
			content: map[string]interface{}{
				"word": "vok",
				"definition": "to call",
				"definitions": map[string]interface{}{"en": "to call", "nl": "roepen", "de": "rufen", "fr": "appeler", "es": "llamar", "pt": "chamar, chamado", "ar": "ينادي", "be": "звать", "ca": "cridar (fer venir algú, avisar)", "cs": "volat", "da": "ringe", "el": "το να καλώ", "fa": "صدا زدن", "frp": "appeler", "ga": "glaoigh", "he": "לקרוא בקול", "hi": "to call", "hr": "zvati", "hu": "hív", "id": "panggil", "it": "chiamare", "ja": "呼びかける", "kk": "to call", "km": "to call", "ko": "부르다", "ku": "صدا زدن", "lo": "to call", "mg": "miantso , manisy anarana", "ms": "pangil,kata akar", "my": "to call", "pl": "dzwonić", "ro": "appeler", "ru": "звать", "sk": "volať", "sl": "klicati", "sv": "kalla, ropa", "sw": "appeler", "th": "เรียก", "tok": "to call", "tr": "seslenmek", "uk": "кликати", "ur": "to call", "vi": "gọi", "yo": "to call", "zh-tw": "叫", "zh": "叫,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 279,
		},
		{
			slug: "vortaro-radiko-vol", typ: "vocab",
			content: map[string]interface{}{
				"word": "vol",
				"definition": "to want",
				"definitions": map[string]interface{}{"en": "to want", "nl": "willen", "de": "wollen", "fr": "vouloir", "es": "querer", "pt": "querer, desejo", "ar": "يريد", "be": "желать, хотеть", "ca": "voler", "cs": "chtít", "da": "ville", "el": "το να θέλω", "fa": "خواستن", "frp": "vouloir", "ga": "teastaigh ó", "he": "רצון", "hi": "want", "hr": "htjeti", "hu": "akar", "id": "mau, ingin", "it": "volere", "ja": "したいと思う", "kk": "want", "km": "want", "ko": "원하다", "ku": "خواستن", "lo": "want", "mg": "tia (té) , mikasa", "ms": "ingin,kata akar", "my": "want", "pl": "chcieć", "ro": "vouloir", "ru": "желать, хотеть", "sk": "chcieť", "sl": "hoteti", "sv": "vilja", "sw": "vouloir", "th": "ต้องการ", "tok": "to want", "tr": "istemek", "uk": "хотіти", "ur": "چاہنا", "vi": "muốn", "yo": "to want", "zh-tw": "想要", "zh": "要,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 280,
		},
		{
			slug: "vortaro-radiko-vort", typ: "vocab",
			content: map[string]interface{}{
				"word": "vort",
				"definition": "word",
				"definitions": map[string]interface{}{"en": "word", "nl": "woord", "de": "Wort", "fr": "mot", "es": "palabra", "pt": "palavra", "ar": "كلمة", "be": "слово", "ca": "paraula", "cs": "slovo", "da": "ord", "el": "λέξη", "fa": "واژه, لغت, کلمه", "frp": "mot", "ga": "focal", "he": "מלה", "hi": "word", "hr": "riječ", "hu": "szó", "id": "kata", "it": "parola", "ja": "語", "kk": "сөз", "km": "ពាក្យ", "ko": "단어", "ku": "واژه, لغت, کلمه", "lo": "word", "mg": "teny vava", "ms": "perkataan,kata akar", "my": "word", "pl": "słowo", "ro": "mot", "ru": "слово", "sk": "slovo", "sl": "beseda", "sv": "ord", "sw": "mot", "th": "คำ, คำศัพท์", "tok": "word", "tr": "kelime", "uk": "слово", "ur": "لفظ", "vi": "lời, từ", "yo": "word", "zh-tw": "詞", "zh": "字,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 281,
		},
		{
			slug: "vortaro-radiko-vocx", typ: "vocab",
			content: map[string]interface{}{
				"word": "voĉ",
				"definition": "voice",
				"definitions": map[string]interface{}{"en": "voice", "nl": "stem", "de": "Stimme", "fr": "voix", "es": "voz", "pt": "voz", "ar": "صوت", "be": "голос", "ca": "veu", "cs": "hlas", "da": "stemme", "el": "φωνή", "fa": "صدای شخص", "frp": "voix", "ga": "guth", "he": "קול", "hi": "voice", "hr": "glas", "hu": "hang", "id": "suara", "it": "voce", "ja": "声", "kk": "дауыс", "km": "voice", "ko": "음성", "ku": "صدای شخص", "lo": "voice", "mg": "feo", "ms": "suara,kata akar", "my": "voice", "pl": "głos", "ro": "voix", "ru": "голос", "sk": "hlas", "sl": "glas", "sv": "röst", "sw": "voix", "th": "เสียงร้อง", "tok": "voice", "tr": "ses", "uk": "голос", "ur": "آواز", "vi": "giọng nói", "yo": "voice", "zh-tw": "說話聲", "zh": "声音,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 282,
		},
		{
			slug: "vortaro-radiko-zorg", typ: "vocab",
			content: map[string]interface{}{
				"word": "zorg",
				"definition": "to care",
				"definitions": map[string]interface{}{"en": "to care", "nl": "zorgen", "de": "sorgen", "fr": "se soucier", "es": "preocuparse, encargarse", "pt": "cuidado, cuidar", "ar": "مهتم", "be": "забота", "ca": "ocupar-se, tenir cura (de), preocupar-se, amoïnar-se", "cs": "starost", "da": "tage sig af", "el": "το να φροντίζω", "fa": "اهمیت دادن, توجه کردن, نگران بودن", "frp": "soucis", "ga": "cúram", "he": "דאג", "hi": "care", "hr": "brinuti", "hu": "gond", "id": "urus, peduli", "it": "preoccupazione", "ja": "気を配る", "kk": "care", "km": "care", "ko": "돌보다, 주의하다", "ku": "اهمیت دادن, توجه کردن, نگران بودن", "lo": "care", "mg": "miahy , manana ahiahy", "ms": "penjagaan,kata akar", "my": "care", "pl": "dbać", "ro": "se soucier", "ru": "забота", "sk": "starosť, starostlivosť", "sl": "skrbeti", "sv": "bry sig", "sw": "souci", "th": "ดูแล, เอาใจใส่", "tok": "to care", "tr": "ilgilenmek, endişelenmek", "uk": "турбуватись", "ur": "care", "vi": "quan tâm", "yo": "to care", "zh-tw": "照顧", "zh": "照顾,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 283,
		},
		{
			slug: "vortaro-radiko-cxambr", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉambr",
				"definition": "room",
				"definitions": map[string]interface{}{"en": "room", "nl": "kamer", "de": "Zimmer", "fr": "pièce (d'un logement)", "es": "habitación", "pt": "quarto, sala", "ar": "غرفة", "be": "комната", "ca": "habitació", "cs": "pokoj", "da": "værelse, rum", "el": "δωμάτιο", "fa": "اتاق", "frp": "pièce (d'un logement)", "ga": "seomra", "he": "חדר", "hi": "room", "hr": "soba", "hu": "szoba", "id": "kamar", "it": "stanza", "ja": "部屋", "kk": "бөлім", "km": "បន្ទប់", "ko": "방", "ku": "اتاق", "lo": "room", "mg": "efitra (d'un logement)", "ms": "bilik,kata akar", "my": "room", "pl": "pokój", "ro": "pièce (d'un logement)", "ru": "комната", "sk": "izba", "sl": "soba", "sv": "rum", "sw": "pièce (d'un logement)", "th": "ห้อง", "tok": "room", "tr": "oda", "uk": "кімната", "ur": "کمرہ", "vi": "căn phòng", "yo": "room", "zh-tw": "房間", "zh": "房间,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 284,
		},
		{
			slug: "vortaro-radiko-cxef", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉef",
				"definition": "boss",
				"definitions": map[string]interface{}{"en": "boss", "nl": "chef, hoofd", "de": "Chef", "fr": "chef", "es": "jefe", "pt": "patrão, chefe", "ar": "أساس, زعيم، مدير", "be": "главный", "ca": "cap (directiu)", "cs": "šéf", "da": "chef", "el": "αρχηγός", "fa": "رییس", "frp": "chef", "ga": "saoiste", "he": "בוס, מנהל", "hi": "boss", "hr": "glavni, šef", "hu": "főnök", "id": "bos", "it": "capo", "ja": "ボス", "kk": "boss", "km": "boss", "ko": "지휘자, 우두머리", "ku": "رییس", "lo": "boss", "mg": "loha , sefo", "ms": "ketua,kata akar", "my": "boss", "pl": "szef", "ro": "chef", "ru": "главный", "sk": "šéf", "sl": "šef", "sv": "chef", "sw": "chef", "th": "หัวหน้า, เป็นหลัก", "tok": "boss", "tr": "patron", "uk": "шеф", "ur": "باس، آقا", "vi": "trưởng phòng, giám đốc, ông trùm, ông chủ", "yo": "boss", "zh-tw": "主要, 首領", "zh": "老板、主要负责人,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 285,
		},
		{
			slug: "vortaro-radiko-gxen", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝen",
				"definition": "to distrub, to bother",
				"definitions": map[string]interface{}{"en": "to distrub, to bother", "nl": "storen", "de": "stören, belästigen", "fr": "gêner", "es": "molestar", "pt": "incomodar", "ar": "إزعاج", "be": "беспокоить, стеснять", "ca": "molestar, fer nosa", "cs": "obtěžovat", "da": "forstyrre, irritere", "el": "το να ενοχλώ", "fa": "آزار دادن, زحمت دادن", "frp": "gêner", "ga": "cuir isteach ar, cuir as do", "he": "להפריע, להטריד", "hi": "to distrub, to bother", "hr": "smetati", "hu": "zavar", "id": "ganggu", "it": "disturbare, infastidire", "ja": "じゃまをする, 気をつかわせる", "kk": "to distrub, to bother", "km": "to distrub, to bother", "ko": "괴롭히다, 폐끼치다", "ku": "آزار دادن, زحمت دادن", "lo": "to distrub, to bother", "mg": "manahirana", "ms": "kacau,kata akar", "my": "to distrub, to bother", "pl": "przeszkadzać, dręczyć", "ro": "gêner", "ru": "беспокоить, стеснять", "sk": "obťažovať", "sl": "motiti", "sv": "störa, besvära", "sw": "gêner", "th": "รบกวน", "tok": "to distrub, to bother", "tr": "can sıkmak, rahatsızlık vermek", "uk": "турбувати, завдавати клопоту, надокучати, набридати", "ur": "to distrub, to bother", "vi": "quấy rầy, làm phiền", "yo": "to distrub, to bother", "zh-tw": "干擾, 困擾", "zh": "干扰,词根, 骚扰,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 286,
		},
		{
			slug: "vortaro-radiko-gxeneral", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝeneral",
				"definition": "general",
				"definitions": map[string]interface{}{"en": "general", "nl": "algemeen", "de": "allgemein", "fr": "géneral", "es": "general", "pt": "geral", "ar": "العام", "be": "общий", "ca": "general", "cs": "obecný", "da": "generelt", "el": "γενικό", "fa": "عمومی بودن", "frp": "géneral", "ga": "ginearálta", "he": "כללי", "hi": "general", "hr": "opći", "hu": "általános", "id": "umum", "it": "generale", "ja": "全体の, 一般的な", "kk": "general", "km": "general", "ko": "일반적인", "ku": "عمومی بودن", "lo": "general", "mg": "rehetra, ankapobe", "ms": "umum,kata akar", "my": "general", "pl": "ogólny", "ro": "géneral", "ru": "общий", "sk": "všeobecný", "sl": "splošno", "sv": "generellt", "sw": "géneral", "th": "ทั่ว ๆ ไป", "tok": "general", "tr": "genel", "uk": "загальний, ґенеральний", "ur": "عمومی", "vi": "chủ yếu", "yo": "general", "zh-tw": "普通", "zh": "普通,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 287,
		},
		{
			slug: "vortaro-radiko-gxoj", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝoj",
				"definition": "glad",
				"definitions": map[string]interface{}{"en": "glad", "nl": "blij, vrolijk", "de": "froh", "fr": "joie", "es": "contento/a", "pt": "alegre, alegria", "ar": "فرح", "be": "радость", "ca": "alegrar-se, estar content/a, feliç", "cs": "radostný", "da": "glædelig", "el": "το να χαίρομαι", "fa": "شاد", "frp": "joie", "ga": "áthas", "he": "שמח", "hi": "glad", "hr": "veseo, radostan", "hu": "öröm", "id": "senang", "it": "contento", "ja": "うれしい", "kk": "glad", "km": "glad", "ko": "기쁜", "ku": "شاد", "lo": "glad", "mg": "hafaliana", "ms": "gembira,kata akar", "my": "glad", "pl": "radość", "ro": "joie", "ru": "радость", "sk": "radostný", "sl": "veselje", "sv": "glad", "sw": "joie", "th": "ดีใจ, ร่าเริง", "tok": "glad", "tr": "memnun, hoşnut", "uk": "-радіти - радуватися -тішитися -веселитися", "ur": "خوشی", "vi": "vui mừng, hân hoan", "yo": "glad", "zh-tw": "高興", "zh": "高兴,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 288,
		},
		{
			slug: "vortaro-radiko-gxust", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝust",
				"definition": "correct",
				"definitions": map[string]interface{}{"en": "correct", "nl": "juist", "de": "richtig", "fr": "juste", "es": "correcto/a", "pt": "correto", "ar": "صحيح", "be": "точный, верный, правильный", "ca": "correcte", "cs": "správně", "da": "korrekt", "el": "σωστός-ή-ό", "fa": "درست", "frp": "juste", "ga": "ceart", "he": "נכון, מדוייק", "hi": "correct", "hr": "pravi", "hu": "helyes", "id": "benar, tepat", "it": "giust, corretto", "ja": "的確な", "kk": "to correct", "km": "to correct", "ko": "옳은", "ku": "درست", "lo": "to correct", "mg": "marina", "ms": "membetul,kata akar", "my": "to correct", "pl": "poprawny", "ro": "juste", "ru": "точный, верный, правильный", "sk": "správny", "sl": "pravilno", "sv": "rätt, riktig", "sw": "juste", "th": "ถูกต้อง", "tok": "correct", "tr": "düzeltmek", "uk": "точний, правильний", "ur": "درست کرنا", "vi": "chính xác, hợp lý", "yo": "correct", "zh-tw": "正確", "zh": "修正，改正,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 289,
		},
		{
			slug: "vortaro-radiko-jxet", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĵet",
				"definition": "to throw",
				"definitions": map[string]interface{}{"en": "to throw", "nl": "werpen, gooien", "de": "werfen", "fr": "jeter", "es": "lanzar", "pt": "arremessar", "ar": "رمي", "be": "кидать", "ca": "tirar, llançar", "cs": "hodit", "da": "kaste", "el": "το να ρίχνω", "fa": "پرتاب کردن", "frp": "jeter", "ga": "caith", "he": "לזרוק, להשליך", "hi": "to throw", "hr": "baciti", "hu": "dob", "id": "lempar", "it": "lanciare", "ja": "投げる", "kk": "to throw", "km": "to throw", "ko": "던지다", "ku": "پرتاب کردن", "lo": "to throw", "mg": "manipy , mitoraka", "ms": "membuang,kata akar", "my": "to throw", "pl": "rzucać", "ro": "jeter", "ru": "кидать", "sk": "hodiť", "sl": "vreči", "sv": "kasta", "sw": "jeter", "th": "ขว้าง", "tok": "to throw", "tr": "fırlatmak, atmak", "uk": "кидати", "ur": "پھینکنا", "vi": "ném, vứt", "yo": "to throw", "zh-tw": "丢, 扔", "zh": "丢,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 290,
		},
		{
			slug: "vortaro-radiko-sxajn", typ: "vocab",
			content: map[string]interface{}{
				"word": "ŝajn",
				"definition": "seem",
				"definitions": map[string]interface{}{"en": "seem", "nl": "schijnen, lijken", "de": "scheinen", "fr": "sembler", "es": "parecer", "pt": "parecer", "ar": "يبدو", "be": "казаться, представляться", "ca": "semblar", "cs": "zdát se", "da": "virke", "el": "το να φαίνεται", "fa": "به نظر رسیدن", "frp": "sembler", "ga": "cuma", "he": "נראה", "hi": "seem", "hr": "činiti se", "hu": "tűnik", "id": "tampak", "it": "sembrare", "ja": "…の様子である, …らしい", "kk": "seem", "km": "seem", "ko": "~한 모양이다, ~인 것 같다", "ku": "به نظر رسیدن", "lo": "seem", "mg": "hoatra , toy", "ms": "kelihatan,kata akar", "my": "seem", "pl": "wydawać (się)", "ro": "sembler", "ru": "казаться, представляться", "sk": "zdať sa", "sl": "zdeti se", "sv": "förefalla, tyckas, verka", "sw": "sembler", "th": "ดูเหมือน", "tok": "seem", "tr": "gibi görünmek, sanki", "uk": "здаватися, видаватися", "ur": "seem", "vi": "hình như, dường như", "yo": "seem", "zh-tw": "似乎", "zh": "似乎,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 291,
		},
		{
			slug: "vortaro-radiko-sxangx", typ: "vocab",
			content: map[string]interface{}{
				"word": "ŝanĝ",
				"definition": "to change",
				"definitions": map[string]interface{}{"en": "to change", "nl": "veranderen, wijzigen", "de": "wechseln", "fr": "changer", "es": "cambiar", "pt": "mudar, mudança", "ar": "تغيير", "be": "менять", "ca": "canviar", "cs": "měnit", "da": "ændre", "el": "το να αλλάζω", "fa": "تغییر دادن", "frp": "changer", "ga": "athraigh", "he": "לשנות", "hi": "to change", "hr": "mijenjati", "hu": "cserél, vált", "id": "ubah", "it": "cambiare", "ja": "代える", "kk": "to change", "km": "to change", "ko": "변하다", "ku": "تغییر دادن", "lo": "to change", "mg": "miova , manakalo , manova", "ms": "tukar,kata akar", "my": "to change", "pl": "zmieniać", "ro": "changer", "ru": "менять", "sk": "meniť", "sl": "spremeniti", "sv": "ändra, byta", "sw": "changer", "th": "เปลี่ยน", "tok": "to change", "tr": "değiştirmek", "uk": "міняти", "ur": "تبدیل کرنا", "vi": "thay đổi", "yo": "to change", "zh-tw": "改, 改變", "zh": "改、改变,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 292,
		},
		{
			slug: "vortaro-radiko-sxat", typ: "vocab",
			content: map[string]interface{}{
				"word": "ŝat",
				"definition": "to like",
				"definitions": map[string]interface{}{"en": "to like", "nl": "graag hebben, lusten", "de": "mögen, gern haben", "fr": "aimer", "es": "gustar", "pt": "gostar", "ar": "حب", "be": "оценить, ценить, нравится", "ca": "agradar, valorar, preuar, apreciar", "cs": "mít rád", "da": "kunne lide", "el": "το να μου αρέσει", "fa": "دوست داشتن", "frp": "aimer", "ga": "is maith le", "he": "לחבב, למצוא חן", "hi": "to like", "hr": "voljeti", "hu": "szeret", "id": "suka", "it": "gradire, apprezzare", "ja": "高く評価する, 好む", "kk": "to like", "km": "to like", "ko": "좋아하다", "ku": "دوست داشتن", "lo": "to like", "mg": "tia", "ms": "suka,kata akar", "my": "to like", "pl": "lubić", "ro": "aimer", "ru": "оценить, ценить, нравится", "sk": "mať rád", "sl": "imeti rad", "sv": "tycka om, gilla", "sw": "aimer", "th": "ชอบ", "tok": "to like", "tr": "beğenmek", "uk": "цінувати, любити", "ur": "پسند کرنا", "vi": "thích", "yo": "to like", "zh-tw": "喜歡", "zh": "喜欢,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 293,
		},
		{
			slug: "vortaro-radiko-sxip", typ: "vocab",
			content: map[string]interface{}{
				"word": "ŝip",
				"definition": "ship",
				"definitions": map[string]interface{}{"en": "ship", "nl": "schip", "de": "Schiff", "fr": "navire, bateau", "es": "barco", "pt": "navio", "ar": "سفينة", "be": "корабль, судно", "ca": "vaixell", "cs": "loď", "da": "skib", "el": "πλοίο", "fa": "کِشتی", "frp": "navire, bateau", "ga": "long", "he": "אניה", "hi": "ship", "hr": "brod", "hu": "hajó", "id": "kapal", "it": "nave", "ja": "船", "kk": "кеме", "km": "ship", "ko": "배", "ku": "کِشتی", "lo": "ship", "mg": "sambo be, sambo", "ms": "kapal laut,kata akar", "my": "ship", "pl": "statek", "ro": "navire, bateau", "ru": "корабль, судно", "sk": "loď", "sl": "ladja", "sv": "båt, fartyg", "sw": "navire, bateau", "th": "เรือ", "tok": "ship", "tr": "gemi", "uk": "корабель", "ur": "بحری جہاز", "vi": "thuyền, tàu thuỷ", "yo": "ship", "zh-tw": "船", "zh": "船,词根"},
			},
			tags:        []string{"vortaro", "radiko"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "vortaro-radiko",
			seriesOrder: 294,
		},
		{
			slug: "vortaro-adverbo-almenaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "almenaŭ",
				"definition": "at least",
				"definitions": map[string]interface{}{"en": "at least", "nl": "minstens, ten minste", "de": "wenigstens", "fr": "au moins", "es": "al menos", "pt": "ao menos", "ar": "على الأقل", "be": "по крайней мере, по меньшей мере", "ca": "almenys", "cs": "alespoň", "da": "i det mindste", "el": "τουλάχιστον", "fa": "حداقل", "frp": "u muens", "ga": "ar a laghad", "he": "לפחות", "hi": "at least", "hr": "barem", "hu": "legalább", "id": "setidaknya", "it": "almeno", "ja": "少なくとも", "kk": "at least", "km": "at least", "ko": "적어도", "ku": "حداقل", "lo": "ຢ່າງນ້ອຍ", "mg": "farafaharatsiny", "ms": "sekurang-kurangnya", "my": "at least", "pl": "co najmniej, przynajmniej", "ro": "au moins", "ru": "по крайней мере, по меньшей мере", "sk": "aspoň", "sl": "vsaj", "sv": "åtmistone", "sw": "au moins", "th": "อย่างน้อย", "tr": "hiç olmazsa, en azından", "uk": "принаймні, хоч", "ur": "آخر کار", "vi": "ít nhất", "yo": "at least", "zh-tw": "至少", "zh": "至少"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-adverbo-ambaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "ambaŭ",
				"definition": "both",
				"definitions": map[string]interface{}{"en": "both", "nl": "beide, beiden", "de": "beide", "fr": "les deux", "es": "ambos/as", "pt": "ambos", "ar": "على حد سواء, كلاهما", "be": "оба, обе", "ca": "ambdós, ambdues", "cs": "oba", "da": "begge", "el": "και τα δύο", "fa": "هر دو", "frp": "los dous", "ga": "an dá, beirt", "he": "שניהם", "hi": "both", "hr": "oba", "hu": "mindkettő", "id": "keduanya", "it": "entrambi", "ja": "両方", "kk": "екеу, қос", "km": "both", "ko": "둘다", "ku": "هر دو", "lo": "ທັງສອງ", "mg": "izy roa", "ms": "kedua-dua", "my": "both", "pl": "oba, obie, obydwaj", "ro": "les deux", "ru": "оба, обе", "sk": "obaja", "sl": "oba", "sv": "båda, bägge", "sw": "les deux", "th": "ทั้งคู่, ทั้งสอง", "tr": "her ikisi", "uk": "обидва, обоє, обидві", "ur": "دونوں", "vi": "cả hai", "yo": "both", "zh-tw": "雙雙, 兩者", "zh": "双，两个"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-adverbo-ankaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "ankaŭ",
				"definition": "also",
				"definitions": map[string]interface{}{"en": "also", "nl": "ook, eveneens", "de": "auch", "fr": "aussi", "es": "también", "pt": "também", "ar": "أيضا", "be": "тоже, также", "ca": "també", "cs": "také", "da": "også", "el": "επίσης", "fa": "هم, نیز, همچنین", "frp": "asse, etot", "ga": "freisin", "he": "גם", "hi": "also", "hr": "također", "hu": "is", "id": "juga", "it": "anche", "ja": "～も", "kk": "also", "km": "also", "ko": "또한", "ku": "هم, نیز, همچنین", "lo": "ຄືກັນ", "mg": "ihany koa", "ms": "juga", "my": "also", "pl": "też, również, także", "ro": "aussi", "ru": "тоже, также", "sk": "tiež", "sl": "tudi", "sv": "även, också", "sw": "aussi", "th": "ด้วย", "tr": "de, da", "uk": "також", "ur": "بھی", "vi": "cũng", "yo": "also", "zh-tw": "也是", "zh": "也是"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-adverbo-ankoraux", typ: "vocab",
			content: map[string]interface{}{
				"word": "ankoraŭ",
				"definition": "still, yet",
				"definitions": map[string]interface{}{"en": "still, yet", "nl": "nog", "de": "noch", "fr": "encore, toujours", "es": "todavía", "pt": "ainda", "ar": "مازال", "be": "ещё", "ca": "encara", "cs": "ještě", "da": "stadig, endnu", "el": "ακόμα", "fa": "هنوز, همچنان", "frp": "oncor", "ga": "fós, go fóill, i gcónaí", "he": "עדיין", "hi": "still, yet", "hr": "još", "hu": "még", "id": "masih, tetap", "it": "ancora", "ja": "まだ", "kk": "still, yet", "km": "still, yet", "ko": "아직도, 여전히", "ku": "هنوز, همچنان", "lo": "ຍັງ", "mg": "indray, foana", "ms": "juga, belum", "my": "still, yet", "pl": "jeszcze", "ro": "encore, toujours", "ru": "ещё", "sk": "ešte", "sl": "še", "sv": "ännu, fortfarande", "sw": "encore, toujours", "th": "ยังคง", "tr": "hala", "uk": "ще, крім того", "ur": "ابھی, ابھی", "vi": "vẫn, chưa, còn nữa", "yo": "still, yet", "zh-tw": "還沒, 還在", "zh": "还没, 还在"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-adverbo-baldaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "baldaŭ",
				"definition": "soon",
				"definitions": map[string]interface{}{"en": "soon", "nl": "weldra, binnenkort", "de": "bald", "fr": "bientôt", "es": "pronto", "pt": "em breve", "ar": "قريبا", "be": "скоро", "ca": "aviat", "cs": "brzy", "da": "snart", "el": "σύντομα", "fa": "به زودی", "frp": "bentôt", "ga": "go luath", "he": "עוד מעט", "hi": "soon", "hr": "uskoro", "hu": "hamarosan, majd", "id": "segera", "it": "presto", "ja": "まもなく", "kk": "жылдам; тез; шапшаң", "km": "soon", "ko": "곧", "ku": "به زودی", "lo": "ໃນໄວໆນີ້", "mg": "tsy ho ela", "ms": "tidak lama lagi", "my": "soon", "pl": "wkrótce, wnet, niebawem", "ro": "bientôt", "ru": "скоро", "sk": "čoskoro", "sl": "kmalu", "sv": "snart", "sw": "bientôt", "th": "เร็ว ๆ นี้, ในไม่ช้า", "tr": "yakında", "uk": "незабаром, невдовзі, скоро, найближчим часом", "ur": "جلد", "vi": "chẳng mấy chốc", "yo": "soon", "zh-tw": "不久後", "zh": "不久后"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-adverbo-do", typ: "vocab",
			content: map[string]interface{}{
				"word": "do",
				"definition": "so",
				"definitions": map[string]interface{}{"en": "so", "nl": "dus", "de": "also", "fr": "donc", "es": "por lo tanto", "pt": "então", "ar": "وبالتالي, اذن", "be": "итак", "ca": "per tant", "cs": "tedy", "da": "så", "el": "έτσι, λοιπόν", "fa": "پس", "frp": "adonc", "ga": "mar sin", "he": "אז, ובכן", "hi": "so", "hr": "dakle", "hu": "hát", "id": "jadi", "it": "quindi", "ja": "だから", "kk": "so", "km": "so", "ko": "따라서", "ku": "پس", "lo": "so", "mg": "noho izany", "ms": "begitu", "my": "so", "pl": "więc", "ro": "donc", "ru": "итак", "sk": "teda", "sl": "torej", "sv": "då, alltså", "sw": "donc", "th": "ดังนั้น", "tr": "dolayısı ile", "uk": "отже, отож, же, ж", "ur": "سو", "vi": "vì thế nên", "yo": "so", "zh-tw": "如此", "zh": "如此"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-adverbo-ecx", typ: "vocab",
			content: map[string]interface{}{
				"word": "eĉ",
				"definition": "even",
				"definitions": map[string]interface{}{"en": "even", "nl": "zelfs", "de": "sogar, selbst", "fr": "même", "es": "incluso, aun", "pt": "até, mesmo", "ar": "حتى", "be": "даже", "ca": "inclús, fins i tot", "cs": "ba i, dokonce", "da": "selv, endda", "el": "ακόμα και", "fa": "حتی", "frp": "mèmo", "ga": "fiú", "he": "אפילו", "hi": "even", "hr": "čak", "hu": "még ... is", "id": "bahkan", "it": "perfino", "ja": "～でさえ", "kk": "да; де; екеш; тұрмақ; түгіл; тіпті", "km": "even", "ko": "심지어", "ku": "حتی", "lo": "even", "mg": "mitovy ,ihany", "ms": "walaupun", "my": "even", "pl": "nawet, aż", "ro": "même", "ru": "даже", "sk": "ba aj, dokonca", "sl": "celo", "sv": "till och med, ens", "sw": "même", "th": "แม้แต่", "tr": "bile", "uk": "навіть", "ur": "حتی", "vi": "kể cả, thậm chí", "yo": "even", "zh-tw": "甚至", "zh": "甚至"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-adverbo-hieraux", typ: "vocab",
			content: map[string]interface{}{
				"word": "hieraŭ",
				"definition": "yesterday",
				"definitions": map[string]interface{}{"en": "yesterday", "nl": "gisteren", "de": "gestern", "fr": "hier", "es": "ayer", "pt": "ontem", "ar": "أمس, البارحة", "be": "вчера", "ca": "ahir", "cs": "včera", "da": "i går", "el": "χθες", "fa": "دیروز", "frp": "hièr", "ga": "inné", "he": "אתמול", "hi": "yesterday", "hr": "jučer", "hu": "tegnap", "id": "kemarin", "it": "ieri", "ja": "昨日", "kk": "кеше", "km": "yesterday", "ko": "어제", "ku": "دیروز", "lo": "yesterday", "mg": "omaly", "ms": "kemalrin", "my": "yesterday", "pl": "wczoraj", "ro": "hier", "ru": "вчера", "sk": "včera", "sl": "včeraj", "sv": "igår", "sw": "hier", "th": "เมื่อวาน", "tr": "dün", "uk": "вчора", "ur": "گذرہ ہوا کل", "vi": "hôm qua", "yo": "yesterday", "zh-tw": "昨天", "zh": "昨天"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-adverbo-hodiaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "hodiaŭ",
				"definition": "today",
				"definitions": map[string]interface{}{"en": "today", "nl": "vandaag", "de": "heute", "fr": "aujourd'hui", "es": "hoy", "pt": "hoje", "ar": "اليوم", "be": "сегодня", "ca": "avui", "cs": "dnes", "da": "i dag", "el": "σήμερα", "fa": "امروز", "frp": "houè enc'houè", "ga": "inniu", "he": "היום", "hi": "today", "hr": "danas", "hu": "ma", "id": "hari ini", "it": "oggi", "ja": "今日", "kk": "бүгін", "km": "today", "ko": "오늘", "ku": "امروز", "lo": "today", "mg": "androany", "ms": "hari ini", "my": "today", "pl": "dziś, dzisiaj", "ro": "aujourd'hui", "ru": "сегодня", "sk": "dnes", "sl": "danes", "sv": "idag", "sw": "aujourd'hui", "th": "วันนี้", "tr": "bugüm", "uk": "сьогодні", "ur": "آج", "vi": "hôm nay", "yo": "today", "zh-tw": "今天", "zh": "今天"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-adverbo-jam", typ: "vocab",
			content: map[string]interface{}{
				"word": "jam",
				"definition": "already",
				"definitions": map[string]interface{}{"en": "already", "nl": "reeds, al", "de": "schon", "fr": "déjà", "es": "ya", "pt": "já", "ar": "بالفعل", "be": "уже", "ca": "ja", "cs": "již, už", "da": "allerede", "el": "ήδη", "fa": "دیگر", "frp": "ja", "ga": "cheana", "he": "כבר", "hi": "already", "hr": "već", "hu": "már", "id": "sudah", "it": "già", "ja": "既に", "kk": "әлдеқашан; енді; де", "km": "already", "ko": "이미", "ku": "دیگر", "lo": "already", "mg": "sahady", "ms": "sesudah", "my": "already", "pl": "już", "ro": "déjà", "ru": "уже", "sk": "už", "sl": "že", "sv": "redan", "sw": "déjà", "th": "แล้ว", "tr": "şimdiden", "uk": "вже", "ur": "پہلے سے", "vi": "đã … rồi", "yo": "already", "zh-tw": "已經", "zh": "已经"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-adverbo-jes", typ: "vocab",
			content: map[string]interface{}{
				"word": "jes",
				"definition": "yes",
				"definitions": map[string]interface{}{"en": "yes", "nl": "ja", "de": "ja", "fr": "oui", "es": "sí", "pt": "sim", "ar": "نعم, أجل", "be": "да", "ca": "sí", "cs": "ano", "da": "ja", "el": "ναι", "fa": "بله", "frp": "ouè", "ga": "is ea", "he": "כן", "hi": "yes", "hr": "da", "hu": "igen", "id": "ya", "it": "sì", "ja": "はい", "kk": "иә", "km": "yes", "ko": "예", "ku": "بله", "lo": "yes", "mg": "eny, ya", "ms": "ia", "my": "yes", "pl": "tak", "ro": "oui", "ru": "да", "sk": "áno", "sl": "da", "sv": "ja", "sw": "oui", "th": "ใช่", "tr": "Evet", "uk": "так", "ur": "ہاں", "vi": "đúng vậy", "yo": "yes", "zh-tw": "是的", "zh": "是的"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-adverbo-kvazaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "kvazaŭ",
				"definition": "as if",
				"definitions": map[string]interface{}{"en": "as if", "nl": "alsof, als het ware", "de": "als ob", "fr": "pratiquement", "es": "como si fuese, como si, como", "pt": "como se", "ar": "كما لو", "be": "как будто, словно", "ca": "com si fos, com si, com", "cs": "jakoby", "da": "som om", "el": "λες και", "fa": "مثل این که", "frp": "quâsi", "ga": "amhail is", "he": "כאילו", "hi": "as if", "hr": "kao da", "hu": "mintha", "id": "seolah-olah", "it": "come se", "ja": "まるで", "kk": "as if", "km": "as if", "ko": "마치", "ku": "مثل این که", "lo": "as if", "mg": "saika , madiva ho", "ms": "seolah-olah", "my": "as if", "pl": "jakby, niby", "ro": "pratiquement", "ru": "как будто, словно", "sk": "akoby", "sl": "kakor da", "sv": "liksom, som om", "sw": "pratiquement", "th": "ราวกับ", "tr": "sanki", "uk": "ніби, наче, неначе, немовби", "ur": "as if", "vi": "cứ như", "yo": "as if", "zh-tw": "彷彿", "zh": "仿佛"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-adverbo-morgaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "morgaŭ",
				"definition": "tomorrow",
				"definitions": map[string]interface{}{"en": "tomorrow", "nl": "morgen", "de": "morgen", "fr": "demain", "es": "mañana", "pt": "amanhã", "ar": "غدا", "be": "завтра", "ca": "demà", "cs": "zítra", "da": "i morgen", "el": "αύριο", "fa": "فردا", "frp": "deman", "ga": "amárach", "he": "מחר", "hi": "tomorrow", "hr": "sutra", "hu": "holnap", "id": "besok", "it": "domani", "ja": "明日", "kk": "ертең", "km": "tomorrow", "ko": "내일", "ku": "فردا", "lo": "tomorrow", "mg": "rahampitso", "ms": "esok", "my": "tomorrow", "pl": "jutro", "ro": "demain", "ru": "завтра", "sk": "zajtra", "sl": "jutri", "sv": "imorgon", "sw": "demain", "th": "พรุ่งนี้", "tr": "yarın", "uk": "завтра", "ur": "آنے والا کل", "vi": "ngày mai", "yo": "tomorrow", "zh-tw": "明天", "zh": "明天"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-adverbo-nun", typ: "vocab",
			content: map[string]interface{}{
				"word": "nun",
				"definition": "now",
				"definitions": map[string]interface{}{"en": "now", "nl": "nu", "de": "nun, jetzt", "fr": "maintenant", "es": "ahora", "pt": "agora", "ar": "الآن, حاليا", "be": "сейчас, теперь", "ca": "ara", "cs": "nyní", "da": "nu", "el": "τώρα", "fa": "اکنون, حالا, الآن", "frp": "ôra", "ga": "anois", "he": "עכשיו", "hi": "now", "hr": "sada", "hu": "most", "id": "sekarang", "it": "adesso", "ja": "今", "kk": "қазір", "km": "now", "ko": "지금", "ku": "اکنون, حالا, الآن", "lo": "now", "mg": "ankehitriny", "ms": "sekarang", "my": "now", "pl": "teraz", "ro": "maintenant", "ru": "сейчас, теперь", "sk": "teraz", "sl": "sedaj", "sv": "nu", "sw": "maintenant", "th": "ตอนนี้, เดี๋ยวนี้", "tr": "şimdi", "uk": "зараз, тепер", "ur": "اب", "vi": "bây giờ", "yo": "now", "zh-tw": "現在", "zh": "现在"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-adverbo-nur", typ: "vocab",
			content: map[string]interface{}{
				"word": "nur",
				"definition": "only",
				"definitions": map[string]interface{}{"en": "only", "nl": "slechts, enkel", "de": "nur, bloß", "fr": "seulement", "es": "solo", "pt": "apenas", "ar": "فقط", "be": "только, лишь", "ca": "només", "cs": "pouze", "da": "kun", "el": "μόνο", "fa": "فقط", "frp": "solètament", "ga": "an t-aon", "he": "רק", "hi": "only", "hr": "samo", "hu": "csak", "id": "hanya", "it": "soltanto", "ja": "～だけ", "kk": "тек; ғана; қана", "km": "only", "ko": "단지", "ku": "فقط", "lo": "only", "mg": "ihany", "ms": "hanya", "my": "only", "pl": "tylko", "ro": "seulement", "ru": "только, лишь", "sk": "iba, len", "sl": "samo", "sv": "bara, endast", "sw": "seulement", "th": "เท่านั้น", "tr": "yalnız", "uk": "лише, лиш, тільки, лишень", "ur": "صرف", "vi": "chỉ có, duy nhất", "yo": "only", "zh-tw": "只是", "zh": "只是"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-adverbo-plej", typ: "vocab",
			content: map[string]interface{}{
				"word": "plej",
				"definition": "most (superlative)",
				"definitions": map[string]interface{}{"en": "most (superlative)", "nl": "meest (superlatief, overtreffende trap)", "de": "meist (Superlativ)", "fr": "le plus (Superlatif)", "es": "más (superlativo)", "pt": "mais (superlativo)", "ar": "صيغة التفضيل) أكثر )", "be": "наиболее, самый", "ca": "més (superlatiu)", "cs": "nej (3. stupeň přídavných jmen)", "da": "mest (superlativ)", "el": "πλεον (υπερθετικά)", "fa": "بیش‌ترین, بیش‌تر از هر چیز, ترین", "frp": "lo més", "ga": "is (sárchéim)", "he": "הכי", "hi": "most (superlative)", "hr": "naj-", "hu": "leg -bb (felsőfok)", "id": "paling (superlatif)", "it": "il più (superlativo)", "ja": "もっとも (最上級)", "kk": "most (superlative)", "km": "most (superlative)", "ko": "가장 (최상급)", "ku": "بیش‌ترین, بیش‌تر از هر چیز, ترین", "lo": "most (superlative)", "mg": "ny tena", "ms": "paling", "my": "most (superlative)", "pl": "najwięcej, najbardziej", "ro": "le plus (Superlatif)", "ru": "наиболее, самый", "sk": "naj (3. stupeň prídavných mien)", "sl": "naj", "sv": "mest", "sw": "le plus (Superlatif)", "th": "มากที่สุด", "tr": "en (üstünlük)", "uk": "най-, найбільш", "ur": "most (superlative)", "vi": "so sánh nhất", "yo": "most (superlative)", "zh-tw": "最", "zh": "最"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-adverbo-pli", typ: "vocab",
			content: map[string]interface{}{
				"word": "pli",
				"definition": "more (comparative)",
				"definitions": map[string]interface{}{"en": "more (comparative)", "nl": "meer (comparatief, vergelijkende trap)", "de": "mehr (Komparativ)", "fr": "plus (Comparatif)", "es": "más (comparativo)", "pt": "mais (comparativo)", "ar": "مقارن) أكثر)", "be": "более", "ca": "més (comparatiu)", "cs": "víc (2. stupeň přídavných jmen)", "da": "mere (komparativ)", "el": "πιο (συγκριτικά)", "fa": "بیش‌تر, تر", "frp": "més (durâ)", "ga": "níos (breischéim)", "he": "יותר", "hi": "more (comparative)", "hr": "više", "hu": "-bb (középfok)", "id": "lebih (komparatif)", "it": "più (comparativo)", "ja": "もっと (比較級)", "kk": "more (comparative)", "km": "more (comparative)", "ko": "더 (비교급)", "ku": "بیش‌تر, تر", "lo": "more (comparative)", "mg": "bebe kokoa (Fampitahana)", "ms": "lebih", "my": "more (comparative)", "pl": "więcej, bardziej", "ro": "plus (Comparatif)", "ru": "более", "sk": "viac (2. stupeň prídavných mien)", "sl": "več", "sv": "mer", "sw": "plus (Comparatif)", "th": "มากกว่า", "tr": "daha (kıyaslamalı)", "uk": "більше (частка), більш", "ur": "more (comparative)", "vi": "so sánh hơn", "yo": "more (comparative)", "zh-tw": "多", "zh": "多于"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-adverbo-plu", typ: "vocab",
			content: map[string]interface{}{
				"word": "plu",
				"definition": "further, anymore",
				"definitions": map[string]interface{}{"en": "further, anymore", "nl": "verder", "de": "mehr, weiterhin", "fr": "encore", "es": "más (en el tiempo)", "pt": "além", "ar": "إضافة", "be": "далее, дальше", "ca": "més (durada en el temps)", "cs": "dále, více", "da": "endnu", "el": "πια", "fa": "باز هم, دیگر", "frp": "més (cantitâ)", "ga": "níos sia, tuilleadh, thall", "he": "עוד, יותר", "hi": "further, anymore", "hr": "dalje, više", "hu": "tovább", "id": "lagi", "it": "oltre, più (con negativo)", "ja": "さらに", "kk": "further, anymore", "km": "further, anymore", "ko": "더, 계속더", "ku": "باز هم, دیگر", "lo": "further, anymore", "mg": "indray", "ms": "lagi", "my": "further, anymore", "pl": "w dalszym ciągu, dalej, nadal", "ro": "encore", "ru": "далее, дальше", "sk": "ďalej", "sl": "dalje, več", "sv": "vidare, ytterligare, mer (om tid)", "sw": "encore", "th": "เพิ่มอีก, มากขึ้นอีก", "tr": "ileriye, daha fazla", "uk": "більше, більш, далі, ще", "ur": "further, anymore", "vi": "hơn nữa", "yo": "further, anymore", "zh-tw": "進一步, 再", "zh": "进一步, 再"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-adverbo-preskaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "preskaŭ",
				"definition": "almost",
				"definitions": map[string]interface{}{"en": "almost", "nl": "bijna", "de": "fast", "fr": "presque", "es": "casi", "pt": "quase", "ar": "تقريبا", "be": "почти", "ca": "quasi", "cs": "skoro, téměř", "da": "næsten", "el": "σχεδόν", "fa": "تقریبا", "frp": "quâsi", "ga": "beagnach", "he": "כמעט", "hi": "almost", "hr": "gotovo, skoro", "hu": "majdnem", "id": "hampir", "it": "quasi, circa", "ja": "ほとんど", "kk": "дерлік; жуық; қасы", "km": "almost", "ko": "거의", "ku": "تقریبا", "lo": "almost", "mg": "saika ,efa ho", "ms": "hampir", "my": "almost", "pl": "prawie", "ro": "presque", "ru": "почти", "sk": "skoro, takmer", "sl": "skoraj", "sv": "nästan", "sw": "presque", "th": "เกือบจะ", "tr": "hemen hemen, yaklaşık olarak", "uk": "майже, сливе, трохи", "ur": "تقریبا", "vi": "suýt chút nữa, hầu như", "yo": "almost", "zh-tw": "幾乎", "zh": "几乎"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-adverbo-tamen", typ: "vocab",
			content: map[string]interface{}{
				"word": "tamen",
				"definition": "however",
				"definitions": map[string]interface{}{"en": "however", "nl": "toch, nochtans", "de": "dennoch", "fr": "cependant", "es": "sin embargo", "pt": "porém", "ar": "لكن, مع ذلك", "be": "однако", "ca": "tanmateix", "cs": "nicméně, přesto, však", "da": "men, dog, imidlertid", "el": "ωστόσο", "fa": "به هر حال", "frp": "vorendrèt", "ga": "mar sin féin", "he": "בכל זאת", "hi": "however", "hr": "ipak", "hu": "mégis", "id": "bagaimanapun", "it": "tuttavia", "ja": "しかし", "kk": "бірақ", "km": "however", "ko": "그러나", "ku": "به هر حال", "lo": "however", "mg": "kanefa", "ms": "bagaimanapun", "my": "however", "pl": "jednak", "ro": "cependant", "ru": "однако", "sk": "avšak, predsa", "sl": "vendar", "sv": "ändå", "sw": "cependant", "th": "อย่างไรก็ตาม", "tr": "ancak", "uk": "однак, проте", "ur": "however", "vi": "thế nhưng, tuy vậy", "yo": "however", "zh-tw": "然而", "zh": "然而"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-adverbo-tre", typ: "vocab",
			content: map[string]interface{}{
				"word": "tre",
				"definition": "very",
				"definitions": map[string]interface{}{"en": "very", "nl": "zeer", "de": "sehr", "fr": "très", "es": "muy", "pt": "muito", "ar": "جدا", "be": "очень", "ca": "molt", "cs": "velmi", "da": "meget", "el": "πολύ (πάντα προηγείται)", "fa": "بسیار", "frp": "franc, grôs", "ga": "an-", "he": "מאוד", "hi": "very", "hr": "vrlo", "hu": "nagyon", "id": "sangat", "it": "molto", "ja": "とても", "kk": "аса; өте; тым; тіпті; ерен", "km": "very", "ko": "매우", "ku": "بسیار", "lo": "very", "mg": "tena", "ms": "sangat", "my": "very", "pl": "bardzo", "ro": "très", "ru": "очень", "sk": "veľmi", "sl": "zelo", "sv": "mycket (i hög grad)", "sw": "très", "th": "มาก", "tr": "çok", "uk": "дуже, вельми", "ur": "بہت", "vi": "rất", "yo": "very", "zh-tw": "非常", "zh": "非常"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-adverbo-tro", typ: "vocab",
			content: map[string]interface{}{
				"word": "tro",
				"definition": "too, too much",
				"definitions": map[string]interface{}{"en": "too, too much", "nl": "te, teveel", "de": "allzu", "fr": "trop", "es": "demasiado", "pt": "demais", "ar": "كثيرا", "be": "слишком, чересчур", "ca": "massa", "cs": "příliš", "da": "for, for meget", "el": "πάρα πολύ", "fa": "بیش از حد, بسیار زیاد, خیلی", "frp": "trop", "ga": "ró", "he": "יותר מדי", "hi": "too, too much", "hr": "previše", "hu": "túl", "id": "terlalu", "it": "troppo", "ja": "～過ぎる", "kk": "too, too much", "km": "too, too much", "ko": "너무, 너무 많이", "ku": "بیش از حد, بسیار زیاد, خیلی", "lo": "too, too much", "mg": "loatra , be loatra", "ms": "terlalu banyak", "my": "too, too much", "pl": "za, zbyt", "ro": "trop", "ru": "слишком, чересчур", "sk": "príliš", "sl": "preveč", "sv": "allför (mycket)", "sw": "trop", "th": "มากเกินไป", "tr": "aşırı, aşırı fazla", "uk": "занадто, надто", "ur": "too, too much", "vi": "quá, quá nhiều", "yo": "too, too much", "zh-tw": "太過", "zh": "也是, 非常多"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 22,
		},
		{
			slug: "vortaro-adverbo-tuj", typ: "vocab",
			content: map[string]interface{}{
				"word": "tuj",
				"definition": "immediately",
				"definitions": map[string]interface{}{"en": "immediately", "nl": "onmiddellijk, dadelijk", "de": "sofort", "fr": "immédiatement", "es": "en seguida, inmediatamente", "pt": "imediatamente", "ar": "للتو", "be": "сразу", "ca": "desseguida, immediatament", "cs": "hned", "da": "med det samme", "el": "αμέσως", "fa": "فورا", "frp": "imèdiatement", "ga": "láithreach", "he": "מיד", "hi": "immediately", "hr": "odmah", "hu": "azonnal, rögtön", "id": "langsung, segera", "it": "subito", "ja": "すぐに", "kk": "тез", "km": "immediately", "ko": "즉시", "ku": "فورا", "lo": "immediately", "mg": "avy hatrany,haingana", "ms": "segera", "my": "immediately", "pl": "natychmiast", "ro": "immédiatement", "ru": "сразу", "sk": "okamžite, hneď, ihneď", "sl": "takoj", "sv": "genast, omedelbart", "sw": "immédiatement", "th": "เดี๋ยวนี้", "tr": "hemen", "uk": "одразу, негайно", "ur": "فورا", "vi": "lập tức", "yo": "immediately", "zh-tw": "即刻", "zh": "即刻"},
			},
			tags:        []string{"vortaro", "adverbo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-adverbo",
			seriesOrder: 23,
		},
		{
			slug: "vortaro-cifero-unu", typ: "vocab",
			content: map[string]interface{}{
				"word": "unu",
				"definition": "one",
				"definitions": map[string]interface{}{"en": "one", "nl": "een, één", "de": "ein, eine, eines, eins", "fr": "un", "es": "uno", "pt": "um", "ar": "واحد", "be": "один", "ca": "u/un/una", "cs": "jedna", "da": "én, ét, en, et", "el": "ένας, μία, ένα", "fa": "یک", "frp": "yun", "ga": "aon", "he": "אחת", "hi": "one", "hr": "jedan", "hu": "egy", "id": "satu", "it": "uno", "ja": "一", "kk": "бір", "km": "មួយ", "ko": "하나, 일(1)", "ku": "یک", "lo": "one", "mg": ",isa ,iray", "ms": "satu", "my": "one", "pl": "jeden", "ro": "un", "ru": "один", "sk": "jeden", "sl": "ena", "sv": "ett", "sw": "un", "th": "หนึ่ง", "tok": "wan", "tr": "bir", "uk": "один", "ur": "ایک", "vi": "một", "yo": "one", "zh-tw": "一", "zh": "一"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-cifero-du", typ: "vocab",
			content: map[string]interface{}{
				"word": "du",
				"definition": "two",
				"definitions": map[string]interface{}{"en": "two", "nl": "twee", "de": "zwei", "fr": "deux", "es": "dos", "pt": "dois", "ar": "اثنان", "be": "два", "ca": "dos/dues", "cs": "dva", "da": "to", "el": "δύο", "fa": "دو", "frp": "dous", "ga": "dó", "he": "שתיים", "hi": "two", "hr": "dva", "hu": "kettő", "id": "dua", "it": "due", "ja": "二", "kk": "екі", "km": "ពីរ", "ko": "둘, 이(2)", "ku": "دو", "lo": "two", "mg": "roa", "ms": "dua", "my": "two", "pl": "dwa", "ro": "deux", "ru": "два", "sk": "dva", "sl": "dve", "sv": "två", "sw": "deux", "th": "สอง", "tok": "tu", "tr": "iki", "uk": "два", "ur": "دو", "vi": "hai", "yo": "two", "zh-tw": "二", "zh": "二"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-cifero-tri", typ: "vocab",
			content: map[string]interface{}{
				"word": "tri",
				"definition": "three",
				"definitions": map[string]interface{}{"en": "three", "nl": "drie", "de": "drei", "fr": "trois", "es": "tres", "pt": "três", "ar": "ثلاثة", "be": "три", "ca": "tres", "cs": "tři", "da": "tre", "el": "τρείς, τρία", "fa": "سه", "frp": "três", "ga": "trí", "he": "שלוש", "hi": "three", "hr": "tri", "hu": "három", "id": "tiga", "it": "tre", "ja": "三", "kk": "үш", "km": "បី", "ko": "셋, 삼(3)", "ku": "سه", "lo": "three", "mg": "telo", "ms": "tiga", "my": "three", "pl": "trzy", "ro": "trois", "ru": "три", "sk": "tri", "sl": "tri", "sv": "tre", "sw": "trois", "th": "สาม", "tok": "tu wan", "tr": "üç", "uk": "три", "ur": "تین", "vi": "ba", "yo": "three", "zh-tw": "三", "zh": "三"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-cifero-kvar", typ: "vocab",
			content: map[string]interface{}{
				"word": "kvar",
				"definition": "four",
				"definitions": map[string]interface{}{"en": "four", "nl": "vier", "de": "vier", "fr": "quatre", "es": "cuatro", "pt": "quatro", "ar": "أربعة", "be": "четыре", "ca": "quatre", "cs": "čtyři", "da": "fire", "el": "τέσσερεις, τέσσερα", "fa": "چهار", "frp": "catro", "ga": "ceathair", "he": "ארבע", "hi": "four", "hr": "četiri", "hu": "négy", "id": "empat", "it": "quattro", "ja": "四", "kk": "төрт", "km": "បួននាក់", "ko": "넷, 사(4)", "ku": "چهار", "lo": "four", "mg": "efatra", "ms": "empat", "my": "four", "pl": "cztery", "ro": "quatre", "ru": "четыре", "sk": "štyri", "sl": "štiri", "sv": "fyra", "sw": "quatre", "th": "สี่", "tok": "tu tu", "tr": "dört", "uk": "чотири", "ur": "چار", "vi": "bốn", "yo": "four", "zh-tw": "四", "zh": "四"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-cifero-kvin", typ: "vocab",
			content: map[string]interface{}{
				"word": "kvin",
				"definition": "five",
				"definitions": map[string]interface{}{"en": "five", "nl": "vijf", "de": "fünf", "fr": "cinq", "es": "cinco", "pt": "cinco", "ar": "خمسة", "be": "пять", "ca": "cinc", "cs": "pět", "da": "fem", "el": "πέντε", "fa": "پنج", "frp": "cinc", "ga": "cúig", "he": "חמש", "hi": "five", "hr": "pet", "hu": "öt", "id": "lima", "it": "cinque", "ja": "五", "kk": "бес", "km": "ប្រាំនាក់", "ko": "다섯, 오(5)", "ku": "پنج", "lo": "five", "mg": "dimy", "ms": "lima", "my": "five", "pl": "pięć", "ro": "cinq", "ru": "пять", "sk": "päť", "sl": "pet", "sv": "fem", "sw": "cinq", "th": "ห้า", "tok": "luka (nanpa)", "tr": "beş", "uk": "п'ять", "ur": "پانچ", "vi": "năm", "yo": "five", "zh-tw": "五", "zh": "五"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-cifero-ses", typ: "vocab",
			content: map[string]interface{}{
				"word": "ses",
				"definition": "six",
				"definitions": map[string]interface{}{"en": "six", "nl": "zes", "de": "sechs", "fr": "six", "es": "seis", "pt": "seis", "ar": "ستة", "be": "шесть", "ca": "sis", "cs": "šest", "da": "seks", "el": "έξι", "fa": "شش", "frp": "siés", "ga": "sé", "he": "שש", "hi": "six", "hr": "šest", "hu": "hat", "id": "enam", "it": "sei", "ja": "六", "kk": "алты", "km": "ប្រាំមួយ", "ko": "여섯, 육(6)", "ku": "شش", "lo": "six", "mg": "enina", "ms": "enam", "my": "six", "pl": "sześć", "ro": "six", "ru": "шесть", "sk": "šesť", "sl": "šest", "sv": "sex", "sw": "six", "th": "หก", "tok": "luka wan", "tr": "altı", "uk": "шість", "ur": "چھ", "vi": "sáu", "yo": "six", "zh-tw": "六", "zh": "六"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-cifero-sep", typ: "vocab",
			content: map[string]interface{}{
				"word": "sep",
				"definition": "seven",
				"definitions": map[string]interface{}{"en": "seven", "nl": "zeven", "de": "sieben", "fr": "sept", "es": "siete", "pt": "sete", "ar": "سبعة", "be": "семь", "ca": "set", "cs": "sedm", "da": "syv", "el": "επτά", "fa": "هفت", "frp": "sète", "ga": "seacht", "he": "שבע", "hi": "seven", "hr": "sedam", "hu": "hét", "id": "tujuh", "it": "sette", "ja": "七", "kk": "жеті", "km": "ចំនួនប្រាំពីរ", "ko": "일곱, 칠(7)", "ku": "هفت", "lo": "seven", "mg": "fito", "ms": "tujuk", "my": "seven", "pl": "siedem", "ro": "sept", "ru": "семь", "sk": "sedem", "sl": "sedem", "sv": "sju", "sw": "sept", "th": "เจ็ด", "tok": "luka tu", "tr": "yedi", "uk": "сім", "ur": "سات", "vi": "bảy", "yo": "seven", "zh-tw": "七", "zh": "七"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-cifero-ok", typ: "vocab",
			content: map[string]interface{}{
				"word": "ok",
				"definition": "eight",
				"definitions": map[string]interface{}{"en": "eight", "nl": "acht", "de": "acht", "fr": "huit", "es": "ocho", "pt": "oito", "ar": "ثمانية", "be": "восемь", "ca": "vuit", "cs": "osm", "da": "otte", "el": "οκτώ", "fa": "هشت", "frp": "uèt", "ga": "ocht", "he": "שמונה", "hi": "eight", "hr": "osam", "hu": "nyolc", "id": "delapan", "it": "otto", "ja": "八", "kk": "сегіз", "km": "ប្រាំបី", "ko": "여덟, 팔(8)", "ku": "هشت", "lo": "eight", "mg": "valo", "ms": "lapan", "my": "eight", "pl": "osiem", "ro": "huit", "ru": "восемь", "sk": "osem", "sl": "osem", "sv": "åtta", "sw": "huit", "th": "แปด", "tok": "luka tu wan", "tr": "sekiz", "uk": "вісім", "ur": "آٹھ", "vi": "tám", "yo": "eight", "zh-tw": "八", "zh": "八"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-cifero-naux", typ: "vocab",
			content: map[string]interface{}{
				"word": "naŭ",
				"definition": "nine",
				"definitions": map[string]interface{}{"en": "nine", "nl": "negen", "de": "neun", "fr": "neuf", "es": "nueve", "pt": "nove", "ar": "تسعة", "be": "девять", "ca": "nou", "cs": "devět", "da": "ni", "el": "εννέα", "fa": "نُه", "frp": "nou", "ga": "naoi", "he": "תשע", "hi": "nine", "hr": "devet", "hu": "kilenc", "id": "sembilan", "it": "nove", "ja": "九", "kk": "тоғыз", "km": "ប្រាំបួន", "ko": "아홉, 구(9)", "ku": "نُه", "lo": "nine", "mg": "sivy", "ms": "sembilan", "my": "nine", "pl": "dziewięć", "ro": "neuf", "ru": "девять", "sk": "deväť", "sl": "devet", "sv": "nio", "sw": "neuf", "th": "เก้า", "tok": "luka tu tu", "tr": "dokuz", "uk": "дев'ять", "ur": "نو", "vi": "chín", "yo": "nine", "zh-tw": "九", "zh": "九"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-cifero-dek", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek",
				"definition": "ten",
				"definitions": map[string]interface{}{"en": "ten", "nl": "tien", "de": "zehn", "fr": "dix", "es": "diez", "pt": "dez", "ar": "عشرة", "be": "десять", "ca": "deu", "cs": "deset", "da": "ti", "el": "δέκα", "fa": "ده", "frp": "diés", "ga": "deich", "he": "עשר", "hi": "ten", "hr": "deset", "hu": "tíz", "id": "sepuluh", "it": "dieci", "ja": "十", "kk": "он", "km": "ទាំងដប់", "ko": "열, 십(10)", "ku": "ده", "lo": "ten", "mg": "folo", "ms": "sepuluh", "my": "ten", "pl": "dziesięć", "ro": "dix", "ru": "десять", "sk": "desať", "sl": "deset", "sv": "tio", "sw": "dix", "th": "สิบ", "tok": "luka luka", "tr": "on", "uk": "десять", "ur": "دس", "vi": "mười", "yo": "ten", "zh-tw": "十", "zh": "十"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-cifero-dek-unu", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek unu",
				"definition": "eleven",
				"definitions": map[string]interface{}{"en": "eleven", "nl": "elf", "de": "elf", "fr": "onze", "es": "once", "pt": "onze", "ar": "أحد عشر, أحد عشرة", "be": "одинадцать", "ca": "onze", "cs": "jedenáct", "da": "elleve", "el": "έντεκα", "fa": "یازده", "frp": "onzye", "ga": "aon déag", "he": "אחת עשרה", "hi": "eleven", "hr": "jedanaest", "hu": "tizenegy", "id": "sebelas", "it": "undici", "ja": "十一", "kk": "он бір", "km": "សាវ័កដប់មួយរូប", "ko": "열하나, 11", "ku": "یازده", "lo": "eleven", "mg": "iraika ambin'ny folo", "ms": "sebelas", "my": "eleven", "pl": "jedenaście", "ro": "onze", "ru": "одинадцать", "sk": "jedenásť", "sl": "enajst", "sv": "elva", "sw": "onze", "th": "สิบเอ็ด", "tok": "luka luka wan", "tr": "onbir", "uk": "одинадцять", "ur": "گیارہ", "vi": "mười một", "yo": "eleven", "zh-tw": "十一", "zh": "十一"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-cifero-dek-du", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek du",
				"definition": "twelve",
				"definitions": map[string]interface{}{"en": "twelve", "nl": "twaalf", "de": "zwölf", "fr": "douze", "es": "doce", "pt": "doze", "ar": "اثنا عشر, اثنا عشرة", "be": "двенадцать", "ca": "dotze", "cs": "dvanáct", "da": "tolv", "el": "δώδεκα", "fa": "دوازده", "frp": "douzye", "ga": "dó dhéag", "he": "שתיים עשרה", "hi": "twelve", "hr": "dvanaest", "hu": "tizenkettő", "id": "dua belas", "it": "dodici", "ja": "十二", "kk": "он екі", "km": "ទាំងដប់ពីររូប", "ko": "열둘, 12", "ku": "دوازده", "lo": "twelve", "mg": "roa ambin'ny folo", "ms": "dua belas", "my": "twelve", "pl": "dwanaście", "ro": "douze", "ru": "двенадцать", "sk": "dvanásť", "sl": "dvanajst", "sv": "tolv", "sw": "douze", "th": "สิบสอง", "tok": "luka luka tu", "tr": "oniki", "uk": "дванадцять", "ur": "بارہ", "vi": "mười hai", "yo": "twelve", "zh-tw": "十二", "zh": "十二"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-cifero-dek-tri", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek tri",
				"definition": "thirteen",
				"definitions": map[string]interface{}{"en": "thirteen", "nl": "dertien", "de": "dreizehn", "fr": "treize", "es": "trece", "pt": "treze", "ar": "ثلاثة عشر, ثلاثة عشرة", "be": "тринадцать", "ca": "tretze", "cs": "třináct", "da": "tretten", "el": "δεκατρείς, δεκατρία", "fa": "سیزده", "frp": "trêzye", "ga": "trí déag", "he": "שלוש עשרה", "hi": "thirteen", "hr": "trinaest", "hu": "tizenhárom", "id": "tiga belas", "it": "tredici", "ja": "十三", "kk": "он үш", "km": "ដប់បី", "ko": "열셋, 13", "ku": "سیزده", "lo": "thirteen", "mg": "telo ambin'ny folo", "ms": "tiga belas", "my": "thirteen", "pl": "trzynaście", "ro": "treize", "ru": "тринадцать", "sk": "trinásť", "sl": "trinajst", "sv": "tretton", "sw": "treize", "th": "สิบสาม", "tok": "luka luka tu wan", "tr": "onüç", "uk": "тринадцять", "ur": "تیرہ", "vi": "mười ba", "yo": "thirteen", "zh-tw": "十三", "zh": "十三"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-cifero-dek-kvar", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek kvar",
				"definition": "fourteen",
				"definitions": map[string]interface{}{"en": "fourteen", "nl": "veertien", "de": "vierzehn", "fr": "quatorze", "es": "catorce", "pt": "quatorze", "ar": "أربعة عشر, أربعة عشرة", "be": "четырнадцать", "ca": "catorze", "cs": "čtrnáct", "da": "fjorten", "el": "δεκατέσερεις, δεκατέσσερα", "fa": "چهارده", "frp": "catorzye", "ga": "ceathair déag", "he": "ארבע עשרה", "hi": "fourteen", "hr": "četrnaest", "hu": "tizennégy", "id": "empat belas", "it": "quattordici", "ja": "十四", "kk": "он төрт", "km": "ដប់បួន", "ko": "열넷, 14", "ku": "چهارده", "lo": "fourteen", "mg": "efatra ambin'ny folo", "ms": "empat belas", "my": "fourteen", "pl": "czternaście", "ro": "quatorze", "ru": "четырнадцать", "sk": "štrnásť", "sl": "štirinajst", "sv": "fjorton", "sw": "quatorze", "th": "สิบสี่", "tok": "luka luka tu tu", "tr": "ondört", "uk": "чотирнадцять", "ur": "چودہ", "vi": "mười bốn", "yo": "fourteen", "zh-tw": "十四", "zh": "十四"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-cifero-dek-kvin", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek kvin",
				"definition": "fiveteen",
				"definitions": map[string]interface{}{"en": "fiveteen", "nl": "vijftien", "de": "fünfzehn", "fr": "quinze", "es": "quince", "pt": "quinze", "ar": "خمسة عشر, خمسة عشرة", "be": "пятнадцать", "ca": "quinze", "cs": "patnáct", "da": "femten", "el": "δεκαπέντε", "fa": "پانزده", "frp": "quinzye", "ga": "cúig déag", "he": "חמש עשרה", "hi": "fiveteen", "hr": "petnaest", "hu": "tizenöt", "id": "lima belas", "it": "quindici", "ja": "十五", "kk": "он бес", "km": "ដប់ប្រាំ", "ko": "열다섯, 15", "ku": "پانزده", "lo": "fiveteen", "mg": "dimy  ambin'ny folo", "ms": "lima belas", "my": "fiveteen", "pl": "piętnaście", "ro": "quinze", "ru": "пятнадцать", "sk": "päťnásť", "sl": "petnajst", "sv": "femton", "sw": "quinze", "th": "สิบห้า", "tok": "luka luka luka", "tr": "onbeş", "uk": "п'ятнадцять", "ur": "پندرہ", "vi": "mười lăm", "yo": "fiveteen", "zh-tw": "十五", "zh": "十五"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-cifero-dek-ses", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek ses",
				"definition": "sixteen",
				"definitions": map[string]interface{}{"en": "sixteen", "nl": "zestien", "de": "sechszehn", "fr": "seize", "es": "dieciséis", "pt": "dezesseis", "ar": "ستة عشر, ستة عشرة", "be": "шестнадцать", "ca": "setze", "cs": "šestnáct", "da": "seksten", "el": "δεκαέξι", "fa": "شانزده", "frp": "sêzye", "ga": "sé déag", "he": "שש עשרה", "hi": "sixteen", "hr": "šesnaest", "hu": "tizenhat", "id": "enam belas", "it": "sedici", "ja": "十六", "kk": "он алты", "km": "ដប់ប្រាំមួយ", "ko": "열여섯, 16", "ku": "شانزده", "lo": "sixteen", "mg": "enina  ambin'ny folo", "ms": "enam belas", "my": "sixteen", "pl": "szesnaście", "ro": "seize", "ru": "шестнадцать", "sk": "šestnásť", "sl": "šesnajst", "sv": "sexton", "sw": "seize", "th": "สิบหก", "tok": "luka luka luka wan", "tr": "onaltı", "uk": "шістнадцять", "ur": "سولہ", "vi": "mười sáu", "yo": "sixteen", "zh-tw": "十六", "zh": "十六"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-cifero-dek-sep", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek sep",
				"definition": "seventeen",
				"definitions": map[string]interface{}{"en": "seventeen", "nl": "zeventien", "de": "siebzehn", "fr": "dix-sept", "es": "diecisiete", "pt": "dezessete", "ar": "سبعة عشر, سبعة عشرة", "be": "семнадцать", "ca": "disset", "cs": "sedmnáct", "da": "sytten", "el": "δεκαπτά", "fa": "هفده", "frp": "diéssète", "ga": "seacht déag", "he": "שבע עשרה", "hi": "seventeen", "hr": "sedamnaest", "hu": "tizenhét", "id": "tujuh belas", "it": "diciasette", "ja": "十七", "kk": "он жеті", "km": "ដប់ប្រាំពីរ", "ko": "열일곱, 17", "ku": "هفده", "lo": "seventeen", "mg": "fito ambin'ny folo", "ms": "tujuh belas", "my": "seventeen", "pl": "siedemnaście", "ro": "dix-sept", "ru": "семнадцать", "sk": "sedemnásť", "sl": "sedemnajst", "sv": "sjutton", "sw": "dix-sept", "th": "สิบเจ็ด", "tok": "luka luka luka tu", "tr": "onyedi", "uk": "сімнадцять", "ur": "سترہ", "vi": "mười bảy", "yo": "seventeen", "zh-tw": "十七", "zh": "十七"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-cifero-dek-ok", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek ok",
				"definition": "eightteen",
				"definitions": map[string]interface{}{"en": "eightteen", "nl": "achttien", "de": "achtzehn", "fr": "dix-huit", "es": "dieciocho", "pt": "dezoito", "ar": "ثمانية عشر, ثمانية عشرة", "be": "восемнадцать", "ca": "divuit", "cs": "osmnáct", "da": "atten", "el": "δεκαοκτώ", "fa": "هجده", "frp": "diésuèt", "ga": "ocht déag", "he": "שמונה עשרה", "hi": "eightteen", "hr": "osamnaest", "hu": "tizennyolc", "id": "delapan belas", "it": "diciotto", "ja": "十八", "kk": "он сегіз", "km": "ដប់ប្រាំបី", "ko": "열여덟, 18", "ku": "هجده", "lo": "eightteen", "mg": "valo ambin'ny folo", "ms": "lapan belas", "my": "eightteen", "pl": "osiemnaście", "ro": "dix-huit", "ru": "восемнадцать", "sk": "osemnásť", "sl": "osemnajst", "sv": "arton", "sw": "dix-huit", "th": "สิบแปด", "tok": "luka luka luka tu wan", "tr": "onsekiz", "uk": "вісімнадцять", "ur": "اٹھارہ", "vi": "mười tám", "yo": "eightteen", "zh-tw": "十八", "zh": "十八"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-cifero-dek-naux", typ: "vocab",
			content: map[string]interface{}{
				"word": "dek naŭ",
				"definition": "nineteen",
				"definitions": map[string]interface{}{"en": "nineteen", "nl": "negentien", "de": "neunzehn", "fr": "dix-neuf", "es": "diecinueve", "pt": "dezenove", "ar": "تسعة عشر, تسعة عشرة", "be": "девятнадцать", "ca": "dinou", "cs": "devatenáct", "da": "nitten", "el": "δεκαεννέα", "fa": "نوزده", "frp": "disnou", "ga": "naoi déag", "he": "תשע עשרה", "hi": "nineteen", "hr": "devetnaest", "hu": "tizenkilenc", "id": "sembilan belas", "it": "diciannove", "ja": "十九", "kk": "он тоғыз", "km": "ដប់របា", "ko": "열아홉, 19", "ku": "نوزده", "lo": "nineteen", "mg": "sivy ambin'ny folo", "ms": "sembilan belas", "my": "nineteen", "pl": "dziewiętnaście", "ro": "dix-neuf", "ru": "девятнадцать", "sk": "devätnásť", "sl": "devetnajst", "sv": "nitton", "sw": "dix-neuf", "th": "สิบเก้า", "tok": "luka luka luka tu tu", "tr": "ondokuz", "uk": "дев'ятнадцять", "ur": "انیس", "vi": "mười chín", "yo": "nineteen", "zh-tw": "十九", "zh": "十九"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-cifero-dudek", typ: "vocab",
			content: map[string]interface{}{
				"word": "dudek",
				"definition": "twenty",
				"definitions": map[string]interface{}{"en": "twenty", "nl": "twintig", "de": "zwanzig", "fr": "vingt", "es": "veinte", "pt": "vinte", "ar": "عشرون", "be": "двадцать", "ca": "vint", "cs": "dvacet", "da": "tyve", "el": "είκοσι", "fa": "بیست", "frp": "vint", "ga": "fiche", "he": "עשרים", "hi": "twenty", "hr": "dvadeset", "hu": "húsz", "id": "dua puluh", "it": "venti", "ja": "二十", "kk": "жиырма", "km": "ម្ភៃ", "ko": "스물, 20", "ku": "بیست", "lo": "twenty", "mg": "roa-polo", "ms": "dua puluh", "my": "twenty", "pl": "dwadzieścia", "ro": "vingt", "ru": "двадцать", "sk": "dvadsať", "sl": "dvajset", "sv": "tjugo", "sw": "vingt", "th": "ยี่สิบ", "tok": "mute (nanpa)", "tr": "yirmi", "uk": "двадцять", "ur": "بیس", "vi": "hai mươi", "yo": "twenty", "zh-tw": "二十", "zh": "二十"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-cifero-tridek", typ: "vocab",
			content: map[string]interface{}{
				"word": "tridek",
				"definition": "thirty",
				"definitions": map[string]interface{}{"en": "thirty", "nl": "dertig", "de": "dreißig", "fr": "trente", "es": "treinta", "pt": "trinta", "ar": "ثلاثون", "be": "тридцать", "ca": "trenta", "cs": "třicet", "da": "tredive", "el": "τριάντα", "fa": "سی", "frp": "tranta", "ga": "tríocha", "he": "שלושים", "hi": "thirty", "hr": "trideset", "hu": "harminc", "id": "tiga puluh", "it": "trenta", "ja": "三十", "kk": "отыз", "km": "សាមសិប", "ko": "서른, 30", "ku": "سی", "lo": "thirty", "mg": "telo-polo", "ms": "tiga puluh", "my": "thirty", "pl": "trzydzieści", "ro": "trente", "ru": "тридцать", "sk": "tridsať", "sl": "trideset", "sv": "trettio", "sw": "trente", "th": "สามสิบ", "tok": "mute luka luka", "tr": "otuz", "uk": "тридцять", "ur": "تیس", "vi": "ba mươi", "yo": "thirty", "zh-tw": "三十", "zh": "三十"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-cifero-kvardek", typ: "vocab",
			content: map[string]interface{}{
				"word": "kvardek",
				"definition": "forty",
				"definitions": map[string]interface{}{"en": "forty", "nl": "veerzig", "de": "vierzig", "fr": "quarante", "es": "cuarenta", "pt": "quarenta", "ar": "أربعون", "be": "сорок", "ca": "quaranta", "cs": "čtyřicet", "da": "fjorten", "el": "σαράντα", "fa": "چهل", "frp": "quaranta", "ga": "daichead", "he": "ארבעים", "hi": "forty", "hr": "četrdeset", "hu": "negyven", "id": "empat puluh", "it": "quaranta", "ja": "四十", "kk": "қырық", "km": "សែសិប", "ko": "마흔, 40", "ku": "چهل", "lo": "fourty", "mg": "efa-polo", "ms": "empat puluh", "my": "fourty", "pl": "czterdzieści", "ro": "quarante", "ru": "сорок", "sk": "štyridsať", "sl": "štirideset", "sv": "fyrtio", "sw": "quarante", "th": "สี่สิบ", "tok": "mute mute", "tr": "kırk", "uk": "сорок", "ur": "چالیس", "vi": "bốn mươi", "yo": "forty", "zh-tw": "四十", "zh": "四十"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 22,
		},
		{
			slug: "vortaro-cifero-cent", typ: "vocab",
			content: map[string]interface{}{
				"word": "cent",
				"definition": "hundred",
				"definitions": map[string]interface{}{"en": "hundred", "nl": "honderd", "de": "hundert", "fr": "cent", "es": "cien", "pt": "cem", "ar": "مئة", "be": "сто", "ca": "cent", "cs": "sto", "da": "hundrede", "el": "εκατό", "fa": "صد", "frp": "cent", "ga": "céad", "he": "מאה", "hi": "hundred", "hr": "sto", "hu": "száz", "id": "seratus", "it": "cento", "ja": "百", "kk": "жүз", "km": "មួយ​រយ", "ko": "백, 100", "ku": "صد", "lo": "hundred", "mg": "zato", "ms": "seratus", "my": "hundred", "pl": "pięćdziesiąt", "ro": "cent", "ru": "сто", "sk": "sto", "sl": "sto", "sv": "hundra", "sw": "cent", "th": "หนึ่งร้อย", "tok": "ale (nanpa)", "tr": "yüz", "uk": "сто", "ur": "سو", "vi": "một trăm", "yo": "hundred", "zh-tw": "百", "zh": "一百"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 23,
		},
		{
			slug: "vortaro-cifero-ducent", typ: "vocab",
			content: map[string]interface{}{
				"word": "ducent",
				"definition": "two hundred",
				"definitions": map[string]interface{}{"en": "two hundred", "nl": "tweehonderd", "de": "zweihundert", "fr": "deux cents", "es": "doscientos", "pt": "duzentos", "ar": "مائتان", "be": "двести", "ca": "dos-cents", "cs": "dvě stě", "da": "to hundrede", "el": "διακόσια", "fa": "دویست", "frp": "dous cents", "ga": "dhá chéad", "he": "מאתיים", "hi": "two hundred", "hr": "dvjesto", "hu": "kétszáz", "id": "dua ratus", "it": "duecento", "ja": "二百", "kk": "екі жүз", "km": "ពីរ​រយ", "ko": "이백, 200", "ku": "دویست", "lo": "two hundred", "mg": "roan-jato", "ms": "dua ratus", "my": "two hundred", "pl": "dwieście", "ro": "deux cents", "ru": "двести", "sk": "dvesto", "sl": "dvesto", "sv": "tvåhundra", "sw": "deux cents", "th": "สองร้อย", "tok": "ale ale", "tr": "ikiyüz", "uk": "двісті", "ur": "دو سو", "vi": "hai trăm", "yo": "two hundred", "zh-tw": "二百", "zh": "二百"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 24,
		},
		{
			slug: "vortaro-cifero-tricent", typ: "vocab",
			content: map[string]interface{}{
				"word": "tricent",
				"definition": "three hundred",
				"definitions": map[string]interface{}{"en": "three hundred", "nl": "driehonderd", "de": "dreihundert", "fr": "trois cents", "es": "trescientos", "pt": "trezentos", "ar": "ثلاث مائة", "be": "триста", "ca": "tres-cents", "cs": "tři sta", "da": "tre hundrede", "el": "τριακόσια", "fa": "سیصد", "frp": "três cents", "ga": "trí chéad", "he": "שלוש מאות", "hi": "three hundred", "hr": "tristo", "hu": "háromszáz", "id": "tiga ratus", "it": "trecento", "ja": "三百", "kk": "үш жүз", "km": "បី​រយ", "ko": "삼백, 300", "ku": "سیصد", "lo": "three hundred", "mg": "telon-jato", "ms": "tiga ratus", "my": "three hundred", "pl": "trzysta", "ro": "trois cents", "ru": "триста", "sk": "tristo", "sl": "tristo", "sv": "trehundra", "sw": "trois cents", "th": "สามร้อย", "tok": "ale ale ale", "tr": "üçyüz", "uk": "триста", "ur": "تین سو", "vi": "ba trăm", "yo": "three hundred", "zh-tw": "三百", "zh": "三百"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 25,
		},
		{
			slug: "vortaro-cifero-mil", typ: "vocab",
			content: map[string]interface{}{
				"word": "mil",
				"definition": "thousand",
				"definitions": map[string]interface{}{"en": "thousand", "nl": "duizend", "de": "tausend", "fr": "mille", "es": "mil", "pt": "mil", "ar": "ألف", "be": "тысяча", "ca": "mil", "cs": "tisíc", "da": "tusind", "el": "χίλια", "fa": "هزار", "frp": "mile", "ga": "míle", "he": "אלף", "hi": "thousand", "hr": "tisuću", "hu": "ezer", "id": "seribu", "it": "mille", "ja": "千", "kk": "мың", "km": "មួយ​ពាន់", "ko": "천, 1,000", "ku": "هزار", "lo": "thousand", "mg": "arivo", "ms": "ribu", "my": "thousand", "pl": "tysiąc", "ro": "mille", "ru": "тысяча", "sk": "tisíc", "sl": "tisoč", "sv": "tusen", "sw": "mille", "th": "หนึ่งพัน", "tok": "ale ale ale ale ale ale ale ale ale ale", "tr": "bin", "uk": "тисяча", "ur": "ہزار", "vi": "một nghìn", "yo": "thousand", "zh-tw": "千", "zh": "一千"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 26,
		},
		{
			slug: "vortaro-cifero-dumil", typ: "vocab",
			content: map[string]interface{}{
				"word": "dumil",
				"definition": "two thousand",
				"definitions": map[string]interface{}{"en": "two thousand", "nl": "tweeduizend", "de": "zweitausend", "fr": "deux mille", "es": "dos mil", "pt": "dois mil", "ar": "ألفان", "be": "две тысячи", "ca": "dos mil", "cs": "dva tisíce", "da": "to tusinde", "el": "δύο χιλιάδες", "fa": "دو هزار", "frp": "dous mile", "ga": "dhá mhíle", "he": "אלפיים", "hi": "two thousand", "hr": "dvije tisuće", "hu": "kétezer", "id": "dua ribu", "it": "duemila", "ja": "二千", "kk": "екі мың", "km": "ពីរ​ពាន់", "ko": "이천, 2,000", "ku": "دو هزار", "lo": "two thousand", "mg": "roa arivo", "ms": "dua ribu", "my": "two thousand", "pl": "dwa tysiące", "ro": "deux mille", "ru": "две тысячи", "sk": "dvetisíc", "sl": "dva tisoč", "sv": "tvåtusen", "sw": "deux mille", "th": "สองพัน", "tok": "ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale ale", "tr": "ikibin", "uk": "дві тисячі", "ur": "دو ہزار", "vi": "hai nghìn", "yo": "two thousand", "zh-tw": "二千", "zh": "二千"},
			},
			tags:        []string{"vortaro", "cifero"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-cifero",
			seriesOrder: 27,
		},
		{
			slug: "vortaro-koloro-blanka", typ: "vocab",
			content: map[string]interface{}{
				"word": "blanka",
				"definition": "white",
				"definitions": map[string]interface{}{"en": "white", "nl": "wit, witte", "de": "weiß", "fr": "blanc", "es": "blanco/a", "pt": "branco", "ar": "أبيض", "be": "белый", "ca": "blanc/a", "cs": "bílá", "da": "hvid", "el": "λευκός-η-ο", "fa": "سفید", "frp": "bllanc", "ga": "bán", "he": "לבן", "hi": "white", "hr": "bijela", "hu": "fehér", "id": "putih", "it": "bianco", "ja": "白", "kk": "ақ", "km": "ពណ៌ស", "ko": "하얀", "ku": "سفید", "lo": "white", "mg": "fotsy", "ms": "putih", "my": "white", "pl": "biały", "ro": "blanc", "ru": "белый", "sk": "biela", "sl": "bela", "sv": "vit", "sw": "blanc", "th": "สีขาว", "tok": "walo", "tr": "beyaz", "uk": "білий", "ur": "سفید", "vi": "màu trắng", "yo": "white", "zh-tw": "白", "zh": "白"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-koloro-nigra", typ: "vocab",
			content: map[string]interface{}{
				"word": "nigra",
				"definition": "black",
				"definitions": map[string]interface{}{"en": "black", "nl": "zwart, zwarte", "de": "schwarz", "fr": "noir", "es": "negro/a", "pt": "preto", "ar": "أسود", "be": "чёрный", "ca": "negre/a", "cs": "černá", "da": "sort", "el": "μαύρος-η-ο", "fa": "سیاه, مشکی", "frp": "nêr", "ga": "dubh", "he": "שחור", "hi": "black", "hr": "crna", "hu": "fekete", "id": "hitam", "it": "nero", "ja": "黒", "kk": "қара", "km": "ខ្មៅ", "ko": "검은", "ku": "سیاه, مشکی", "lo": "black", "mg": "mainty", "ms": "hitam", "my": "black", "pl": "czarny", "ro": "noir", "ru": "чёрный", "sk": "čierna", "sl": "črna", "sv": "svart", "sw": "noir", "th": "สีดำ", "tok": "pimeja", "tr": "siyah", "uk": "чорний", "ur": "سیاہ/کالا", "vi": "màu đen", "yo": "black", "zh-tw": "黑", "zh": "黑"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-koloro-rugxa", typ: "vocab",
			content: map[string]interface{}{
				"word": "ruĝa",
				"definition": "red",
				"definitions": map[string]interface{}{"en": "red", "nl": "rood, rode", "de": "rot", "fr": "rouge", "es": "rojo/a", "pt": "vermelho", "ar": "أحمر", "be": "красный", "ca": "roig/roja", "cs": "červená", "da": "rød", "el": "κόκκινος-η-ο", "fa": "سرخ, قرمز", "frp": "rojo", "ga": "dearg", "he": "אדום", "hi": "red", "hr": "crvena", "hu": "piros", "id": "merah", "it": "rosso", "ja": "赤", "kk": "қызыл", "km": "ក្រហម", "ko": "붉은", "ku": "سرخ, قرمز", "lo": "red", "mg": "mena", "ms": "merah", "my": "red", "pl": "czerwony", "ro": "rouge", "ru": "красный", "sk": "červená", "sl": "rdeča", "sv": "röd", "sw": "rouge", "th": "สีแดง", "tok": "loje", "tr": "kırmızı", "uk": "червоний", "ur": "سرخ/لال", "vi": "màu đỏ", "yo": "red", "zh-tw": "紅", "zh": "红"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-koloro-flava", typ: "vocab",
			content: map[string]interface{}{
				"word": "flava",
				"definition": "yellow",
				"definitions": map[string]interface{}{"en": "yellow", "nl": "geel, gele", "de": "gelb", "fr": "jaune", "es": "amarillo/a", "pt": "amarelo", "ar": "أصفر", "be": "жёлтый", "ca": "groc/groga", "cs": "žlutá", "da": "gul", "el": "κίτρινος-η-ο", "fa": "زرد", "frp": "jôno", "ga": "buí", "he": "צהוב", "hi": "yellow", "hr": "žuta", "hu": "sárga", "id": "kuning", "it": "giallo", "ja": "黄色", "kk": "сары", "km": "លឿង", "ko": "노란", "ku": "زرد", "lo": "yellow", "mg": "mavo", "ms": "kuning", "my": "yellow", "pl": "żółty", "ro": "jaune", "ru": "жёлтый", "sk": "žltá", "sl": "rumena", "sv": "gul", "sw": "jaune", "th": "สีเหลือง", "tok": "jelo", "tr": "sarı", "uk": "жовтий", "ur": "پیلا/زرد", "vi": "màu vàng", "yo": "yellow", "zh-tw": "黄", "zh": "黄"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-koloro-verda", typ: "vocab",
			content: map[string]interface{}{
				"word": "verda",
				"definition": "green",
				"definitions": map[string]interface{}{"en": "green", "nl": "groen, groene", "de": "grün", "fr": "vert", "es": "verde", "pt": "verde", "ar": "أخضر", "be": "зелёный", "ca": "verd/a", "cs": "zelená", "da": "grøn", "el": "πράσινος-η-ο", "fa": "سبز", "frp": "vèrt", "ga": "glas - uaine", "he": "ירוק", "hi": "green", "hr": "zelena", "hu": "zöld", "id": "hijau", "it": "verde", "ja": "緑", "kk": "жасыл", "km": "បៃតង", "ko": "녹색의", "ku": "سبز", "lo": "green", "mg": "maintso", "ms": "hijau", "my": "green", "pl": "zielony", "ro": "vert", "ru": "зелёный", "sk": "zelená", "sl": "zelena", "sv": "grön", "sw": "vert", "th": "สีเขียว", "tok": "laso kasi", "tr": "yeşil", "uk": "зелений", "ur": "سبز", "vi": "màu xanh lá", "yo": "green", "zh-tw": "綠", "zh": "青"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-koloro-blua", typ: "vocab",
			content: map[string]interface{}{
				"word": "blua",
				"definition": "blue",
				"definitions": map[string]interface{}{"en": "blue", "nl": "blauw, blauwe", "de": "blau", "fr": "bleu", "es": "azul", "pt": "azul", "ar": "أزرق", "be": "синий", "ca": "blau/blava", "cs": "modrá", "da": "blå", "el": "(ο,η,το) μπλε", "fa": "آبی", "frp": "blu", "ga": "gorm", "he": "כחול", "hi": "blue", "hr": "plava", "hu": "kék", "id": "biru", "it": "blu", "ja": "青", "kk": "көк", "km": "ខៀវ", "ko": "푸른", "ku": "آبی", "lo": "blue", "mg": "manga", "ms": "biru", "my": "blue", "pl": "niebieski", "ro": "bleu", "ru": "синий", "sk": "modrá", "sl": "modra", "sv": "blå", "sw": "bleu", "th": "สีน้ำเงิน", "tok": "laso sewi", "tr": "mavi", "uk": "синій", "ur": "نیلا", "vi": "màu xanh dương", "yo": "blue", "zh-tw": "藍", "zh": "蓝"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-koloro-bruna", typ: "vocab",
			content: map[string]interface{}{
				"word": "bruna",
				"definition": "brown",
				"definitions": map[string]interface{}{"en": "brown", "nl": "bruin, bruine", "de": "braun", "fr": "marron", "es": "marrón", "pt": "marrom", "ar": "بنى", "be": "коричневый", "ca": "marró", "cs": "hnědá", "da": "brun", "el": "(ο,η,το) καφέ", "fa": "قهوه‌ای", "frp": "marron", "ga": "donn", "he": "חום", "hi": "brown", "hr": "smeđa", "hu": "barna", "id": "coklat", "it": "marrone", "ja": "茶色", "kk": "қоңыр", "km": "ត្នោត", "ko": "갈색의", "ku": "قهوه‌ای", "lo": "brown", "mg": "mavo manja", "ms": "warna coklat", "my": "brown", "pl": "brązowy", "ro": "marron", "ru": "коричневый", "sk": "hnedá", "sl": "rjava", "sv": "brun", "sw": "marron", "th": "สีน้ำตาล", "tok": "loje pimeja", "tr": "kahve", "uk": "коричневий", "ur": "بھورا", "vi": "màu nâu", "yo": "brown", "zh-tw": "褐色", "zh": "褐色"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-koloro-griza", typ: "vocab",
			content: map[string]interface{}{
				"word": "griza",
				"definition": "gray",
				"definitions": map[string]interface{}{"en": "gray", "nl": "grijs, grijze", "de": "grau", "fr": "gris", "es": "gris", "pt": "cinza", "ar": "رمادي", "be": "серый", "ca": "gris/a", "cs": "šedá", "da": "grå", "el": "γκριζος-α-ο", "fa": "خاکستری, توسی", "frp": "gris", "ga": "liath", "he": "אפור", "hi": "gray", "hr": "siva", "hu": "szürke", "id": "abu-abu", "it": "grigio", "ja": "灰色", "kk": "сұр", "km": "ប្រផេះ", "ko": "회색의", "ku": "خاکستری, توسی", "lo": "gray", "mg": "mavo vasoka", "ms": "kelabu", "my": "gray", "pl": "szary", "ro": "gris", "ru": "серый", "sk": "sivá", "sl": "siva", "sv": "grå", "sw": "gris", "th": "สีเทา", "tok": "walo pimeja", "tr": "gri", "uk": "сірий", "ur": "سلیٹی", "vi": "màu xám", "yo": "gray", "zh-tw": "灰色", "zh": "灰色"},
			},
			tags:        []string{"vortaro", "koloro"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-koloro",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-konjunkcio-aux", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭ",
				"definition": "or",
				"definitions": map[string]interface{}{"en": "or", "nl": "of", "de": "oder", "fr": "ou", "es": "o", "pt": "ou", "ar": "أو", "be": "или", "ca": "o", "cs": "nebo", "da": "eller", "el": "ή", "fa": "یا", "frp": "ou", "ga": "nó", "he": "או", "hi": "or", "hr": "ili", "hu": "vagy", "id": "atau", "it": "o, oppure", "ja": "または", "kk": "or", "km": "or", "ko": "또는", "ku": "یا", "lo": "or", "mg": "sa ,na", "ms": "atau", "my": "or", "pl": "lub", "ro": "ou", "ru": "или", "sk": "alebo", "sl": "ali", "sv": "eller", "sw": "ou", "th": "หรือ", "tr": "veya", "uk": "або", "ur": "یا", "vi": "hoặc là, hay là", "yo": "or", "zh-tw": "或者", "zh": "或者"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-konjunkcio-kaj", typ: "vocab",
			content: map[string]interface{}{
				"word": "kaj",
				"definition": "and",
				"definitions": map[string]interface{}{"en": "and", "nl": "en", "de": "und", "fr": "et", "es": "y", "pt": "e", "ar": "و", "be": "и", "ca": "i", "cs": "a", "da": "og", "el": "και", "fa": "و", "frp": "et", "ga": "agus", "he": "ו, וגם", "hi": "and", "hr": "i", "hu": "és", "id": "dan", "it": "e", "ja": "～と, そして", "kk": "and", "km": "and", "ko": "그리고", "ku": "و", "lo": "and", "mg": "sy , ary , ka", "ms": "dan", "my": "and", "pl": "i", "ro": "et", "ru": "и", "sk": "a", "sl": "in", "sv": "och", "sw": "et", "th": "และ", "tr": "ve", "uk": "і", "ur": "اور", "vi": "và", "yo": "and", "zh-tw": "和", "zh": "和"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-konjunkcio-ke", typ: "vocab",
			content: map[string]interface{}{
				"word": "ke",
				"definition": "that",
				"definitions": map[string]interface{}{"en": "that", "nl": "dat", "de": "dass", "fr": "que", "es": "que", "pt": "que", "ar": "أن", "be": "что, чтобы", "ca": "que", "cs": "že", "da": "at", "el": "ότι", "fa": "که", "frp": "que", "ga": "go", "he": "ש", "hi": "that", "hr": "da", "hu": "hogy", "id": "bahwa", "it": "che", "ja": "～ということ", "kk": "that", "km": "that", "ko": "~라는 것", "ku": "که", "lo": "that", "mg": "izay", "ms": "bahawa", "my": "that", "pl": "że", "ro": "que", "ru": "что, чтобы", "sk": "že", "sl": "da", "sv": "att", "sw": "que", "th": "ว่า", "tr": "ki", "uk": "що", "ur": "کہ", "vi": "rằng, mà", "yo": "that", "zh-tw": "那", "zh": "那"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-konjunkcio-kvankam", typ: "vocab",
			content: map[string]interface{}{
				"word": "kvankam",
				"definition": "although",
				"definitions": map[string]interface{}{"en": "although", "nl": "hoewel", "de": "obwohl", "fr": "bien que", "es": "aunque", "pt": "embora", "ar": "رغم أن, على الرغم من", "be": "хотя", "ca": "encara que", "cs": "ačkoli", "da": "selvom", "el": "αν και", "fa": "اگرچه, با این که", "frp": "bien que", "ga": "cé go", "he": "אף על פי כן", "hi": "although", "hr": "iako, premda", "hu": "habár", "id": "meskipun", "it": "sebbene, benché", "ja": "～にもかかわらず", "kk": "although", "km": "although", "ko": "~에도 불구하고", "ku": "اگرچه, با این که", "lo": "although", "mg": "na dia ... aza", "ms": "walaupun", "my": "although", "pl": "chociaż", "ro": "bien que", "ru": "хотя", "sk": "hoci", "sl": "čeprav, četudi", "sv": "fastän", "sw": "bien que", "th": "ถึงแม้ว่า", "tr": "rağmen", "uk": "хоч", "ur": "اگرچہ", "vi": "mặc dù", "yo": "although", "zh-tw": "雖然", "zh": "虽然"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-konjunkcio-nek", typ: "vocab",
			content: map[string]interface{}{
				"word": "nek",
				"definition": "neither (… nor)",
				"definitions": map[string]interface{}{"en": "neither (… nor)", "nl": "noch (… noch)", "de": "weder (… noch)", "fr": "ni (… ni)", "es": "ni", "pt": "nem", "ar": "(لا هذا(...ولا ذاك", "be": "ни", "ca": "ni", "cs": "ani", "da": "hverken (… eller)", "el": "ούτε", "fa": "نَه", "frp": "ni (… ni)", "ga": "ní (… ná)", "he": "וגם לא", "hi": "neither (… nor)", "hr": "niti, ni", "hu": "sem ... sem", "id": "bukan (… ataupun)", "it": "né", "ja": "～でもない", "kk": "neither (… nor)", "km": "neither (… nor)", "ko": "~도 아닌", "ku": "نَه", "lo": "neither (… nor)", "mg": "na (… na)", "ms": "tidak", "my": "neither (… nor)", "pl": "ani (… ani)", "ro": "ni (… ni)", "ru": "ни", "sk": "ani", "sl": "niti", "sv": "varken (… eller)", "sw": "ni (… ni)", "th": "และไม่", "tr": "ne o... ne de bu...", "uk": "ні... ні", "ur": "نہ ۔۔۔ نہ", "vi": "không (… mà cũng không)", "yo": "neither (… nor)", "zh-tw": "既非, 也非", "zh": "非，非"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-konjunkcio-ol", typ: "vocab",
			content: map[string]interface{}{
				"word": "ol",
				"definition": "than",
				"definitions": map[string]interface{}{"en": "than", "nl": "dan", "de": "als", "fr": "que (plus/moins… que)", "es": "que (comparativo)", "pt": "ou (comparativo)", "ar": "من", "be": "чем", "ca": "que (comparatiu)", "cs": "než", "da": "end", "el": "παρά, από", "fa": "از این که, از, نسبت به", "frp": "que (plus/moins… que)", "ga": "ná", "he": "מאשר", "hi": "than", "hr": "nego", "hu": "mint", "id": "daripada", "it": "di (comparativo)", "ja": "～よりも", "kk": "than", "km": "than", "ko": "~보다", "ku": "از این که, از, نسبت به", "lo": "than", "mg": "noho (mihoatra/latsaka...noho)", "ms": "daripada", "my": "than", "pl": "niż", "ro": "que (plus/moins… que)", "ru": "чем", "sk": "než", "sl": "kot", "sv": "än (vid jämförelse)", "sw": "que (plus/moins… que)", "th": "กว่า", "tr": "-dan (karşılaştırma)", "uk": "ніж (більше/менше... ніж)", "ur": "سے", "vi": "hơn", "yo": "than", "zh-tw": "比", "zh": "多于"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-konjunkcio-se", typ: "vocab",
			content: map[string]interface{}{
				"word": "se",
				"definition": "if",
				"definitions": map[string]interface{}{"en": "if", "nl": "indien, als, op voorwaarde dat", "de": "wenn", "fr": "si", "es": "si (condicional)", "pt": "se", "ar": "إذا", "be": "если", "ca": "si (condicional)", "cs": "když", "da": "hvis", "el": "αν", "fa": "اگر", "frp": "si", "ga": "má, dá", "he": "אם", "hi": "if", "hr": "ako", "hu": "ha", "id": "jika", "it": "se", "ja": "もし", "kk": "if", "km": "if", "ko": "만약", "ku": "اگر", "lo": "if", "mg": "raha", "ms": "jika", "my": "if", "pl": "jeśli, jeżeli", "ro": "si", "ru": "если", "sk": "ak, keby", "sl": "če", "sv": "om", "sw": "si", "th": "ถ้า", "tr": "eğer", "uk": "якщо", "ur": "اگر", "vi": "nếu như, giá mà, cho dù", "yo": "if", "zh-tw": "假如, 如果", "zh": "假如"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-konjunkcio-sed", typ: "vocab",
			content: map[string]interface{}{
				"word": "sed",
				"definition": "but",
				"definitions": map[string]interface{}{"en": "but", "nl": "maar", "de": "aber", "fr": "mais", "es": "pero", "pt": "mas", "ar": "لكن", "be": "но", "ca": "però", "cs": "ale", "da": "men", "el": "αλλά", "fa": "اما", "frp": "mais", "ga": "ach", "he": "אבל", "hi": "but", "hr": "a, ali, već, nego", "hu": "de", "id": "tapi", "it": "ma", "ja": "しかし", "kk": "but", "km": "but", "ko": "그러나", "ku": "اما", "lo": "but", "mg": "fa , saingy , faingy", "ms": "tetapi", "my": "but", "pl": "ale", "ro": "mais", "ru": "но", "sk": "ale", "sl": "a, ter, ampak", "sv": "men", "sw": "mais", "th": "แต่", "tr": "fakat, ama", "uk": "але", "ur": "لیکن", "vi": "nhưng mà", "yo": "but", "zh-tw": "但是", "zh": "但是"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-konjunkcio-cxar", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉar",
				"definition": "because, for",
				"definitions": map[string]interface{}{"en": "because, for", "nl": "omdat, want", "de": "weil, denn", "fr": "parce que, pour (que)", "es": "porque", "pt": "porque", "ar": "لأن, لأجل", "be": "потому что, так как, ибо", "ca": "perquè", "cs": "protože, kvůli", "da": "fordi, for", "el": "επειδή, διότι", "fa": "زیرا, چون", "frp": "parce que, pour (que)", "ga": "mar, toisc go", "he": "בגלל", "hi": "because, for", "hr": "jer, budući da", "hu": "mert, mivel", "id": "karena", "it": "perché", "ja": "なぜなら, ～なので", "kk": "because, for", "km": "because, for", "ko": "왜냐하면", "ku": "زیرا, چون", "lo": "because, for", "mg": "satria, noho izany, ka izay", "ms": "kerana", "my": "because, for", "pl": "bo, ponieważ", "ro": "parce que, pour (que)", "ru": "потому что, так как, ибо", "sk": "pretože, lebo", "sl": "ker, kajti", "sv": "därför att, eftersom", "sw": "parce que, pour (que)", "th": "เพราะว่า", "tr": "çünkü", "uk": "тому що, бо", "ur": "کیونکہ, کے لیے", "vi": "bởi vì, tại vì", "yo": "because, for", "zh-tw": "因為", "zh": "因为"},
			},
			tags:        []string{"vortaro", "konjunkcio"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-konjunkcio",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-monato-januaro", typ: "vocab",
			content: map[string]interface{}{
				"word": "januaro",
				"definition": "January",
				"definitions": map[string]interface{}{"en": "January", "nl": "januari", "de": "Januar", "fr": "janvier", "es": "enero", "pt": "Janeiro", "ar": "يناير", "be": "январь", "ca": "gener", "cs": "leden", "da": "januar", "el": "Ιανουάριος", "fa": "ژانویه", "frp": "janviér", "ga": "Eanáir", "he": "ינואר", "hi": "January", "hr": "siječanj", "hu": "január", "id": "Januari", "it": "gennaio", "ja": "一月", "kk": "қаңтар", "km": "ខែមករា", "ko": "1월", "ku": "ژانویه", "lo": "January", "mg": "janoary", "ms": "Januari", "my": "January", "pl": "styczeń", "ro": "janvier", "ru": "январь", "sk": "január", "sl": "januar", "sv": "januari", "sw": "janvier", "th": "มกราคม", "tok": "tenpo mun #1", "tr": "Ocak", "uk": "січень", "ur": "جنوری", "vi": "tháng một, tháng giêng", "yo": "January", "zh-tw": "一月", "zh": "一月，正月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-monato-februaro", typ: "vocab",
			content: map[string]interface{}{
				"word": "februaro",
				"definition": "February",
				"definitions": map[string]interface{}{"en": "February", "nl": "februari", "de": "Februar", "fr": "février", "es": "febrero", "pt": "Fevereiro", "ar": "فبراير", "be": "февраль", "ca": "febrer", "cs": "únor", "da": "februar", "el": "Φεβρουάριος", "fa": "فوریه", "frp": "fèvriér", "ga": "Feabhra", "he": "פברואר", "hi": "February", "hr": "veljača", "hu": "február", "id": "Februari", "it": "febbraio", "ja": "二月", "kk": "ақпан", "km": "ខែកុម្ភៈ", "ko": "2월", "ku": "فوریه", "lo": "February", "mg": "febroary", "ms": "Februari", "my": "February", "pl": "luty", "ro": "février", "ru": "февраль", "sk": "február", "sl": "februar", "sv": "februari", "sw": "février", "th": "กุมภาพันธ์", "tok": "tenpo mun #2", "tr": "Şubat", "uk": "лютий", "ur": "فروری", "vi": "tháng hai", "yo": "February", "zh-tw": "二月", "zh": "二月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-monato-marto", typ: "vocab",
			content: map[string]interface{}{
				"word": "marto",
				"definition": "March",
				"definitions": map[string]interface{}{"en": "March", "nl": "maart", "de": "März", "fr": "mars", "es": "marzo", "pt": "Março", "ar": "مارس", "be": "март", "ca": "març", "cs": "březen", "da": "marts", "el": "Μάρτιος", "fa": "مارس", "frp": "mârs", "ga": "Márta", "he": "מרץ", "hi": "March", "hr": "ožujak", "hu": "március", "id": "Maret", "it": "marzo", "ja": "三月", "kk": "наурыз", "km": "ខែមីនា", "ko": "3월", "ku": "مارس", "lo": "March", "mg": "martsa", "ms": "Mac", "my": "March", "pl": "marzec", "ro": "mars", "ru": "март", "sk": "marec", "sl": "marec", "sv": "mars", "sw": "mars", "th": "มีนาคม", "tok": "tenpo mun #3", "tr": "Mart", "uk": "березень", "ur": "مارچ", "vi": "tháng ba", "yo": "March", "zh-tw": "三月", "zh": "三月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-monato-aprilo", typ: "vocab",
			content: map[string]interface{}{
				"word": "aprilo",
				"definition": "April",
				"definitions": map[string]interface{}{"en": "April", "nl": "april", "de": "April", "fr": "avril", "es": "abril", "pt": "Abril", "ar": "أبريل", "be": "апрель", "ca": "abril", "cs": "duben", "da": "april", "el": "Απρίλιος", "fa": "آوریل", "frp": "avril", "ga": "Aibreán", "he": "אפריל", "hi": "April", "hr": "travanj", "hu": "április", "id": "April", "it": "aprile", "ja": "四月", "kk": "сәуір", "km": "ខែមេសា", "ko": "4월", "ku": "آوریل", "lo": "April", "mg": "aprily", "ms": "April", "my": "April", "pl": "kwiecień", "ro": "avril", "ru": "апрель", "sk": "apríl", "sl": "april", "sv": "april", "sw": "avril", "th": "เมษายน", "tok": "tenpo mun #4", "tr": "Nisan", "uk": "квітень", "ur": "اپریل", "vi": "tháng bốn, tháng tư", "yo": "April", "zh-tw": "四月", "zh": "四月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-monato-majo", typ: "vocab",
			content: map[string]interface{}{
				"word": "majo",
				"definition": "May",
				"definitions": map[string]interface{}{"en": "May", "nl": "mei", "de": "Mai", "fr": "mai", "es": "mayo", "pt": "Maio", "ar": "مايو", "be": "май", "ca": "maig", "cs": "květen", "da": "maj", "el": "Μάιος", "fa": "می", "frp": "mê", "ga": "Bealtaine", "he": "מאי", "hi": "May", "hr": "svibanj", "hu": "május", "id": "Mei", "it": "maggio", "ja": "五月", "kk": "мамыр", "km": "ឧសភា", "ko": "5월", "ku": "می", "lo": "May", "mg": "mey", "ms": "Mei", "my": "May", "pl": "maj", "ro": "mai", "ru": "май", "sk": "máj", "sl": "maj", "sv": "maj", "sw": "mai", "th": "พฤษภาคม", "tok": "tenpo mun #5", "tr": "Mayıs", "uk": "травень", "ur": "مئی", "vi": "tháng năm", "yo": "May", "zh-tw": "五月", "zh": "五月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-monato-junio", typ: "vocab",
			content: map[string]interface{}{
				"word": "junio",
				"definition": "June",
				"definitions": map[string]interface{}{"en": "June", "nl": "juni", "de": "Juni", "fr": "juin", "es": "junio", "pt": "Junho", "ar": "يونيو", "be": "июнь", "ca": "juny", "cs": "červen", "da": "juni", "el": "Ιούνιος", "fa": "ژوئن", "frp": "juen", "ga": "Meitheamh", "he": "יוני", "hi": "June", "hr": "lipanj", "hu": "június", "id": "Juni", "it": "giugno", "ja": "六月", "kk": "маусым", "km": "ខែមិថុនា", "ko": "6월", "ku": "ژوئن", "lo": "June", "mg": "jona", "ms": "Jun", "my": "June", "pl": "czerwiec", "ro": "juin", "ru": "июнь", "sk": "jún", "sl": "junij", "sv": "juni", "sw": "juin", "th": "มิถุนายน", "tok": "tenpo mun #6", "tr": "Haziran", "uk": "червень", "ur": "جون", "vi": "tháng sáu", "yo": "June", "zh-tw": "六月", "zh": "六月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-monato-julio", typ: "vocab",
			content: map[string]interface{}{
				"word": "julio",
				"definition": "July",
				"definitions": map[string]interface{}{"en": "July", "nl": "juli", "de": "Juli", "fr": "juillet", "es": "julio", "pt": "Julho", "ar": "يوليو", "be": "июль", "ca": "juliol", "cs": "červenec", "da": "juli", "el": "Ιούλιος", "fa": "ژوئیه", "frp": "julyèt", "ga": "Iúil", "he": "יולי", "hi": "July", "hr": "srpanj", "hu": "július", "id": "Juli", "it": "luglio", "ja": "七月", "kk": "шілде", "km": "ខែកក្កដា", "ko": "7월", "ku": "ژوئیه", "lo": "July", "mg": "jolay", "ms": "Julai", "my": "July", "pl": "lipiec", "ro": "juillet", "ru": "июль", "sk": "júl", "sl": "julij", "sv": "juli", "sw": "juillet", "th": "กรกฎาคม", "tok": "tenpo mun #7", "tr": "Temmuz", "uk": "липень", "ur": "جولائی", "vi": "tháng bảy", "yo": "July", "zh-tw": "七月", "zh": "七月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-monato-auxgusto", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭgusto",
				"definition": "August",
				"definitions": map[string]interface{}{"en": "August", "nl": "augustus", "de": "August", "fr": "août", "es": "agosto", "pt": "Agosto", "ar": "أغسطس", "be": "август", "ca": "agost", "cs": "srpen", "da": "august", "el": "Αύγουστος", "fa": "آگوست", "frp": "out", "ga": "Lúnasa", "he": "אוגוסט", "hi": "August", "hr": "kolovoz", "hu": "augusztus", "id": "Agustus", "it": "agosto", "ja": "八月", "kk": "тамыз", "km": "ខែសីហា", "ko": "8월", "ku": "آگوست", "lo": "August", "mg": "aogositra", "ms": "Ogos", "my": "August", "pl": "sierpień", "ro": "août", "ru": "август", "sk": "august", "sl": "avgust", "sv": "augusti", "sw": "août", "th": "สิงหาคม", "tok": "tenpo mun #8", "tr": "Ağustos", "uk": "серпень", "ur": "اگست", "vi": "tháng tám", "yo": "August", "zh-tw": "八月", "zh": "八月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-monato-septembro", typ: "vocab",
			content: map[string]interface{}{
				"word": "septembro",
				"definition": "September",
				"definitions": map[string]interface{}{"en": "September", "nl": "september", "de": "September", "fr": "septembre", "es": "septiembre", "pt": "Setembro", "ar": "سبتمبر, أيلول", "be": "сентябрь", "ca": "setembre", "cs": "září", "da": "september", "el": "Σεπτέμβριος", "fa": "سپتامبر", "frp": "sèptembro", "ga": "Meán Fómhair", "he": "ספטמבר", "hi": "September", "hr": "rujan", "hu": "szeptember", "id": "September", "it": "settembre", "ja": "九月", "kk": "қыркүйек", "km": "ខែកញ្ញា", "ko": "9월", "ku": "سپتامبر", "lo": "September", "mg": "septambra", "ms": "September", "my": "September", "pl": "wrzesień", "ro": "septembre", "ru": "сентябрь", "sk": "september", "sl": "september", "sv": "september", "sw": "septembre", "th": "กันยายน", "tok": "tenpo mun #9", "tr": "Eylül", "uk": "вересень", "ur": "ستمبر", "vi": "tháng chín", "yo": "September", "zh-tw": "九月", "zh": "九月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-monato-oktobro", typ: "vocab",
			content: map[string]interface{}{
				"word": "oktobro",
				"definition": "October",
				"definitions": map[string]interface{}{"en": "October", "nl": "oktober", "de": "Oktober", "fr": "octobre", "es": "octubre", "pt": "Outubro", "ar": "أكتوبر", "be": "октябрь", "ca": "octubre", "cs": "říjen", "da": "oktober", "el": "Οκτώβριος", "fa": "اکتبر", "frp": "octobro", "ga": "Deireadh Fómhair", "he": "אוקטובר", "hi": "October", "hr": "listopad", "hu": "október", "id": "Oktober", "it": "ottobre", "ja": "十月", "kk": "қазан", "km": "ខែតុលា", "ko": "10월", "ku": "اکتبر", "lo": "October", "mg": "oktobra", "ms": "Oktober", "my": "October", "pl": "październik", "ro": "octobre", "ru": "октябрь", "sk": "október", "sl": "oktober", "sv": "oktober", "sw": "octobre", "th": "ตุลาคม", "tok": "tenpo mun #10", "tr": "Ekim", "uk": "жовтень", "ur": "اکتوبر", "vi": "tháng mười", "yo": "October", "zh-tw": "十月", "zh": "十月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-monato-novembro", typ: "vocab",
			content: map[string]interface{}{
				"word": "novembro",
				"definition": "November",
				"definitions": map[string]interface{}{"en": "November", "nl": "november", "de": "November", "fr": "novembre", "es": "noviembre", "pt": "Novembro", "ar": "نوفمبر", "be": "ноябрь", "ca": "novembre", "cs": "listopad", "da": "november", "el": "Νοέμβριος", "fa": "نوامبر", "frp": "novembro", "ga": "Samhain", "he": "נובמבר", "hi": "November", "hr": "studeni", "hu": "november", "id": "November", "it": "novembre", "ja": "十一月", "kk": "қараша", "km": "ខែវិច្ឆិកា", "ko": "11월", "ku": "نوامبر", "lo": "November", "mg": "novambra", "ms": "November", "my": "November", "pl": "listopad", "ro": "novembre", "ru": "ноябрь", "sk": "november", "sl": "november", "sv": "november", "sw": "novembre", "th": "พฤศจิกายน", "tok": "tenpo mun #11", "tr": "Kasım", "uk": "листопад", "ur": "نومبر", "vi": "tháng mười một", "yo": "November", "zh-tw": "十一月", "zh": "十一月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-monato-decembro", typ: "vocab",
			content: map[string]interface{}{
				"word": "decembro",
				"definition": "December",
				"definitions": map[string]interface{}{"en": "December", "nl": "december", "de": "Dezember", "fr": "décembre", "es": "diciembre", "pt": "Dezembro", "ar": "ديسمبر", "be": "декабрь", "ca": "desembre", "cs": "prosinec", "da": "december", "el": "Δεκέμβριος", "fa": "دسامبر", "frp": "dècembro", "ga": "Nollaig", "he": "דצמבר", "hi": "December", "hr": "prosinac", "hu": "december", "id": "Desember", "it": "dicembre", "ja": "十二月", "kk": "желтоқсан", "km": "ខែធ្នូ", "ko": "12월", "ku": "دسامبر", "lo": "December", "mg": "désambra", "ms": "Disember", "my": "December", "pl": "grudzień", "ro": "décembre", "ru": "декабрь", "sk": "december", "sl": "december", "sv": "december", "sw": "décembre", "th": "ธันวาคม", "tok": "tenpo mun #12", "tr": "Aralık", "uk": "грудень", "ur": "دسمبر", "vi": "tháng mười hai, tháng chạp", "yo": "December", "zh-tw": "十二月", "zh": "十二月"},
			},
			tags:        []string{"vortaro", "monato"},
			source:      "La Zagreba Metodo",
			rating: 900, rd: 200,
			seriesSlug:  "vortaro-monato",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-prepozicio-al", typ: "vocab",
			content: map[string]interface{}{
				"word": "al",
				"definition": "to, in the direction of",
				"definitions": map[string]interface{}{"en": "to, in the direction of", "nl": "naar, aan", "de": "nach, zu", "fr": "à, dans la direction de", "es": "a", "pt": "a, na direção de", "ar": "إلى, في اتجاه", "be": "к, на, в, дательный падеж", "ca": "a, cap a, vers", "cs": "k, ke", "da": "til, i retning af", "el": "εις, προς", "fa": "به, به سوی", "frp": "à, dans la direction de", "ga": "chuig, i dtreo", "he": "אל, בכיוון", "hi": "to, in the direction of", "hr": "k, ka, prema", "hu": "-nak/nek, -hoz/hez/höz, felé", "id": "ke, menuju", "it": "a (complemento di termine), a, verso (moto a luogo)", "ja": "へ, の方向へ", "kk": "to, in the direction of", "km": "to, in the direction of", "ko": "~ 까지, ~의 방향으로", "ku": "به, به سوی", "lo": "to, in the direction of", "mg": "ao, amin'ny lalana mankany", "ms": "ke", "my": "to, in the direction of", "pl": "do, ku", "ro": "à, dans la direction de", "ru": "к, на, в, дательный падеж", "sk": "k, ku", "sl": "k", "sv": "till, åt", "sw": "à, dans la direction de", "th": "ถึง, ไปยัง", "tok": "to, in the direction of", "tr": "-e, -a, yönünde", "uk": "прийменник, що відповідає давальному відмінку, до", "ur": "کو, کی طرف", "vi": "tới, đi đến", "yo": "to, in the direction of", "zh-tw": "向, 往", "zh": "向, 往一个方向"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-prepozicio-anstataux", typ: "vocab",
			content: map[string]interface{}{
				"word": "anstataŭ",
				"definition": "instead of",
				"definitions": map[string]interface{}{"en": "instead of", "nl": "in plaats van", "de": "anstatt", "fr": "à la place de", "es": "en vez de", "pt": "ao invés, em vez", "ar": "بدلا من", "be": "вместо", "ca": "en comptes de", "cs": "namísto", "da": "i stedet for", "el": "αντί για", "fa": "به جای", "frp": "à la place de", "ga": "in ionad", "he": "במקום", "hi": "instead of", "hr": "umjesto", "hu": "helyett", "id": "alih-alih", "it": "invece", "ja": "の代わりに", "kk": "instead of", "km": "instead of", "ko": "~ 대신에", "ku": "به جای", "lo": "instead of", "mg": "fa tsy", "ms": "bukannya", "my": "instead of", "pl": "zamiast", "ro": "à la place de", "ru": "вместо", "sk": "namiesto", "sl": "namesto", "sv": "istället för", "sw": "à la place de", "th": "แทน", "tok": "instead of", "tr": "yerine", "uk": "замість", "ur": "کی بجائے", "vi": "thay vì", "yo": "instead of", "zh-tw": "並非, 取代", "zh": "而非"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-prepozicio-antaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "antaŭ",
				"definition": "before, in front of",
				"definitions": map[string]interface{}{"en": "before, in front of", "nl": "vóór", "de": "vor", "fr": "avant, devant", "es": "ante, antes de, delante de", "pt": "antes, na frente de", "ar": "قبل, أمام", "be": "перед, прежде", "ca": "davant, abans", "cs": "před", "da": "før, foran", "el": "προ, πριν, μπροστά από", "fa": "پیش از, قبل, جلوی", "frp": "avant, devant", "ga": "roimh, os comhair", "he": "לפני", "hi": "before, in front of", "hr": "prije, ispred", "hu": "előtt", "id": "sebelum, di depan", "it": "prima, davanti", "ja": "の前の, の前へ", "kk": "before, in front of", "km": "before, in front of", "ko": "~ 이전에 (시간), ~ 앞에 (공간)", "ku": "پیش از, قبل, جلوی", "lo": "before, in front of", "mg": "aloha, mialoha", "ms": "sebelum, di hadapan", "my": "before, in front of", "pl": "przed", "ro": "avant, devant", "ru": "перед, прежде", "sk": "pred", "sl": "pred", "sv": "framför, före", "sw": "avant, devant", "th": "ข้างหน้า, ก่อน", "tok": "before, in front of", "tr": "önce, önünde", "uk": "перед, заздалегідь", "ur": "پہلے, کے اگے", "vi": "ở ngay phía trước, trước khi", "yo": "before, in front of", "zh-tw": "之前, 前面", "zh": "之前, 前面"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-prepozicio-apud", typ: "vocab",
			content: map[string]interface{}{
				"word": "apud",
				"definition": "next to",
				"definitions": map[string]interface{}{"en": "next to", "nl": "naast", "de": "neben", "fr": "à côté de", "es": "cerca de", "pt": "ao lado de", "ar": "بجوار", "be": "около, возле", "ca": "al costat de", "cs": "vedle", "da": "ved siden af", "el": "κοντά", "fa": "نزدیک", "frp": "à côté de", "ga": "in aice", "he": "ליד", "hi": "next to", "hr": "pored, pokraj", "hu": "mellett", "id": "di dekat, di samping", "it": "accanto", "ja": "のそばで, のそばへ", "kk": "next to", "km": "next to", "ko": "~의 곁에", "ku": "نزدیک", "lo": "next to", "mg": "eo akaikiny", "ms": "dekat", "my": "next to", "pl": "obok, przy", "ro": "à côté de", "ru": "около, возле", "sk": "vedľa", "sl": "poleg", "sv": "bredvid", "sw": "à côté de", "th": "ถัดจาก", "tok": "next to", "tr": "yanında", "uk": "коло, біля, поряд, поруч", "ur": "کے نزدیک", "vi": "bên cạnh", "yo": "next to", "zh-tw": "附近", "zh": "附近"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-prepozicio-da", typ: "vocab",
			content: map[string]interface{}{
				"word": "da",
				"definition": "of",
				"definitions": map[string]interface{}{"en": "of", "nl": "van (een hoeveelheid)", "de": "von (Menge)", "fr": "de", "es": "de (cantidad)", "pt": "de (quantidade)", "ar": "من", "be": "притяжательный предлог, указывающий на количество", "ca": "de (quantitat)", "cs": "(předložka množství)", "da": "af", "el": "από (ποσοτικά)", "fa": "از (مربوط به مقدار)", "frp": "de", "ga": "de", "he": "של", "hi": "of", "hr": "od (partitivni genitiv)", "hu": "mennyiségek után", "id": "dari, tentang", "it": "di (quantità)", "ja": "の", "kk": "of", "km": "of", "ko": "~한 수량의", "ku": "از (مربوط به مقدار)", "lo": "of", "mg": "ny", "ms": "daripada", "my": "of", "pl": "zastępuje dopełniacz (po słowach oznaczających miarę, wagę itp.)", "ro": "de", "ru": "притяжательный предлог, указывающий на количество", "sk": "(predložka množstva)", "sl": "od", "sv": "utav, med (mängd)", "sw": "de", "th": "ของ (ปริมาณ)", "tok": "of", "tr": "(miktar belirtir)", "uk": "прийменник, що позначає міру, кількість", "ur": "of", "vi": "(chỉ số lượng)", "yo": "of", "zh-tw": "的 (數量)", "zh": "的 (数量)"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-prepozicio-de", typ: "vocab",
			content: map[string]interface{}{
				"word": "de",
				"definition": "of, from",
				"definitions": map[string]interface{}{"en": "of, from", "nl": "van", "de": "von", "fr": "de, depuis", "es": "de, desde", "pt": "de (propriedade)", "ar": "من, من عند", "be": "притяжательный предлог, указывающий на качество", "ca": "de, des de", "cs": "od", "da": "af, fra", "el": "από, του (γενική πτώση)", "fa": "از (مبدأ), ی، ـِ (ساختن ترکیب اضافی)", "frp": "de, depuis", "ga": "de, ó", "he": "של, מ", "hi": "of, from", "hr": "od", "hu": "-nak a/nek a, -tól/től", "id": "dari, tentang", "it": "di, da", "ja": "の, から", "kk": "of, from", "km": "of, from", "ko": "~ 의, ~ 로 부터, ~ 에 의해서", "ku": "از (مبدأ), ی، ـِ (ساختن ترکیب اضافی)", "lo": "of, from", "mg": "ny, hatramy", "ms": "dari", "my": "of, from", "pl": "od, z", "ro": "de, depuis", "ru": "притяжательный предлог, указывающий на качество", "sk": "od", "sl": "od", "sv": "från, av", "sw": "de, depuis", "th": "ของ, จาก, แห่ง, โดย", "tok": "of, from", "tr": "-den", "uk": "приймменник, що служить для передачі родового відмінку, від, з, зі (простір), від, з (час)", "ur": "کا، کے، کی, سے", "vi": "của, làm từ", "yo": "of, from", "zh-tw": "屬, 從", "zh": "于, 从"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-prepozicio-dum", typ: "vocab",
			content: map[string]interface{}{
				"word": "dum",
				"definition": "during",
				"definitions": map[string]interface{}{"en": "during", "nl": "gedurende, terwijl", "de": "während", "fr": "pendant", "es": "durante, mediante", "pt": "durante", "ar": "طوال, أثناء, خلال", "be": "во время, в течение", "ca": "durant, mentres", "cs": "během", "da": "under, undervejs, mens", "el": "ενώ, κατά τη διάρκεια", "fa": "طی, در حالی که", "frp": "pendant", "ga": "i rith", "he": "במשך", "hi": "during", "hr": "dok, za vrijeme", "hu": "alatt (időtartam)", "id": "selama, ketika", "it": "durante", "ja": "の間", "kk": "during", "km": "during", "ko": "~ 동안", "ku": "طی, در حالی که", "lo": "during", "mg": "mandritra", "ms": "semasa", "my": "during", "pl": "podczas", "ro": "pendant", "ru": "во время, в течение", "sk": "behom, počas", "sl": "med", "sv": "medan, under", "sw": "pendant", "th": "ในระหว่าง", "tok": "during", "tr": "sırasında", "uk": "поки, у той час як, під час, протягом, упродовж", "ur": "کے دوران", "vi": "trong lúc", "yo": "during", "zh-tw": "期間", "zh": "期间"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-prepozicio-ekde", typ: "vocab",
			content: map[string]interface{}{
				"word": "ekde",
				"definition": "since",
				"definitions": map[string]interface{}{"en": "since", "nl": "sedert", "de": "seit", "fr": "depuis", "es": "desde", "pt": "desde", "ar": "منذ", "be": "с, начиная с, от", "ca": "des de", "cs": "od (čas)", "da": "siden", "el": "από τότε", "fa": "از آغاز", "frp": "depuis", "ga": "ó", "he": "מאז", "hi": "since", "hr": "od", "hu": "óta", "id": "sejak", "it": "da (tempo)", "ja": "から", "kk": "since", "km": "since", "ko": "~ 로 부터", "ku": "از آغاز", "lo": "since", "mg": "hatramy", "ms": "sejak", "my": "since", "pl": "od, odkąd", "ro": "depuis", "ru": "с, начиная с, от", "sk": "od (čas)", "sl": "od", "sv": "från och med", "sw": "depuis", "th": "ตั้งแต่", "tok": "since", "tr": "-den beri", "uk": "від, починаючи з", "ur": "سے", "vi": "kể từ khi", "yo": "since", "zh-tw": "自從", "zh": "自从"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-prepozicio-ekster", typ: "vocab",
			content: map[string]interface{}{
				"word": "ekster",
				"definition": "outside of",
				"definitions": map[string]interface{}{"en": "outside of", "nl": "buiten", "de": "außerhalb", "fr": "hors de", "es": "fuera de", "pt": "do lado de fora", "ar": "خارج من", "be": "вне, снаружи", "ca": "fora de", "cs": "mimo", "da": "undenfor", "el": "εκτός, έξω από", "fa": "بیرون", "frp": "hors de", "ga": "lasmuigh de", "he": "מחוץ", "hi": "outside of", "hr": "izvan", "hu": "kívül (hely)", "id": "di luar", "it": "fuori", "ja": "の外で, の外へ", "kk": "outside of", "km": "outside of", "ko": "~의 바깥에", "ku": "بیرون", "lo": "outside of", "mg": "ivelan'ny", "ms": "di luar", "my": "outside of", "pl": "oprócz, poza", "ro": "hors de", "ru": "вне, снаружи", "sk": "mimo", "sl": "izven", "sv": "utanför", "sw": "hors de", "th": "ข้างนอก", "tok": "outside of", "tr": "dışında", "uk": "поза", "ur": "کے باہر", "vi": "ở ngoài", "yo": "outside of", "zh-tw": "外", "zh": "外"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-prepozicio-el", typ: "vocab",
			content: map[string]interface{}{
				"word": "el",
				"definition": "from, out of",
				"definitions": map[string]interface{}{"en": "from, out of", "nl": "uit", "de": "aus", "fr": "de, hors de", "es": "de", "pt": "de (origem)", "ar": "من, بعيدا عن", "be": "из", "ca": "de (origen, composició)", "cs": "z, ze", "da": "fra, ud af, af", "el": "εκ, μέσα από", "fa": "از, از میان یک گروه, از درون, از جنس", "frp": "de, hors de", "ga": "ó, amach as", "he": "מתוך", "hi": "from, out of", "hr": "iz, od", "hu": "-ból/ből", "id": "dari", "it": "da (estrazione)", "ja": "から, の中から", "kk": "from, out of", "km": "from, out of", "ko": "~로 부터, ~ 중에서", "ku": "از, از میان یک گروه, از درون, از جنس", "lo": "from, out of", "mg": "ny, ivelan'ny", "ms": "daripada", "my": "from, out of", "pl": "od, z, ze", "ro": "de, hors de", "ru": "из", "sk": "z, zo", "sl": "iz, od", "sv": "ut ur, utav (ursprung, urval m.m.)", "sw": "de, hors de", "th": "จาก, ออกจาก", "tok": "from, out of", "tr": "-den, -den dışarı", "uk": "з, зі", "ur": "سے, out of", "vi": "đến từ", "yo": "from, out of", "zh-tw": "出自", "zh": "从, 里头的"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-prepozicio-en", typ: "vocab",
			content: map[string]interface{}{
				"word": "en",
				"definition": "in",
				"definitions": map[string]interface{}{"en": "in", "nl": "in", "de": "in", "fr": "dans, en", "es": "en", "pt": "em", "ar": "في", "be": "в", "ca": "a, en", "cs": "v, ve, do", "da": "i", "el": "εις, μέσα σε", "fa": "در", "frp": "dans, en", "ga": "i, in", "he": "ב", "hi": "in", "hr": "u", "hu": "-ban/ben", "id": "di dalam", "it": "in", "ja": "の中で, の中へ", "kk": "in", "km": "in", "ko": "~안에", "ku": "در", "lo": "in", "mg": "anaty, amy, any", "ms": "di dalam", "my": "in", "pl": "w", "ro": "dans, en", "ru": "в", "sk": "v, vo, do", "sl": "v", "sv": "i", "sw": "dans, en", "th": "ใน", "tok": "in", "tr": "-e, -ye, içeri, içine", "uk": "в, у", "ur": "میں", "vi": "ở trong", "yo": "in", "zh-tw": "在內, 入內", "zh": "在內、入內"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-prepozicio-inter", typ: "vocab",
			content: map[string]interface{}{
				"word": "inter",
				"definition": "between, among",
				"definitions": map[string]interface{}{"en": "between, among", "nl": "tussen", "de": "zwischen", "fr": "entre, parmi", "es": "entre", "pt": "entre", "ar": "بين", "be": "между", "ca": "entre", "cs": "mezi", "da": "mellem, blandt", "el": "μεταξύ", "fa": "بین, میان", "frp": "entre, parmi", "ga": "idir", "he": "בין", "hi": "between, among", "hr": "između, među", "hu": "között", "id": "di antara", "it": "tra", "ja": "の間で, の間へ", "kk": "between, among", "km": "between, among", "ko": "~ 사이에, ~ 중에", "ku": "بین, میان", "lo": "between, among", "mg": "eo, amy , ampovoany", "ms": "antara", "my": "between, among", "pl": "pomiędzy, między", "ro": "entre, parmi", "ru": "между", "sk": "medzi", "sl": "med", "sv": "mellan, bland", "sw": "entre, parmi", "th": "ระหว่าง, ท่ามกลาง", "tok": "between, among", "tr": "arasında", "uk": "між, поміж, серед, посеред", "ur": "درمیان, among", "vi": "giữa, ở giữa, lẫn nhau", "yo": "between, among", "zh-tw": "之間, 其中", "zh": "之间, 其中"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-prepozicio-je", typ: "vocab",
			content: map[string]interface{}{
				"word": "je",
				"definition": "of, with, in",
				"definitions": map[string]interface{}{"en": "of, with, in", "nl": "voorzetsel met meer betekenissen", "de": "Verhältniswort mit mehreren Bedeutungen", "fr": "de, avec, dans", "es": "a* (comodín)", "pt": "de, com, em", "ar": "من, مع, في", "be": "на, с, предлог с неопределённым значением", "ca": "a (comodí)", "cs": "(neurčitá předložka)", "da": "af, med, i", "el": "στις (ώρα), (αόριστη πρόθεση: από, μαζί, μέσα..)", "fa": "از, با, در, بدون معنی معین", "frp": "de, avec, dans", "ga": "de, fara, i", "he": "מילת יחס כללית, כשאחרות לא מתאימות", "hi": "of, with, in", "hr": "od, s, u, (neodređeni prijedlog)", "hu": "kor, állandósult kifejezésekben", "id": "pada", "it": "(preposizione indefinita)", "ja": "（時刻）に, （数量）だけ, について", "kk": "of, with, in", "km": "of, with, in", "ko": "~에, ~로", "ku": "از, با, در, بدون معنی معین", "lo": "of, with, in", "mg": "ny, miaraka, ao", "ms": "daripada, bersama, pada", "my": "of, with, in", "pl": "przyimek uniwersalny", "ro": "de, avec, dans", "ru": "на, с, предлог с неопределённым значением", "sk": "(neurčitá predložka)", "sl": "ob, v, s, joker predlog ki nadomešča pomene, ki jih ne moremo izraziti s katerim koli drugim predlogom", "sv": "obestämd preposition", "sw": "de, avec, dans", "th": "ณ, ใน, ต่อ, กับ", "tok": "of, with, in", "tr": "(genel edat)", "uk": "у, в, на, о, прийменник, що вжививається тоді, коли будь-який інший прийменник за своїм значенням не підходить", "ur": "کا، کے، کی, کے ساتھ, میں", "vi": "giới từ vô định, vào lúc", "yo": "of, with, in", "zh-tw": "於, 在", "zh": "的, 和, 在"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-prepozicio-kontraux", typ: "vocab",
			content: map[string]interface{}{
				"word": "kontraŭ",
				"definition": "against",
				"definitions": map[string]interface{}{"en": "against", "nl": "tegen", "de": "gegen", "fr": "contre", "es": "contra, en contra de", "pt": "contra", "ar": "ضد", "be": "против, контр-", "ca": "davant per davant, en contra de", "cs": "proti", "da": "mod, imod", "el": "έναντι, κατά", "fa": "بر ضد, ضد, علیه, در برابر", "frp": "contre", "ga": "i gcoinne", "he": "מול, בניגוד ל", "hi": "against", "hr": "nasuprot, protiv", "hu": "ellen, szemben", "id": "terhadap, melawan, kontra", "it": "contro, di fronte", "ja": "に反して, に向かって", "kk": "against", "km": "against", "ko": "~에 대항하여, ~와 반대로", "ku": "بر ضد, ضد, علیه, در برابر", "lo": "against", "mg": "manohitra", "ms": "menentang", "my": "against", "pl": "przeciw", "ro": "contre", "ru": "против, контр-", "sk": "proti", "sl": "nasproti, proti", "sv": "mot, emot", "sw": "contre", "th": "ตรงข้าม", "tok": "against", "tr": "karşısında, karşı", "uk": "проти, навпроти, за", "ur": "مخالف", "vi": "chống lại, phản đối", "yo": "against", "zh-tw": "針對, 反對", "zh": "针对, 反对"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-prepozicio-krom", typ: "vocab",
			content: map[string]interface{}{
				"word": "krom",
				"definition": "besides, in addition to",
				"definitions": map[string]interface{}{"en": "besides, in addition to", "nl": "behalve", "de": "außer", "fr": "outre, à part", "es": "además de, aparte de (con negación)", "pt": "além de", "ar": "بالإضافة إلى, باستثناء", "be": "кроме", "ca": "a més de, a banda de, excepte", "cs": "kromě, mimo", "da": "i øvrigt, bortset, undtagen", "el": "εκτός από", "fa": "به جز, علاوه بر", "frp": "outre, à part", "ga": "seachas, fairis", "he": "חוץ מ, בנוסף ל", "hi": "besides, in addition to", "hr": "osim", "hu": "kívül", "id": "di samping itu, sebagai tambahan", "it": "oltre a", "ja": "を除いて, に加えて", "kk": "besides, in addition to", "km": "besides, in addition to", "ko": "~ 이외에, ~ 뿐만 아니라", "ku": "به جز, علاوه بر", "lo": "besides, in addition to", "mg": "fanampin'izany, ankaotra", "ms": "di samping itu, sebagai tambhan kepada", "my": "besides, in addition to", "pl": "oprócz, z wyjątkiem", "ro": "outre, à part", "ru": "кроме", "sk": "okrem, mimo", "sl": "razne", "sv": "förutom, utom", "sw": "outre, à part", "th": "นอกจาก", "tok": "besides, in addition to", "tr": "hariç, ilaveten", "uk": "крім, опріч, окрім", "ur": "علاوہ, in addition to", "vi": "vả lại, ngoài ra", "yo": "besides, in addition to", "zh-tw": "此外, 除外", "zh": "此外, 除外"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-prepozicio-kun", typ: "vocab",
			content: map[string]interface{}{
				"word": "kun",
				"definition": "with",
				"definitions": map[string]interface{}{"en": "with", "nl": "met", "de": "mit", "fr": "avec", "es": "con", "pt": "com", "ar": "مع", "be": "с, со", "ca": "amb, acompanyat de", "cs": "s, se", "da": "med", "el": "με, μαζί με", "fa": "با, همراه", "frp": "avec", "ga": "le, fara", "he": "עם", "hi": "with", "hr": "s", "hu": "-val/vel (társ)", "id": "dengan", "it": "con", "ja": "といっしょに", "kk": "with", "km": "with", "ko": "~와 함께", "ku": "با, همراه", "lo": "with", "mg": "miaraka", "ms": "bersama", "my": "with", "pl": "z", "ro": "avec", "ru": "с, со", "sk": "s, so", "sl": "s, z", "sv": "(tillsammans) med", "sw": "avec", "th": "กับ", "tok": "with", "tr": "ile", "uk": "з", "ur": "کے ساتھ", "vi": "cùng với", "yo": "with", "zh-tw": "跟, 與", "zh": "和"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-prepozicio-laux", typ: "vocab",
			content: map[string]interface{}{
				"word": "laŭ",
				"definition": "according to, along",
				"definitions": map[string]interface{}{"en": "according to, along", "nl": "volgens", "de": "gemäß", "fr": "d'après", "es": "según", "pt": "de acordo, ao alongo", "ar": "وفق", "be": "по, согласно с", "ca": "segons, arran, al llarg de", "cs": "podle, podél", "da": "langs, ifølge", "el": "ως προς, σύμφωνα με", "fa": "بر اساس, طبق, در امتداد, در راستای", "frp": "d'après", "ga": "de réir, feadh", "he": "לפי, לאורך", "hi": "according to, along", "hr": "prema, po", "hu": "szerint, mentén", "id": "berdasarkan, menurut", "it": "secondo (avverbio)", "ja": "に沿って, に従って", "kk": "according to, along", "km": "according to, along", "ko": "~에 따라서", "ku": "بر اساس, طبق, در امتداد, در راستای", "lo": "according to, along", "mg": "avy amin'ny", "ms": "mengikut", "my": "according to, along", "pl": "według", "ro": "d'après", "ru": "по, согласно с", "sk": "podľa, pozdĺž", "sl": "po", "sv": "enligt, längs", "sw": "d'après", "th": "ตามที่", "tok": "according to, along", "tr": "uygun olarak, boyunca", "uk": "за, згідно з..., відповідно до...", "ur": "کے مطابق, along", "vi": "theo, theo cùng với", "yo": "according to, along", "zh-tw": "根據, 順著", "zh": "根据, 顺"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-prepozicio-per", typ: "vocab",
			content: map[string]interface{}{
				"word": "per",
				"definition": "by means of, with",
				"definitions": map[string]interface{}{"en": "by means of, with", "nl": "met, door middel van", "de": "mit, mittels", "fr": "au moyen de, avec", "es": "por medio de, con", "pt": "através de", "ar": "بواسطة, ب", "be": "через, с помощью", "ca": "mitjançant, amb", "cs": "pomocí, (7. pád)", "da": "ved, per, med", "el": "με (τη βοήθεια του), μέσω", "fa": "با استفاده از, با", "frp": "au moyen de, avec", "ga": "le", "he": "על ידי, באמצעות", "hi": "by means of, with", "hr": "pomoću, s", "hu": "-val/vel", "id": "dengan cara, menggunakan", "it": "mediante, tramite", "ja": "を使って, によって", "kk": "by means of, with", "km": "by means of, with", "ko": "~로, ~를 수단으로", "ku": "با استفاده از, با", "lo": "by means of, with", "mg": "amin'ny alàlan'ny, amy , amin' ny", "ms": "bersama", "my": "by means of, with", "pl": "przy pomocy, przez", "ro": "au moyen de, avec", "ru": "через, с помощью", "sk": "pomocou, (inštrumentál, s, so)", "sl": "s, z", "sv": "med (hjälp av)", "sw": "au moyen de, avec", "th": "โดย", "tok": "by means of, with", "tr": "aracılığı ile, ile", "uk": "прийменник, найчастіше вживається для передачі орудного відмінку, за допомогою", "ur": "بذریعہ, with", "vi": "bằng, sử dụng", "yo": "by means of, with", "zh-tw": "以某工具/媒介, 用", "zh": "通过某种工具，媒体, 和"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-prepozicio-por", typ: "vocab",
			content: map[string]interface{}{
				"word": "por",
				"definition": "for",
				"definitions": map[string]interface{}{"en": "for", "nl": "voor", "de": "für", "fr": "pour", "es": "para, por", "pt": "para", "ar": "من أجل, ل", "be": "для", "ca": "per a, per tal de, a fi de", "cs": "pro, na, za (účelové)", "da": "for", "el": "για, δια", "fa": "برای", "frp": "pour", "ga": "do, le haghaidh", "he": "בשביל, כדי", "hi": "for", "hr": "za", "hu": "számára,, részére, -ra/re", "id": "untuk", "it": "per", "ja": "のために", "kk": "for", "km": "for", "ko": "~를 위해서", "ku": "برای", "lo": "for", "mg": "ho , ho any", "ms": "untuk", "my": "for", "pl": "dla", "ro": "pour", "ru": "для", "sk": "pre, na, za (účelovo)", "sl": "za", "sv": "för", "sw": "pour", "th": "สำหรับ, เพื่อ", "tok": "for", "tr": "için", "uk": "для", "ur": "کے لیے", "vi": "cho việc", "yo": "for", "zh-tw": "為了", "zh": "给"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-prepozicio-post", typ: "vocab",
			content: map[string]interface{}{
				"word": "post",
				"definition": "after",
				"definitions": map[string]interface{}{"en": "after", "nl": "na", "de": "nach", "fr": "après", "es": "después de, detrás de", "pt": "depois", "ar": "بعد", "be": "после", "ca": "darrere, després", "cs": "po, za (místně i časově)", "da": "efter", "el": "μετά από, πίσω από", "fa": "پس از, بعد", "frp": "après", "ga": "tar éis", "he": "אחרי", "hi": "after", "hr": "nakon, iza", "hu": "után", "id": "setelah", "it": "dopo", "ja": "より後で, の後ろで, の後ろへ", "kk": "after", "km": "after", "ko": "~ 다음에", "ku": "پس از, بعد", "lo": "after", "mg": "aoriana , manarakaraka", "ms": "selepas", "my": "after", "pl": "za, potem", "ro": "après", "ru": "после", "sk": "po, za (miestne aj časovo)", "sl": "nato, potem", "sv": "efter", "sw": "après", "th": "ข้างหลัง, หลังจาก, หลังจากนั้น", "tok": "after", "tr": "sonra", "uk": "після", "ur": "کے بعد", "vi": "ở ngay phía sau, sau lúc", "yo": "after", "zh-tw": "之後", "zh": "之后"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-prepozicio-pri", typ: "vocab",
			content: map[string]interface{}{
				"word": "pri",
				"definition": "about, concerning",
				"definitions": map[string]interface{}{"en": "about, concerning", "nl": "over, betreffende, aangaande", "de": "über", "fr": "au sujet de, concernant", "es": "en cuanto a, sobre", "pt": "sobre", "ar": "فيما يتعلق ب, حول", "be": "о, об", "ca": "pel que fa a, sobre (temàtica)", "cs": "o", "da": "om, vedrørende", "el": "για, περί, σχετικά με", "fa": "درباره, درباب", "frp": "au sujet de, concernant", "ga": "faoi, mar gheall ar", "he": "על, בקשר, בנושא", "hi": "about, concerning", "hr": "o", "hu": "-ról/ről", "id": "tentang, mengenai", "it": "riguardo a", "ja": "について, に関連して", "kk": "about, concerning", "km": "about, concerning", "ko": "~에 대한, ~에 관한", "ku": "درباره, درباب", "lo": "about, concerning", "mg": "momba ny, momba , amy ny", "ms": "mengenai", "my": "about, concerning", "pl": "o", "ro": "au sujet de, concernant", "ru": "о, об", "sk": "o", "sl": "o", "sv": "om, angående", "sw": "au sujet de, concernant", "th": "เกี่ยวกับ", "tok": "about, concerning", "tr": "hakkında", "uk": "про", "ur": "کے متعلق, concerning", "vi": "về việc", "yo": "about, concerning", "zh-tw": "關於, 相關", "zh": "关于, 相关"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-prepozicio-pro", typ: "vocab",
			content: map[string]interface{}{
				"word": "pro",
				"definition": "because of",
				"definitions": map[string]interface{}{"en": "because of", "nl": "wegens", "de": "wegen", "fr": "à cause de", "es": "a causa de, por", "pt": "por causa de", "ar": "بسبب", "be": "из-за", "ca": "a causa de, degut a, arran de", "cs": "kvůli", "da": "på grund af", "el": "λόγω, ένεκα, εξ αιτίας", "fa": "به خاطر", "frp": "à cause de", "ga": "i ngeall ar", "he": "בגלל", "hi": "because of", "hr": "zbog, radi", "hu": "miatt", "id": "karena", "it": "a causa di", "ja": "のために", "kk": "because of", "km": "because of", "ko": "~ 때문에", "ku": "به خاطر", "lo": "because of", "mg": "noho ny", "ms": "disebabkan", "my": "because of", "pl": "z powodu", "ro": "à cause de", "ru": "из-за", "sk": "kvôli, pre", "sl": "zaradi", "sv": "på grund av", "sw": "à cause de", "th": "เนื่องจาก", "tok": "because of", "tr": "-dan dolayı", "uk": "через, за, заради, ради, задля", "ur": "because of", "vi": "bởi vì", "yo": "because of", "zh-tw": "由於", "zh": "基于"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 22,
		},
		{
			slug: "vortaro-prepozicio-sen", typ: "vocab",
			content: map[string]interface{}{
				"word": "sen",
				"definition": "without",
				"definitions": map[string]interface{}{"en": "without", "nl": "zonder", "de": "ohne", "fr": "sans", "es": "sin", "pt": "sem", "ar": "بدون, بلا", "be": "без", "ca": "sense", "cs": "bez, beze", "da": "uden", "el": "χωρίς", "fa": "بدون", "frp": "sans", "ga": "gan", "he": "בלי", "hi": "without", "hr": "bez", "hu": "nélkül", "id": "tanpa", "it": "senza", "ja": "なしに", "kk": "without", "km": "without", "ko": "~ 없이", "ku": "بدون", "lo": "without", "mg": "tsy , tsy manana , tsy misy", "ms": "tanpa", "my": "without", "pl": "bez", "ro": "sans", "ru": "без", "sk": "bez, bezo", "sl": "brez", "sv": "utan", "sw": "sans", "th": "ปราศจาก", "tok": "without", "tr": "olmaksızın, -sız, -siz vs", "uk": "без", "ur": "کے بغیر", "vi": "không có, không cần", "yo": "without", "zh-tw": "無, 沒有", "zh": "无，没有"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 23,
		},
		{
			slug: "vortaro-prepozicio-sub", typ: "vocab",
			content: map[string]interface{}{
				"word": "sub",
				"definition": "under, beneath",
				"definitions": map[string]interface{}{"en": "under, beneath", "nl": "onder", "de": "unter", "fr": "sous", "es": "debajo de, debajo", "pt": "sob", "ar": "تحت", "be": "под", "ca": "sota", "cs": "pod", "da": "under, nedenunder", "el": "κάτω από, υπό", "fa": "زیر, تحت", "frp": "sous", "ga": "faoi, thíos faoi", "he": "מתחת", "hi": "under, beneath", "hr": "pod, ispod", "hu": "alatt", "id": "di bawah, di balik", "it": "sotto", "ja": "の下で, の下へ", "kk": "under, beneath", "km": "under, beneath", "ko": "~아래에", "ku": "زیر, تحت", "lo": "under, beneath", "mg": "ambany", "ms": "di bawah", "my": "under, beneath", "pl": "pod, poniżej", "ro": "sous", "ru": "под", "sk": "pod", "sl": "pod", "sv": "under", "sw": "sous", "th": "ข้างใต้", "tok": "under, beneath", "tr": "altında", "uk": "під", "ur": "under, beneath", "vi": "ở dưới, bên dưới", "yo": "under, beneath", "zh-tw": "下方, 底下", "zh": "下方, 底下"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 24,
		},
		{
			slug: "vortaro-prepozicio-super", typ: "vocab",
			content: map[string]interface{}{
				"word": "super",
				"definition": "over, above",
				"definitions": map[string]interface{}{"en": "over, above", "nl": "over", "de": "über", "fr": "au-dessus de", "es": "en (encima, no tocando)", "pt": "acima de", "ar": "فوق", "be": "над, сверх, пере-", "ca": "per sobre", "cs": "nad", "da": "over", "el": "πάνω από, υπεράνω", "fa": "بالای, بر فراز", "frp": "au-dessus de", "ga": "os cionn, lastuas de", "he": "מעל", "hi": "over, above", "hr": "nad, iznad", "hu": "felett", "id": "di atas", "it": "sopra", "ja": "の上方で, の上方へ", "kk": "over, above", "km": "over, above", "ko": "~위에", "ku": "بالای, بر فراز", "lo": "over, above", "mg": "ambonin'ny", "ms": "di atas", "my": "over, above", "pl": "nad, powyżej", "ro": "au-dessus de", "ru": "над, сверх, пере-", "sk": "nad", "sl": "nad", "sv": "över, ovanför", "sw": "au-dessus de", "th": "เหนือ", "tok": "over, above", "tr": "üzerinde", "uk": "над, зверху, понад", "ur": "over, above", "vi": "đi lên, ở trên đỉnh", "yo": "over, above", "zh-tw": "上方, 超過", "zh": "上方, 超过"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 25,
		},
		{
			slug: "vortaro-prepozicio-sur", typ: "vocab",
			content: map[string]interface{}{
				"word": "sur",
				"definition": "on",
				"definitions": map[string]interface{}{"en": "on", "nl": "op", "de": "auf", "fr": "sur", "es": "en (encima)", "pt": "sobre (por cima)", "ar": "على", "be": "над", "ca": "damunt, sobre", "cs": "na", "da": "på", "el": "επί, πάνω σε", "fa": "روی", "frp": "sur", "ga": "ar", "he": "על", "hi": "on", "hr": "na", "hu": "-on/en/ön", "id": "pada", "it": "su", "ja": "の上で, の上へ", "kk": "on", "km": "on", "ko": "~의 표면 위에", "ku": "روی", "lo": "on", "mg": "ambony", "ms": "di atas", "my": "on", "pl": "na", "ro": "sur", "ru": "на", "sk": "na", "sl": "na", "sv": "på", "sw": "sur", "th": "บน", "tok": "on", "tr": "üstünde", "uk": "на", "ur": "پر", "vi": "ở trên", "yo": "on", "zh-tw": "在...上面", "zh": "在...上"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 26,
		},
		{
			slug: "vortaro-prepozicio-tra", typ: "vocab",
			content: map[string]interface{}{
				"word": "tra",
				"definition": "through",
				"definitions": map[string]interface{}{"en": "through", "nl": "door, doorheen", "de": "durch", "fr": "par", "es": "a través de, por", "pt": "através de, por entre", "ar": "عبر", "be": "через, сквозь", "ca": "a través, de banda a banda", "cs": "přes, skrz", "da": "gennem", "el": "δια μέσου", "fa": "از میان", "frp": "par", "ga": "trí", "he": "דרך", "hi": "through", "hr": "kroz", "hu": "át, keresztül", "id": "melalui", "it": "attraverso", "ja": "を通り抜けて", "kk": "through", "km": "through", "ko": "~를 통하여, ~를 관통하여", "ku": "از میان", "lo": "through", "mg": "tamin'ny", "ms": "melalui", "my": "through", "pl": "przez", "ro": "par", "ru": "через, сквозь", "sk": "cez, skrz", "sl": "skozi", "sv": "genom", "sw": "par", "th": "ตลอด, ทะลุผ่าน", "tok": "through", "tr": "içinden", "uk": "через, крізь", "ur": "through", "vi": "xuyên qua", "yo": "through", "zh-tw": "穿過", "zh": "通过"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 27,
		},
		{
			slug: "vortaro-prepozicio-trans", typ: "vocab",
			content: map[string]interface{}{
				"word": "trans",
				"definition": "across",
				"definitions": map[string]interface{}{"en": "across", "nl": "aan de overkant van", "de": "jenseits", "fr": "au travers", "es": "a través de", "pt": "do outro lado, para lá de", "ar": "من خلال", "be": "через, за", "ca": "més enllà, dellà, a l'altra banda", "cs": "přes", "da": "over, på kryds af", "el": "πέρα από, κατά μήκος", "fa": "آن سوی", "frp": "au travers", "ga": "trasna", "he": "מעבר", "hi": "across", "hr": "preko", "hu": "túl", "id": "menyeberangi", "it": "al di là", "ja": "の向こうで, の向こうへ", "kk": "across", "km": "across", "ko": "~를 가로질러", "ku": "آن سوی", "lo": "across", "mg": "ny alalan'", "ms": "menyeberangi", "my": "across", "pl": "poprzez", "ro": "au travers", "ru": "через, за", "sk": "za, cez", "sl": "čez", "sv": "på andra sidan av, bortom", "sw": "au travers", "th": "ข้าม", "tok": "across", "tr": "karşıya", "uk": "через", "ur": "across", "vi": "ngang qua", "yo": "across", "zh-tw": "越過", "zh": "越过"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 28,
		},
		{
			slug: "vortaro-prepozicio-cxe", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉe",
				"definition": "at",
				"definitions": map[string]interface{}{"en": "at", "nl": "bij", "de": "bei", "fr": "chez", "es": "en, por", "pt": "em, junto de", "ar": "في, عند", "be": "при, у", "ca": "a tocar, a casa de", "cs": "u, při", "da": "ved, hos", "el": "πολύ κοντά (επαφή), παρά τω, παρά τη", "fa": "نزد, پهلوی", "frp": "chez", "ga": "ag", "he": "אצל", "hi": "at", "hr": "kod", "hu": "-nál/nél", "id": "di", "it": "presso", "ja": "で", "kk": "at", "km": "at", "ko": "~에", "ku": "نزد, پهلوی", "lo": "at", "mg": "amin'ny", "ms": "di", "my": "at", "pl": "u, przy", "ro": "chez", "ru": "при, у", "sk": "u, pri", "sl": "pri", "sv": "hos, intill", "sw": "chez", "th": "ที่", "tok": "at", "tr": "-de, -da", "uk": "у, в, при, за, біля, коло, на", "ur": "at", "vi": "tại nơi", "yo": "at", "zh-tw": "在", "zh": "在"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 29,
		},
		{
			slug: "vortaro-prepozicio-cxirkaux", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉirkaŭ",
				"definition": "around",
				"definitions": map[string]interface{}{"en": "around", "nl": "om, rondom", "de": "um, herum", "fr": "autour de", "es": "alrededor de", "pt": "ao redor, cerca de", "ar": "حول", "be": "вокруг, около (о количестве, времени)", "ca": "al voltant de", "cs": "kolem, okolo, asi", "da": "omkring", "el": "γύρω από", "fa": "اطراف, پیرامون", "frp": "autour de", "ga": "timpeall", "he": "מסביב", "hi": "around", "hr": "okolo, oko", "hu": "körül, körülbelül", "id": "sekitar", "it": "intorno a", "ja": "まわりで, まわりへ, のあたりに, のころ", "kk": "around", "km": "around", "ko": "~ 주변에", "ku": "اطراف, پیرامون", "lo": "around", "mg": "manodidin'ny", "ms": "sekitar", "my": "around", "pl": "około", "ro": "autour de", "ru": "вокруг, около (о количестве, времени)", "sk": "okolo, asi", "sl": "okoli", "sv": "omkring, runt, cirka", "sw": "autour de", "th": "รอบ ๆ, ราว ๆ, ประมาณ", "tok": "around", "tr": "etraf", "uk": "навколо", "ur": "around", "vi": "vòng quanh", "yo": "around", "zh-tw": "左右, 四周", "zh": "左右，四处"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 30,
		},
		{
			slug: "vortaro-prepozicio-gxis", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝis",
				"definition": "until",
				"definitions": map[string]interface{}{"en": "until", "nl": "tot", "de": "bis", "fr": "jusqu'à", "es": "hasta", "pt": "até", "ar": "إلى, حتى", "be": "до", "ca": "fins", "cs": "až k, až do", "da": "indtil", "el": "μέχρι", "fa": "تا", "frp": "jusqu'à", "ga": "go dtí", "he": "עד", "hi": "until", "hr": "do", "hu": "-ig", "id": "sampai", "it": "fino a", "ja": "まで", "kk": "until", "km": "until", "ko": "~ 까지, ~ 이전까지", "ku": "تا", "lo": "until", "mg": "hatramy , hatramin'ny", "ms": "hingga", "my": "until", "pl": "do, aż", "ro": "jusqu'à", "ru": "до", "sk": "až k, až do", "sl": "do", "sv": "ända till", "sw": "jusqu'à", "th": "จนกระทั่ง", "tok": "until", "tr": "-e kadar", "uk": "до", "ur": "تک", "vi": "đến khi", "yo": "until", "zh-tw": "直到", "zh": "直到"},
			},
			tags:        []string{"vortaro", "prepozicio"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "vortaro-prepozicio",
			seriesOrder: 31,
		},
		{
			slug: "vortaro-pronomo-ili", typ: "vocab",
			content: map[string]interface{}{
				"word": "ili",
				"definition": "they",
				"definitions": map[string]interface{}{"en": "they", "nl": "zij (meervoud), ze (meervoud)", "de": "sie (Mehrzahl)", "fr": "ils", "es": "ellos/as", "pt": "eles", "ar": "هم, هن", "be": "яны", "ca": "ells / elles", "cs": "oni", "da": "de", "el": "αυτοί-ές-ά", "fa": "ایشان، این‌ها، آن‌ها", "frp": "ils", "ga": "siad", "he": "הם", "hi": "they", "hr": "oni", "hu": "ők", "id": "mereka", "it": "essi, esse", "ja": "彼ら、彼女ら、それら", "kk": "олар", "km": "they", "ko": "그들", "ku": "ایشان، این‌ها، آن‌ها", "lo": "they", "mg": "izy ireo", "ms": "mereka", "my": "they", "pl": "oni, one", "ro": "ils", "ru": "они", "sk": "oni/ony", "sl": "oni", "sv": "de", "sw": "ils", "th": "พวกเขา, พวกหล่อน, พวกมัน", "tok": "they", "tr": "onlar", "uk": "вони", "ur": "وہ سب", "vi": "họ", "yo": "they", "zh-tw": "他們", "zh": "他们"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-pronomo-li", typ: "vocab",
			content: map[string]interface{}{
				"word": "li",
				"definition": "he",
				"definitions": map[string]interface{}{"en": "he", "nl": "hij", "de": "er", "fr": "il", "es": "él", "pt": "ele", "ar": "هو", "be": "ён", "ca": "ell", "cs": "on", "da": "han", "el": "αυτός", "fa": "او (مذکر)", "frp": "il", "ga": "sé", "he": "הוא", "hi": "he", "hr": "on", "hu": "ő (férfi)", "id": "dia (laki-laki)", "it": "egli, lui", "ja": "彼", "kk": "ол", "km": "he", "ko": "그사람", "ku": "او (مذکر)", "lo": "he", "mg": "izy", "ms": "dia (lelaki)", "my": "he", "pl": "on", "ro": "il", "ru": "он", "sk": "on", "sl": "on", "sv": "han", "sw": "il", "th": "เขา", "tok": "he", "tr": "o (erkek)", "uk": "він", "ur": "وہ (مرد", "vi": "anh ấy", "yo": "he", "zh-tw": "他", "zh": "他"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-pronomo-lia", typ: "vocab",
			content: map[string]interface{}{
				"word": "lia",
				"definition": "his",
				"definitions": map[string]interface{}{"en": "his", "nl": "zijn(e)", "de": "sein(e)", "fr": "son", "es": "su (de él), suyo (de él)", "pt": "dele", "ar": "له", "be": "ягоны", "ca": "el seu / la seva (d'ell)", "cs": "jeho", "da": "hans", "el": "δικός-ή-ό του", "fa": "مال او (مذکر)، ـَش", "frp": "son", "ga": "a", "he": "שלו", "hi": "his", "hr": "njegov", "hu": "övé (férfi)", "id": "miliknya (laki-laki)", "it": "suo", "ja": "彼の", "kk": "оның", "km": "his", "ko": "그의", "ku": "مال او (مذکر)، ـَش", "lo": "his", "mg": "ny ...ny", "ms": "dia punya", "my": "his", "pl": "jego", "ro": "son", "ru": "его", "sk": "jeho", "sl": "njegov", "sv": "hans", "sw": "son", "th": "ของเขา", "tok": "his", "tr": "onun (erkek)", "uk": "його", "ur": "اس کا", "vi": "của anh ấy", "yo": "his", "zh-tw": "他的", "zh": "他的"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-pronomo-mi", typ: "vocab",
			content: map[string]interface{}{
				"word": "mi",
				"definition": "I",
				"definitions": map[string]interface{}{"en": "I", "nl": "ik", "de": "ich", "fr": "je", "es": "yo", "pt": "eu", "ar": "أنا", "be": "я", "ca": "jo", "cs": "já", "da": "jeg", "el": "εγώ, εμένα", "fa": "من", "frp": "je", "ga": "mé", "he": "אני", "hi": "I", "hr": "ja", "hu": "én", "id": "saya", "it": "io", "ja": "私", "kk": "мен", "km": "I", "ko": "나", "ku": "من", "lo": "I", "mg": "aho", "ms": "saya", "my": "I", "pl": "ja", "ro": "je", "ru": "я", "sk": "ja", "sl": "jaz", "sv": "jag", "sw": "je", "th": "ฉัน", "tok": "I", "tr": "ben", "uk": "я", "ur": "میں", "vi": "tôi", "yo": "I", "zh-tw": "我", "zh": "我"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-pronomo-mia", typ: "vocab",
			content: map[string]interface{}{
				"word": "mia",
				"definition": "my",
				"definitions": map[string]interface{}{"en": "my", "nl": "mijn(e)", "de": "mein", "fr": "mon", "es": "mi, mío", "pt": "meu", "ar": "لي", "be": "мой, мая, маё", "ca": "el meu / la meva", "cs": "moje", "da": "min", "el": "δικός-ή-ό μου", "fa": "مال من، ـَم", "frp": "mon", "ga": "mo", "he": "שלי", "hi": "my", "hr": "moj", "hu": "enyém", "id": "kepunyaanku", "it": "mio", "ja": "私の", "kk": "менің", "km": "my", "ko": "나의", "ku": "مال من، ـَم", "lo": "my", "mg": "ny ...ko", "ms": "saya punya", "my": "my", "pl": "mój, moja, moje", "ro": "mon", "ru": "мой, моя, моё", "sk": "moje", "sl": "moj", "sv": "min, mitt", "sw": "mon", "th": "ของฉัน", "tok": "my", "tr": "benim", "uk": "мій, моя, моє", "ur": "میرا، میری", "vi": "của tôi", "yo": "my", "zh-tw": "我的", "zh": "我的"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-pronomo-ni", typ: "vocab",
			content: map[string]interface{}{
				"word": "ni",
				"definition": "we",
				"definitions": map[string]interface{}{"en": "we", "nl": "wij, we", "de": "wir", "fr": "nous", "es": "nosotros/as", "pt": "nós", "ar": "نحن", "be": "мы", "ca": "nosaltres", "cs": "my", "da": "vi", "el": "εμείς, εμάς", "fa": "ما", "frp": "nous", "ga": "sinn", "he": "אנחנו", "hi": "we", "hr": "mi", "hu": "mi", "id": "kami", "it": "noi", "ja": "私たち", "kk": "біз", "km": "we", "ko": "우리", "ku": "ما", "lo": "we", "mg": "isika", "ms": "kita", "my": "we", "pl": "my", "ro": "nous", "ru": "мы", "sk": "my", "sl": "mi", "sv": "vi", "sw": "nous", "th": "พวกเรา", "tok": "we", "tr": "biz", "uk": "ми", "ur": "ہم", "vi": "chúng tôi, chúng ta", "yo": "we", "zh-tw": "我們", "zh": "我们"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-pronomo-oni", typ: "vocab",
			content: map[string]interface{}{
				"word": "oni",
				"definition": "one (indefinite pronoun)",
				"definitions": map[string]interface{}{"en": "one (indefinite pronoun)", "nl": "men", "de": "man", "fr": "on", "es": "se, uno, la gente", "pt": "se (pronome indefinido)", "ar": "واحد  ( ضمير لأجل غير مسمى )", "be": "нявызначаны займеньнік", "ca": "se, un (hom), la gent", "cs": "neurčité zájmeno", "da": "man", "el": "κάποιος-α-ο", "fa": "ضمیر نامعین, بعضی, کسی, یکی, شما", "frp": "on", "ga": "(briathar saor)", "he": "אמירה כללית", "hi": "one (indefinite pronoun)", "hr": "(neodređena osobna zamjenica)", "hu": "az ember (általános alany)", "id": "seseorang, orang", "it": "si (pronome impersonale)", "ja": "それ (不定代名詞)", "kk": "one (indefinite pronoun)", "km": "one (indefinite pronoun)", "ko": "누군가", "ku": "ضمیر نامعین, بعضی, کسی, یکی, شما", "lo": "one (indefinite pronoun)", "mg": "olona , misy", "ms": "sesiapa", "my": "one (indefinite pronoun)", "pl": "zaimek nieosobowy", "ro": "on", "ru": "неопределённое местоимение", "sk": "neurčité zámeno", "sl": "nedoločni zaimek", "sv": "man", "sw": "on", "th": "เรา, คนเรา (สรรพนามบุรุษที่สามไม่เจาะจงอาจมีจำนวนมากหรือน้อยก็ได้)", "tok": "one (indefinite pronoun)", "tr": "birisi (belirsiz şahıs zamiri)", "uk": "неозначено-особовий займенник, відповідає 3-ій особі множини в безособових реченнях і зворотах", "ur": "one (indefinite pronoun)", "vi": "người ta", "yo": "one (indefinite pronoun)", "zh-tw": "人們", "zh": "人"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-pronomo-vi", typ: "vocab",
			content: map[string]interface{}{
				"word": "vi",
				"definition": "you (can be both singular and plural)",
				"definitions": map[string]interface{}{"en": "you (can be both singular and plural)", "nl": "jij, je, gij, ge, u, jullie", "de": "du, ihr", "fr": "tu, vous", "es": "tú, vos, usted, vosotros/as, ustedes", "pt": "você (singular ou plural)", "ar": "أنت, أنتم, أنتن, أنتما", "be": "ты, вы (ветлівае абыходжаньне ці мн. Л)", "ca": "tu, vos, vostè, vosaltres, vostès", "cs": "Ty/Vy", "da": "du, I", "el": "εσύ, εσένα, εσείς, εσάς", "fa": "شما (می تواند هم مفرد و هم جمع باشد)", "frp": "tu, vous", "ga": "tú, sibh", "he": "אתה, אתם", "hi": "you (can be both singular and plural)", "hr": "vi", "hu": "te, ti", "id": "kamu, Anda, kalian", "it": "tu, voi", "ja": "あなた、あなた達 (単複同形)", "kk": "сен, сіз / сендер, сіздер (can be both singular and plural)", "km": "you (can be both singular and plural)", "ko": "당신", "ku": "شما (می تواند هم مفرد و هم جمع باشد)", "lo": "you (can be both singular and plural)", "mg": "ianao, ianareo", "ms": "awak", "my": "you (can be both singular and plural)", "pl": "ty, wy", "ro": "tu, vous", "ru": "ты, вы (вежливое обращение или мн. ч)", "sk": "ty/vy/Vy", "sl": "vi", "sv": "du, ni", "sw": "tu, vous", "th": "คุณ, พวกคุณ", "tok": "you (can be both singular and plural)", "tr": "sen, siz (tekil ve çoğul)", "uk": "ти, ви", "ur": "تم، آپ", "vi": "bạn, mấy bạn", "yo": "you (can be both singular and plural)", "zh-tw": "你", "zh": "你"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-pronomo-gxi", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĝi",
				"definition": "it",
				"definitions": map[string]interface{}{"en": "it", "nl": "het", "de": "es", "fr": "il", "es": "ello", "pt": "ele, ela, isso", "ar": "هذا ( هذا لغير العاقل )", "be": "яно", "ca": "(pronom per a animals i coses, sense distingir sexe)", "cs": "ono", "da": "den, det", "el": "αυτό", "fa": "این، آن", "frp": "il", "ga": "sé, sí (neodrach)", "he": "זה", "hi": "it", "hr": "ono, on, ona", "hu": "ő (állat), az (tárgy)", "id": "itu (benda, binatang)", "it": "esso", "ja": "それ", "kk": "ол", "km": "it", "ko": "그것", "ku": "این، آن", "lo": "it", "mg": "izy", "ms": "dia (unutk binatang atau benda tidak bernyawa)", "my": "it", "pl": "to", "ro": "il", "ru": "оно", "sk": "ono", "sl": "ono", "sv": "den, det", "sw": "il", "th": "มัน", "tok": "it", "tr": "o (nötr)", "uk": "він, вона, воно (особовий займенник, що стосується неживих предметів та істот, стать яких невідома або не виражена у явній формі).", "ur": "وہ (بے جان", "vi": "nó (con vật, đồ vật)", "yo": "it", "zh-tw": "它", "zh": "它"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-pronomo-sxi", typ: "vocab",
			content: map[string]interface{}{
				"word": "ŝi",
				"definition": "she",
				"definitions": map[string]interface{}{"en": "she", "nl": "zij (vrouwelijk enkelvoud)", "de": "sie", "fr": "elle", "es": "ella", "pt": "ela", "ar": "هى", "be": "яна", "ca": "ella", "cs": "ona", "da": "hun", "el": "αυτή", "fa": "او (مؤنث)", "frp": "elle", "ga": "sí", "he": "היא", "hi": "she", "hr": "ona", "hu": "ő (nő)", "id": "dia (perempuan)", "it": "ella, lei", "ja": "彼女", "kk": "ол", "km": "she", "ko": "그녀", "ku": "او (مؤنث)", "lo": "she", "mg": "izy", "ms": "dia (perempuan)", "my": "she", "pl": "ona", "ro": "elle", "ru": "она", "sk": "ona", "sl": "ona", "sv": "hon", "sw": "elle", "th": "หล่อน, เธอ", "tok": "she", "tr": "o (kadın)", "uk": "вона", "ur": "وہ (عورت", "vi": "cô ấy", "yo": "she", "zh-tw": "她", "zh": "她"},
			},
			tags:        []string{"vortaro", "pronomo"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-pronomo",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-tabelvorto-kio", typ: "vocab",
			content: map[string]interface{}{
				"word": "kio",
				"definition": "what",
				"definitions": map[string]interface{}{"en": "what", "nl": "wat", "de": "was", "fr": "qui", "es": "qué", "pt": "o que", "ar": "ما, ماذا, الذي", "be": "что", "ca": "què, quina cosa", "cs": "co", "da": "hvad", "el": "τι, οποίο, 'ό,τι'", "fa": "چه چیزی, چیزی که", "frp": "qui", "ga": "cad", "he": "מה", "hi": "what", "hr": "što", "hu": "mi", "id": "apa, yang", "it": "che cosa", "ja": "なに", "kk": "what", "km": "what", "ko": "무엇", "ku": "چه چیزی, چیزی که", "lo": "what", "mg": "izay", "ms": "apa", "my": "what", "pl": "co", "ro": "qui", "ru": "что", "sk": "čo", "sl": "kaj", "sv": "vad", "sw": "qui", "th": "อะไร/ซึ่ง", "tok": "ijo seme", "tr": "ne", "uk": "що", "ur": "کیا", "vi": "chuyện gì, chuyện mà", "yo": "what", "zh-tw": "何物", "zh": "什么东西"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-tabelvorto-tio", typ: "vocab",
			content: map[string]interface{}{
				"word": "tio",
				"definition": "that",
				"definitions": map[string]interface{}{"en": "that", "nl": "dat (dit)", "de": "jenes, das", "fr": "celui, qui", "es": "eso", "pt": "isso", "ar": "ذلك", "be": "то", "ca": "allò, això, ho", "cs": "to", "da": "den", "el": "εκείνο", "fa": "آن چیز", "frp": "celui, qui", "ga": "é sin", "he": "זה", "hi": "that", "hr": "to, ono", "hu": "az", "id": "itu", "it": "ciò, quella cosa", "ja": "それ", "kk": "that", "km": "that", "ko": "그것", "ku": "آن چیز", "lo": "that", "mg": "ilay , ireo, izay", "ms": "itu", "my": "that", "pl": "tamto", "ro": "celui, qui", "ru": "то", "sk": "to", "sl": "to, tisto", "sv": "det där", "sw": "celui, qui", "th": "สิ่งนั้น", "tok": "ijo ni", "tr": "bu", "uk": "те", "ur": "وہ", "vi": "đó, chuyện đó", "yo": "that", "zh-tw": "那物", "zh": "那个东西"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-tabelvorto-io", typ: "vocab",
			content: map[string]interface{}{
				"word": "io",
				"definition": "something",
				"definitions": map[string]interface{}{"en": "something", "nl": "iets", "de": "irgendwas", "fr": "quelque chose", "es": "algo", "pt": "algo", "ar": "شيء ما", "be": "что-то", "ca": "alguna cosa, quelcom", "cs": "něco, cosi", "da": "noget", "el": "κάτι", "fa": "یک چیزی", "frp": "quelque chose", "ga": "rud éigin", "he": "משהו", "hi": "something", "hr": "išta, nešto", "hu": "valami", "id": "sesuatu", "it": "qualche cosa", "ja": "なにか", "kk": "something", "km": "something", "ko": "무엇인가 (불특정한 그 어느 것)", "ku": "یک چیزی", "lo": "something", "mg": "zavatra", "ms": "sesuatu", "my": "something", "pl": "coś", "ro": "quelque chose", "ru": "что-то", "sk": "niečo, čosi", "sl": "nekaj", "sv": "någonting", "sw": "quelque chose", "th": "บางสิ่ง", "tok": "ijo", "tr": "bir şey", "uk": "щось", "ur": "کچھ", "vi": "cái gì đó", "yo": "something", "zh-tw": "某物", "zh": "某个东西"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-tabelvorto-cxio", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉio",
				"definition": "everything",
				"definitions": map[string]interface{}{"en": "everything", "nl": "alles", "de": "alles", "fr": "tout", "es": "todo", "pt": "tudo", "ar": "كل, كل شئ", "be": "всё", "ca": "tot", "cs": "všechno", "da": "alt", "el": "κάθε τι, τα πάντα", "fa": "همه چیز", "frp": "tout", "ga": "gach rud", "he": "כל דבר", "hi": "everything", "hr": "sve", "hu": "minden", "id": "semuanya", "it": "tutto, ogni cosa", "ja": "すべて", "kk": "everything", "km": "everything", "ko": "모든 것", "ku": "همه چیز", "lo": "everything", "mg": "rehetra , daholo", "ms": "semua", "my": "everything", "pl": "wszystko", "ro": "tout", "ru": "всё", "sk": "všetko", "sl": "vse", "sv": "allting", "sw": "tout", "th": "ทุกสิ่ง", "tok": "ijo ale", "tr": "herşey", "uk": "все", "ur": "سب کچھ", "vi": "mọi thứ", "yo": "everything", "zh-tw": "每物", "zh": "每样东西"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-tabelvorto-nenio", typ: "vocab",
			content: map[string]interface{}{
				"word": "nenio",
				"definition": "nothing",
				"definitions": map[string]interface{}{"en": "nothing", "nl": "niets", "de": "nichts", "fr": "rien", "es": "nada", "pt": "nada", "ar": "لا شىء", "be": "ничего", "ca": "res, no res", "cs": "nic", "da": "intet", "el": "τίποτα", "fa": "هیچ چیز", "frp": "rien", "ga": "faic", "he": "שום דבר", "hi": "nothing", "hr": "ništa", "hu": "semmi", "id": "tidak ada sesuatu", "it": "niente, nessuna cosa", "ja": "なにも～ない", "kk": "nothing", "km": "nothing", "ko": "아무것도 아닌 것", "ku": "هیچ چیز", "lo": "nothing", "mg": "na inona na inona", "ms": "tidak ada apa-apa", "my": "nothing", "pl": "nic", "ro": "rien", "ru": "ничего", "sk": "nič", "sl": "nič", "sv": "ingenting", "sw": "rien", "th": "ไม่มีสักอย่าง", "tok": "ijo ala", "tr": "hiç birşey", "uk": "нічого", "ur": "کچھ نہیں", "vi": "không gì cả", "yo": "nothing", "zh-tw": "無物", "zh": "没有东西"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-tabelvorto-kiu", typ: "vocab",
			content: map[string]interface{}{
				"word": "kiu",
				"definition": "who, which",
				"definitions": map[string]interface{}{"en": "who, which", "nl": "wie", "de": "wer, welcher", "fr": "qui", "es": "quién, cuál", "pt": "quem, qual", "ar": "من, أى, اسم موصول لشخص أو شئ بالتحديد", "be": "кто, который", "ca": "qui, quin, quina", "cs": "kdo, který", "da": "hvem, hvilken", "el": "ποιός-α-ο, όποιος-α-ο", "fa": "چه کسی, کدام, کسی که, آن که", "frp": "qui", "ga": "cé, cé acu", "he": "מי, ש", "hi": "who, which", "hr": "tko, koji", "hu": "ki, melyik", "id": "siapa, yang, yang mana", "it": "chi, quale", "ja": "だれ, どちらの", "kk": "who, which", "km": "who, which", "ko": "누구, 어느", "ku": "چه کسی, کدام, کسی که, آن که", "lo": "who, which", "mg": "izay", "ms": "siapa", "my": "who, which", "pl": "kto, którego", "ro": "qui", "ru": "кто, который", "sk": "kto, ktorý", "sl": "kdo, kateri", "sv": "vem, vilken", "sw": "qui", "th": "ใคร, อันไหน, /ซึ่งเป็นคนที่, ซึ่งเป็นสิ่งที่", "tok": "ijo seme, jan seme", "tr": "kim, hangi", "uk": "хто", "ur": "کون, کون سا", "vi": "ai, người mà, cái nào, cái mà", "yo": "who, which", "zh-tw": "誰", "zh": "谁"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-tabelvorto-tiu", typ: "vocab",
			content: map[string]interface{}{
				"word": "tiu",
				"definition": "that person, that one",
				"definitions": map[string]interface{}{"en": "that person, that one", "nl": "die", "de": "jener", "fr": "cette personne", "es": "ese, esa", "pt": "aquela pessoa, aquilo", "ar": "ذلك الشخص, ذلك الشئ بالتحديد", "be": "тот", "ca": "aquell, aquella", "cs": "ten", "da": "den person, den der", "el": "εκείνος-η-ο, αυτός-η-ο", "fa": "آن شخص, آن یکی", "frp": "cette personne", "ga": "an duine sin, an ceann sin", "he": "זה", "hi": "that person, that one", "hr": "taj, onaj", "hu": "az az ember, az a", "id": "itu", "it": "quello, colui", "ja": "その, その人", "kk": "that person, that one", "km": "that person, that one", "ko": "그 사람, 그것", "ku": "آن شخص, آن یکی", "lo": "that person, that one", "mg": "io olona io", "ms": "itu orang, itu barang", "my": "that person, that one", "pl": "tamta, tamten, tamto", "ro": "cette personne", "ru": "тот", "sk": "ten", "sl": "ta, tisti", "sv": "den där personen, den där, det där", "sw": "cette personne", "th": "คนนั้น, อัน/สิ่งนั้น", "tok": "ijo ni, jan ni", "tr": "bu kişi, bu şey", "uk": "той", "ur": "وہ, وہ شخص, وہ ایک", "vi": "người đó, cái đó", "yo": "that person, that one", "zh-tw": "那人", "zh": "那个人"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-tabelvorto-iu", typ: "vocab",
			content: map[string]interface{}{
				"word": "iu",
				"definition": "someone",
				"definitions": map[string]interface{}{"en": "someone", "nl": "iemand", "de": "jemand", "fr": "quelqu'un", "es": "alguien", "pt": "alguém", "ar": "شحص ما", "be": "кто-то", "ca": "algú, algun, alguna", "cs": "někdo, kdosi", "da": "nogen", "el": "κάποιος-α-ο", "fa": "یک شخصی, برخی, بعضی", "frp": "quelqu'un", "ga": "duine éigin", "he": "מישהו", "hi": "someone", "hr": "itko, netko", "hu": "valaki", "id": "seseorang", "it": "qualcuno", "ja": "だれか", "kk": "someone", "km": "someone", "ko": "누군가", "ku": "یک شخصی, برخی, بعضی", "lo": "someone", "mg": "olona anankiray", "ms": "seseorang", "my": "someone", "pl": "ktoś", "ro": "quelqu'un", "ru": "кто-то", "sk": "niekto, ktosi", "sl": "kdorkoli, nekdo", "sv": "någon, något", "sw": "quelqu'un", "th": "บางคน, บางอัน/สิ่ง", "tok": "ijo ni, jan ni", "tr": "bir kişi", "uk": "хтось", "ur": "کوئی", "vi": "ai đó", "yo": "someone", "zh-tw": "某人", "zh": "某人"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-tabelvorto-cxiu", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉiu",
				"definition": "everyone, every",
				"definitions": map[string]interface{}{"en": "everyone, every", "nl": "iedereen", "de": "jeder", "fr": "tout le monde, tous", "es": "cada uno, todos, todos los", "pt": "tudo, todo", "ar": "كل الأشخاص, كل ذلك الششء بالتحديد", "be": "каждый, все", "ca": "cada, cadascú, tots, tothom", "cs": "každý, veškerý", "da": "alle, hver", "el": "καθένας-μία-ένα", "fa": "هر کس, همه کس, هر, همه", "frp": "tout le monde, tous", "ga": "gach duine, cách, gach", "he": "כל", "hi": "everyone, every", "hr": "svatko, svaki", "hu": "mindenki, mindegyik", "id": "semua orang, setiap", "it": "ognuno, ciascuno", "ja": "それぞれ, みんな（複数で）", "kk": "everyone, every", "km": "everyone, every", "ko": "모든 사람, 모든", "ku": "هر کس, همه کس, هر, همه", "lo": "everyone, every", "mg": "olona rehetra, rehetra , daholo", "ms": "semua orang, semua", "my": "everyone, every", "pl": "każdy, wszyscy", "ro": "tout le monde, tous", "ru": "каждый, все", "sk": "každý, všetci", "sl": "vsak", "sv": "alla, varje", "sw": "tout le monde, tous", "th": "แต่ละคน, ทุก ๆ", "tok": "ijo ale, jan ale", "tr": "her bir kişi", "uk": "кожен, усякий", "ur": "سب, ہر کوئی, ہر", "vi": "mỗi người, mỗi thứ", "yo": "everyone, every", "zh-tw": "每人, 每個", "zh": "每个人, 每个"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-tabelvorto-neniu", typ: "vocab",
			content: map[string]interface{}{
				"word": "neniu",
				"definition": "no-one, none of them",
				"definitions": map[string]interface{}{"en": "no-one, none of them", "nl": "niemand", "de": "keiner", "fr": "personne, aucun d'eux", "es": "ninguno, ningún", "pt": "ninguém, nenhum", "ar": "ليس لأحد, نفى شئ على وجه الخصوص", "be": "никто, никакой из", "ca": "ningú, cap", "cs": "nikdo, žádný", "da": "ingen", "el": "κανένας-μία-ένα", "fa": "هیچ کس, هیچکدام", "frp": "personne, aucun d'eux", "ga": "éinne", "he": "אף אחד", "hi": "no-one, none of them", "hr": "nitko, nijedan", "hu": "senki, semelyik", "id": "tak seorangpun, tak sesuatupun", "it": "nessuno", "ja": "だれも～しない, どの～も～しない", "kk": "no-one, none of them", "km": "no-one, none of them", "ko": "아무도 아닌 사람, 아무것도 아닌", "ku": "هیچ کس, هیچکدام", "lo": "no-one, none of them", "mg": "na iza na iza, tsy misy na dia iray aza", "ms": "tidak orang, tidak ada mereka", "my": "no-one, none of them", "pl": "nikt", "ro": "personne, aucun d'eux", "ru": "никто, никакой из", "sk": "nikto, žiadny", "sl": "nobeden, nihče", "sv": "ingen, inget", "sw": "personne, aucun d'eux", "th": "ไม่มีสักคน, ไม่มีสักสิ่ง/อัน", "tok": "ijo ala, jan ala", "tr": "hiç kimse", "uk": "ніхто", "ur": "no-one, none of them", "vi": "không ai cả, không cái nào cả", "yo": "no-one, none of them", "zh-tw": "無人, 無一", "zh": "没有人, 没有他们"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-tabelvorto-kie", typ: "vocab",
			content: map[string]interface{}{
				"word": "kie",
				"definition": "where",
				"definitions": map[string]interface{}{"en": "where", "nl": "waar", "de": "wo", "fr": "où", "es": "dónde, donde", "pt": "onde", "ar": "أين, المكان الذي", "be": "где", "ca": "on", "cs": "kde", "da": "hvor", "el": "πού, όπου", "fa": "کجا, جایی که", "frp": "où", "ga": "cá", "he": "היכן", "hi": "where", "hr": "gdje", "hu": "hol", "id": "di mana", "it": "dove", "ja": "どこ", "kk": "where", "km": "where", "ko": "어디에", "ku": "کجا, جایی که", "lo": "where", "mg": "aiza", "ms": "di mana", "my": "where", "pl": "gdzie", "ro": "où", "ru": "где", "sk": "kde", "sl": "kje", "sv": "var", "sw": "où", "th": "ที่ไหน/ที่ซึ่ง", "tok": "ma seme", "tr": "nerede", "uk": "де", "ur": "کہاں", "vi": "ở đâu, ở nơi mà", "yo": "where", "zh-tw": "哪裡", "zh": "那里 （问题）"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-tabelvorto-tie", typ: "vocab",
			content: map[string]interface{}{
				"word": "tie",
				"definition": "there",
				"definitions": map[string]interface{}{"en": "there", "nl": "daar", "de": "dort", "fr": "là", "es": "ahí, allí, allá", "pt": "nesse lugar", "ar": "هناك", "be": "там", "ca": "allà", "cs": "tam", "da": "der", "el": "εκεί", "fa": "آن جا", "frp": "là", "ga": "ansin", "he": "שם", "hi": "there", "hr": "tamo, ondje", "hu": "ott", "id": "di sana", "it": "lì", "ja": "そこ", "kk": "there", "km": "there", "ko": "거기에", "ku": "آن جا", "lo": "there", "mg": "ao, ery", "ms": "di sana", "my": "there", "pl": "tam", "ro": "là", "ru": "там", "sk": "tam", "sl": "tam", "sv": "där", "sw": "là", "th": "ที่นั้น", "tok": "ma ni", "tr": "orada", "uk": "там", "ur": "وہاں", "vi": "ở đó", "yo": "there", "zh-tw": "那裡", "zh": "那里"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-tabelvorto-ie", typ: "vocab",
			content: map[string]interface{}{
				"word": "ie",
				"definition": "somewhere",
				"definitions": map[string]interface{}{"en": "somewhere", "nl": "ergens", "de": "irgendwo", "fr": "quelque part", "es": "en algún sitio, en alguna parte", "pt": "em algum lugar", "ar": "مكان ما", "be": "где-то", "ca": "en algún lloc", "cs": "někde, kdesi", "da": "et sted", "el": "κάπου", "fa": "یک جایی", "frp": "quelque part", "ga": "áit éigin", "he": "במקום כלשהו", "hi": "somewhere", "hr": "negdje, igdje", "hu": "valahol", "id": "di suatu tempat", "it": "da qualche parte", "ja": "どこか", "kk": "somewhere", "km": "somewhere", "ko": "어딘가에", "ku": "یک جایی", "lo": "somewhere", "mg": "any ho any", "ms": "di suatu tempat", "my": "somewhere", "pl": "gdzieś", "ro": "quelque part", "ru": "где-то", "sk": "niekde, kdesi", "sl": "nekje, kjerkoli", "sv": "någonstans", "sw": "quelque part", "th": "บางที่", "tok": "ma", "tr": "bir yerde", "uk": "десь", "ur": "کہیں", "vi": "ở đâu đó, ở nơi nào đó", "yo": "somewhere", "zh-tw": "某處", "zh": "某地"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-tabelvorto-cxie", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉie",
				"definition": "everywhere",
				"definitions": map[string]interface{}{"en": "everywhere", "nl": "overal", "de": "überall", "fr": "partout", "es": "en todas partes, en todos los sitios", "pt": "em todo lugar", "ar": "في كل مكان", "be": "везде", "ca": "arreu, pertot", "cs": "všude", "da": "alle steder", "el": "παντού", "fa": "هر جایی", "frp": "partout", "ga": "gach áit", "he": "בכל מקום", "hi": "everywhere", "hr": "svagdje", "hu": "mindenhol", "id": "di mana-mana, di semua tempat", "it": "dovunque", "ja": "どこでも", "kk": "everywhere", "km": "everywhere", "ko": "모든 곳에", "ku": "هر جایی", "lo": "everywhere", "mg": "hatraiza hatraiza", "ms": "semua tempat", "my": "everywhere", "pl": "wszędzie", "ro": "partout", "ru": "везде", "sk": "všade", "sl": "povsod", "sv": "överallt", "sw": "partout", "th": "ทุก ๆ ที่", "tok": "ma ale", "tr": "her yerde", "uk": "усюди", "ur": "ہر جگہ", "vi": "mọi nơi", "yo": "everywhere", "zh-tw": "四處", "zh": "四处"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-tabelvorto-nenie", typ: "vocab",
			content: map[string]interface{}{
				"word": "nenie",
				"definition": "nowhere",
				"definitions": map[string]interface{}{"en": "nowhere", "nl": "nergens", "de": "nirgends", "fr": "nulle part", "es": "en ninguna parte, en ningún sitio", "pt": "em lugar nenhum", "ar": "ليس بأى مكان", "be": "нигде", "ca": "enlloc, a cap lloc", "cs": "nikde", "da": "ingen steder", "el": "πουθενά", "fa": "هیچ جا", "frp": "nulle part", "ga": "in aon áit", "he": "שאף מקום", "hi": "nowhere", "hr": "nigdje", "hu": "sehol", "id": "tak dimanapun", "it": "da nessuna parte", "ja": "どこも～ない", "kk": "nowhere", "km": "nowhere", "ko": "아무데도", "ku": "هیچ جا", "lo": "nowhere", "mg": "naiza naiza", "ms": "di mana-mana", "my": "nowhere", "pl": "nigdzie", "ro": "nulle part", "ru": "нигде", "sk": "nikde", "sl": "nikjer", "sv": "ingenstans", "sw": "nulle part", "th": "ไม่มีสักที่", "tok": "ma ala", "tr": "hiç bir yerde", "uk": "ніде", "ur": "کہیں نہیں", "vi": "không nơi nào", "yo": "nowhere", "zh-tw": "無處", "zh": "无处"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-tabelvorto-kia", typ: "vocab",
			content: map[string]interface{}{
				"word": "kia",
				"definition": "what kind of",
				"definitions": map[string]interface{}{"en": "what kind of", "nl": "welke soort, wat voor", "de": "was für ein", "fr": "de quelle sorte", "es": "qué, qué tipo de", "pt": "de que tipo", "ar": "كيف يبدو", "be": "какой", "ca": "com (sentit adjectiu, no adverbial), quina mena de", "cs": "jaký", "da": "hvilken slags", "el": "τί είδους", "fa": "چگونه, به گونه‌ای که", "frp": "de quelle sorte", "ga": "cén saghas", "he": "איזה סוג", "hi": "what kind of", "hr": "kakav", "hu": "milyen", "id": "jenis apa, betapa", "it": "quale, di quale specie", "ja": "どのような", "kk": "what kind of", "km": "what kind of", "ko": "무슨 종류의, 무슨 특성의", "ku": "چگونه, به گونه‌ای که", "lo": "what kind of", "mg": "karazana inona", "ms": "apa jenis", "my": "what kind of", "pl": "jaki, jaka, jakie", "ro": "de quelle sorte", "ru": "какой", "sk": "aký", "sl": "kakšen", "sv": "hurdan", "sw": "de quelle sorte", "th": "ลักษณะเป็นอย่างไร, แบบไหน, /ซึ่งเป็นลักษณะที่, ซึ่งเป็นแบบที่", "tok": "kule seme", "tr": "ne çeşit", "uk": "який", "ur": "کس قسم کا", "vi": "loại gì vậy, cái loại mà", "yo": "what kind of", "zh-tw": "怎樣的", "zh": "什么样子的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-tabelvorto-tia", typ: "vocab",
			content: map[string]interface{}{
				"word": "tia",
				"definition": "that kind of",
				"definitions": map[string]interface{}{"en": "that kind of", "nl": "zulke", "de": "solch ein", "fr": "de cette sorte", "es": "tal, ese", "pt": "desse tipo", "ar": "من ذلك النوع, من تلك الشاكلة", "be": "такой", "ca": "tal, així (sentit adjectiu, no adverbial)", "cs": "takový", "da": "den slags", "el": "τέτοιος-α-ο", "fa": "آن گونه", "frp": "de cette sorte", "ga": "an saghas sin", "he": "כזה", "hi": "that kind of", "hr": "takav, onakav", "hu": "olyan", "id": "yang begitu, semacam itu", "it": "tale, di quella specie", "ja": "そのような", "kk": "that kind of", "km": "that kind of", "ko": "그런 종류의, 그런 특성의", "ku": "آن گونه", "lo": "that kind of", "mg": "amin'ity karazana ity", "ms": "jenis ini", "my": "that kind of", "pl": "taki, taka, takie", "ro": "de cette sorte", "ru": "такой", "sk": "taký", "sl": "takšen", "sv": "sådan, sådant", "sw": "de cette sorte", "th": "ลักษณะนั้น, แบบนั้น", "tok": "kule ni", "tr": "bu çeşit", "uk": "такий", "ur": "اس قسم کا", "vi": "cái loại đó", "yo": "that kind of", "zh-tw": "那樣的", "zh": "那种样子的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-tabelvorto-ia", typ: "vocab",
			content: map[string]interface{}{
				"word": "ia",
				"definition": "some kind of",
				"definitions": map[string]interface{}{"en": "some kind of", "nl": "eender welke soort", "de": "irgendein", "fr": "une sorte de", "es": "algún tipo de", "pt": "de algum tipo", "ar": "نوعاً ما", "be": "какой-то", "ca": "alguna mena de", "cs": "nějaký, jakýsi", "da": "en slags", "el": "κάποιου είδους", "fa": "یک گونه‌ای", "frp": "une sorte de", "ga": "saghas éigin", "he": "מסוג כשלהו", "hi": "some kind of", "hr": "nekakav, ikakav", "hu": "valamilyen", "id": "sejenis, semacam", "it": "qualche, di qualche specie", "ja": "なんらかの", "kk": "some kind of", "km": "some kind of", "ko": "어떤 특성의", "ku": "یک گونه‌ای", "lo": "some kind of", "mg": "karazany", "ms": "beberapa jenis", "my": "some kind of", "pl": "jakiś", "ro": "une sorte de", "ru": "какой-то", "sk": "nejaký, akýsi", "sl": "nekakšen", "sv": "någon slags", "sw": "une sorte de", "th": "บางลักษณะ", "tok": "kule", "tr": "bir çeşit", "uk": "якийсь", "ur": "کسی قسم کا", "vi": "loại gì đó", "yo": "some kind of", "zh-tw": "某樣的", "zh": "某个样子的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-tabelvorto-cxia", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉia",
				"definition": "every kind of",
				"definitions": map[string]interface{}{"en": "every kind of", "nl": "alle soorten", "de": "jederlei", "fr": "de toute sorte", "es": "de todos los tipos, de todas als clases", "pt": "de todo tipo", "ar": "على كل شاكلة", "be": "всякий", "ca": "de tota mena", "cs": "všelijaký", "da": "alle slags", "el": "κάθε είδους", "fa": "هر گونه", "frp": "de toute sorte", "ga": "gach saghas", "he": "מכל סוג", "hi": "every kind of", "hr": "svakakav", "hu": "mindenféle", "id": "segala macam", "it": "ogni, di ogni specie", "ja": "あらゆる種類の", "kk": "every kind of", "km": "every kind of", "ko": "모든 특성의", "ku": "هر گونه", "lo": "every kind of", "mg": "amin'ny karazany rehetra", "ms": "semua jenis", "my": "every kind of", "pl": "wszelaki", "ro": "de toute sorte", "ru": "всякий", "sk": "všelijaký", "sl": "kakršenkoli", "sv": "allt slags", "sw": "de toute sorte", "th": "ทุกลักษณะ", "tok": "kule ale", "tr": "her çeşit", "uk": "усякий", "ur": "ہر قسم کا", "vi": "đủ mọi loại", "yo": "every kind of", "zh-tw": "每樣的", "zh": "每样的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-tabelvorto-nenia", typ: "vocab",
			content: map[string]interface{}{
				"word": "nenia",
				"definition": "no kind of",
				"definitions": map[string]interface{}{"en": "no kind of", "nl": "geen enkele soort", "de": "keinerlei", "fr": "d'aucune sorte", "es": "de ningún tipo, de ninguna clase", "pt": "de nenhum tipo", "ar": "ليس بأى نوع", "be": "никакой", "ca": "de cap mena", "cs": "nijaký", "da": "ingen slags", "el": "κανενός είδους", "fa": "هیچ گونه", "frp": "d'aucune sorte", "ga": "gan aon saghas", "he": "משום סוג", "hi": "no kind of", "hr": "nikakav", "hu": "semmilyen", "id": "tidak ada semacam itu", "it": "di nessuna specie", "ja": "どんな～も～ない", "kk": "no kind of", "km": "no kind of", "ko": "어떤 특성도 아닌", "ku": "هیچ گونه", "lo": "no kind of", "mg": "na inona na inona", "ms": "tiada jenis", "my": "no kind of", "pl": "nijaki, żaden", "ro": "d'aucune sorte", "ru": "никакой", "sk": "nijaký", "sl": "nikakšen", "sv": "inget slags", "sw": "d'aucune sorte", "th": "ไม่มีสักลักษณะ", "tok": "kule ala", "tr": "hiç bir çeşit", "uk": "ніякий", "ur": "کسی قسم کا نہیں", "vi": "không loại nào", "yo": "no kind of", "zh-tw": "不怎樣的", "zh": "无样的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-tabelvorto-kiel", typ: "vocab",
			content: map[string]interface{}{
				"word": "kiel",
				"definition": "how",
				"definitions": map[string]interface{}{"en": "how", "nl": "hoe", "de": "wie", "fr": "comment", "es": "cómo, como", "pt": "como", "ar": "كيف", "be": "как", "ca": "com (sentit adverbial)", "cs": "jak", "da": "hvordan", "el": "πώς, όπως", "fa": "چطور, به طوری که", "frp": "comment", "ga": "conas", "he": "איך", "hi": "how", "hr": "kako", "hu": "hogyan", "id": "bagaimana", "it": "come", "ja": "どのように", "kk": "how", "km": "how", "ko": "어떻게", "ku": "چطور, به طوری که", "lo": "how", "mg": "ahoana , manao ahoana", "ms": "begaimana", "my": "how", "pl": "jak", "ro": "comment", "ru": "как", "sk": "ako", "sl": "kako", "sv": "hur", "sw": "comment", "th": "วิธีใด, อย่างไร, /ดังเช่น, เหมือนกับ", "tok": "nasin seme", "tr": "nasıl", "uk": "як", "ur": "کیسے", "vi": "làm cách nào, như thế nào, cái cách mà", "yo": "how", "zh-tw": "如何", "zh": "怎么"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-tabelvorto-tiel", typ: "vocab",
			content: map[string]interface{}{
				"word": "tiel",
				"definition": "like that, thus",
				"definitions": map[string]interface{}{"en": "like that, thus", "nl": "zo", "de": "so", "fr": "ainsi", "es": "así, tan", "pt": "assim", "ar": "بتلك الطريقة", "be": "так", "ca": "així, d'aquesta manera, tan", "cs": "tak", "da": "sådan", "el": "έτσι", "fa": "آن طور", "frp": "ainsi", "ga": "mar sin", "he": "כמו, כזה", "hi": "like that, thus", "hr": "tako, onako", "hu": "úgy", "id": "seperti itu", "it": "così", "ja": "そのように", "kk": "like that, thus", "km": "like that, thus", "ko": "그렇게, 그래서", "ku": "آن طور", "lo": "like that, thus", "mg": "tahaka izany", "ms": "begini", "my": "like that, thus", "pl": "tak (w ten sposób)", "ro": "ainsi", "ru": "так", "sk": "tak", "sl": "tako", "sv": "så, på det sättet", "sw": "ainsi", "th": "วิธีนั้น, อย่างนั้น", "tok": "nasin ni", "tr": "bu şekilde", "uk": "так", "ur": "ویسے", "vi": "bằng cách đấy, vì vậy nên", "yo": "like that, thus", "zh-tw": "那樣地, 如此", "zh": "这样，如此"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 22,
		},
		{
			slug: "vortaro-tabelvorto-iel", typ: "vocab",
			content: map[string]interface{}{
				"word": "iel",
				"definition": "in some way",
				"definitions": map[string]interface{}{"en": "in some way", "nl": "op een of andere manier", "de": "irgendwie", "fr": "d'une certaine manière", "es": "de alguna manera, de algún modo", "pt": "de alguma maneira", "ar": "بطريقة ما", "be": "как-то", "ca": "d'alguna manera", "cs": "nějak, jaksi", "da": "på en eller anden måde", "el": "κάπως", "fa": "به روشی", "frp": "d'une certaine manière", "ga": "ar shlí éigin", "he": "באיזה שהוא אופן", "hi": "in some way", "hr": "nekako, ikako", "hu": "valahogyan", "id": "dengan suatu cara", "it": "in qualche modo", "ja": "なんとかして", "kk": "in some way", "km": "in some way", "ko": "어떤 방식으로든", "ku": "به روشی", "lo": "in some way", "mg": "toa", "ms": "dalam sesuatu cara", "my": "in some way", "pl": "jakoś", "ro": "d'une certaine manière", "ru": "как-то", "sk": "nejako, akosi", "sl": "nekako", "sv": "på något sätt", "sw": "d'une certaine manière", "th": "บางอย่าง", "tok": "nasin", "tr": "bir şekilde", "uk": "якось", "ur": "کسی طرح", "vi": "bằng cách nào đó", "yo": "in some way", "zh-tw": "以某方式", "zh": "在某种程度上来说"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 23,
		},
		{
			slug: "vortaro-tabelvorto-cxiel", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉiel",
				"definition": "in every way",
				"definitions": map[string]interface{}{"en": "in every way", "nl": "op alle manieren", "de": "auf jede Weise", "fr": "de toute manière", "es": "de todas maneras, de todos modos", "pt": "de toda maneira", "ar": "في كل الأحوال", "be": "всячески, по-всякому", "ca": "de totes les maneres", "cs": "všelijak", "da": "på alle måder", "el": "με κάθε τρόπο", "fa": "به هر روشی", "frp": "de toute manière", "ga": "i ngach slí", "he": "בכל אופן", "hi": "in every way", "hr": "svakako", "hu": "mindenhogyan", "id": "dengan cara apapun", "it": "in ogni modo", "ja": "あらゆる方法で", "kk": "in every way", "km": "in every way", "ko": "모든 방식으로", "ku": "به هر روشی", "lo": "in every way", "mg": "ihany", "ms": "semua cara", "my": "in every way", "pl": "na wszelki sposób", "ro": "de toute manière", "ru": "всячески, по-всякому", "sk": "všelijako", "sl": "na katerikoli način", "sv": "på alla sätt", "sw": "de toute manière", "th": "ทุกวิธี", "tok": "nasin ale", "tr": "her şekilde", "uk": "по всякому, у будь-який спосіб", "ur": "ہر طرح", "vi": "bằng mọi cách", "yo": "in every way", "zh-tw": "以各方式", "zh": "各方面"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 24,
		},
		{
			slug: "vortaro-tabelvorto-neniel", typ: "vocab",
			content: map[string]interface{}{
				"word": "neniel",
				"definition": "in no way",
				"definitions": map[string]interface{}{"en": "in no way", "nl": "op geen enkele manier", "de": "in keiner Weise", "fr": "d'aucune manière", "es": "de ninguna manera, de ningún modo", "pt": "de jeito nenhum", "ar": "استحالة", "be": "никак", "ca": "de cap manera", "cs": "nijak", "da": "på ingen måde", "el": "με κανένα τρόπο", "fa": "به هیچ روشی", "frp": "d'aucune manière", "ga": "in aon slí", "he": "בשום אופן", "hi": "in no way", "hr": "nikako, ni na koji način", "hu": "sehogyan", "id": "bagaimanapun tidak, tidak sama sekali", "it": "in nessun modo", "ja": "どうしても～ない", "kk": "in no way", "km": "in no way", "ko": "어떤 방식도 아닌", "ku": "به هیچ روشی", "lo": "in no way", "mg": "na ahoana na ahoana", "ms": "tidak ada cara", "my": "in no way", "pl": "w żaden sposób", "ro": "d'aucune manière", "ru": "никак", "sk": "nijako", "sl": "nikakor, na noben način", "sv": "på inget sätt", "sw": "d'aucune manière", "th": "ไม่มีสักวิธี", "tok": "nasin ala", "tr": "hiçbir şekilde", "uk": "ніяк", "ur": "کسی طرح نہیں", "vi": "không có cách nào", "yo": "in no way", "zh-tw": "無從", "zh": "无从"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 25,
		},
		{
			slug: "vortaro-tabelvorto-kial", typ: "vocab",
			content: map[string]interface{}{
				"word": "kial",
				"definition": "why",
				"definitions": map[string]interface{}{"en": "why", "nl": "waarom", "de": "warum", "fr": "pourquoi", "es": "por qué", "pt": "por que", "ar": "لماذا", "be": "почему", "ca": "per què", "cs": "proč", "da": "hvorfor", "el": "γιατί, επειδή", "fa": "چرا, به دلیل آن که", "frp": "pourquoi", "ga": "cén fáth", "he": "למה", "hi": "why", "hr": "zašto", "hu": "miért", "id": "mengapa", "it": "perché", "ja": "なぜ", "kk": "why", "km": "why", "ko": "왜", "ku": "چرا, به دلیل آن که", "lo": "why", "mg": "nahoana", "ms": "kenapa", "my": "why", "pl": "dlaczego", "ro": "pourquoi", "ru": "почему", "sk": "prečo", "sl": "zakaj", "sv": "varför", "sw": "pourquoi", "th": "ทำไม, เหตุใด", "tok": "tan seme", "tr": "neden", "uk": "чому", "ur": "کیوں", "vi": "tại sao", "yo": "why", "zh-tw": "為什麼，為何", "zh": "为什么，为何"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 26,
		},
		{
			slug: "vortaro-tabelvorto-tial", typ: "vocab",
			content: map[string]interface{}{
				"word": "tial",
				"definition": "for that reason",
				"definitions": map[string]interface{}{"en": "for that reason", "nl": "daarom", "de": "deshalb", "fr": "pour cette raison", "es": "por esa razón, por ese motivo", "pt": "por essa razão", "ar": "لذلك", "be": "потому", "ca": "per tal raó, per tal motiu, per això", "cs": "proto", "da": "derfor", "el": "για αυτό το λόγο", "fa": "به آن دلیل", "frp": "pour cette raison", "ga": "ar an gcúis sin", "he": "ככה, מסיבה זו", "hi": "for that reason", "hr": "zbog toga, zato", "hu": "azért", "id": "untuk alasan itu", "it": "perciò, per questa ragione", "ja": "だから", "kk": "for that reason", "km": "for that reason", "ko": "그 이유로", "ku": "به آن دلیل", "lo": "for that reason", "mg": "noho io antony io", "ms": "untuk alasan itu", "my": "for that reason", "pl": "dlatego", "ro": "pour cette raison", "ru": "потому", "sk": "preto", "sl": "zato", "sv": "därför", "sw": "pour cette raison", "th": "เหตุนั้น", "tok": "tan ni", "tr": "bu sebepten dolayı", "uk": "тому", "ur": "اس وجہ سے", "vi": "vì lý do đó", "yo": "for that reason", "zh-tw": "為那原因", "zh": "基于那个原因"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 27,
		},
		{
			slug: "vortaro-tabelvorto-ial", typ: "vocab",
			content: map[string]interface{}{
				"word": "ial",
				"definition": "for some reason",
				"definitions": map[string]interface{}{"en": "for some reason", "nl": "om een of andere reden", "de": "aus irgendeinem Grund", "fr": "pour une certaine raison", "es": "por alguna razón, por algún motivo", "pt": "por alguma razão", "ar": "لسبب ما", "be": "почему-то", "ca": "per alguna raó, per algún motiu", "cs": "z nějakého důvodu", "da": "af en eller anden årsag", "el": "για κάποιο λόγο", "fa": "به دلیلی", "frp": "pour une certaine raison", "ga": "ar chúis éigin", "he": "מסיבה כלשהי", "hi": "for some reason", "hr": "zbog nekog razloga", "hu": "valamiért", "id": "suatu alasan", "it": "per qualche motivo", "ja": "なにかの理由で", "kk": "for some reason", "km": "for some reason", "ko": "(불특정한) 어떤 이유로", "ku": "به دلیلی", "lo": "for some reason", "mg": "noho ny antony sasany", "ms": "untuk alasan lain", "my": "for some reason", "pl": "z jakiegoś powodu", "ro": "pour une certaine raison", "ru": "почему-то", "sk": "z nejakého dôvodu", "sl": "zaradi nečesa", "sv": "av någon anledning", "sw": "pour une certaine raison", "th": "บางเหตุผล", "tok": "tan ijo", "tr": "bir sebepten dolayı", "uk": "чомусь", "ur": "کسی وجہ سے", "vi": "vì lý do nào đó", "yo": "for some reason", "zh-tw": "為某原因", "zh": "基于一些原因"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 28,
		},
		{
			slug: "vortaro-tabelvorto-cxial", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉial",
				"definition": "for every reason",
				"definitions": map[string]interface{}{"en": "for every reason", "nl": "om alle redenen", "de": "aus jedem Grund", "fr": "pour toutes les raisons", "es": "por cualquier razón, por cualquier motivo", "pt": "por toda razão", "ar": "لكل الأسباب", "be": "по-всякой (любой) причине", "ca": "per qualsevol raó, per qualsevol motiu, per tots els motius, per tot", "cs": "ze všech důvodů", "da": "af alle årsager", "el": "για κάθε λόγο (αιτία)", "fa": "به هر دلیلی", "frp": "pour toutes les raisons", "ga": "ar gach cúis", "he": "מכל סיבה", "hi": "for every reason", "hr": "zbog svega", "hu": "mindenért", "id": "untuk setiap alasan", "it": "per tutte le ragioni", "ja": "あらゆる理由で", "kk": "for every reason", "km": "for every reason", "ko": "모든 이유로", "ku": "به هر دلیلی", "lo": "for every reason", "mg": "noho ny antony rehetra", "ms": "untuk semua alasan", "my": "for every reason", "pl": "z każdego powodu", "ro": "pour toutes les raisons", "ru": "по-всякой (любой) причине", "sk": "zo všetkých dôvodov", "sl": "zaradi vsega", "sv": "av alla skäl", "sw": "pour toutes les raisons", "th": "ทุก ๆ เหตุผล", "tok": "tan ale", "tr": "her sebepten dolayı", "uk": "із усякої причини", "ur": "ہر وجہ سے", "vi": "vì mọi lý do", "yo": "for every reason", "zh-tw": "為各原因", "zh": "基于每一个原因"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 29,
		},
		{
			slug: "vortaro-tabelvorto-nenial", typ: "vocab",
			content: map[string]interface{}{
				"word": "nenial",
				"definition": "for no reason",
				"definitions": map[string]interface{}{"en": "for no reason", "nl": "om geen enkele reden", "de": "aus keinem Grund", "fr": "pour aucune raison", "es": "por ninguna razón, por ningún motivo", "pt": "por nenhuma razão", "ar": "بدون أى أسباب", "be": "без причины", "ca": "per cap raó, per cap motiu", "cs": "bez důvodů", "da": "uden årsag", "el": "για κανένα λόγο", "fa": "به هیچ دلیلی", "frp": "pour aucune raison", "ga": "gan chúis", "he": "משום סיבה", "hi": "for no reason", "hr": "ni zbog čega", "hu": "semmiért", "id": "bukan alasan apapun", "it": "per nessun motivo", "ja": "どんな理由でも～ない", "kk": "for no reason", "km": "for no reason", "ko": "아무 이유도 아니게", "ku": "به هیچ دلیلی", "lo": "for no reason", "mg": "tsy misy antony", "ms": "untuk tidak ada alasan", "my": "for no reason", "pl": "z żadnego powodu", "ro": "pour aucune raison", "ru": "без причины", "sk": "bez dôvodu", "sl": "iz nobenega vzroka", "sv": "av ingen orsak", "sw": "pour aucune raison", "th": "ไม่มีเหตุผล", "tok": "tan ala", "tr": "hiç bir sebeten dolayı", "uk": "просто так, безпричинно, без жодної причини", "ur": "کسی وجہ سے نہیں", "vi": "không vì lý do nào cả", "yo": "for no reason", "zh-tw": "不為任何原因", "zh": "基于没有原因"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 30,
		},
		{
			slug: "vortaro-tabelvorto-kiam", typ: "vocab",
			content: map[string]interface{}{
				"word": "kiam",
				"definition": "when",
				"definitions": map[string]interface{}{"en": "when", "nl": "wanneer", "de": "wann", "fr": "quand", "es": "cuándo, cuando", "pt": "quando", "ar": "حينما, متى", "be": "когда", "ca": "quan", "cs": "kdy, když", "da": "hvornår", "el": "πότε, όταν", "fa": "چه زمانی, هنگامی که", "frp": "quand", "ga": "cathain, nuair", "he": "מתי", "hi": "when", "hr": "kada", "hu": "mikor", "id": "ketika", "it": "quando", "ja": "いつ", "kk": "when", "km": "when", "ko": "언제", "ku": "چه زمانی, هنگامی که", "lo": "when", "mg": "rahoviana", "ms": "bila", "my": "when", "pl": "kiedy", "ro": "quand", "ru": "когда", "sk": "kedy, keď", "sl": "kdaj", "sv": "när", "sw": "quand", "th": "เวลาใด, ตอนไหน, /เมื่อ", "tok": "tenpo seme", "tr": "ne zaman", "uk": "коли", "ur": "کب", "vi": "khi nào, khi mà", "yo": "when", "zh-tw": "何時", "zh": "什么时候，何时"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 31,
		},
		{
			slug: "vortaro-tabelvorto-tiam", typ: "vocab",
			content: map[string]interface{}{
				"word": "tiam",
				"definition": "then",
				"definitions": map[string]interface{}{"en": "then", "nl": "dan", "de": "dann", "fr": "à ce moment", "es": "entonces", "pt": "nesse momento", "ar": "ومن ثمَّ", "be": "тогда", "ca": "llavors, aleshores, en aquell moment", "cs": "tehdy", "da": "på det tidspunkt", "el": "τότε", "fa": "آنگاه", "frp": "à ce moment", "ga": "ansin", "he": "אז", "hi": "then", "hr": "tada, onda", "hu": "akkor", "id": "waktu itu", "it": "allora", "ja": "そのとき", "kk": "then", "km": "then", "ko": "그때", "ku": "آنگاه", "lo": "then", "mg": "amin'izao fotoana izao", "ms": "masa itu", "my": "then", "pl": "wtedy", "ro": "à ce moment", "ru": "тогда", "sk": "vtedy", "sl": "takrat", "sv": "då", "sw": "à ce moment", "th": "เวลานั้น, ตอนนั้น", "tok": "tenpo ni", "tr": "bu zaman", "uk": "тоді", "ur": "تب", "vi": "lúc đấy", "yo": "then", "zh-tw": "那時", "zh": "那个时候"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 32,
		},
		{
			slug: "vortaro-tabelvorto-iam", typ: "vocab",
			content: map[string]interface{}{
				"word": "iam",
				"definition": "some time, ever",
				"definitions": map[string]interface{}{"en": "some time, ever", "nl": "ooit", "de": "irgendwann", "fr": "un jour, à ce moment", "es": "en algún momento", "pt": "em algum momento", "ar": "في وفت ما", "be": "когда-то", "ca": "en algún moment, mai (en preguntes i condicions)", "cs": "někdy, kdysi", "da": "engang", "el": "κάποτε", "fa": "یک زمانی", "frp": "un jour, à ce moment", "ga": "uair éigin", "he": "מתי שהוא", "hi": "some time, ever", "hr": "ikad, nekad", "hu": "valamikor", "id": "pernah, pada suatu waktu", "it": "una volta, qualche volta", "ja": "いつか", "kk": "some time, ever", "km": "some time, ever", "ko": "언젠가", "ku": "یک زمانی", "lo": "some time, ever", "mg": "indray andro any, amin'izao fotoana izao", "ms": "sewaktu", "my": "some time, ever", "pl": "kiedyś", "ro": "un jour, à ce moment", "ru": "когда-то", "sk": "niekedy, kedysi", "sl": "nekoč", "sv": "någon gång, en gång, vid någon tid", "sw": "un jour, à ce moment", "th": "บางเวลา, เคย", "tok": "tenpo", "tr": "bir zaman", "uk": "колись", "ur": "کبھی", "vi": "đôi lúc", "yo": "some time, ever", "zh-tw": "某時", "zh": "有时候"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 33,
		},
		{
			slug: "vortaro-tabelvorto-cxiam", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉiam",
				"definition": "always, every time",
				"definitions": map[string]interface{}{"en": "always, every time", "nl": "altijd", "de": "immer", "fr": "toujours", "es": "siempre", "pt": "sempre", "ar": "دائماً", "be": "всегда", "ca": "sempre", "cs": "vždy", "da": "altid", "el": "πάντα", "fa": "همیشه", "frp": "toujours", "ga": "i gcónaí", "he": "תמיד", "hi": "always, every time", "hr": "uvijek", "hu": "mindig", "id": "selalu, setiap saat", "it": "sempre", "ja": "いつも", "kk": "always, every time", "km": "always, every time", "ko": "항상", "ku": "همیشه", "lo": "always, every time", "mg": "mandrakariva", "ms": "selalu", "my": "always, every time", "pl": "zawsze", "ro": "toujours", "ru": "всегда", "sk": "vždy", "sl": "vedno", "sv": "alltid", "sw": "toujours", "th": "ทุกเวลา, เสมอ ๆ", "tok": "tenpo ale", "tr": "her zaman", "uk": "завжди", "ur": "ہمیشہ", "vi": "lúc nào, mọi lúc", "yo": "always, every time", "zh-tw": "總是", "zh": "经常，每一次"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 34,
		},
		{
			slug: "vortaro-tabelvorto-neniam", typ: "vocab",
			content: map[string]interface{}{
				"word": "neniam",
				"definition": "never, no time",
				"definitions": map[string]interface{}{"en": "never, no time", "nl": "nooit", "de": "nie", "fr": "jamais", "es": "nunca", "pt": "nunca, jamais", "ar": "أبداً", "be": "никогда", "ca": "mai (en negacions)", "cs": "nikdy", "da": "aldrig", "el": "ποτέ", "fa": "هیچ وقت", "frp": "jamais", "ga": "riamh,choíche", "he": "אף פעם", "hi": "never, no time", "hr": "nikad", "hu": "soha", "id": "tidak pernah", "it": "mai", "ja": "決して～ない", "kk": "never, no time", "km": "never, no time", "ko": "영원히 아닌", "ku": "هیچ وقت", "lo": "never, no time", "mg": "na oviana na oviana", "ms": "tidak pernah", "my": "never, no time", "pl": "nigdy", "ro": "jamais", "ru": "никогда", "sk": "nikdy", "sl": "nikoli", "sv": "aldrig", "sw": "jamais", "th": "ไม่มีสักเวลา, ไม่เคย", "tok": "tenpo ala", "tr": "hiç bir zaman", "uk": "ніколи", "ur": "کبھی نہیں", "vi": "không bao giờ", "yo": "never, no time", "zh-tw": "從不", "zh": "从不"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 35,
		},
		{
			slug: "vortaro-tabelvorto-kiom", typ: "vocab",
			content: map[string]interface{}{
				"word": "kiom",
				"definition": "how much",
				"definitions": map[string]interface{}{"en": "how much", "nl": "hoeveel", "de": "wieviel", "fr": "combien", "es": "cuánto", "pt": "quanto", "ar": "للسؤال عن الكمية", "be": "сколько", "ca": "quant, quants/quantes", "cs": "kolik", "da": "hvor meget", "el": "πόσο, όσο", "fa": "چقدر, چند تا, مقداری که", "frp": "combien", "ga": "cé mhéad", "he": "כמה", "hi": "how much", "hr": "koliko", "hu": "mennyi", "id": "seberapa banyak", "it": "quanto", "ja": "どれくらい", "kk": "how much", "km": "how much", "ko": "얼마나 많이", "ku": "چقدر, چند تا, مقداری که", "lo": "how much", "mg": "firy, hoatrinona", "ms": "berapa", "my": "how much", "pl": "ile", "ro": "combien", "ru": "сколько", "sk": "koľko", "sl": "koliko", "sv": "hur mycket, hur många", "sw": "combien", "th": "เท่าไหร่", "tok": "mute seme", "tr": "ne kadar", "uk": "скільки", "ur": "کتنا", "vi": "bao nhiêu", "yo": "how much", "zh-tw": "多少", "zh": "多少"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 36,
		},
		{
			slug: "vortaro-tabelvorto-tiom", typ: "vocab",
			content: map[string]interface{}{
				"word": "tiom",
				"definition": "that much",
				"definitions": map[string]interface{}{"en": "that much", "nl": "zoveel", "de": "soviel", "fr": "de cette quantité", "es": "tanto, tan", "pt": "tanto, tão", "ar": "تلك الكمية, الكثير", "be": "столько", "ca": "tant, tants/tantes", "cs": "tolik", "da": "så meget", "el": "τόσο", "fa": "آن قدر, به آن تعداد", "frp": "de cette quantité", "ga": "an méid sin", "he": "בכמות כזו", "hi": "that much", "hr": "toliko, onoliko", "hu": "annyi", "id": "sejumlah itu", "it": "tanto", "ja": "それだけ", "kk": "that much", "km": "that much", "ko": "그만큼 많이", "ku": "آن قدر, به آن تعداد", "lo": "that much", "mg": "be loatra", "ms": "begitu banyak", "my": "that much", "pl": "tyle", "ro": "de cette quantité", "ru": "столько", "sk": "toľko", "sl": "toliko", "sv": "så mycket, så många", "sw": "de cette quantité", "th": "จำนวนนั้น", "tok": "mute ni", "tr": "şu kadar", "uk": "стільки", "ur": "اتنا", "vi": "bấy nhiêu đó", "yo": "that much", "zh-tw": "那些", "zh": "那样的数量"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 37,
		},
		{
			slug: "vortaro-tabelvorto-iom", typ: "vocab",
			content: map[string]interface{}{
				"word": "iom",
				"definition": "to some extent, a certain amount",
				"definitions": map[string]interface{}{"en": "to some extent, a certain amount", "nl": "een zekere hoeveelheid, een beetje", "de": "etwas, ein wenig", "fr": "un peu", "es": "un poco", "pt": "até certo ponto, certa quantidade", "ar": "كمية ما, القليل", "be": "сколько-то", "ca": "una mica, una certa quantitat, gens (en preguntes)", "cs": "trochu", "da": "en hvis mængde", "el": "λίγο (σε κάποια ποσότητα)", "fa": "مقداری, تعدادی", "frp": "un peu", "ga": "méid áirithe, oiread", "he": "קצת", "hi": "to some extent, a certain amount", "hr": "ikoliko, nekoliko", "hu": "valamennyi, egy kis", "id": "sejumlah", "it": "un poco, qualche volta", "ja": "ちょっと, いくらかの", "kk": "to some extent, a certain amount", "km": "to some extent, a certain amount", "ko": "약간량의", "ku": "مقداری, تعدادی", "lo": "to some extent, a certain amount", "mg": "kely", "ms": "sehingga satu tahap, jumlah tertentu", "my": "to some extent, a certain amount", "pl": "trochę, ileś", "ro": "un peu", "ru": "сколько-то", "sk": "niekoľko", "sl": "nekoliko", "sv": "något, lite, en smula", "sw": "un peu", "th": "จำนวนหนึ่ง", "tok": "mute", "tr": "bir miktar", "uk": "трохи, деяка кількість, якоюсь мірою", "ur": "to some extent, a certain amount", "vi": "một chút, số lượng cụ thể", "yo": "to some extent, a certain amount", "zh-tw": "有些", "zh": "在某种程度上, 一定的数量"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 38,
		},
		{
			slug: "vortaro-tabelvorto-cxiom", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉiom",
				"definition": "all of it, the whole amount",
				"definitions": map[string]interface{}{"en": "all of it, the whole amount", "nl": "de hele hoeveelheid", "de": "alles", "fr": "toutes les quantités", "es": "todo", "pt": "tudo disso", "ar": "كل الكمية", "be": "всё", "ca": "tot (quantitat absoluta)", "cs": "všechno", "da": "hele mængden", "el": "όλο, το παν", "fa": "همه‌ی مقدار, همه‌ی تعداد", "frp": "toutes les quantités", "ga": "é go léir, an méid uile", "he": "בכל כמות", "hi": "all of it, the whole amount", "hr": "sve, svekoliko", "hu": "mindannyi, mindahány", "id": "semua, keseluruhan jumlah", "it": "tutto, interamente", "ja": "すべて, ～の全部", "kk": "all of it, the whole amount", "km": "all of it, the whole amount", "ko": "전부다, 전체 분량의", "ku": "همه‌ی مقدار, همه‌ی تعداد", "lo": "all of it, the whole amount", "mg": "ny habetsany", "ms": "semua, jumlah keseluruhan", "my": "all of it, the whole amount", "pl": "całość, każda ilość", "ro": "toutes les quantités", "ru": "всё", "sk": "všetko", "sl": "vse", "sv": "alltsammans", "sw": "toutes les quantités", "th": "ทั้งหมด", "tok": "ale", "tr": "tümü", "uk": "весь (про кількість)", "ur": "all of it, the whole amount", "vi": "hết bấy nhiêu đó, cả một đống đó", "yo": "all of it, the whole amount", "zh-tw": "所有", "zh": "所有的数量, 每种数量"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 39,
		},
		{
			slug: "vortaro-tabelvorto-neniom", typ: "vocab",
			content: map[string]interface{}{
				"word": "neniom",
				"definition": "none of it, no amount",
				"definitions": map[string]interface{}{"en": "none of it, no amount", "nl": "niets", "de": "nichts", "fr": "aucune quantité", "es": "nada", "pt": "nada disso", "ar": "نفى الكمية", "be": "нисколько", "ca": "no gens, cap quantitat", "cs": "ani trochu", "da": "ingen mængde", "el": "καθόλου", "fa": "هیچ مقداری, هیچ تعدادی", "frp": "aucune quantité", "ga": "faic de", "he": "בשופ כמות", "hi": "none of it, no amount", "hr": "nikoliko, ništa", "hu": "semennyi, sehány", "id": "tak sedikitpun", "it": "niente, nessuna quantità", "ja": "少しも～ない（分量）, 少しも～ない（程度）", "kk": "none of it, no amount", "km": "none of it, no amount", "ko": "전혀 없는, 조금도 아닌", "ku": "هیچ مقداری, هیچ تعدادی", "lo": "none of it, no amount", "mg": "tsy misy habe", "ms": "tidak ada, tidak ada jumlah", "my": "none of it, no amount", "pl": "ani trochę, żadna ilość", "ro": "aucune quantité", "ru": "нисколько", "sk": "ani trochu", "sl": "nič", "sv": "ingenting alls", "sw": "aucune quantité", "th": "ไม่มีเลย", "tok": "mute ala", "tr": "hiç", "uk": "ніскільки", "ur": "none of it, no amount", "vi": "không bao nhiêu cả, không số lượng nào", "yo": "none of it, no amount", "zh-tw": "全無", "zh": "全无, 没有数量"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 40,
		},
		{
			slug: "vortaro-tabelvorto-kies", typ: "vocab",
			content: map[string]interface{}{
				"word": "kies",
				"definition": "whose",
				"definitions": map[string]interface{}{"en": "whose", "nl": "wiens", "de": "wessen", "fr": "de qui", "es": "de quién", "pt": "de quem", "ar": "السؤال عن الملكية, اسم موصول للملكية", "be": "чей", "ca": "de qui", "cs": "čí", "da": "hvis", "el": "τίνος, του οποίου-ας-ου-ων", "fa": "مال چه کسی, مال آن کس که", "frp": "de qui", "ga": "cé leis", "he": "של מי", "hi": "whose", "hr": "čiji, čije", "hu": "kié", "id": "milik siapa", "it": "di chi", "ja": "だれの", "kk": "whose", "km": "whose", "ko": "누구의", "ku": "مال چه کسی, مال آن کس که", "lo": "whose", "mg": "avy amin'iza", "ms": "siapa punya", "my": "whose", "pl": "czyj", "ro": "de qui", "ru": "чей", "sk": "čí", "sl": "čigav", "sv": "vilkens, vems", "sw": "de qui", "th": "ของใคร/ซึ่งเป็นของ", "tok": "pi ijo seme", "tr": "kimin", "uk": "чий", "ur": "whose", "vi": "của ai vậy", "yo": "whose", "zh-tw": "誰的", "zh": "是谁的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 41,
		},
		{
			slug: "vortaro-tabelvorto-ties", typ: "vocab",
			content: map[string]interface{}{
				"word": "ties",
				"definition": "that one's",
				"definitions": map[string]interface{}{"en": "that one's", "nl": "diens", "de": "dessen", "fr": "de celui-ci", "es": "de ese", "pt": "dessa pessoa", "ar": "ذلك الذى صاحبه, تلك التى صاحبها", "be": "того", "ca": "d'aquell", "cs": "toho, těch", "da": "den ders", "el": "αυτού-ης-ου-ων, εκείνου-ης-ου-ων", "fa": "مال آن کس", "frp": "de celui-ci", "ga": "leis an duine sin", "he": "שלו", "hi": "that one's", "hr": "toga, onoga", "hu": "azé", "id": "kepunyaannya", "it": "di lui/lei", "ja": "その人の", "kk": "that one's", "km": "that one's", "ko": "그 사람의", "ku": "مال آن کس", "lo": "that one's", "mg": "avy amin'ireto", "ms": "orang lain punya", "my": "that one's", "pl": "tego", "ro": "de celui-ci", "ru": "того", "sk": "toho", "sl": "od tega", "sv": "dess, deras", "sw": "de celui-ci", "th": "ของคนนั้น, ของสิ่งนั้น", "tok": "pi ijo ni", "tr": "bunun", "uk": "того", "ur": "that one's", "vi": "của người đó", "yo": "that one's", "zh-tw": "那人的", "zh": "那人的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 42,
		},
		{
			slug: "vortaro-tabelvorto-ies", typ: "vocab",
			content: map[string]interface{}{
				"word": "ies",
				"definition": "someone's",
				"definitions": map[string]interface{}{"en": "someone's", "nl": "iemands", "de": "irgend jemandes", "fr": "à quelqu'un", "es": "de alguien", "pt": "de alguém", "ar": "صاحب ما", "be": "чей-то", "ca": "d'algú", "cs": "něčí, čísi", "da": "nogens", "el": "κάποιου-ας-ου, κάποιων", "fa": "مال یک کسی", "frp": "à quelqu'un", "ga": "le duine éigin", "he": "של מישהו", "hi": "someone's", "hr": "ičiji, nečiji", "hu": "valakié", "id": "kepunyaan seseorang", "it": "di qualcuno", "ja": "だれかの", "kk": "someone's", "km": "someone's", "ko": "(불특정한) 누군가의", "ku": "مال یک کسی", "lo": "someone's", "mg": "an'olona", "ms": "seseorang punya", "my": "someone's", "pl": "czyjś", "ro": "à quelqu'un", "ru": "чей-то", "sk": "niečí, čísi", "sl": "od nekoga", "sv": "någons", "sw": "à quelqu'un", "th": "ของบางคน, ของบางสิ่ง", "tok": "(tan) ijo", "tr": "birisinin's", "uk": "чийсь", "ur": "someone's", "vi": "của ai đó", "yo": "someone's", "zh-tw": "某人的", "zh": "某人的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 43,
		},
		{
			slug: "vortaro-tabelvorto-cxies", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉies",
				"definition": "everyone's",
				"definitions": map[string]interface{}{"en": "everyone's", "nl": "ieders", "de": "jedermanns", "fr": "à tout le monde", "es": "de todos", "pt": "de todos", "ar": "كل ما يملكه", "be": "общий", "ca": "de tothom", "cs": "všech, každého", "da": "alles", "el": "καθενός-μιας-ενός", "fa": "مال همه کس", "frp": "à tout le monde", "ga": "le cách", "he": "של כולם", "hi": "everyone's", "hr": "svačiji", "hu": "mindenkié", "id": "milik semua orang", "it": "di tutti", "ja": "みんなの", "kk": "everyone's", "km": "everyone's", "ko": "모든 사람의", "ku": "مال همه کس", "lo": "everyone's", "mg": "an'olona rehetra", "ms": "semua orang punya", "my": "everyone's", "pl": "każdego", "ro": "à tout le monde", "ru": "общий", "sk": "všetkých, každého", "sl": "od vseh", "sv": "allas", "sw": "à tout le monde", "th": "ของทุกคน", "tok": "pi ijo ale", "tr": "herkesin", "uk": "усіх", "ur": "everyone's", "vi": "của mọi người", "yo": "everyone's", "zh-tw": "每人的", "zh": "每人的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 44,
		},
		{
			slug: "vortaro-tabelvorto-nenies", typ: "vocab",
			content: map[string]interface{}{
				"word": "nenies",
				"definition": "no-one's",
				"definitions": map[string]interface{}{"en": "no-one's", "nl": "niemands", "de": "niemandes", "fr": "à personne", "es": "de nadie, de ninguno", "pt": "de ninguém", "ar": "نفى الملكية", "be": "ничей", "ca": "de ningú", "cs": "ničí, nikoho", "da": "ingens", "el": "κανενός-καμιάς-κανενός", "fa": "مال هیچ کس", "frp": "à personne", "ga": "le duine ar bith", "he": "של אף אחד", "hi": "no-one's", "hr": "ničiji", "hu": "senkié", "id": "bukan milik siapapun", "it": "di nessuno", "ja": "だれの～でもない", "kk": "no-one's", "km": "no-one's", "ko": "아무의 소유도 아닌", "ku": "مال هیچ کس", "lo": "no-one's", "mg": "tsy an'olona", "ms": "tidak ada orang punya", "my": "no-one's", "pl": "niczyj", "ro": "à personne", "ru": "ничей", "sk": "ničí, nikoho", "sl": "od nikogar", "sv": "ingens", "sw": "à personne", "th": "ไม่มีเจ้าของ", "tok": "pi ijo ala", "tr": "kimsenin", "uk": "нічий", "ur": "no-one's", "vi": "không của ai cả", "yo": "no-one's", "zh-tw": "無人的", "zh": "无人的"},
			},
			tags:        []string{"vortaro", "tabelvorto"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "vortaro-tabelvorto",
			seriesOrder: 45,
		},
		{
			slug: "vortaro-tagoj-lundo", typ: "vocab",
			content: map[string]interface{}{
				"word": "lundo",
				"definition": "Monday",
				"definitions": map[string]interface{}{"en": "Monday", "nl": "maandag", "de": "Montag", "fr": "lundi", "es": "lunes", "pt": "Segunda-feira", "ar": "الإثنين", "be": "понедельник", "ca": "dilluns", "cs": "pondělí", "da": "mandag", "el": "Δευτέρα", "fa": "دوشنبه", "frp": "dìlun", "ga": "Luan", "he": "יום שני", "hi": "Monday", "hr": "ponedjeljak", "hu": "hétfő", "id": "Senin", "it": "lunedì", "ja": "月曜日", "kk": "дүйсенбі", "km": "ថ្ងៃចន្ទ", "ko": "월요일", "ku": "دوشنبه", "lo": "Monday", "mg": "alatsinainy", "ms": "Isnin", "my": "ថ្ងៃចន្ទ", "pl": "poniedziałek", "ro": "lundi", "ru": "понедельник", "sk": "pondelok", "sl": "ponedjeljek", "sv": "måndag", "sw": "lundi", "th": "วันจันทร์", "tok": "tenpo esun #1", "tr": "Pazartesi", "uk": "понеділок", "ur": "پیر", "vi": "thứ hai", "yo": "Monday", "zh-tw": "星期一", "zh": "星期一"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-tagoj-mardo", typ: "vocab",
			content: map[string]interface{}{
				"word": "mardo",
				"definition": "Tuesday",
				"definitions": map[string]interface{}{"en": "Tuesday", "nl": "dinsdag", "de": "Dienstag", "fr": "mardi", "es": "martes", "pt": "Terça-feira", "ar": "الثلاثاء", "be": "вторник", "ca": "dimarts", "cs": "úterý", "da": "tirsdag", "el": "Τρίτη", "fa": "سه‌شنبه", "frp": "dìmâr", "ga": "Máirt", "he": "יום שלישי", "hi": "Tuesday", "hr": "utorak", "hu": "kedd", "id": "Selasa", "it": "martedì", "ja": "火曜日", "kk": "сейсенбі", "km": "កាលពីថ្ងៃអង្គារ", "ko": "화요일", "ku": "سه‌شنبه", "lo": "Tuesday", "mg": "talata", "ms": "Selasa", "my": "កាលពីថ្ងៃអង្គារ", "pl": "wtorek", "ro": "mardi", "ru": "вторник", "sk": "utorok", "sl": "torek", "sv": "tisdag", "sw": "mardi", "th": "วันอังคาร", "tok": "tenpo esun #2", "tr": "Salı", "uk": "вівторок", "ur": "منگل", "vi": "thứ ba", "yo": "Tuesday", "zh-tw": "星期二", "zh": "星期二"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-tagoj-merkredo", typ: "vocab",
			content: map[string]interface{}{
				"word": "merkredo",
				"definition": "Wednesday",
				"definitions": map[string]interface{}{"en": "Wednesday", "nl": "woensdag", "de": "Mittwoch", "fr": "mercredi", "es": "miércoles", "pt": "Quarta-feira", "ar": "الأربعاء", "be": "среда", "ca": "dimecres", "cs": "středa", "da": "onsdag", "el": "Τετάρτη", "fa": "چهارشنبه", "frp": "dìmècro", "ga": "Céadaoin", "he": "יום רביעי", "hi": "Wednesday", "hr": "srijeda", "hu": "szerda", "id": "Rabu", "it": "mercoledì", "ja": "水曜日", "kk": "сәрсенбі", "km": "ថ្ងៃពុធ", "ko": "수요일", "ku": "چهارشنبه", "lo": "Wednesday", "mg": "alarobia", "ms": "Rabu", "my": "ថ្ងៃពុធ", "pl": "środa", "ro": "mercredi", "ru": "среда", "sk": "streda", "sl": "sreda", "sv": "onsdag", "sw": "mercredi", "th": "วันพุธ", "tok": "tenpo esun #3", "tr": "Çarşamba", "uk": "середа", "ur": "بدھ", "vi": "thứ tư", "yo": "Wednesday", "zh-tw": "星期三", "zh": "星期三"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-tagoj-jxauxdo", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĵaŭdo",
				"definition": "Thursday",
				"definitions": map[string]interface{}{"en": "Thursday", "nl": "donderdag", "de": "Donnerstag", "fr": "jeudi", "es": "jueves", "pt": "Quinta-feira", "ar": "الخميس", "be": "четверг", "ca": "dijous", "cs": "čtvrtek", "da": "torsdag", "el": "Πέμπτη", "fa": "پنج‌شنبه", "frp": "dìjou", "ga": "Déardaoin", "he": "יום חמישי", "hi": "Thursday", "hr": "četvrtak", "hu": "csütörtök", "id": "Kamis", "it": "giovedì", "ja": "木曜日", "kk": "бейсенбі", "km": "ថ្ងៃព្រហស្បតិ៍", "ko": "목요일", "ku": "پنج‌شنبه", "lo": "Thursday", "mg": "alakamisy", "ms": "Khamis", "my": "ថ្ងៃព្រហស្បតិ៍", "pl": "czwartek", "ro": "jeudi", "ru": "четверг", "sk": "štvrtok", "sl": "četrtek", "sv": "torsdag", "sw": "jeudi", "th": "วันพฤหัสบดี", "tok": "tenpo esun #4", "tr": "Perşembe", "uk": "четвер", "ur": "جمعرات", "vi": "thứ năm", "yo": "Thursday", "zh-tw": "星期四", "zh": "星期四"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-tagoj-vendredo", typ: "vocab",
			content: map[string]interface{}{
				"word": "vendredo",
				"definition": "Friday",
				"definitions": map[string]interface{}{"en": "Friday", "nl": "vrijdag", "de": "Freitag", "fr": "vendredi", "es": "viernes", "pt": "Sexta-feira", "ar": "الجمعة", "be": "пятница", "ca": "divendres", "cs": "pátek", "da": "fredag", "el": "Παρασκευή", "fa": "جمعه, آدینه", "frp": "dìvendro", "ga": "Aoine", "he": "יום שישי", "hi": "Friday", "hr": "petak", "hu": "péntek", "id": "Jumat", "it": "venerdì", "ja": "金曜日", "kk": "жұма", "km": "ថ្ងៃសុក្រ", "ko": "금요일", "ku": "جمعه, آدینه", "lo": "Friday", "mg": "zoma", "ms": "Jumaat", "my": "កាលពីថ្ងៃសុក្រ", "pl": "piątek", "ro": "vendredi", "ru": "пятница", "sk": "piatok", "sl": "petek", "sv": "fredag", "sw": "vendredi", "th": "วันศุกร์", "tok": "tenpo esun #5", "tr": "Cuma", "uk": "п'ятниця", "ur": "جمعہ", "vi": "thứ sáu", "yo": "Friday", "zh-tw": "星期五", "zh": "星期五"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-tagoj-sabato", typ: "vocab",
			content: map[string]interface{}{
				"word": "sabato",
				"definition": "Saturday",
				"definitions": map[string]interface{}{"en": "Saturday", "nl": "zaterdag", "de": "Samstag", "fr": "samedi", "es": "sábado", "pt": "Sábado", "ar": "السبت", "be": "суббота", "ca": "dissabte", "cs": "sobota", "da": "lørdag", "el": "Σάββατο", "fa": "شنبه", "frp": "dìssando", "ga": "Satharn", "he": "יום שבת", "hi": "Saturday", "hr": "subota", "hu": "szombat", "id": "Sabtu", "it": "sabato", "ja": "土曜日", "kk": "сенбі", "km": "ថ្ងៃសៅរ៍", "ko": "토요일", "ku": "شنبه", "lo": "Saturday", "mg": "sabotsy", "ms": "Sabtu", "my": "ថ្ងៃសៅរ៍", "pl": "sobota", "ro": "samedi", "ru": "суббота", "sk": "sobota", "sl": "sobota", "sv": "lördag", "sw": "samedi", "th": "วันเสาร์", "tok": "tenpo suno lape #1", "tr": "Cumartesi", "uk": "субота", "ur": "ہفتہ", "vi": "thứ bảy", "yo": "Saturday", "zh-tw": "星期六", "zh": "星期六"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-tagoj-dimancxo", typ: "vocab",
			content: map[string]interface{}{
				"word": "dimanĉo",
				"definition": "Sunday",
				"definitions": map[string]interface{}{"en": "Sunday", "nl": "zondag", "de": "Sonntag", "fr": "dimanche", "es": "domingo", "pt": "Domingo", "ar": "الأحد", "be": "воскресенье", "ca": "diumenge", "cs": "neděle", "da": "søndag", "el": "Κυριακή", "fa": "یک‌شنبه", "frp": "dìmenge", "ga": "Domhnach", "he": "יום ראשון", "hi": "Sunday", "hr": "nedjelja", "hu": "vasárnap", "id": "Minggu", "it": "domenica", "ja": "日曜日", "kk": "жексенбі", "km": "ថ្ងៃអាទិត្យ", "ko": "일요일", "ku": "یک‌شنبه", "lo": "Sunday", "mg": "alahady", "ms": "Ahad", "my": "ថ្ងៃអាទិត្យ", "pl": "niedziela", "ro": "dimanche", "ru": "воскресенье", "sk": "nedeľa", "sl": "nedelja", "sv": "söndag", "sw": "dimanche", "th": "วันอาทิตย์", "tok": "tenpo suno lape #2", "tr": "Pazar", "uk": "неділя", "ur": "اتوار", "vi": "chủ nhật", "yo": "Sunday", "zh-tw": "星期日", "zh": "星期日"},
			},
			tags:        []string{"vortaro", "tago_en_la_semajno"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-tagoj",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-sezono-printempo", typ: "vocab",
			content: map[string]interface{}{
				"word": "printempo",
				"definition": "spring",
				"definitions": map[string]interface{}{"en": "spring", "nl": "lente", "de": "Frühling", "fr": "printemps", "es": "primavera", "pt": "primavera", "ar": "ربيع", "be": "весна", "ca": "primavera", "cs": "jaro", "da": "forår", "el": "άνοιξη", "fa": "بهار", "frp": "printemps", "ga": "earrach", "he": "אביב", "hi": "spring", "hr": "proljeće", "hu": "tavasz", "id": "musim semi", "it": "primavera", "ja": "春", "kk": "көктем", "km": "និទាឃរដូវ។", "ko": "봄", "ku": "بهار", "lo": "spring", "mg": "lohataona", "ms": "musim bunga", "my": "និទាឃរដូវ", "pl": "wiosna", "ro": "printemps", "ru": "весна", "sk": "jar", "sl": "pomlad", "sv": "vår", "sw": "printemps", "th": "ฤดูใบไม้ผลิ", "tok": "tenpo pi seli lili", "tr": "ilkbahar", "uk": "весна", "ur": "spring", "vi": "mùa xuân", "yo": "spring", "zh-tw": "春", "zh": "春"},
			},
			tags:        []string{"vortaro", "sezono"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-sezono",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-sezono-somero", typ: "vocab",
			content: map[string]interface{}{
				"word": "somero",
				"definition": "summer",
				"definitions": map[string]interface{}{"en": "summer", "nl": "zomer", "de": "Sommer", "fr": "été", "es": "verano", "pt": "verão", "ar": "صيف", "be": "лето", "ca": "estiu", "cs": "léto", "da": "sommer", "el": "καλοκαίρι", "fa": "تابستان", "frp": "chôd-temps", "ga": "samhradh", "he": "קיץ", "hi": "summer", "hr": "ljeto", "hu": "nyár", "id": "musim panas", "it": "estate", "ja": "夏", "kk": "жаз", "km": "នៅរដូវក្តៅ", "ko": "여름", "ku": "تابستان", "lo": "summer", "mg": "fahavaratra", "ms": "musim panas", "my": "រដូវក្តៅ", "pl": "lato", "ro": "été", "ru": "лето", "sk": "leto", "sl": "poletje", "sv": "sommar", "sw": "été", "th": "ฤดูร้อน", "tok": "tenpo seli", "tr": "yaz", "uk": "літо", "ur": "summer", "vi": "mùa hạ, mùa hè", "yo": "summer", "zh-tw": "夏", "zh": "夏"},
			},
			tags:        []string{"vortaro", "sezono"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-sezono",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-sezono-auxtuno", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭtuno",
				"definition": "autumn",
				"definitions": map[string]interface{}{"en": "autumn", "nl": "herfst", "de": "Herbst", "fr": "automne", "es": "otoño", "pt": "outono", "ar": "خريف", "be": "осень", "ca": "tardor", "cs": "podzim", "da": "efterår", "el": "φθινόπωρο", "fa": "پاییز", "frp": "ôtono", "ga": "fómhar", "he": "סתיו", "hi": "autumn", "hr": "jesen", "hu": "ősz", "id": "musim gugur", "it": "autunno", "ja": "秋", "kk": "күз", "km": "រដូវស្លឹកឈើជ្រុះ", "ko": "가을", "ku": "پاییز", "lo": "autumn", "mg": "fararano", "ms": "musim luruh", "my": "រដូវស្លឹកឈើជ្រុះ", "pl": "jesień", "ro": "automne", "ru": "осень", "sk": "jeseň", "sl": "jesen", "sv": "höst", "sw": "automne", "th": "ฤดูใบไม้ร่วง", "tok": "tenpo pi lete lili", "tr": "sonbahar", "uk": "осінь", "ur": "autumn", "vi": "mùa thu", "yo": "autumn", "zh-tw": "秋", "zh": "秋"},
			},
			tags:        []string{"vortaro", "sezono"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-sezono",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-sezono-vintro", typ: "vocab",
			content: map[string]interface{}{
				"word": "vintro",
				"definition": "winter",
				"definitions": map[string]interface{}{"en": "winter", "nl": "winter", "de": "Winter", "fr": "hiver", "es": "invierno", "pt": "inverno", "ar": "شتاء", "be": "зима", "ca": "hivern", "cs": "zima", "da": "vinter", "el": "χειμώνας", "fa": "زمستان", "frp": "hivèrn", "ga": "geimhreadh", "he": "חורף", "hi": "winter", "hr": "zima", "hu": "tél", "id": "musim dingin", "it": "inverno", "ja": "冬", "kk": "қыс", "km": "រដូវរងារ", "ko": "겨울", "ku": "زمستان", "lo": "winter", "mg": "ririnina", "ms": "musim salji", "my": "រដូវរងារ", "pl": "zima", "ro": "hiver", "ru": "зима", "sk": "zima", "sl": "zima", "sv": "vinter", "sw": "hiver", "th": "ฤดูหนาว", "tok": "tenpo lete", "tr": "kış", "uk": "зима", "ur": "winter", "vi": "mùa đông", "yo": "winter", "zh-tw": "冬", "zh": "冬"},
			},
			tags:        []string{"vortaro", "sezono"},
			source:      "La Zagreba Metodo",
			rating: 850, rd: 200,
			seriesSlug:  "vortaro-sezono",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-grava-cxu-ne", typ: "vocab",
			content: map[string]interface{}{
				"word": "ĉu ne",
				"definition": "isn't it",
				"definitions": map[string]interface{}{"en": "isn't it", "nl": "nietwaar?, niet?, toch?", "de": "nicht wahr", "fr": "n'est-ce pas", "es": "¿no?", "pt": "não é?", "ar": "أليس كذلك", "be": "не так ли", "ca": "no?", "cs": "ne?", "da": "ikke, er det ikke", "el": "μήπως όχι;, έτσι δεν είναι;", "fa": "مگر نه", "frp": "n'est-ce pas", "ga": "nach ea", "he": "האם לא", "hi": "isn't it", "hr": "zar ne", "hu": "ugye", "id": "ya,kan?", "it": "no?", "ja": "～ね？", "kk": "isn't it", "km": "isn't it", "ko": "그렇죠? 아닌가요?", "ku": "مگر نه", "lo": "isn't it", "mg": "tsy izany", "ms": "bukankah itu", "my": "isn't it", "ro": "n'est-ce pas", "ru": "не так ли", "sk": "nie?", "sl": "ali ne", "sv": "eller hur", "sw": "n'est-ce pas", "th": "ใช่ไหม", "tok": "isn't it", "tr": "değil mi", "uk": "чи не так", "ur": "isn't it", "vi": "đúng không", "yo": "isn't it", "zh-tw": "不是嗎", "zh": "不是吗"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-grava-iom-post-iom", typ: "vocab",
			content: map[string]interface{}{
				"word": "iom post iom",
				"definition": "little by little",
				"definitions": map[string]interface{}{"en": "little by little", "nl": "beetje per beetje", "de": "nach und nach", "fr": "petit à petit", "es": "poco a poco", "pt": "pouco a pouco", "ar": "شيئأً فشيئاً", "be": "шаг за шагом", "ca": "de mica en mica", "cs": "postupně", "da": "lidt efter lidt", "el": "λίγο - λίγο", "fa": "کم‌کم", "frp": "petit à petit", "ga": "de réir a chéile", "he": "לאט לאט", "hi": "little by little", "hr": "malo po malo", "hu": "apránként", "id": "sedikit demi sedikit", "it": "un po' alla volta", "ja": "少しずつ", "kk": "little by little", "km": "little by little", "ko": "조금씩 조금씩", "ku": "کم‌کم", "lo": "little by little", "mg": "tsikelikely", "ms": "sedikit demi sedikit", "my": "little by little", "pl": "stopniowo, krok po kroku", "ro": "petit à petit", "ru": "шаг за шагом", "sk": "postupne", "sl": "malo po malo", "sv": "lite i taget", "sw": "petit à petit", "th": "ทีละนิด", "tok": "little by little", "tr": "azar azar", "uk": "поступово", "ur": "تھوڑا تھوڑا", "vi": "từng li từng tí", "yo": "little by little", "zh-tw": "漸漸, 一點一點地", "zh": "一点一点地，一些一些"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-grava-pli-malpli", typ: "vocab",
			content: map[string]interface{}{
				"word": "pli-malpli",
				"definition": "more or less",
				"definitions": map[string]interface{}{"en": "more or less", "nl": "min of meer", "de": "mehr oder weniger", "fr": "plus ou moins", "es": "más o menos", "pt": "mais ou menos", "ar": "أكثر أو أقل", "be": "более-менее", "ca": "més o menys", "cs": "víceméně", "da": "mere eller mindre", "el": "πάνω - κάτω", "fa": "کم‌وبیش", "frp": "plus ou moins", "ga": "a bheag nó a mhór", "he": "יותר או פחות", "hi": "more or less", "hr": "više-manje", "hu": "többé-kevésbé", "id": "kurang lebih", "it": "più o meno", "ja": "多かれ少なかれ", "kk": "азды көпті", "km": "more or less", "ko": "대략", "ku": "کم‌وبیش", "lo": "more or less", "mg": "be kokoa na kely kokoa", "ms": "lebih atau kurang", "my": "more or less", "pl": "mniej więcej", "ro": "plus ou moins", "ru": "более-менее", "sk": "viac-menej", "sl": "več-manj", "sv": "mer eller mindre", "sw": "plus ou moins", "th": "ไม่มากไม่น้อย", "tok": "more or less", "tr": "üç aşağı beş yukarı", "uk": "більш-менш", "ur": "کم یا زیادہ", "vi": "khoảng chừng", "yo": "more or less", "zh-tw": "多多少少", "zh": "多或少"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-grava-mem-kompreneble", typ: "vocab",
			content: map[string]interface{}{
				"word": "mem kompreneble",
				"definition": "obviously",
				"definitions": map[string]interface{}{"en": "obviously", "nl": "natuurlijk, vanzelfsprekend", "de": "selbstverständlich", "fr": "évidemment", "pt": "obviamente", "ar": "واضح في ذاته", "be": "само собой разумеющееся", "cs": "samozřejmě", "da": "selvfølgelig", "fa": "مسلما", "frp": "évidemment", "ga": "dar ndóigh", "he": "מובן מאליו", "hi": "obviously", "hr": "samo po sebi razumljivo, očito", "hu": "természetesen", "id": "tentu saja", "it": "ovviamente", "ja": "明らかに", "kk": "өзінен-өзі түсінікті", "km": "obviously", "ko": "자명하게 (스스로 이해될 만하게)", "ku": "مسلما", "lo": "obviously", "mg": "mazava ho azy", "ms": "sudah jelas", "my": "obviously", "pl": "oczywiście", "ro": "évidemment", "ru": "само собой разумеющееся", "sk": "samozrejme", "sl": "samo po sebi razumljivo, očitno", "sv": "självklart", "sw": "évidemment", "th": "เข้าใจได้ด้วยตนเอง", "tok": "obviously", "tr": "belli ki, açık olarak", "uk": "само собою зрозуміло", "ur": "بالکل ایسا ہی ہے", "vi": "dĩ nhiên", "yo": "obviously", "zh-tw": "明顯地", "zh": "明显地"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-grava-temas-pri", typ: "vocab",
			content: map[string]interface{}{
				"word": "temas pri",
				"definition": "it is about",
				"definitions": map[string]interface{}{"en": "it is about", "nl": "het gaat over, gaat over", "de": "es handelt sich um", "fr": "c'est au sujet de", "es": "trata de/sobre", "pt": "é sobre, tem a ver com", "ar": "يدور حول", "be": "это о", "ca": "es tracta de, té a veure amb", "cs": "je to o", "da": "handler om", "el": "αφορά το, αφορά τι", "fa": "هست درباره‌ی", "frp": "c'est au sujet de", "ga": "baineann sé le", "he": "זה עוסק ב, הנושא הוא", "hi": "it is about", "hr": "radi se o", "hu": "arról van szó", "id": "adalah tentang", "it": "it is about", "ja": "～について", "kk": "it is about", "km": "it is about", "ko": "~에 관한 것이다", "ku": "هست درباره‌ی", "lo": "it is about", "mg": "momba ny", "ms": "ia mengenali sebagai", "my": "it is about", "pl": "chodzi o", "ro": "c'est au sujet de", "ru": "это о", "sk": "ide o", "sl": "gre za", "sv": "(det) handlar om", "sw": "c'est au sujet de", "th": "เป็นเรื่องเกี่ยวกับ", "tok": "it is about", "tr": "hakkındadır", "uk": "мова про", "ur": "it is about", "vi": "về việc", "yo": "it is about", "zh-tw": "關於", "zh": "关于"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-grava-iomete", typ: "vocab",
			content: map[string]interface{}{
				"word": "iomete",
				"definition": "a little",
				"definitions": map[string]interface{}{"en": "a little", "nl": "een beetje, een ietsje", "de": "etwas, ein bißchen", "fr": "un peu", "es": "un poquito", "pt": "um pouco", "ar": "قليلاً", "be": "чуть-чуть", "ca": "una miqueta", "cs": "trochu", "da": "lidt, en smule", "el": "λιγάκι", "fa": "اندکی", "frp": "un peu", "ga": "beagán, beagáinín", "he": "קצת", "hi": "a little", "hr": "malo", "hu": "egy kicsit", "id": "sedikit", "it": "un po'", "ja": "ほんの少し", "kk": "аз; көп емес", "km": "a little", "ko": "약간, 조금", "ku": "اندکی", "lo": "a little", "mg": "kely", "ms": "sedikit", "my": "a little", "pl": "trochę, troszkę", "ro": "un peu", "ru": "чуть-чуть", "sk": "troška", "sl": "malo", "sv": "lite grann", "sw": "un peu", "th": "เล็กน้อย, นิดหน่อย", "tok": "a little", "tr": "azıcık", "uk": "трошки", "ur": "تھوڑا سا", "vi": "một chút", "yo": "a little", "zh-tw": "一點點, 一些些", "zh": "一点, 一些"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-grava-rilate-al", typ: "vocab",
			content: map[string]interface{}{
				"word": "rilate al",
				"definition": "relating to",
				"definitions": map[string]interface{}{"en": "relating to", "nl": "betreffende, in betrekking met", "de": "betreffend", "fr": "en ce qui concerne", "es": "en relación a", "pt": "em relação a", "ar": "المتعلقة ب", "be": "отноcящееся к", "ca": "en relació a", "cs": "ve vztahu k", "da": "i forbindelse med", "el": "σχετικά με", "fa": "در ارتباط با", "frp": "en ce qui concerne", "ga": "maidir le", "he": "ביחס ל", "hi": "relating to", "hr": "u odnosu na", "hu": "valamivel kapcsolatban", "id": "berkaitan dengan", "it": "riguardo a", "ja": "～に関して", "kk": "relating to", "km": "relating to", "ko": "~와 관계하여", "ku": "در ارتباط با", "lo": "relating to", "mg": "ny momba ny", "ms": "mengenal", "my": "relating to", "pl": "w stosunku do", "ro": "en ce qui concerne", "ru": "отноcящееся к", "sk": "vo vzťahu k", "sl": "glede na", "sv": "i förhållande till", "sw": "en ce qui concerne", "th": "มีความสัมพันธ์กับ", "tok": "relating to", "tr": "ile ilgili olarak", "uk": "стосовно до", "ur": "سے متعلق", "vi": "liên quan đến việc", "yo": "relating to", "zh-tw": "關於", "zh": "关系到"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-grava-ni-diru", typ: "vocab",
			content: map[string]interface{}{
				"word": "ni diru",
				"definition": "let's say",
				"definitions": map[string]interface{}{"en": "let's say", "nl": "laat ons zeggen", "de": "sagen wir", "fr": "disons", "es": "digamos", "pt": "digamos", "ar": "نقول", "be": "давай скажем", "ca": "diguem", "cs": "řekněme", "da": "lad os sige", "el": "ας πούμε", "fa": "باید بگوییم, بگذارید بگوییم", "frp": "disons", "ga": "abraimis", "he": "בוא נגיד", "hi": "let's say", "hr": "recimo", "hu": "mondjuk", "id": "kita katakan, anggap", "it": "diciamo", "ja": "話しましょう", "kk": "let's say", "km": "let's say", "ko": "어디보자", "ku": "باید بگوییم, بگذارید بگوییم", "lo": "let's say", "mg": "dia ataovy hoe", "ms": "kita berkata", "my": "let's say", "pl": "powiedzmy", "ro": "disons", "ru": "давай скажем", "sk": "povedzme", "sl": "recimo", "sv": "låt (oss) säga", "sw": "disons", "th": "จงพูดว่า", "tok": "let's say", "tr": "diyelim ki, varsayalım", "uk": "скажімо", "ur": "چلو ہم کہتے ہیں", "vi": "giả sử", "yo": "let's say", "zh-tw": "讓我們這樣說", "zh": "我们说"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-grava-versxajne", typ: "vocab",
			content: map[string]interface{}{
				"word": "verŝajne",
				"definition": "probably",
				"definitions": map[string]interface{}{"en": "probably", "nl": "waarschijnlijk", "de": "wahrscheinlich", "fr": "probablement", "es": "al parecer", "pt": "provavelmente", "ar": "من المحتمل", "be": "вероятно", "ca": "sembla ser, segurament", "cs": "pravděpodobně", "da": "sandsynligvis", "el": "πιθανώς", "fa": "انگار, همانا", "frp": "probablement", "ga": "is dócha", "he": "כנראה", "hi": "probably", "hr": "vjerojatno", "hu": "valószínűleg", "id": "tampaknya", "it": "probabilmente", "ja": "おそらく", "kk": "бәлкім; мүмкін; ықтимал", "km": "probably", "ko": "분명히", "ku": "انگار, همانا", "lo": "probably", "mg": "angamba", "ms": "kemungkinan", "my": "probably", "pl": "prawdopodobnie", "ro": "probablement", "ru": "вероятно", "sk": "pravdepodobne", "sl": "verjetno", "sv": "troligen, förmodligen", "sw": "probablement", "th": "ดูเหมือนจะ", "tok": "probably", "tr": "muhtemelen", "uk": "правдоподібно, можливо, імовірно", "ur": "غالبا", "vi": "hầu như chắc chắn rằng", "yo": "probably", "zh-tw": "似乎, 或許", "zh": "似乎、或许"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-grava-eble", typ: "vocab",
			content: map[string]interface{}{
				"word": "eble",
				"definition": "maybe, perhaps",
				"definitions": map[string]interface{}{"en": "maybe, perhaps", "nl": "mogelijk, misschien", "de": "möglich, vielleicht", "fr": "peut-être", "es": "quizá(s), tal vez, posiblemente", "pt": "talvez", "ar": "ربما, يمكن, لعل", "be": "возможно", "ca": "potser, és possible que, possiblement", "cs": "možná", "da": "måske, muligvis", "el": "θα μπορούσε, ίσως", "fa": "احتمالا, شاید", "frp": "peut-être", "ga": "b'fhéidir", "he": "אולי", "hi": "maybe, perhaps", "hr": "možda, moguće", "hu": "talán", "id": "mungkin", "it": "forse, può darsi", "ja": "多分, きっと", "kk": "maybe, мүмкін", "km": "maybe, perhaps", "ko": "아마도, 아마", "ku": "احتمالا, شاید", "lo": "maybe, perhaps", "mg": "mety", "ms": "mungkin", "my": "maybe, perhaps", "pl": "może, chyba", "ro": "peut-être", "ru": "возможно", "sk": "možno", "sl": "morda, mogoče", "sv": "kanske", "sw": "peut-être", "th": "บางที, อาจเป็นไปได้", "tok": "maybe, perhaps", "tr": "belki", "uk": "можливо", "ur": "شائد, شائد", "vi": "có lẽ", "yo": "maybe, perhaps", "zh-tw": "或許, 可能", "zh": "或许, 可能"},
			},
			tags:        []string{"vortaro", "grava_esprimo"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "vortaro-grava",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-sufikso-ad", typ: "vocab",
			content: map[string]interface{}{
				"word": "ad",
				"definition": "continuous action",
				"definitions": map[string]interface{}{"en": "continuous action", "nl": "duur (herhaling)", "de": "Dauer", "fr": "action continue", "es": "acción continuada", "pt": "ação contínua", "ar": "للإستمرار", "be": "продолжительное действие", "ca": "acció continuada", "cs": "trvání děje", "da": "fortsat handling", "el": "επίθημα -> (ουσ.)ενέργεια, (ρ.)συνεχόμενη πράξη", "fa": "عمل پیوسته, اسم مصدر", "frp": "action continue", "ga": "gníomhaíocht leanúnach", "he": "פעולה נמשכת", "hi": "continuous action", "hr": "trajanje", "hu": "ismétlődő cselekvés, -gat/get", "id": "perbuatan yang berulang-ulang", "it": "azione continuata o ripetuta", "ja": "行為の継続", "kk": "continuous action", "km": "continuous action", "ko": "반복적인 동작, 지속적인 동작", "ku": "عمل پیوسته, اسم مصدر", "lo": "continuous action", "mg": "hetsika mitohy", "ms": "aksi berdarut", "my": "continuous action", "pl": "ciągła czynność", "ro": "action continue", "ru": "продолжительное действие", "sk": "trvanie deja", "sl": "trajanje", "sv": "(upprepad, varaktig) handling", "sw": "action continue", "th": "การกระทำที่ต่อเนื่อง", "tok": "continuous action", "tr": "yinelenen aksiyon", "uk": "тривалість дії, процесу", "ur": "continuous action", "vi": "thực hiện nhiều lần", "yo": "continuous action", "zh-tw": "持續", "zh": "持续的动作"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-sufikso-an", typ: "vocab",
			content: map[string]interface{}{
				"word": "an",
				"definition": "member of a group",
				"definitions": map[string]interface{}{"en": "member of a group", "nl": "lid, aanhanger, inwoner", "de": "Mitglied, Anhänger", "fr": "membre d'un groupe", "es": "miembro, partidario", "pt": "membro de um grupo", "ar": "عضو, تابع", "be": "член, участник", "ca": "membre, partidari", "cs": "člen skupiny", "da": "medlem af en gruppe", "el": "επίθημα -> μέλος ομάδας", "fa": "عضو گروه", "frp": "membre d'un groupe", "ga": "ball de ghrúpa", "he": "חברות בקבוצה", "hi": "member of a group", "hr": "član grupe", "hu": "egy csoport tagja", "id": "anggota sebuah kelompok", "it": "membro di un gruppo", "ja": "集団の構成員", "kk": "member of a group", "km": "member of a group", "ko": "멤버, 소속 구성원", "ku": "عضو گروه", "lo": "member of a group", "mg": "mpikambana ao amin'ny vondrona iray", "ms": "ahli kumpulan", "my": "member of a group", "pl": "członek grupy", "ro": "membre d'un groupe", "ru": "член, участник", "sk": "člen skupiny", "sl": "član skupine", "sv": "medlem, anhängare, invånare", "sw": "membre d'un groupe", "th": "สมาชิก", "tok": "member of a group", "tr": "grup uyesi", "uk": "член якого-нубудь колективу; житель міста, країни;", "ur": "member of a group", "vi": "thành viên một nhóm", "yo": "member of a group", "zh-tw": "成員", "zh": "成员"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-sufikso-ar", typ: "vocab",
			content: map[string]interface{}{
				"word": "ar",
				"definition": "group, collection",
				"definitions": map[string]interface{}{"en": "group, collection", "nl": "verzameling", "de": "Sammelbegriff", "fr": "groupe, ensemble", "es": "conjunto", "pt": "grupo, coleção", "ar": "مجموعة", "be": "совокупность, набор", "ca": "conjunt", "cs": "vyšší celek", "da": "gruppe, samling, sæt", "el": "επίθημα -> ομάδα, συλλογή, άθροισμα", "fa": "گروه، مجموعه", "frp": "groupe, ensemble", "ga": "grúpa, bailiúchán", "he": "קבוצה, אוסף", "hi": "group, collection", "hr": "skupina, mnoštvo", "hu": "csoport, gyűjtemény", "id": "grup, sekumpulan", "it": "gruppo, aggregato", "ja": "グループ, リスト・カタログ", "kk": "group, collection", "km": "group, collection", "ko": "그룹, 집합", "ku": "گروه، مجموعه", "lo": "group, collection", "mg": "vondrona, miaraka", "ms": "grup atau koleksi", "my": "group, collection", "pl": "grupa, zbiór", "ro": "groupe, ensemble", "ru": "совокупность, набор", "sk": "množina alebo zbierka vecí rovnakého druhu", "sl": "skupina, množica", "sv": "samling", "sw": "groupe, ensemble", "th": "กลุ่มของสิ่งที่เหมือนกัน", "tok": "group, collection", "tr": "grup, bir araya gelen şeylerin bütünü", "uk": "groupe сукупність", "ur": "group, collection", "vi": "một nhóm, tập hợp", "yo": "group, collection", "zh-tw": "群, 組", "zh": "组，团"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-sufikso-acx", typ: "vocab",
			content: map[string]interface{}{
				"word": "aĉ",
				"definition": "indicates undesirable quality",
				"definitions": map[string]interface{}{"en": "indicates undesirable quality", "nl": "uiterlijk afkerig", "de": "äußere Verschlechterung", "fr": "indique un aspect négatif", "es": "baja calidad", "pt": "indica qualidade ruim", "ar": "للدلالة على القبح", "be": "обозначает низкое качество, непригодность, никчёмность", "ca": "lleig", "cs": "zhoršená kvalita", "da": "ubehagelig, dårlig, ringe", "el": "επίθημα -> παλιό-, κακό-", "fa": "نشان دادن کیفیت نامطلوب یا پست شمردن", "frp": "indique un aspect négatif", "ga": "díspeagúil", "he": "מגעיל, דוחה", "hi": "indicates undesirable quality", "hr": "gadno, nepoželjno svojstvo", "hu": "alacsony minőségű, silány", "id": "menyatakan kualitas yang tidak diinginkan", "it": "dispregiativo", "ja": "劣悪さ・下品さのニュアンスを加える", "kk": "indicates undesirable quality", "km": "indicates undesirable quality", "ko": "바람직하지 않은 상태, 나쁜, 조악한 상태", "ku": "نشان دادن کیفیت نامطلوب یا پست شمردن", "lo": "indicates undesirable quality", "mg": "manondro lafiny ratsy", "ms": "menujukkan yang tidak bagus", "my": "indicates undesirable quality", "pl": "wskazuje na niepożądaną jakość", "ro": "indique un aspect négatif", "ru": "обозначает низкое качество, непригодность, никчёмность", "sk": "zlá kvalita, nevhodnosť, bezcennosť", "sl": "ogabnost, grdoba", "sv": "usel, dålig kvalitet", "sw": "indique un aspect négatif", "th": "คุณภาพแย่, ไม่ดี", "tok": "indicates undesirable quality", "tr": "istenmeyen kalite gösterir", "uk": "низька якість, нікчемність, непридатність, презирство, зневага", "ur": "indicates undesirable quality", "vi": "hư hỏng, tệ hại", "yo": "indicates undesirable quality", "zh-tw": "劣質", "zh": "表示不好的质量"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-sufikso-ajx", typ: "vocab",
			content: map[string]interface{}{
				"word": "aĵ",
				"definition": "thing, concrete manifestation",
				"definitions": map[string]interface{}{"en": "thing, concrete manifestation", "nl": "concrete zaak", "de": "konkrete Sache", "fr": "chose concrète", "es": "cosa", "pt": "coisa, manifestação concreta", "ar": "لإبراز شئ من معنى واسع", "be": "вещь, конкретное проявление чего-либо", "ca": "cosa, objecte", "cs": "věc, konkrétní věc, projev", "da": "ting, konkret manifestation", "el": "επίθημα -> πράγμα, αντικείμενο", "fa": "چیز, شیء, اسم ذات", "frp": "chose concrète", "ga": "rud, léiriú nithiúil", "he": "דבר, עצם מוחשי", "hi": "thing, concrete manifestation", "hr": "stvar, konkretno", "hu": "dolog, konkrétum", "id": "benda, perwujudan segala sesuatu yang konkrit", "it": "cosa concreta", "ja": "こと・もの, ～される物", "kk": "thing, concrete manifestation", "km": "thing, concrete manifestation", "ko": "사물, 어떤한 것", "ku": "چیز, شیء, اسم ذات", "lo": "thing, concrete manifestation", "mg": "zavatra mivaingana", "ms": "barang, manifestasi yang kekal", "my": "thing, concrete manifestation", "pl": "rzecz, przedmiot", "ro": "chose concrète", "ru": "вещь, конкретное проявление чего-либо", "sk": "vec, predmet, konkrétna vec", "sl": "stvar, konkretno", "sv": "konkret sak, maträtt", "sw": "chose concrète", "th": "สิ่งของ, สิ่งที่เป็นรูปธรรม", "tok": "thing, concrete manifestation", "tr": "şey, somut anlam", "uk": "предмет з певного матеріалу, об’єкт діяльності", "ur": "thing, concrete manifestation", "vi": "vật chất, sản phẩm", "yo": "thing, concrete manifestation", "zh-tw": "物品, 品項", "zh": "东西, 看见的东西"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-sufikso-ebl", typ: "vocab",
			content: map[string]interface{}{
				"word": "ebl",
				"definition": "possibility",
				"definitions": map[string]interface{}{"en": "possibility", "nl": "mogelijkheid, -baar", "de": "Möglichkeit (-bar)", "fr": "possibilité", "es": "posibilidad", "pt": "possibilidade", "ar": "قابل", "be": "возможность", "ca": "possible", "cs": "možnost", "da": "mulighed", "el": "επίθημα -> δυνατότητα", "fa": "امکان", "frp": "possibilité", "ga": "féidearthacht", "he": "אפשרות", "hi": "possibility", "hr": "mogućnost", "hu": "-ható/hető", "id": "kemampuan", "it": "possibilità", "ja": "できる", "kk": "possibility", "km": "possibility", "ko": "가능성", "ku": "امکان", "lo": "possibility", "mg": "ny ahazoana atao", "ms": "kemungkinan", "my": "possibility", "pl": "możliwość", "ro": "possibilité", "ru": "возможность", "sk": "možnosť", "sl": "možnost", "sv": "möjlig att göraas", "sw": "possibilité", "th": "เป็นไปได้", "tok": "possibility", "tr": "olasılık", "uk": "пасивна можливість", "ur": "possibility", "vi": "có khả năng", "yo": "possibility", "zh-tw": "可能", "zh": "可能"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 6,
		},
		{
			slug: "vortaro-sufikso-ec", typ: "vocab",
			content: map[string]interface{}{
				"word": "ec",
				"definition": "abstract quality",
				"definitions": map[string]interface{}{"en": "abstract quality", "nl": "abstract begrip, eigenschap, -heid", "de": "abstrakter Begriff (-keit)", "fr": "qualité abstraite", "es": "cualidad abstracta", "pt": "qualidade abstrata", "ar": "تجريد المعنى", "be": "абстрактное качество", "ca": "qualitat abstracta", "cs": "vlastnost", "da": "abstrakt kvalitet", "el": "επίθημα -> αφηρημένη ιδιότητα", "fa": "ویژگی, کیفیت", "frp": "qualité abstraite", "ga": "cáilíocht theibí", "he": "תכונה מופשטת", "hi": "abstract quality", "hr": "apstraktno", "hu": "-ság/ség (tulajdonság)", "id": "kualitas abstrak", "it": "qualità astratta", "ja": "抽象的な性質", "kk": "abstract quality", "km": "abstract quality", "ko": "추상적인 특징", "ku": "ویژگی, کیفیت", "lo": "abstract quality", "mg": "kalitao foronin'ny saina", "ms": "kuali abstrak", "my": "abstract quality", "pl": "cecha, atrybut, własność", "ro": "qualité abstraite", "ru": "абстрактное качество", "sk": "vlastnosť", "sl": "abstraktno", "sv": "egenskap", "sw": "qualité abstraite", "th": "สิ่งที่เป็นนามธรรม", "tok": "abstract quality", "tr": "soyut kalite", "uk": "властивість, якість", "ur": "abstract quality", "vi": "phẩm chất", "yo": "abstract quality", "zh-tw": "性質", "zh": "抽象性质"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 7,
		},
		{
			slug: "vortaro-sufikso-eg", typ: "vocab",
			content: map[string]interface{}{
				"word": "eg",
				"definition": "big, augmentative",
				"definitions": map[string]interface{}{"en": "big, augmentative", "nl": "Vergroting", "de": "Vergrößerung", "fr": "grand, augmentatif", "es": "aumentativo", "pt": "grande, aumentativo", "ar": "تكبير, تقوية المعنى", "be": "большой, учеличительный суффикс", "ca": "gran", "cs": "zvětšení", "da": "stor, augmentativ", "el": "επίθημα -> μεγενθυτικό", "fa": "بزرگ, بزرگ‌نمایی", "frp": "grand, augmentatif", "ga": "mór, méadú", "he": "סיומת הגדלה", "hi": "big, augmentative", "hr": "uvećanica", "hu": "nagy, nagyítás", "id": "besar, meningkatkan", "it": "accrescitivo", "ja": "大きい, 巨大化で生じた別概念", "kk": "big, augmentative", "km": "big, augmentative", "ko": "큰", "ku": "بزرگ, بزرگ‌نمایی", "lo": "big, augmentative", "mg": "ngéza, lehibe, mampitombo", "ms": "besar, bertengkar", "my": "big, augmentative", "pl": "wielki, powiększenie", "ro": "grand, augmentatif", "ru": "большой, учеличительный суффикс", "sk": "zväčšenie", "sl": "povečanje", "sv": "förstärkning, förstoring", "sw": "grand, augmentatif", "th": "ขนาดใหญ่", "tok": "big, augmentative", "tr": "büyük", "uk": "збільшувальний суфікс", "ur": "big, augmentative", "vi": "to lớn, phóng đại", "yo": "big, augmentative", "zh-tw": "大, 巨", "zh": "大, 巨"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 8,
		},
		{
			slug: "vortaro-sufikso-ej", typ: "vocab",
			content: map[string]interface{}{
				"word": "ej",
				"definition": "place",
				"definitions": map[string]interface{}{"en": "place", "nl": "plaats, ruimte", "de": "Ort, Raum, Stelle", "fr": "lieu", "es": "lugar", "pt": "lugar", "ar": "موقع, مكان", "be": "место", "ca": "lloc", "cs": "místo", "da": "sted", "el": "επίθημα -> τόπος, χώρος που γίνεται κάτι", "fa": "مکان", "frp": "lieu", "ga": "áit", "he": "מקום", "hi": "place", "hr": "mjesto", "hu": "hely", "id": "tempat", "it": "luogo", "ja": "場所", "kk": "place", "km": "place", "ko": "장소", "ku": "مکان", "lo": "place", "mg": "toerana", "ms": "tempat", "my": "place", "pl": "miejsce", "ro": "lieu", "ru": "место", "sk": "miesto", "sl": "kraj", "sv": "plats, lokal", "sw": "lieu", "th": "สถานที่", "tok": "place", "tr": "yer", "uk": "місце", "ur": "place", "vi": "nơi chốn", "yo": "place", "zh-tw": "場所", "zh": "地方"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 9,
		},
		{
			slug: "vortaro-sufikso-er", typ: "vocab",
			content: map[string]interface{}{
				"word": "er",
				"definition": "fragment, small piece, particle",
				"definitions": map[string]interface{}{"en": "fragment, small piece, particle", "nl": "fragment, stukje van", "de": "Fragment, Stück von", "fr": "fragment, petite pièce, particule", "es": "fragmento, pieza pequeña, partícula", "pt": "fragmento, pequeno pedaço", "ar": "وحدة, قطرة", "be": "часть, фрагмент", "ca": "fragment, peça petita, partícula", "cs": "část", "da": "fragment, del af homogen helhed, partikel", "el": "επίθημα -> θραύσμα, κομματάκι, σωματίδιο", "fa": "خرده، تکه‌ی کوچک, ذره", "frp": "fragment, petite pièce, particule", "ga": "giota, píosa beag, cáithnín", "he": "חלק קטן מ", "hi": "fragment, small piece, particle", "hr": "komadić", "hu": "rész, részecske", "id": "bagian terkecil, partikel", "it": "una parte del tutto", "ja": "部分、断片、要素, 粒子", "kk": "fragment, small piece, particle", "km": "fragment, small piece, particle", "ko": "원소, 낱개의 구성물", "ku": "خرده، تکه‌ی کوچک, ذره", "lo": "fragment, small piece, particle", "mg": "tapatapaky, efitrano kely, sombin-javatra", "ms": "serpihan, sebahagian kecil, partikle", "my": "fragment, small piece, particle", "pl": "cząstka, oddzielna jednostka", "ro": "fragment, petite pièce, particule", "ru": "часть, фрагмент", "sk": "časť celku", "sl": "košček", "sv": "fragment, liten del, partikel", "sw": "fragment, petite pièce, particule", "th": "สิ่งที่เป็นชิ้น ๆ", "tok": "fragment, small piece, particle", "tr": "küçük parça, partikül", "uk": "частка чого-небудь", "ur": "fragment, small piece, particle", "vi": "hạt vụn, li ti, đơn vị nhỏ nhất", "yo": "fragment, small piece, particle", "zh-tw": "小片, 零碎", "zh": "碎片，小片, 零碎"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 10,
		},
		{
			slug: "vortaro-sufikso-estr", typ: "vocab",
			content: map[string]interface{}{
				"word": "estr",
				"definition": "chief, head",
				"definitions": map[string]interface{}{"en": "chief, head", "nl": "leider, chef, baas", "de": "Leiter, Chef, Vorstand", "fr": "chef, principal", "es": "jefe", "pt": "chefe, líder", "ar": "مدير", "be": "главный, правитель", "ca": "cap, màxima autoritat", "cs": "vedoucí", "da": "chef, leder", "el": "επίθημα -> αρχηγός, επικεφαλής, ηγέτης", "fa": "رییس", "frp": "chef, principal", "ga": "taoiseach, ceann", "he": "סיומת לניהול", "hi": "chief, head", "hr": "šef, glavni", "hu": "főnök, vezető", "id": "kepala, pemimpin", "it": "capo", "ja": "組織の長, 業務の長", "kk": "chief, head", "km": "chief, head", "ko": "조직의 수장, 회장", "ku": "رییس", "lo": "chief, head", "mg": "loha, voalohany", "ms": "ketua", "my": "chief, head", "pl": "dowódca, szef", "ro": "chef, principal", "ru": "главный, правитель", "sk": "vedúci", "sl": "šef, vodja", "sv": "ledare", "sw": "chef, principal", "th": "หัวหน้า", "tok": "chief, head", "tr": "şef, baş, yönetici", "uk": "особа, яка керує, завідує предметом, зазначеним у корені", "ur": "chief, head", "vi": "chỉ huy, chức trưởng", "yo": "chief, head", "zh-tw": "首長, 主管", "zh": "老板, 主要领导人"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 11,
		},
		{
			slug: "vortaro-sufikso-et", typ: "vocab",
			content: map[string]interface{}{
				"word": "et",
				"definition": "small, diminutive",
				"definitions": map[string]interface{}{"en": "small, diminutive", "nl": "verkleining", "de": "Verkleinerung", "fr": "petit, diminutif", "es": "diminutivo", "pt": "pequeno, diminutivo", "ar": "للتصغير, تضعيف المعنى", "be": "маленький, уменьшительный суффикс", "ca": "petit", "cs": "zmenšení", "da": "lille, diminutiv", "el": "επίθημα -> μικρού μεγέθους, χαϊδευτικό", "fa": "کوچک, کوچک‌نمایی", "frp": "petit, diminutif", "ga": "beag, laghdaitheach", "he": "סיומת הקטנה", "hi": "small, diminutive", "hr": "umanjenica", "hu": "kicsi, kicsinyítés", "id": "kecil", "it": "diminutivo", "ja": "小さい, 微小化で生じた別概念", "kk": "small, diminutive", "km": "small, diminutive", "ko": "작은", "ku": "کوچک, کوچک‌نمایی", "lo": "small, diminutive", "mg": "kely, mampihena", "ms": "kecil", "my": "small, diminutive", "pl": "mały, zdrobnienie", "ro": "petit, diminutif", "ru": "маленький, уменьшительный суффикс", "sk": "zmenšenie", "sl": "zmanjšanje", "sv": "försvagning, förminskning", "sw": "petit, diminutif", "th": "ขนาดเล็ก", "tok": "small, diminutive", "tr": "küçük", "uk": "зменшувальний суфікс", "ur": "small, diminutive", "vi": "bé, kém hơn", "yo": "small, diminutive", "zh-tw": "小, 微", "zh": "小, 微"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 12,
		},
		{
			slug: "vortaro-sufikso-ig", typ: "vocab",
			content: map[string]interface{}{
				"word": "ig",
				"definition": "cause something",
				"definitions": map[string]interface{}{"en": "cause something", "nl": "doen, maken", "de": "machen", "fr": "faire devenir, rendre, faire", "es": "causar algo", "pt": "tornar algo", "ar": "ليجعل", "be": "заставить, превратить", "ca": "causar alguna cosa", "cs": "udělat něco nějakým, způsobit", "da": "forårsage noget", "el": "επίθημα -> κάνω κάποιον ή κάτι να .. -> μετ. ρήμα", "fa": "تأثیرگذاری, مفعول‌پذیر کردن فعل, واداشتن, کردن", "frp": "faire quelque chose", "ga": "faoi deara rud éigin, cúis", "he": "לגרום", "hi": "cause something", "hr": "učiniti da nešto bude", "hu": "-tat/tet (műveltetés), -ít", "id": "menyebabkan sesuatu", "it": "far diventare", "ja": "～させる", "kk": "cause something", "km": "cause something", "ko": "~하게 만드는", "ku": "تأثیرگذاری, مفعول‌پذیر کردن فعل, واداشتن, کردن", "lo": "cause something", "mg": "ho tonga, mamerina , manome, manao", "ms": "menjadi sesuatu", "my": "cause something", "pl": "spowodować coś", "ro": "faire devenir, rendre, faire", "ru": "заставить, превратить", "sk": "urobiť niečo nejakým, spôsobiť", "sl": "napraviti, povzročiti", "sv": "göra, förvandla, förmå", "sw": "faire quelque chose", "th": "ทำให้", "tok": "cause something", "tr": "bir şey yapmak (geçişli fiil oluşturur)", "uk": "робити яким-небудь, ким-небудь, чим-нубудь", "ur": "cause something", "vi": "làm", "yo": "cause something", "zh-tw": "使, 引發", "zh": "改变东西、事情"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 13,
		},
		{
			slug: "vortaro-sufikso-il", typ: "vocab",
			content: map[string]interface{}{
				"word": "il",
				"definition": "tool, means",
				"definitions": map[string]interface{}{"en": "tool, means", "nl": "werktuig, gereedschap, middel", "de": "Werkzeug, Gerät, Mittel", "fr": "outil", "es": "herramienta, medio", "pt": "ferramenta, meio", "ar": "أداة", "be": "инструмент, средство", "ca": "eina, mitjà", "cs": "nástroj", "da": "værktøj, metode", "el": "επίθημα -> εργαλείο, όργανο, μέσο, συσκευή", "fa": "ابزار", "frp": "outil", "ga": "uirlis, modh", "he": "מכשיר, כלי", "hi": "tool, means", "hr": "sredstvo, oruđe", "hu": "eszköz", "id": "alat", "it": "strumento, mezzo", "ja": "道具を表す, 手段を表す", "kk": "tool, means", "km": "tool, means", "ko": "도구", "ku": "ابزار", "lo": "tool, means", "mg": "fitaovana", "ms": "alat", "my": "tool, means", "pl": "narzędzie, środek (np. transportu)", "ro": "outil", "ru": "инструмент, средство", "sk": "nástroj", "sl": "sredstvo, orodje", "sv": "verktyg, medel", "sw": "outil", "th": "เครื่องมือ", "tok": "tool, means", "tr": "alet, araç", "uk": "те, що служить для здійснення чого-небудь", "ur": "tool, means", "vi": "công cụ, phương tiện", "yo": "tool, means", "zh-tw": "器具, 工具", "zh": "器具, 方式"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 14,
		},
		{
			slug: "vortaro-sufikso-in", typ: "vocab",
			content: map[string]interface{}{
				"word": "in",
				"definition": "female",
				"definitions": map[string]interface{}{"en": "female", "nl": "vrouwelijk", "de": "weiblich", "fr": "féminin", "es": "femenino", "pt": "feminino", "ar": "للتأنيث", "be": "женщина, феминитив", "ca": "femení", "cs": "ženský rod", "da": "kvindelig", "el": "επίθημα -> θηλυκό γένος", "fa": "مؤنث", "frp": "féminin", "ga": "baineann", "he": "סיומת לנקבה", "hi": "female", "hr": "žensko", "hu": "nő, nőstény(állat)", "id": "perempuan", "it": "femminile", "ja": "女性", "kk": "female", "km": "female", "ko": "여성", "ku": "مؤنث", "lo": "female", "mg": "any ny vehivavy", "ms": "betina", "my": "female", "pl": "żenski", "ro": "féminin", "ru": "женщина, феминитив", "sk": "ženský rod", "sl": "ženska", "sv": "kvinna, hona", "sw": "féminin", "th": "เพศหญิง", "tok": "female", "tr": "kadın", "uk": "жіночий рід", "ur": "female", "vi": "phụ nữ, con gái", "yo": "female", "zh-tw": "雌性", "zh": "雌性"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 15,
		},
		{
			slug: "vortaro-sufikso-ind", typ: "vocab",
			content: map[string]interface{}{
				"word": "ind",
				"definition": "worthy of",
				"definitions": map[string]interface{}{"en": "worthy of", "nl": "-waard, -waardig", "de": "-wert, -würdig", "fr": "digne de", "es": "que vale para", "pt": "digno de, que vale a pena", "ar": "للاستحقاق", "be": "достойный", "ca": "que val la pena", "cs": "hodný (čeho)", "da": "værdig af", "el": "επίθημα -> άξιο για κάτι", "fa": "شایستگی", "frp": "digne de", "ga": "is fiú", "he": "ראוי", "hi": "worthy of", "hr": "vrijedno čega", "hu": "méltó, érdemes", "id": "pantas di", "it": "degno di", "ja": "～する価値がある", "kk": "worthy of", "km": "worthy of", "ko": "~할 가치가 있는", "ku": "شایستگی", "lo": "worthy of", "mg": "mendrika ny", "ms": "bernilai", "my": "worthy of", "pl": "godny, godzien, wart", "ro": "digne de", "ru": "достойный", "sk": "hodný (niečoho)", "sl": "vredno česa", "sv": "värd att göras", "sw": "digne de", "th": "น่า, มีคุณค่า", "tok": "worthy of", "tr": "değer", "uk": "гідний/вартий чого-небудь", "ur": "worthy of", "vi": "đáng để", "yo": "worthy of", "zh-tw": "值得", "zh": "有价值"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 16,
		},
		{
			slug: "vortaro-sufikso-ist", typ: "vocab",
			content: map[string]interface{}{
				"word": "ist",
				"definition": "profession, habitual association",
				"definitions": map[string]interface{}{"en": "profession, habitual association", "nl": "beroep of levensbeschouwing", "de": "Beruf, Weltanschauung", "fr": "profession", "es": "profesión, algo habitual", "pt": "profissão, associação habitual", "ar": "لاحقة للمهن و للإعتقاد", "be": "-ист-, профессия, определённый род занятий", "ca": "professió, activitat característica", "cs": "povolání", "da": "profession, stærk hobbyist", "el": "επίθημα -> επάγγελμα, συνεχής απασχόληση", "fa": "حرفه, اتحاد معمول", "frp": "profession", "ga": "gairm, gnáthcheangal", "he": "סיומת לציון מקצוע", "hi": "profession, habitual association", "hr": "zanimanje, pobornik čega", "hu": "foglalkozás", "id": "profesi, pelaku kebiasaan", "it": "mestiere", "ja": "職業人・専門家, 係", "kk": "profession, habitual association", "km": "profession, habitual association", "ko": "직업", "ku": "حرفه, اتحاد معمول", "lo": "profession, habitual association", "mg": "raharaha", "ms": "pakar", "my": "profession, habitual association", "pl": "fach, zawód", "ro": "profession", "ru": "-ист-, профессия, определённый род занятий", "sk": "povolanie", "sl": "poklic, pripadnik ideologije", "sv": "en som ofta sysslar med något, yrkesman", "sw": "profession", "th": "อาชีพ, ผู้ที่มีระบบความคิด", "tok": "profession, habitual association", "tr": "meslek", "uk": "особу певної професії, оcобу, що є прихильником громадського руху, наукового чи філософського напрямку", "ur": "profession, habitual association", "vi": "việc làm, công việc", "yo": "profession, habitual association", "zh-tw": "職業, 某理念支持者", "zh": "职业, 某种理念支持者"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 17,
		},
		{
			slug: "vortaro-sufikso-igx", typ: "vocab",
			content: map[string]interface{}{
				"word": "iĝ",
				"definition": "become",
				"definitions": map[string]interface{}{"en": "become", "nl": "worden", "de": "werden", "fr": "devenir", "es": "volverse en, convertirse en", "pt": "tornar-se", "ar": "يصبح", "be": "превратиться, стать", "ca": "esdevenir, convertir-se en, passar a ser/estar", "cs": "stát se", "da": "blive, forblive", "el": "επίθημα -> γίνομαι, καθίσταμαι .. -> αμετ. ρήμα", "fa": "تأثیرپذیری, شدن, مجهول کردن فعل", "frp": "devenir", "ga": "ag éirí níos", "he": "נעשה", "hi": "become", "hr": "postati", "hu": "válik valamivé, -ul/ül", "id": "menjadi", "it": "diventare", "ja": "～になる", "kk": "become", "km": "become", "ko": "~이 되는", "ku": "تأثیرپذیری, شدن, مجهول کردن فعل", "lo": "become", "mg": "tonga , miha...", "ms": "menjadi (bukan dipaksakan)", "my": "become", "pl": "stawać się, zostać, zostawać", "ro": "devenir", "ru": "превратиться, стать", "sk": "stať sa", "sl": "postati", "sv": "bli", "sw": "devenir", "th": "กลายเป็น, รับสถานะใหม่", "tok": "become", "tr": "olmak (geçişsiz fiil oluşturur)", "uk": "робитися/ставати  яким-небудь, ким-небудь, чим-небудь", "ur": "become", "vi": "trở nên, trở thành", "yo": "become", "zh-tw": "成為, 變成", "zh": "向新状态、新地点的转变"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 18,
		},
		{
			slug: "vortaro-sufikso-uj", typ: "vocab",
			content: map[string]interface{}{
				"word": "uj",
				"definition": "container, tree, country",
				"definitions": map[string]interface{}{"en": "container, tree, country", "nl": "vat (container), land, boom", "de": "Behälter, Land, Baum", "fr": "contenant, pays", "es": "contenedor, árbol, país", "pt": "recipiente, árvore de, país", "ar": "حافظة, وعاء, شجرة, بلد", "be": "вместилище, страна", "ca": "contenidor, arbre, país", "cs": "nádoba, strom, stát", "da": "beholder, træ, land", "el": "επίθημα -> δοχείο, φορέας, δέντρο, χώρα", "fa": "ظرف, درخت یا بوته, کشور", "frp": "contenant, pays", "ga": "gabhdán, crann, tír", "he": "מיכל, סוג עץ, שם מדינה", "hi": "container, tree, country", "hr": "posuda, stablo, država", "hu": "tartály, ország (ritka)", "id": "wadah, pohon, negara", "it": "contenitore totale", "ja": "入れ物, ～の木, 国", "kk": "container, tree, country", "km": "container, tree, country", "ko": "그릇, 용기, 나무, 나라", "ku": "ظرف, درخت یا بوته, کشور", "lo": "container, tree, country", "mg": "fasiana, misy, tany , fari-tany", "ms": "bekas, pokok, negara", "my": "container, tree, country", "pl": "pojemnik, drzewo, kraj", "ro": "contenant, pays", "ru": "вместилище, страна", "sk": "nádoba, puzdro, štát", "sl": "posoda, steblo, država", "sv": "behållare, träd/buske, land", "sw": "contenant, pays", "th": "ภาชนะ, ต้นไม้, ประเทศ", "tok": "container, tree, country", "tr": "kap, ağaç, ülke", "uk": "предмет, у якому що-небудь зберігається, дерево, країна", "ur": "container, tree, country", "vi": "đồ đựng, cây (cũ), quốc gia (cũ)", "yo": "container, tree, country", "zh-tw": "容器, 樹, 國", "zh": "容器, 树, 国家"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 19,
		},
		{
			slug: "vortaro-sufikso-ul", typ: "vocab",
			content: map[string]interface{}{
				"word": "ul",
				"definition": "person",
				"definitions": map[string]interface{}{"en": "person", "nl": "persoon", "de": "Person", "fr": "personne", "es": "individuo", "pt": "pessoa", "ar": "شخص", "be": "лицо с выраженным качеством", "ca": "individu", "cs": "přípona pro osobu, mající vlastnost obsaženou ve kmeni", "da": "person", "el": "επίθημα -> ιδιότητα προσώπου, όντος", "fa": "شخص", "frp": "personne", "ga": "duine", "he": "אדם", "hi": "person", "hr": "osoba", "hu": "személy", "id": "orang", "it": "individuo", "ja": "人", "kk": "person", "km": "person", "ko": "~한 특성을 가진 사람", "ku": "شخص", "lo": "person", "mg": "olona", "ms": "orang", "my": "person", "pl": "osoba", "ro": "personne", "ru": "лицо с выраженным качеством", "sk": "prípona znamenajúca živú bytosť, alebo vec majúcu vlastnosť slovného kmeňa", "sl": "oseba", "sv": "person med en viss egenskap", "sw": "personne", "th": "บุคคล", "tok": "person", "tr": "şahıs", "uk": "особа, яка володіє даною якістю", "ur": "person", "vi": "người", "yo": "person", "zh-tw": "人", "zh": "人"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 20,
		},
		{
			slug: "vortaro-sufikso-um", typ: "vocab",
			content: map[string]interface{}{
				"word": "um",
				"definition": "indefinite meaning",
				"definitions": map[string]interface{}{"en": "indefinite meaning", "nl": "zonder bepaalde betekenis", "de": "ohne bestimmte Bedeutung (Joker)", "fr": "signification indéfinie", "es": "significado indefinido", "pt": "significado indefinido", "ar": "لاحقة لاشتقاق الأفعال والحال من الأسماء", "be": "неопределённое значение", "ca": "significat indefinit (comodí per diferenciar sentit o matís)", "cs": "(používá se v různým významech)", "da": "ubestemt betydning", "el": "επίθημα -> αόριστο νόημα", "fa": "بدون معنی مشخص", "frp": "signification indéfinie", "ga": "brí éiginnte", "he": "סיומת לא מוגדרת", "hi": "indefinite meaning", "hr": "neodređeno značenje", "hu": "határozatlan jelentés", "id": "makna yang tidak tentu", "it": "suffisso indefinito", "ja": "付く語に関連したなんらかの動作", "kk": "indefinite meaning", "km": "indefinite meaning", "ko": "(불특정한) 해당 단어의 특성에 따름", "ku": "بدون معنی مشخص", "lo": "indefinite meaning", "mg": "dikany tsy voafetra", "ms": "makna tidak ditentukan", "my": "indefinite meaning", "pl": "niejasno określone znaczenie", "ro": "signification indéfinie", "ru": "неопределённое значение", "sk": "prípona neurčitého významu", "sl": "joker pripona uporablja se, kjer nobena druga pripona ne ustreza", "sv": "(obestämt suffix)", "sw": "signification indéfinie", "th": "ปัจจัยพิเศษ แสดงความหมายพิเศษเชื่อมโยงกับรากคำ", "tok": "indefinite meaning", "tr": "belirsiz anlamı olan ek", "uk": "не має окресленого значення", "ur": "indefinite meaning", "vi": "ý nghĩa không rõ", "yo": "indefinite meaning", "zh-tw": "搞, 無固定義", "zh": "没有规定的解释"},
			},
			tags:        []string{"vortaro", "sufikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-sufikso",
			seriesOrder: 21,
		},
		{
			slug: "vortaro-prefikso-dis", typ: "vocab",
			content: map[string]interface{}{
				"word": "dis",
				"definition": "dispersal, breaking up",
				"definitions": map[string]interface{}{"en": "dispersal, breaking up", "nl": "uiteen, uit-", "de": "Teilung, auseinander-, zer-", "fr": "dispersion, rupture", "es": "(separarse)", "pt": "dispersar, fragmentar", "ar": "إنتشار, انفصال", "be": "разъединение, раз-, рас-", "ca": "(separació, dispersió, distribució)", "cs": "roz-, distribuce", "da": "spredning, opløsning", "el": "πρόθημα -> διασκορπισμό, διαχωρισμό, διαμερισμό", "fa": "پراکندگی, گستردگی, تفکیک", "frp": "dispersion, rupture", "ga": "scaipeadh, briseadh as a chéile", "he": "פיזור, פיצול, הפרדה", "hi": "dispersal, breaking up", "hr": "raskid, raspršenje", "hu": "szét", "id": "penyebaran", "it": "dispersione, separazione", "ja": "ばらばらに, ～の反対", "kk": "dispersal, breaking up", "km": "dispersal, breaking up", "ko": "흩어지는, 멀리 퍼지는", "ku": "پراکندگی, گستردگی, تفکیک", "lo": "dispersal, breaking up", "mg": "fampielezana , fanelezana, fanapahana", "ms": "penyebaran, memecakkan", "my": "dispersal, breaking up", "pl": "roz-", "ro": "dispersion, rupture", "ru": "разъединение, раз-, рас-", "sk": "roz-, distribúcia", "sl": "raz", "sv": "isär, itu, åt olika håll", "sw": "dispersion, rupture", "th": "กระจาย", "tok": "dispersal, breaking up", "tr": "dağıtma, parçalara ayırma", "uk": "префікс, що позначає роз’єднання, поширення, роз-", "ur": "dispersal, breaking up", "vi": "phân tán, rải rác", "yo": "dispersal, breaking up", "zh-tw": "分散, 拆散", "zh": "分散, 拆散"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 1,
		},
		{
			slug: "vortaro-prefikso-ek", typ: "vocab",
			content: map[string]interface{}{
				"word": "ek",
				"definition": "beginning of action, suddenness",
				"definitions": map[string]interface{}{"en": "beginning of action, suddenness", "nl": "beginnende handelng", "de": "beginnende Handlung", "fr": "début de l'action, soudainement", "es": "(comienzo de acción), (acción momentánea)", "pt": "começo da ação", "ar": "بداية", "be": "начало действия, внезапность", "ca": "(començament d'una acció), (acció momentània)", "cs": "začátek akce, náhlost", "da": "begyndelse af handling, pludselighed", "el": "πρόθημα -> αρχή δράσης, στιγμιαία ενέργεια", "fa": "شروع عمل, عمل ناگهانی", "frp": "début de l'action, soudainement", "ga": "tús le gníomh, tobainne", "he": "תחילת פעולה, פתאומיות", "hi": "beginning of action, suddenness", "hr": "početak", "hu": "cselekvés kezdete, mozzanatos ige", "id": "awal dari sebuah tindakan, tiba-tiba", "it": "inizio di un'azione, subitaneità", "ja": "～しはじめる, 一瞬", "kk": "beginning of action, suddenness", "km": "beginning of action, suddenness", "ko": "막 시작하는, 갑작스러움", "ku": "شروع عمل, عمل ناگهانی", "lo": "beginning of action, suddenness", "mg": "fanombohana hetsika, tampoka", "ms": "mula satu perbuatan, tiba-tiba", "my": "beginning of action, suddenness", "pl": "początek lub chwilowość akcji, start!, naprzód!", "ro": "début de l'action, soudainement", "ru": "начало действия, внезапность", "sk": "začiatok činu, náhlosť", "sl": "začetek", "sv": "börja, plötsligt, kortvarigt", "sw": "début de l'action, soudainement", "th": "เริ่ม", "tok": "beginning of action, suddenness", "tr": "aksiyon başlangıc, ani başlangıç", "uk": "префікс, що позначає миттєвість або початок дії", "ur": "beginning of action, suddenness", "vi": "bắt đầu thực hiện, đột ngột", "yo": "beginning of action, suddenness", "zh-tw": "啟動, 突然", "zh": "马上行动, 突然间"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 2,
		},
		{
			slug: "vortaro-prefikso-for", typ: "vocab",
			content: map[string]interface{}{
				"word": "for",
				"definition": "away, off",
				"definitions": map[string]interface{}{"en": "away, off", "nl": "weg-, voort-", "de": "fort-, weg-, ab-", "fr": "éloignement, disparition", "pt": "longe, fora", "ar": "ابتعاد, اختفاء, زوال", "be": "прочь, вон, долой", "cs": "pryč", "da": "væk, fra", "el": "πρόθημα -> μακριά, εκτός", "fa": "دور, به دور", "frp": "éloignement, disparition", "ga": "ar shiúl, imithe", "he": "רחוק", "hi": "away, off", "hr": "udaljavanje (ne samo doslovno)", "hu": "el, tova", "id": "jauh", "it": "lontano, via", "ja": "離れて, ～し尽くす", "kk": "away, off", "km": "away, off", "ko": "멀리, 단절되어 멀어지는", "ku": "دور, به دور", "lo": "away, off", "mg": "fahalavirana , fanalavirana, fanjavonana , tsy fahitana", "ms": "pergi dari", "my": "away, off", "pl": "daleko", "ro": "éloignement, disparition", "ru": "прочь, вон, долой", "sk": "preč", "sl": "stran, proč", "sv": "bort, iväg", "sw": "éloignement, disparition", "th": "ไกล, หายหมด", "tok": "away, off", "tr": "uzak, yok etme", "uk": "геть, далеко", "ur": "away, off", "vi": "cách xa, ra khỏi", "yo": "away, off", "zh-tw": "遠離, 不在", "zh": "离开, 不在"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 3,
		},
		{
			slug: "vortaro-prefikso-ge", typ: "vocab",
			content: map[string]interface{}{
				"word": "ge",
				"definition": "pertaining of both sexes",
				"definitions": map[string]interface{}{"en": "pertaining of both sexes", "nl": "personen van beide geslachten", "de": "Personen beiderlei Geschlechts", "fr": "relatif aux deux sexes", "es": "(ambos sexos)", "pt": "ambos os sexos", "ar": "تتعلق بكلا الجنسين", "be": "группа лиц обоего пола, пара с женщиной и мужчиной", "ca": "(ambdós sexes inclosos)", "cs": "obsahující jedince obou pohlaví", "da": "af flere køn", "el": "πρόθημα -> και τα δύο γένη", "fa": "شامل دو جنس مذکر و مؤنث", "frp": "relatif aux deux sexes", "ga": "a bhaineann leis an dá ghnéas", "he": "כולל את שני המינים", "hi": "pertaining of both sexes", "hr": "oba spola", "hu": "kétféle nemű együtt", "id": "menyatakan bentuk yang mencakup perempuan dan laki-laki", "it": "ambo i sessi", "ja": "（男女の）", "kk": "pertaining of both sexes", "km": "pertaining of both sexes", "ko": "양성의, 남녀의", "ku": "شامل دو جنس مذکر و مؤنث", "lo": "pertaining of both sexes", "mg": "mifandraika amin'ny vavy sy lahy", "ms": "untuk menujukkan dua jantina", "my": "pertaining of both sexes", "pl": "obojga płci", "ro": "relatif aux deux sexes", "ru": "группа лиц обоего пола, пара с женщиной и мужчиной", "sk": "obsahujúci jedincov oboch pohlaví", "sl": "oba spola", "sv": "bägge könen", "sw": "relatif aux deux sexes", "th": "เพศชายและเพศหญิง", "tok": "pertaining of both sexes", "tr": "her iki cinse dair", "uk": "префікс, що  позначає осіб обох статей", "ur": "pertaining of both sexes", "vi": "chỉ cả hai giới", "yo": "pertaining of both sexes", "zh-tw": "兩性兼有", "zh": "前辍词表示两个性别在一起"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 4,
		},
		{
			slug: "vortaro-prefikso-mal", typ: "vocab",
			content: map[string]interface{}{
				"word": "mal",
				"definition": "opposite",
				"definitions": map[string]interface{}{"en": "opposite", "nl": "tegengestelde", "de": "Gegenteil", "fr": "opposition", "es": "(contrario, siendo antónimos absolutos)", "pt": "oposto", "ar": "نقيض", "be": "наоборот, противоположность", "ca": "(contrari, antònim)", "cs": "opak", "da": "modsat", "el": "πρόθημα -> το αντίθετο", "fa": "مخالف, متضاد", "frp": "opposition", "ga": "malairt", "he": "הפוך", "hi": "opposite", "hr": "suprotno", "hu": "ellentét", "id": "lawan kata", "it": "opposto", "ja": "～の反対", "kk": "opposite", "km": "opposite", "ko": "반대의", "ku": "مخالف, متضاد", "lo": "opposite", "mg": "mpanohitra , fanohirana", "ms": "bertentangan", "my": "opposite", "pl": "nie-, przeciwieństwo", "ro": "opposition", "ru": "наоборот, противоположность", "sk": "opak", "sl": "nasprotje", "sv": "motsatsen", "sw": "opposition", "th": "ตรงข้าม", "tok": "opposite", "tr": "zıt", "uk": "префікс, що позначає пряму протилежність", "ur": "opposite", "vi": "đối nghịch", "yo": "opposite", "zh-tw": "反義", "zh": "反义词的前辍"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 5,
		},
		{
			slug: "vortaro-prefikso-re", typ: "vocab",
			content: map[string]interface{}{
				"word": "re",
				"definition": "again, re-",
				"definitions": map[string]interface{}{"en": "again, re-", "nl": "weer-, terug-", "de": "wieder-, zurück-", "fr": "de nouveau, re-", "es": "de nuevo, otra vez", "pt": "de novo", "ar": "من جديد", "be": "ещё раз, пере-", "ca": "de nou, altra vegada, repetició", "cs": "znovu, opakování", "da": "igen, gen-", "el": "πρόθημα -> επανάλληψη πράξης, επιστροφή", "fa": "باز, وا, مجدد, دوباره", "frp": "de nouveau, re-", "ga": "arís, ath-", "he": "שוב ושוב, חזרה", "hi": "again, re-", "hr": "ponovno", "hu": "vissza, újra", "id": "lagi, re-", "it": "di nuovo, ri-", "ja": "再び, 繰り返し", "kk": "again, re-", "km": "again, re-", "ko": "다시, 새로운", "ku": "باز, وا, مجدد, دوباره", "lo": "again, re-", "mg": "indray, idray", "ms": "sekali lagi", "my": "again, re-", "pl": "powtórzenie, re-", "ro": "de nouveau, re-", "ru": "ещё раз, пере-", "sk": "znovu, opakovanie", "sl": "ponovno", "sv": "åter, tillbaka, på nytt", "sw": "de nouveau, re-", "th": "อีกครั้ง", "tok": "again, re-", "tr": "tekrar, geriye", "uk": "префікс, що позначає зворотну або повторну дію", "ur": "again, re-", "vi": "làm lại lần nữa", "yo": "again, re-", "zh-tw": "重新, 再來", "zh": "重新, 再来"},
			},
			tags:        []string{"vortaro", "prefikso"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "vortaro-prefikso",
			seriesOrder: 6,
		},
	}

	out := make([]*model.ContentItem, len(items))
	for i, s := range items {
		out[i] = &model.ContentItem{
			Slug:        s.slug,
			Type:        s.typ,
			Content:     s.content,
			Tags:        s.tags,
			Source:      s.source,
			AuthorID:    "seed",
			Status:      "approved",
			Rating:      s.rating,
			RD:          s.rd,
			Volatility:  0.06,
			Version:     1,
			SeriesSlug:  s.seriesSlug,
			SeriesOrder: s.seriesOrder,
		}
	}
	return out
}

// InitialSetup handles GET /admin/initial.
// This route is protected by GAE's "login: admin" handler in app.yaml,
// so only the Google Account owner/admins of the GAE project can reach it.
//
// Flow:
//  1. User visits the site normally → anonymous token stored in localStorage.
//  2. User visits /enskribi → copies magic link → browser follows it → token cookie is set.
//  3. User visits /admin/initial → GAE redirects to Google login → returns here.
//  4. Handler reads token from cookie → promotes that user to admin.
//  5. Redirects to /admin.
func (h *AdminHandler) InitialSetup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract the token from the cookie.
	cookie, err := r.Cookie("token")
	if err != nil || cookie.Value == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!DOCTYPE html><html lang="eo"><head><meta charset="UTF-8">
<title>Komenca agordo</title></head><body>
<h1>Komenca agordo de administranto</h1>
<p>Vi estas aŭtentikigita kiel GAE-administranto.</p>
<p>Por fariĝi administranto de la platformo, vi bezonas:</p>
<ol>
  <li>Iru al <a href="/">la hejmpaĝo</a> — via token estos kreita aŭtomate.</li>
  <li>Iru al <a href="/enskribi">/enskribi</a> — kopiu vian sekretligilon kaj sekvu ĝin en la retumilo por fiksi la kuketojn.</li>
  <li>Revenu al <a href="/admin/initial">/admin/initial</a>.</li>
</ol>
</body></html>`)
		return
	}

	u, err := h.users.GetByToken(ctx, cookie.Value)
	if err != nil || u == nil {
		http.Error(w, "Token nevalida — bonvolu sekvi la sekretligilon unue.", http.StatusBadRequest)
		return
	}

	if u.Role == "admin" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	u.Role = "admin"
	if err := h.users.Update(ctx, u); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
// seedContentItems returns reading and fill-in seed content (Zagreba Metodo + esperanto-kurso.net).
func seedContentItems() []*model.ContentItem {
	type si struct {
		slug, typ    string
		content      map[string]interface{}
		tags         []string
		source       string
		rating, rd   float64
		seriesSlug   string
		seriesOrder  int
	}
	items := []si{
		{
			slug: "zagr-l01-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 1: Amiko Marko", "text": "Marko estas mia amiko. Li estas lernanto kaj sportisto. Li nun sidas en ĉambro kaj lernas. Sur tablo estas paperoj kaj libroj. Ĝi estas skribotablo. La libroj sur la tablo estas lernolibroj.\n\nLa patro kaj la patrino de mia amiko ne estas en la ĉambro. Ili nun laboras. Lia patro estas laboristo, li laboras en hotelo. La patrino instruas. Ŝi estas instruistino.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-01", "legado"},
			source:      "La Zagreba Metodo",
			rating: 950, rd: 200,
			seriesSlug:  "zagr-l01",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l02-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 2: En la urbo", "text": "Marko havas amikinon. Ŝia nomo estas Ana. Ŝi estas juna kaj bela. Ana kaj Marko estas geamikoj.\n\nAna venis al la hejmo de Marko.\n\n– Bonvolu eniri, amikino.\n\n– Saluton Marko. Kion vi faras?\n\n– Saluton. Mi legis, sed nun mi volas paroli kun vi.\n\n– Ĉu vi havas bonan libron?\n\n– Jes, mi havas. Mi ŝatas legi nur bonajn librojn. Ĉu vi volas trinki kafon?\n\n– Jes. Ĉu viaj gepatroj kaj gefratoj estas en via hejmo?\n\n– Ne. Jen la kafo, ĝi estas ankoraŭ varma. Mi nun kuiris ĝin.\n\n– Dankon. Mi povas trinki ankaŭ malvarman kafon.\n\nMarko rigardis la belajn okulojn de Ana. Mi vidas, ke li amas ŝin.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-02", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1000, rd: 200,
			seriesSlug:  "zagr-l02",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l03-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 3: Vojaĝo", "text": "Ana kaj Marko longe parolis antaŭ la lernejo. Poste ili ĝoje iris tra la strato. Estis varma tago. Ili vidis kafejon kaj en ĝi kukojn.\n\n– Ni eniru kaj manĝu – diris Marko.\n\n– Ĉu vi havas sufiĉan monon?\n\n– Kompreneble, mi havas cent dolarojn.\n\n– Ho, vi estas riĉa!\n\nLa kelnero alportis multajn kukojn. Ili manĝis kaj post la manĝo la kelnero denove venis.\n\nMarko serĉis en unu loko … serĉis en alia … Lia vizaĝo estis malĝoja. Fine li diris per malforta voĉo:\n\n– Mi ne havas monon.\n\nKio okazis poste? Ĉu vi scias?", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-03", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1050, rd: 200,
			seriesSlug:  "zagr-l03",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l04-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 4: Familio", "text": "Marko skribis al mi belan leteron. Ĝi estas vera amletero. En la letero estis ankaŭ lia foto.\n\nKiam mi revenis el la lernejo, mi volis ĝin denove legi. Mi rapide eniris en mian ĉambron. Tie staris mia fratino Vera antaŭ mia tablo kaj legis mian leteron. En ŝia mano estis ankaŭ la foto.\n\nMi diris kolere:\n\n– Kion vi faras tie? Ne legu leterojn de aliaj!\n\nMia fratino fariĝis ruĝa. La foto falis el ŝia mano sur la tablon kaj la letero falis ankaŭ.\n\nŜi diris:\n\n– Pardonu … mi serĉis mian libron …\n\n– Sed vi bone scias, ke viaj libroj ne estas sur mia tablo. Ankaŭ ne en mia ĉambro! Redonu la leteron al mi.\n\nŜi ricevis bonan lecionon: ŝi ne plu legos leterojn de aliaj.\n\nAnkaŭ mi ricevis lecionon: mi devas bone fermi la pordon de mia ĉambro.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-04", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-l04",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l05-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 5: Hejmo", "text": "Sinjoro Rapid, amiko de Marko, eniris vendejon de aŭtoj. Tie li vidas belan sinjorinon, kiu plaĉas al li. Vendisto venas al li. Sinjoro Rapid demandas:\n\n– Kiu aŭto estas la plej bona?\n\n– Tiu ĉi estas la plej bona el ĉiuj.\n\n– Ĉu ĝi estas rapida?\n\n– Ĝi estas pli rapida ol aliaj.\n\n– Ĉu ĝi estas ankaŭ forta?\n\n– Ho jes, ĝi estas tre forta.\n\n– Certe ĝi estas multekosta?\n\n– Kompreneble, ĝi estas la plej multekosta el ĉiuj.\n\n– Dankon, bedaŭrinde mi ne aĉetos novan aŭton, ĉar mia malnova aŭto estas la plej malmultekosta el ĉiuj. Mi ankoraŭ veturos per ĝi.\n\nDum li tion diris, li denove rigardis la belan sinjorinon. Sed la vendisto diris:\n\n– Estos pli bone, ke vi ne rigardu tiun ĉi sinjorinon. Ankaŭ ŝi estas tre multekosta. Mi scias, ĉar mi estas ŝia edzo.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-05", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-l05",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l06-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 6: Manĝo", "text": "Maja estas beleta junulino. Ĉiuj ŝiaj geamikoj amas ŝin. Antaŭ tri tagoj malbono okazis: sur strato aŭto faligis ŝin.\n\nMaja malsaniĝis kaj havas altan temperaturon. Ŝia patrino diris, ke ŝi devas resti hejme. Morgaŭ ŝi vokos doktoron.\n\nLa doktoro venis je la naŭa horo por helpi al Maja.\n\n– Diru \"A\" … montru viajn manojn … montru viajn piedojn … malvestu vin!\n\nLa doktoro ĉion bone rigardis kaj fine diris:\n\n– Unu semajnon restu hejme kaj … ne iru sub aŭton. Trinku multan teon matene kaj vespere. Mi deziras, ke vi estu trankvila. Post unu semajno vi fartos pli bone. Mi venos por revidi vin.\n\n– Sinjoro doktoro, ĉu mia bonega amiko Karlo povas veni vidi min?\n\n– Hm, jes … jes. Li povas veni … sed ankaŭ li estu trankvila.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-06", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-l06",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l07-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 7: Laboro", "text": "Marko dormis matene tre longe.\n\nKiam li vidis la sunon, subite li eksaltis. Ĉiam li malfruas, kiam li devas iri al la lernejo. Li scias, ke la instruistino malŝatas tion.\n\nSed hodiaŭ li ne volas malfrui. Li rapide metis la vestaĵon. Rapidege li trinkis la kafon. Poste kun la libroj en la mano li kuris kaj saltis laŭ la strato. La homoj rigardis lin kaj diris: \" Kia malsaĝa knabo. \"\n\nBaldaŭ li estis antaŭ la lernejo. Li volis eniri en la lernejon, sed li ne povis. Ĝi estis fermita. Li vidis nek instruiston nek gelernantojn.\n\nLi eksidis antaŭ la lernejo kaj pensis: kio okazis?\n\n– Diable, nun mi memoras! Hodiaŭ estas dimanĉo!", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-07", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-l07",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l08-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 8: Libertempo", "text": "Hodiaŭ mia patrino estis tre maltrankvila. Mi venis el la lernejo kaj raportis al ŝi, ke mi ricevis malbonan noton. Ŝi diris: \" Knabaĉo, vi devas pli multe laborigi vian kapon. Kiom da malbonaj notoj vi havas?.\n\nMi estis malĝoja.\n\n– Iru en vian ĉambron kaj lernu! – ŝi diris.\n\nMi sidis en mia ĉambro, sed mi povis nek lerni nek legi. Miaj pensoj estis ĉe ludo kun miaj amikoj kaj ĉe sporto.\n\nMi diris al mi mem: Ĉu mi estas tiom malsaĝa, kvankam mi lernis multe?\n\nTiam la patrino eniris en la ĉambron kaj aŭdis miajn vortojn. Ŝi tuj respondis:\n\n– Ne, mia kara, vi ne estas malsaĝa. La problemo estas, ke vi devas lerni multe pli. Tiam vi ne havos nur malbonajn notojn.\n\nAnkaŭ nun mi ne havas ilin. Mi havas bonan noton pri muziko – mi diris.\n\nLa patrino subite ekridis. Tio denove plibeligis mian tagon. Sed la problemo tamen restis.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-08", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-l08",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l09-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 9: Naturo", "text": "Petro kaj Maria volis vojaĝi al alia lando en libertempo. Ili decidis iri al Italio, bela lando kun malnovaj urboj kaj aliaj belaĵoj. Ili estis feliĉaj:\n\n– Kie estas mia vestaĵo? – ŝi demandis.\n\n– Ni ne forgesu kunporti la libron por lerni Esperanton. Ni povus komenci jam en la vagonaro.\n\n– En la vagonaro ni nenion faros, kara Petro. Ni veturos per aŭto. Mi havas sekreton: mia riĉa onklo Bonifacio plenumis mian deziron kaj donis al ni sian aŭton por uzo. Ĝi jam staras antaŭ la domo.\n\n– Sed mi devas diri …\n\n– Nenion diru, karulo, ni eksidu en la aŭton kaj veturu al Italio.\n\nPetro kaj Maria sidis en la aŭto. Ankaŭ la vestaĵoj estis en la aŭto. Sed ili ankoraŭ sidadis kaj atendis.\n\n– Do, karulo – ŝi diris – kial ni ne ekveturas?\n\n– Karulino, ankaŭ mi havas sekreton. Mi veturigus la aŭton, sed mi ne scias. Mi neniam lernis tion.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-09", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1350, rd: 200,
			seriesSlug:  "zagr-l09",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l10-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 10: Vetero", "text": "– Kiel rapide pasas tagoj! Morgaŭ estas la naskiĝtago de via patro, Marko. Alvenos kelkaj familianoj kaj geamikoj. Ĉu vi bonvolus iom helpi? Trovu iomete da tempo almenaŭ por ordigi vian ĉambron. Mi ŝatus kuiri ion bonan. Por la naskiĝtaga kuko mankas lakto en la hejmo. Necesas havigi iom da manĝaĵo el vendejo …\n\n– Bone, mi aĉetumos por la naskiĝtago. Sed, anstataŭ sidi kaj manĝegi kun neinteresaj familianoj, mi ŝatus ekskursi kun klubanoj.\n\n– Kun kiuj klubanoj? – diris la patrino kaj ĵetis iajn paperojn en paperujon.\n\n– Morgaŭ frue kunvenas anoj de mia sporta klubo.\n\n– Sed, kara mia, ĉu ĝuste morgaŭ vi devas foresti? Ĉu vi ne intervidiĝas kun viaj samklubanoj trifoje semajne?\n\n– Ĉu mi rajtas ion proponi: Mi kunmanĝos kun la familianoj. Sed en la dua parto de la tago mi foriros kun miaj geamikoj. Post la kuko Maria kaj Petro certe parolos pri la malsukcesa veturado per aŭto. Malinterese! Mi ne plu volas tion aŭdi! Kial ni ne havas iun ideoriĉan onklon Bonifacio en nia familio?!\n\nLa patrino ridetis je liaj vortoj. Ŝi scias, ke Marko iom tro parolas hodiaŭ.\n\n– Kara Marko, ne sufiĉas nur riĉeco de la ideoj. Necesas ankaŭ plena monujo por realigi ilin.\n\nLa patrino post iom da silento daŭrigis:\n\n– Nu, antaŭ kelkaj minutoj telefonis Ana, ke ankaŭ ŝi vizitos nin morgaŭ. Ŝi tre ŝatas dolĉajn kukojn, kvankam ŝi atentos por ne dikiĝi.\n\nTuj Marko ŝanĝis la opinion. Sen Ana dum la ekskurso li estus tre soleca.\n\n– Patrino, tiuokaze mi tamen pasigos la morgaŭan tagon hejme.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-10", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1400, rd: 200,
			seriesSlug:  "zagr-l10",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l11-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 11: Sano", "text": "Estis vespero en Venecio. Centoj da homoj plenigadis la Sankt-Markan placon. Junuloj kaj militistoj, maristoj de sur la ŝipoj, elstaraj sinjorinoj, kaj junulinoj, alilandaj vojaĝantoj, ĉambristoj kaj gondolistoj – ĉiuj moviĝis al la urbomezo. Iom plue sed ne malproksime de la maro troviĝis malgranda placo. Fine de ĝi, proksime de la maro staris homo. Laŭ la vestaĵo oni facile povis ekscii, ke li estas gondolisto de iu riĉulo. Seninterese li rigardis la ĝojan hommulton. Subite rideto lumigis lian vizaĝon, kiam li ekvidis mariston, kiu alvenis el la flanko de la maro.\n\n– Ĉu tio estas vi, Stefano? – ekkriis la gondolisto. Ĉiuj diras, ke vi falis en la manojn de la Turkoj.\n\n– Vere. Ni renkontis unu el iliaj ŝipoj. Ĝi sekvis nin dum pli ol unu horo. Sed ne estas facile iri pli rapide ol nia ŝipo. Do, kiaj novaĵoj estas tie ĉi en Venecio?\n\n– Nenio interesa – nur granda malfeliĉo por Pietro. Ĉu vi konas Pietron?\n\n– Kompreneble, ke mi konas.\n\n– Granda ŝipo subakvigis lian gondolon.\n\n– Kaj kio pri Pietro?\n\n– Lia gondolo subakviĝis. Okazis, ke ni troviĝis proksime, tiel ke Ĝorĝo kaj mi prenis Pietron en nian gondolon. En la sama tempo mia estro subakviĝis por helpi junulinon, kiu preskaŭ jam mortis kun sia onklo.\n\n– Ho, tie estis junulino – kaj onklo?\n\n– Lin ni ne povis helpi. Estis grava homo. Sed kio vin alvenigas en Venecion, amiko mia?\n\nLa maristo rapide ekrigardis sian amikon kaj komencis diri:\n\n– Do, vi scias, Ĝino, mi alportis …\n\nLa gondolisto subite haltigis lin.\n\n– Vidu. – li diris.\n\nTiam iu homo pasis apud ili. Li ankoraŭ ne estis tridekjara, kaj lia vizaĝo estis senkolora. Lia iro estis certa kaj facila. Lia vizaĝo estis malĝoja.\n\n– Jakopo – diris Ĝino, kiam li ekvidis la homon – oni diras, ke multaj gravuloj donas sekretajn laborojn al li. Li scias tro da sekretoj.\n\n– Kaj tial ili timas sendi lin en malliberejon – diris la maristo, kaj dum li parolis, li montris la domegon de la Doĝoj.\n\n– Vere, multaj gravuloj bezonas lian helpon – diris Ĝino.\n\n– Kaj kiom ili pagas al li por unu mortigo?\n\n– Certe ne malpli ol cent monerojn. Ne forgesu, ke li laboras por tiuj, kiuj havas sufiĉe da mono por pagi al li. Sed, Stefano, en Venecio estas aferoj, kiujn estas pli bone forgesi, se vi volas trankvile manĝi vian panon.\n\nEstis iom da silento kaj poste la gondolisto denove ekparolis:\n\n– Vi venis ĝustatempe por vidi ŝipkuron.\n\n– Ĝino – iu diris malforte proksime de la gondolisto.\n\n– Sinjoro?\n\nDon Kamilo Monforte, la estro de Ĝino senvorte montris al la gondolo.\n\n– Ĝis revido – diris la gondolisto kaj prenis la manon de sia amiko.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-11", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1450, rd: 200,
			seriesSlug:  "zagr-l11",
			seriesOrder: 1,
		},
		{
			slug: "zagr-l12-teksto", typ: "reading",
			content: map[string]interface{}{"title": "Leciono 12: Reveno", "text": "S-ro Pipelbom estis alta, dika, peza kaj larĝa. Tio donis al li havindan korpoforton, kaj lia grandula aspekto ludis ne etan rolon en la fakto, ke la plimulto el la homoj, se ne lin timis, certe respektis kaj almenaŭ pripensis antaŭ ol decidi malkontentigi lin.\n\nSed tio, kio estas ofte utila dumtage, prezentas per si malplaĉan ĝenon dumnokte en loko arboplena. Ĉefe kiam oni devojiĝis kaj ekstervoje iraĉas en nehoma sovaĝejo.\n\nLa dika, peza, alta larĝa korpo ne sukcesis pasi senbrue inter la arbetoj; krome tuŝi la naturaĵojn, ĝenerale malsekajn, meze de kiuj li malfacile paŝis, estis travivaĵo ne malofte doloriga kaj ĉiam malagrabla. Ju pli li antaŭeniris en la vivo – kaj ju pli antaŭeniris en la ĉe kastela arbaro des pli Adriano Pipelbom malŝatis la noktan naturon.\n\nViro kun okuloj, kiuj kapablis bone vidi en la mallumo, rigardis la malfacilan iron de nia industriisto. Li ridetis.\n\nApud tiu viro, en vestaĵo de policano, staris alia persono, en simila vestaĵo. Ĉi lasta estis virino.\n\n\" Ni iru, \" la viro diris mallaŭte en la orelon de sia kunulino. \" Nun estas la ĝusta momento. \"\n\nAmbaŭ paŝis direkte al la alta, larĝa, peza, dika persono. Ili zorgis fari kiel eble plej malmulte da bruo. Junaj, facilmovaj, ili preskaŭ plene sukcesis.\n\nEn ĉi tiu aĉa situacio, kie arboj sovaĝe ĵetis siajn malsekajn branĉojn rekte en la vizaĝon de la industriisto, kie multpiedaj bestetoj prenis liajn piedojn por promenejo, kie naturo faligis lian piedon plej dolorige en kavon plenan de akvo aĉodora, Adriano Pipelbom rapide alvenis al la penso, ke oni devas ĉi tie esti preta por iu ajn malplaĉa renkonto. Ion ajn li efektive atendis, krom homa voĉo. Kiam do voĉo aŭdiĝis tuj proksime, li faris, ekmire, belan surlokan salton, kiu movis lian koron, ŝajne, supren ĝis la buŝo.\n\n\" Kio? \" li diris kun la provo sensukcese rekapti iom da trankvilo.\n\nSed la lumo de lampeto, kiun oni direktis rekte al liaj okuloj, malhelpis la repaciĝon. Krome, ŝajnis al li, ke la homformo, kiu tenis la lampon, surhavas vestojn policajn.\n\n\" Kion vi faras ĉi tie? \" sonis la aŭtoritata voĉo.\n\n\" Mi … mi … nuuuu … eee … \" fuŝparolis Pipelbom.\n\n\" Bonvolu paroli iom pli klare, \" la alia diris eĉ pli grav- tone.\n\n\" Mi … mi … mi promenas. \"\n\n\" Ha ha. Vi promenas en la bieno de la grafino de Montokalva, meze de la nokto. Ĉu la grafino vin invitis? Ĉu ŝi scias pri via ĉeesto ĉi tie? \"\n\n\" N … nu … N … ne. Mi … \"\n\n\" Mi do devas peti vin min sekvi. \" Kaj li klarigis, ke ekde la fuŝa provo kapti la grafinon, fare de teroristoj, la polico kontrolas atente, kio okazas en la ĉirkaŭaĵo de la kastelo.\n\nAl la industriisto ŝajnis, ke lia koro ĉi foje falis ĝis liaj piedoj. Li sciis, ke nur malfacile li povos trovi akcepteblan klarigon pri sia ĉeesto, kaj la ideo, ke la grafino ĉion scios, perdigis al li la malmulton da espero, kiu restis post la unuaj vortoj de la policano.", "question": "", "answer": ""},
			tags:        []string{"leciono", "zagr-12", "legado"},
			source:      "La Zagreba Metodo",
			rating: 1500, rd: 200,
			seriesSlug:  "zagr-l12",
			seriesOrder: 1,
		},
		{
			slug: "ekc-la-alfabeto-de-esperanto", typ: "reading",
			content: map[string]interface{}{"title": "La alfabeto de Esperanto", "text": "La Esperanta alfabeto havas 28 literojn: a, b, c, ĉ, d, e, f, g, ĝ, h, ĥ, i, j, ĵ, k, l, m, n, o, p, r, s, ŝ, t, u, ŭ, v, z.\n\nĈiu litero havas nur unu sonon. La akĉento ĉiam falas sur la antaŭlasta silabo.\n\nEsperanto estas fonetika lingvo — oni skribas kiel oni parolas.", "question": "", "answer": ""},
			tags:        []string{"alfabeto", "elparolo", "A0"},
			source:      "esperanto-kurso.net",
			rating: 850, rd: 200,
			seriesSlug:  "leciono-A0",
			seriesOrder: 1,
		},
		{
			slug: "ekc-salutado-kaj-adiauxado", typ: "reading",
			content: map[string]interface{}{"title": "Salutado kaj Adiaŭado", "text": "Saluton! — ĝenerala saluto\nBonan matenon! — matene\nBonan tagon! — tage\nBonan vesperon! — vespere\nBonan nokton! — dormante\n\nĜis revido! — ĝenerala adiaŭo\nĜis! — neformale\nAdiaŭ! — adiaŭo\n\nKiel vi fartas? — Bone, dankon. Kaj vi?", "question": "", "answer": ""},
			tags:        []string{"salutado", "A0", "leciono"},
			source:      "esperanto-kurso.net",
			rating: 900, rd: 200,
			seriesSlug:  "leciono-A0",
			seriesOrder: 2,
		},
		// familio vocab items (replaces the old reading word-list entry)
		{slug: "voc-familio-patro",   typ: "vocab", content: map[string]interface{}{"word": "patro",   "en": "father"},      tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-patrino", typ: "vocab", content: map[string]interface{}{"word": "patrino", "en": "mother"},      tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-filo",    typ: "vocab", content: map[string]interface{}{"word": "filo",    "en": "son"},         tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-filino",  typ: "vocab", content: map[string]interface{}{"word": "filino",  "en": "daughter"},    tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-frato",   typ: "vocab", content: map[string]interface{}{"word": "frato",   "en": "brother"},     tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-fratino", typ: "vocab", content: map[string]interface{}{"word": "fratino", "en": "sister"},      tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-avo",     typ: "vocab", content: map[string]interface{}{"word": "avo",     "en": "grandfather"}, tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-avino",   typ: "vocab", content: map[string]interface{}{"word": "avino",   "en": "grandmother"}, tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-edzo",    typ: "vocab", content: map[string]interface{}{"word": "edzo",    "en": "husband"},     tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-edzino",  typ: "vocab", content: map[string]interface{}{"word": "edzino",  "en": "wife"},        tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-familio-infano",  typ: "vocab", content: map[string]interface{}{"word": "infano",  "en": "child"},       tags: []string{"familio", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		// objektoj vocab items (replaces the old reading word-list entry)
		{slug: "voc-obj-libro",      typ: "vocab", content: map[string]interface{}{"word": "libro",      "en": "book"},      tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-tablo",      typ: "vocab", content: map[string]interface{}{"word": "tablo",      "en": "table"},     tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-segxo",      typ: "vocab", content: map[string]interface{}{"word": "seĝo",       "en": "chair"},     tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-fenestro",   typ: "vocab", content: map[string]interface{}{"word": "fenestro",   "en": "window"},    tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-pordo",      typ: "vocab", content: map[string]interface{}{"word": "pordo",      "en": "door"},      tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-lampo",      typ: "vocab", content: map[string]interface{}{"word": "lampo",      "en": "lamp"},      tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-akvo",       typ: "vocab", content: map[string]interface{}{"word": "akvo",       "en": "water"},     tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-pano",       typ: "vocab", content: map[string]interface{}{"word": "pano",       "en": "bread"},     tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-telefono",   typ: "vocab", content: map[string]interface{}{"word": "telefono",   "en": "telephone"}, tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{slug: "voc-obj-komputilo",  typ: "vocab", content: map[string]interface{}{"word": "komputilo",  "en": "computer"},  tags: []string{"objektoj", "vortaro"}, source: "esperanto-kurso.net", rating: 950, rd: 200},
		{
			slug: "ekc-baza-gramatiko-substantivoj-ka", typ: "reading",
			content: map[string]interface{}{"title": "Baza Gramatiko — Substantivoj kaj Verboj", "text": "SUBSTANTIVOJ finiĝas per -o:\nlibro, tablo, homo, urbo\n\nAKUZATIVO (direkto, objekto) finiĝas per -n:\nMi vidas libron. Ni iras en urbon.\n\nPLURalo finiĝas per -j:\nlibroj, tabloj, homoj\n\nVERBOJ:\n-as = nuno (mi lernas)\n-is = pasinto (mi lernis)\n-os = estonto (mi lernos)\n-u = ordono (lernu!)\n-i = infinitivo (lerni)", "question": "", "answer": ""},
			tags:        []string{"gramatiko", "A2", "leciono"},
			source:      "esperanto-kurso.net",
			rating: 1100, rd: 200,
			seriesSlug:  "leciono-A2",
			seriesOrder: 1,
		},
		{
			slug: "ekc-literatura-la-eta-princo-komen", typ: "reading",
			content: map[string]interface{}{"title": "Literatura — La Eta Princo: Komenco", "text": "Iam, kiam mi estis sesjara, mi vidis belegan bildon en iu libro pri la praarbaro, titolita 'Travivitaj rakontoj'. Tiu bildo prezentis boaon, kiu glutis sovaĝan beston.\n\nEn la libro estis skribite: 'Boaoj glutas sian predon tutaj, sen maĉi ĝin. Poste ili ne povas movi sin, kaj dormas dum ses monatoj dum digestado.'\n\nTiam mi multe primeditis la aventurojn de la ĝangalo, kaj mi sukcesis fari, per koloro, mian unuan desegnaĵon.", "question": "", "answer": ""},
			tags:        []string{"literatura", "C1", "legado"},
			source:      "esperanto-kurso.net",
			rating: 1700, rd: 200,
			seriesSlug:  "lit-eta-princo",
			seriesOrder: 1,
		},
		{
			slug: "ekc-literatura-la-sep-kapridoj", typ: "reading",
			content: map[string]interface{}{"title": "Literatura — La Sep Kapridoj", "text": "Estis iam maljuna kaprino, kiu havis sep idojn kaj amis ilin, kiel ĉiu patrino amas siajn infanojn. Iam ŝi volis iri en la arbaron por alporti manĝaĵon. Tiam ŝi kunvokis ĉiujn siajn idojn kaj diris:\n\n— Karaj infanoj, mi devas iri en la arbaron. Gardu vin antaŭ la lupo! Se li eniras, li manĝos vin ĉiujn — haŭton kaj harojn. La kanajlo ofte sin kaŝas, sed vi rekonos lin laŭ lia malglata voĉo kaj liaj nigraj piedoj.", "question": "", "answer": ""},
			tags:        []string{"literatura", "C1", "legado"},
			source:      "esperanto-kurso.net",
			rating: 1650, rd: 200,
			seriesSlug:  "lit-fabeloj",
			seriesOrder: 1,
		},
	}

	out := make([]*model.ContentItem, len(items))
	for i, s := range items {
		out[i] = &model.ContentItem{
			Slug:        s.slug,
			Type:        s.typ,
			Content:     s.content,
			Tags:        s.tags,
			Source:      s.source,
			AuthorID:    "seed",
			Status:      "approved",
			Rating:      s.rating,
			RD:          s.rd,
			Volatility:  0.06,
			Version:     1,
			SeriesSlug:  s.seriesSlug,
			SeriesOrder: s.seriesOrder,
		}
	}
	return out
}
// seedVideoItems returns Mazi, Pasporto, music video, and PMEG seed content.
func seedVideoItems() []*model.ContentItem {
	type vi struct {
		slug, typ   string
		title       string
		videoURL    string
		text        string
		tags        []string
		source      string
		rating      float64
		seriesSlug  string
		seriesOrder int
	}
	videos := []vi{
		// Mazi en Gondolando (12 episodes, sorted by episode number)
		{slug: "mazi-parto-01", typ: "video", title: "Mazi en Gondolando, parto 1", videoURL: "https://www.youtube.com/embed/fLFbqPBVOTg", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 1},
		{slug: "mazi-parto-02", typ: "video", title: "Mazi en Gondolando, parto 2", videoURL: "https://www.youtube.com/embed/pM9lYkBjqM8", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 2},
		{slug: "mazi-parto-03", typ: "video", title: "Mazi en Gondolando, parto 3", videoURL: "https://www.youtube.com/embed/_9XxngR3csI", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 3},
		{slug: "mazi-parto-04", typ: "video", title: "Mazi en Gondolando, parto 4", videoURL: "https://www.youtube.com/embed/vQaDTmdsHLw", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 4},
		{slug: "mazi-parto-05", typ: "video", title: "Mazi en Gondolando, parto 5", videoURL: "https://www.youtube.com/embed/WzkA3NUNhO8", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 5},
		{slug: "mazi-parto-06", typ: "video", title: "Mazi en Gondolando, parto 6", videoURL: "https://www.youtube.com/embed/zdGEBQp8Mwc", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 6},
		{slug: "mazi-parto-07", typ: "video", title: "Mazi en Gondolando, parto 7", videoURL: "https://www.youtube.com/embed/RkZgzjXCFsQ", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 7},
		{slug: "mazi-parto-08", typ: "video", title: "Mazi en Gondolando, parto 8", videoURL: "https://www.youtube.com/embed/KJRwTbIBERQ", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 8},
		{slug: "mazi-parto-09", typ: "video", title: "Mazi en Gondolando, parto 9", videoURL: "https://www.youtube.com/embed/mnKANu62kcM", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 9},
		{slug: "mazi-parto-10", typ: "video", title: "Mazi en Gondolando, parto 10", videoURL: "https://www.youtube.com/embed/Bkq_uxBxNCc", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 10},
		{slug: "mazi-parto-11", typ: "video", title: "Mazi en Gondolando, parto 11", videoURL: "https://www.youtube.com/embed/Oag9u2ilTxA", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 11},
		{slug: "mazi-parto-12", typ: "video", title: "Mazi en Gondolando, parto 12", videoURL: "https://www.youtube.com/embed/GScgLQKU9BQ", tags: []string{"mazi", "video"}, source: "Mazi en Gondolando", rating: 1000, seriesSlug: "mazi", seriesOrder: 12},
		// Pasporto al la Tuta Mondo (16 episodes)
		{slug: "pasporto-parto-01", typ: "video", title: "Pasporto al la Tuta Mondo, parto 1", videoURL: "https://www.youtube.com/embed/OquSnGAKYGc", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 1},
		{slug: "pasporto-parto-02", typ: "video", title: "Pasporto al la Tuta Mondo, parto 2", videoURL: "https://www.youtube.com/embed/7BHZdq2A5lM", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 2},
		{slug: "pasporto-parto-03", typ: "video", title: "Pasporto al la Tuta Mondo, parto 3", videoURL: "https://www.youtube.com/embed/DAhb9Zej93o", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 3},
		{slug: "pasporto-parto-04", typ: "video", title: "Pasporto al la Tuta Mondo, parto 4", videoURL: "https://www.youtube.com/embed/raq3A0WSS1U", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 4},
		{slug: "pasporto-parto-05", typ: "video", title: "Pasporto al la Tuta Mondo, parto 5", videoURL: "https://www.youtube.com/embed/iRFNEdVkXcg", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 5},
		{slug: "pasporto-parto-06", typ: "video", title: "Pasporto al la Tuta Mondo, parto 6", videoURL: "https://www.youtube.com/embed/8o5oH1zaj_M", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 6},
		{slug: "pasporto-parto-07", typ: "video", title: "Pasporto al la Tuta Mondo, parto 7", videoURL: "https://www.youtube.com/embed/7M95i0pN66o", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 7},
		{slug: "pasporto-parto-08", typ: "video", title: "Pasporto al la Tuta Mondo, parto 8", videoURL: "https://www.youtube.com/embed/erfCfwjHco8", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 8},
		{slug: "pasporto-parto-09", typ: "video", title: "Pasporto al la Tuta Mondo, parto 9", videoURL: "https://www.youtube.com/embed/wcJ4C9P7u00", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 9},
		{slug: "pasporto-parto-10", typ: "video", title: "Pasporto al la Tuta Mondo, parto 10", videoURL: "https://www.youtube.com/embed/1oWmSY38mUo", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 10},
		{slug: "pasporto-parto-11", typ: "video", title: "Pasporto al la Tuta Mondo, parto 11", videoURL: "https://www.youtube.com/embed/unnDQijvVjI", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 11},
		{slug: "pasporto-parto-12", typ: "video", title: "Pasporto al la Tuta Mondo, parto 12", videoURL: "https://www.youtube.com/embed/zUXkh4goHvc", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 12},
		{slug: "pasporto-parto-13", typ: "video", title: "Pasporto al la Tuta Mondo, parto 13", videoURL: "https://www.youtube.com/embed/QusvdgPQvwQ", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 13},
		{slug: "pasporto-parto-14", typ: "video", title: "Pasporto al la Tuta Mondo, parto 14", videoURL: "https://www.youtube.com/embed/00XI4N2YvZs", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 14},
		{slug: "pasporto-parto-15", typ: "video", title: "Pasporto al la Tuta Mondo, parto 15", videoURL: "https://www.youtube.com/embed/pVDS89uIaWY", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 15},
		{slug: "pasporto-parto-16", typ: "video", title: "Pasporto al la Tuta Mondo, parto 16", videoURL: "https://www.youtube.com/embed/vOnh09gXBuU", tags: []string{"pasporto", "video"}, source: "Pasporto al la Tuta Mondo", rating: 1200, seriesSlug: "pasporto", seriesOrder: 16},
		// Esperanto music videos
		{slug: "muzikv-berlino-sen-vi", typ: "video", title: "Berlino sen vi (inicialoj dc)", videoURL: "https://www.youtube.com/embed/530Y4a6jomI", tags: []string{"muziko", "video"}, source: "Esperanta muziko", rating: 1100, seriesSlug: "muziko", seriesOrder: 1},
		{slug: "muzikv-dankon-jonny-m", typ: "video", title: "Dankon (Jonny M)", videoURL: "https://www.youtube.com/embed/_T1u5Tq6jsU", tags: []string{"muziko", "video"}, source: "Esperanta muziko", rating: 1100, seriesSlug: "muziko", seriesOrder: 2},
		{slug: "muzikv-la-pluvo", typ: "video", title: "La pluvo (María Villalón)", videoURL: "https://www.youtube.com/embed/fOBkKcbJUAE", tags: []string{"muziko", "video"}, source: "Esperanta muziko", rating: 1100, seriesSlug: "muziko", seriesOrder: 3},
		{slug: "muzikv-superbazaro", typ: "video", title: "Superbazaro (Martin Wiese)", videoURL: "https://www.youtube.com/embed/gWiH8BlpU0U", tags: []string{"muziko", "video"}, source: "Esperanta muziko", rating: 1100, seriesSlug: "muziko", seriesOrder: 4},
	}

	type ri struct {
		slug, title, text string
		tags               []string
		source             string
		rating             float64
		seriesOrder        int
	}
	readings := []ri{
		{
			slug:  "pmeg-substantivoj",
			title: "PMEG – Substantivoj (O-vortoj)",
			text: "Vorto kun la finaĵo O nomiĝas O-vorto. O-vortoj estas nomoj de aferoj, konkretaĵoj, abstraktaĵoj, homoj, bestoj, fenomenoj, agoj, kvalitoj, specoj, individuoj ktp.\n\ntablo = nomo de konkretaĵo\nhundo = nomo de bestospeco\nsaĝo = nomo de kvalito\namo = nomo de sento\nkuro = nomo de ago\nPetro = nomo de persono\nBerlino = nomo de urbo\n\nPost O-finaĵo povas sekvi J-finaĵo por multenombro, kaj N-finaĵo por frazrolo. Oni ankaŭ povas meti ambaŭ, sed ĉiam J antaŭ N: tabloj — tablon — tablojn.\n\nOni povas anstataŭigi la finaĵon O per apostrofo ('), sed nur kiam ne sekvas J aŭ N: hund' = hundo, saĝ' = saĝo, Berlin' = Berlino.\n\nEn Esperanto ne ekzistas gramatika sekso. Sekso estas nur parto de la signifo de iuj O-vortoj. La finaĵo O neniel esprimas sekson. Ekzistas tri signifoklasoj: sekse neŭtraj radikoj (homo, infano, kato, amiko, studento...), virseksaj radikoj (viro, patro, frato, knabo...) kaj inseksaj radikoj (virino, patrino, fratino...).\n\nPor ina formo de sekse neŭtra radiko oni uzas la sufikson IN: instruistino, leonino, hundino. La sufikso GE indikas ambaŭsekse: gepatroj = patro kaj patrino kune, gefratoj = frato kaj fratino kune.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1450, seriesOrder: 1,
		},
		{
			slug:  "pmeg-adjektivoj",
			title: "PMEG – Adjektivoj (A-vortoj)",
			text: "Vorto kun la finaĵo A nomiĝas A-vorto. A-vortoj montras ecojn, kvalitojn, apartenojn, rilatojn ktp., kaj estas uzataj por priskribi. La A-finaĵo aldonas la ĝeneralan ideon «karakterizata de tio, kion esprimas la radiko».\n\nlonga = havanta multe da longo\nruĝa = havanta ruĝon kiel econ\nbona = karakterizata de bono\nhoma = rilata al homoj\n\nPost A-finaĵo povas sekvi J-finaĵo por multenombro, kaj N-finaĵo por frazrolo, ĉiam J antaŭ N: longaj — longan — longajn.\n\nA-vortoj estas uzataj precipe por priskribi O-vortojn. Rekte priskribantaj A-vortoj staras plej ofte antaŭ la priskribata O-vorto, sed povas ankaŭ stari poste:\ngranda domo — domo granda\nla longa tago — la tago longa\nfama Franca verkisto\n\nA-vorto povas ankaŭ priskribi ion pere de verbo (perverba priskribo):\nLa domo estas granda.\nTiuj ĉi verkistoj estas famaj.\nMi farbis mian domon blanka.\n\nAnkaŭ posedaj pronomoj kaj vicordaj nombrovortoj estas A-vortoj: ŝia, nia, sia, dua, sesa, dek-unua.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1500, seriesOrder: 2,
		},
		{
			slug:  "pmeg-verboj",
			title: "PMEG – Verbaj finaĵoj",
			text: "Verboj kun AS, IS, OS, US aŭ U rolas kiel ĉefverboj de frazo. Verboj kun I-finaĵo ne rolas kiel ĉefverbo, sed havas diversajn aliajn rolojn. I-verbo estas tradicie rigardataj kiel la baza formo de verbo — tial verboj aperas en I-formo en vortaroj.\n\nLa diversaj verbfinaĵoj prezentas agojn en kvar modoj: la neŭtrala modo (I-finaĵo), la reala modo, la vola modo (U-finaĵo) kaj la imaga modo (US-finaĵo). En la reala modo oni distingas inter tri tempoj.\n\nNeŭtrala modo — I-finaĵo: nur nomas agon aŭ staton, sen montri ĉu temas pri realaĵo, volo aŭ imago. Mi volas labori. Estas tede labori.\n\nNun-tempo — AS-finaĵo: la ago estas reala, efektiva, kaj komenciĝis sed ne finiĝis. Mi sidas sur seĝo. Kvar kaj dek ok faras dudek du. En la vintro oni hejtas la fornojn.\n\nPasinta tempo — IS-finaĵo: la ago estas reala, sed okazis iam antaŭ la momento de parolado. Mi sidis tiam sur seĝo. Hieraŭ mi renkontis vian filon, kaj li ĝentile salutis min.\n\nVenonta tempo — OS-finaĵo: la ago estos reala, sed ankoraŭ ne komenciĝis. Mi iros morgaŭ. Ĉu vi estos tie?\n\nVola modo — U-finaĵo: esprimas volon, peton, ordonon aŭ celon. Venu! Ni iru. Mi volas, ke vi legu tion.\n\nImaga modo — US-finaĵo: esprimas ion hipotezan. Se mi havus monon, mi vojaĝus. Mi estus feliĉa.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1550, seriesOrder: 3,
		},
		{
			slug:  "pmeg-pronomoj",
			title: "PMEG – Pronomoj",
			text: "Pronomoj estas vortetoj, kiujn oni uzas por paroli pri tute konataj aferoj. En Esperanto ekzistas dek personaj pronomoj:\n\nmi = la parolanto\nni = la parolanto kaj alia(j) persono(j)\nvi = la alparolato(j)\nli = la priparolata vira persono aŭ persono kun nekonata sekso\nŝi = la priparolata ina persono\nĝi = la priparolata aĵo, besto aŭ infaneto\nili = la priparolataj personoj, aĵoj aŭ bestoj\noni = neprecizigita(j) persono(j)\nsi = la sama persono kiel la subjekto, se tiu ne estas mi, ni aŭ vi\n\nPersonaj pronomoj povas ricevi la finaĵon N: Mi amas vin. Ilin konas Karlo. Ĉu vi ĝin vidas? Elizabeto lavas sin en la lago.\n\nSe oni aldonas la finaĵon A al personaj pronomoj, oni kreas posedajn pronomojn:\nmia = (la)... de mi\nnia = (la)... de ni\nvia = (la)... de vi\nlia = (la)... de li\nŝia = (la)... de ŝi\nĝia = (la)... de ĝi\nilia = (la)... de ili\nsia = (la)... de si\n\nLa pronomo si estas uzata por referi returne al la subjekto de la sama frazo: Petro lavas sin. Li prenis sian ĉapelon. Tio estas alia ol: Petro lavas lin (= iun alian homon).",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1600, seriesOrder: 4,
		},
		{
			slug:  "pmeg-prepozicioj",
			title: "PMEG – Rolvortetoj (Prepozicioj)",
			text: "Rolvortetoj estas vortetoj, kiujn oni metas antaŭ frazpartoj por montri frazrolojn. Frazparto kun rolvorteto povas roli aŭ kiel komplemento de verbo, aŭ kiel priskribo de alia frazparto.\n\nLokaj rolvortetoj montras pozicion:\nen — interne de io: en la domo, en Berlino\nsur — sur la surfaco de io: sur la tablo\nsub — malsupre de io: sub la lito\nsuper — pli alte ol io, sen tuŝo: super la nuboj\napud — flanke proksime de io: apud la pordo\nĉe — tre proksime de io: ĉe la fenestro\ninter — meze de du aŭ pli da aĵoj: inter la domoj\n\nDirektaj rolvortetoj montras direkton de movo:\nal — direkto al celo: iri al la urbo\nel — direkto el iu loko: veni el la domo\nde — deveno, fonto: letero de la amiko\nĝis — fino de movo aŭ daŭro: iri ĝis la rivero\n\nAliaj gravaj rolvortetoj:\nkun — kuneco: iri kun amiko\nsen — manko: kafi sen sukero\npor — celo aŭ destinulo: libro por infanoj\nper — ilo aŭ rimedo: skribi per krajono\npri — temo: paroli pri la vetero\npro — kaŭzo: danki pro la helpo\ndum — daŭro: labori dum la tuta tago\npost — posteco: veni post li",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1650, seriesOrder: 5,
		},
		{
			slug:  "pmeg-nombroj",
			title: "PMEG – Nombro kaj Multenombro",
			text: "Ĉe O-vortoj kaj A-vortoj oni devas distingi inter ununombro kaj multenombro. Ununombro signifas, ke temas pri unu afero. Multenombron oni montras per la finaĵo J. Manko de J-finaĵo montras ununombron.\n\n(unu) tago — (pluraj) tagoj\n(unu) granda domo — (pluraj) grandaj domoj\nilia granda domo — iliaj grandaj domoj\nla kato estas nigra — la katoj estas nigraj\n\nEventuala N-finaĵo staras post J: tagojn, grandajn, nigrajn, iliajn.\n\nRadikoj estas neŭtralaj pri nombro. Ili povas montri jen unu aferon, jen plurajn:\nokula = rilata al okulo aŭ okuloj\nokulkuracisto = kuracisto de okuloj (ne: okulojkuracisto)\nlibrovendejo = vendejo de libroj (ne: librojvendejo)\n\nOni do ne uzas J-finaĵojn ene de kunmetitaj vortoj.\n\nNombrovortoj en Esperanto: nul, unu, du, tri, kvar, kvin, ses, sep, ok, naŭ, dek, dudek, cent, mil, miliono. Ili ne havas finaĵon propre. Vicordaj nombroj (ordinaloj) estas A-vortoj: unua, dua, tria, kvara... Multioblaj nombroj: duobla, triobla. Frakciaj nombroj: duono, triono, kvarono.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1700, seriesOrder: 6,
		},
		{
			slug:  "pmeg-tabelvortoj",
			title: "PMEG – Tabelvortoj",
			text: "45 vortetoj nomiĝas tabelvortoj, ĉar oni povas ilin aranĝi en tabelo laŭ similaj formoj kaj similaj signifoj. Ĉiu tabelvorto konsistas el antaŭparto kaj postparto.\n\nAntaŭpartoj:\nKI- = demandovorto, rilata vorto\nTI- = montrovorto\nI- = nedifinita vorto\nĈI- = tutampleksa vorto\nNENI- = nea vorto\n\nPostpartoj:\n-U = individuo: kiu, tiu, iu, ĉiu, neniu\n-O = afero: kio, tio, io, ĉio, nenio\n-A = eco, speco: kia, tia, ia, ĉia, nenia\n-E = loko: kie, tie, ie, ĉie, nenie\n-AM = tempo: kiam, tiam, iam, ĉiam, neniam\n-AL = kaŭzo: kial, tial, ial, ĉial, nenial\n-EL = maniero: kiel, tiel, iel, ĉiel, neniel\n-OM = kvanto: kiom, tiom, iom, ĉiom, neniom\n\nLa tabelvortoj je U kaj A povas ricevi la finaĵojn J kaj N: kiujn, tiujn, iujn, ĉiujn, neniujn. La tabelvortoj je O povas ricevi N, sed normale ne J: kion, tion, ion, ĉion, nenion.\n\nEkzemploj: Kiu estas tiu homo? Tio estas bela. Mi ne scias, kio okazis. Ĉiuj venis. Li neniam helpas.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1750, seriesOrder: 7,
		},
		{
			slug:  "pmeg-participoj",
			title: "PMEG – Participoj",
			text: "Participoj estas vortoj, kiuj prezentas agon aŭ staton kvazaŭ econ de ĝia subjekto aŭ objekto. Participojn oni formas per specialaj participaj sufiksoj. Ekzistas ses participaj sufiksoj, tri aktivaj kaj tri pasivaj.\n\nAktivaj participoj (priskribas la subjekton de la ago):\nANT — la ago ankoraŭ ne finiĝis: leganta = tia, ke oni ankoraŭ legas\nINT — la ago jam finiĝis: leginta = tia, ke oni antaŭe legis\nONT — la ago ankoraŭ ne komenciĝis: legonta = tia, ke oni poste legos\n\nPasivaj participoj (priskribas la objekton de la ago):\nAT — la ago ankoraŭ ne finiĝis: legata = tia, ke iu ĝin legas\nIT — la ago jam finiĝis: legita = tia, ke iu ĝin jam legis\nOT — la ago ankoraŭ ne komenciĝis: legota = tia, ke iu ĝin poste legos\n\nEkzemploj:\nViro, kiu ankoraŭ legas, estas leganta viro.\nViro, kiu antaŭe legis, estas leginta viro.\nLibro, kiun oni ankoraŭ legas, estas legata libro.\nLibro, kiun oni antaŭe legis, estas legita libro.\n\nParticipoj kun O-finaĵo funkcias kiel substantivoj: leganto = tiu, kiu nun legas; leginto = tiu, kiu legis; legitaro = grupo de legintoj. Participoj kun E-finaĵo funkcias kiel adverboj: li falis kuŝante = li falis dum li kuŝis.",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1800, seriesOrder: 8,
		},
		{
			slug:  "pmeg-vortfarado",
			title: "PMEG – Afiksoj kaj Vortfarado",
			text: "Malgranda grupo de radikoj (ĉirkaŭ 40) nomiĝas afiksoj. Kelkaj estas sufiksoj (postafiksoj), aliaj estas prefiksoj (antaŭafiksoj). Afiksoj partoprenas en vortfarado laŭ specialaj reguloj.\n\nGravaj prefiksoj:\nmal- = kontraŭo de la baza signifo: bona→malbona, granda→malgranda, ami→malami\nre- = denova ago aŭ reveno: fari→refari, veni→reveni\nek- = komenco de ago: kuri→ekkuri, ridi→ekridi\ndis- = disigo, disiĝo: doni→disdoni, fali→disfali\nmis- = malbona, erara ago: uzi→misuzi, kompreni→miskompreni\nge- = ambaŭ seksoj kune: patro→gepatroj, frato→gefratoj\n\nGravaj sufiksoj:\n-ist = profesiulo pri io: lerni→lernisto, instrui→instruisto, muziko→muzikisto\n-in = ina formo: kato→katino, instruisto→instruistino, leono→leonino\n-et = malgranda: domo→dometo, ridi→rideti, varma→varmeta\n-eg = granda, intensa: domo→domego, varma→varmega, bela→belega\n-aĵ = konkreta manifesto de io: manĝi→manĝaĵo, bela→belaĵo, nova→novaĵo\n-ar = kolekto de similaj aferoj: vorto→vortaro, arbo→arbaro, homo→homaro\n-ej = loko destinita por io: lerni→lernejo, manĝi→manĝejo, libro→librejo\n-il = ilo por fari ion: skribi→skribilo, tranĉi→tranĉilo, kombi→kombilo\n-an = membro de grupo: urbo→urbano, klubo→klubano, Eŭropo→Eŭropano",
			tags: []string{"pmeg", "gramatiko"}, source: "PMEG", rating: 1850, seriesOrder: 9,
		},
	}

	var items []*model.ContentItem
	for _, v := range videos {
		items = append(items, &model.ContentItem{
			Slug:        v.slug,
			Type:        v.typ,
			Content:     map[string]interface{}{"title": v.title, "video_url": v.videoURL},
			Tags:        v.tags,
			Source:      v.source,
			Status:      "approved",
			Rating:      v.rating,
			RD:          200,
			Volatility:  0.06,
			SeriesSlug:  v.seriesSlug,
			SeriesOrder: v.seriesOrder,
		})
	}
	for _, r := range readings {
		items = append(items, &model.ContentItem{
			Slug:        r.slug,
			Type:        "reading",
			Content:     map[string]interface{}{"title": r.title, "text": r.text},
			Tags:        r.tags,
			Source:      r.source,
			Status:      "approved",
			Rating:      r.rating,
			RD:          200,
			Volatility:  0.06,
			SeriesSlug:  "pmeg",
			SeriesOrder: r.seriesOrder,
		})
	}
	return items
}
// seedExtraItems returns literary texts, news articles, grammar exercises, and the alphabet video.
func seedExtraItems() []*model.ContentItem {
	var items []*model.ContentItem

	// Alphabet video (YouTube embed — overrides the reading we already have under a different slug)
	items = append(items, &model.ContentItem{
		Slug: "la-alfabeto-video", Type: "video",
		Content: map[string]interface{}{
			"title":     "La alfabeto de Esperanto (video)",
			"video_url": "https://www.youtube.com/embed/OPsdp1M5pjQ",
		},
		Tags: []string{"alfabeto", "video", "komencanto"},
		Source: "esperanto-kurso.net", Status: "approved",
		Rating: 800, RD: 200, Volatility: 0.06,
		SeriesSlug: "", SeriesOrder: 0,
	})

	// Grammar multiple-choice exercises (from TiddlyWiki "Ekzerco - Baza Gramatiko")
	grammarMC := []struct {
		slug, question string
		options        []string
		correct        int
		tags           []string
		rating         float64
		order          int
	}{
		{
			slug:     "gramatiko-mc-01-plurnombro",
			question: `Kiel oni faras plurnombron el la vorto "libro"?`,
			options:  []string{"libroj", "libros", "library"},
			correct:  0,
			tags:     []string{"gramatiko", "substantivoj"},
			rating:   1100,
			order:    1,
		},
		{
			slug:     "gramatiko-mc-02-akuzativo",
			question: "Kiu frazo estas ĝusta?",
			options:  []string{"Mi vidas hundon", "Mi vidas hundo", "Mi vidas de hundo"},
			correct:  0,
			tags:     []string{"gramatiko", "akuzativo"},
			rating:   1150,
			order:    2,
		},
		{
			slug:     "gramatiko-mc-03-adjektivo",
			question: `Kiel oni diras "bela domo" en plurnombro?`,
			options:  []string{"belaj domoj", "bela domoj", "belaj domo"},
			correct:  0,
			tags:     []string{"gramatiko", "adjektivoj"},
			rating:   1200,
			order:    3,
		},
		{
			slug:     "gramatiko-mc-04-pasinteco",
			question: "Kiel oni diras la pasintecon de manĝi?",
			options:  []string{"mi manĝis", "mi manĝas", "mi manĝos"},
			correct:  0,
			tags:     []string{"gramatiko", "verboj"},
			rating:   1100,
			order:    4,
		},
		{
			slug:     "gramatiko-mc-05-prepozicio",
			question: "Kiu frazo estas ĝusta?",
			options:  []string{"La libro estas sur la tablo", "La libro estas sur la tablon", "La libro estas sur la tablen"},
			correct:  0,
			tags:     []string{"gramatiko", "prepozicioj"},
			rating:   1250,
			order:    5,
		},
	}
	for _, q := range grammarMC {
		items = append(items, &model.ContentItem{
			Slug: q.slug, Type: "multiplechoice",
			Content: map[string]interface{}{
				"question":      q.question,
				"options":       q.options,
				"correct_index": q.correct,
			},
			Tags: q.tags, Source: "esperanto-kurso.net", Status: "approved",
			Rating: q.rating, RD: 200, Volatility: 0.06,
			SeriesSlug: "gramatiko-mc", SeriesOrder: q.order,
		})
	}

	// Literary readings — direct copies from esperantolibroj repo
	literary := []struct {
		slug, title, text, source string
		rating                    float64
		order                     int
	}{
		{
			slug:  "lit-la-eta-princo-komenco",
			title: "La Eta Princo — Ĉapitro I (Antoine de Saint-Exupéry)",
			text:  "Iam, kiam mi estis sesjara, mi vidis belegan bildon en iu libro pri la praarbaro, titolita \"Travivitaj rakontoj\". Tiu bildo prezentis boaon, kiu glutas rabobeston.\n\nEn la libro oni diris: \"La boaj glutas sian rabaĵon unuglute, sen-maĉe. Sekve ili ne plu povas moviĝi kaj dormas dum sia sesmona-ta digestado.\"\n\nEkde tiam mi multe meditis pri la aventuroj en ĝangalo kaj per kolorkrajono mi sukcesis miavice fari mian unuan desegnon. Mian desegnon numero Unu.\n\nMi montris mian ĉefverkon al granduloj kaj ilin demandis, ĉu mia desegno timigis ilin.\n\nIli al mi respondis: \"Kial ĉapelo timigus?\"\n\nMia desegno ne prezentis ĉapelon. Ĝi prezentis boaon, kiu digestadas elefanton. Do, mi desegnis la enhavon de la boao, por komprenigi al granduloj. Ili ĉiam bezonas klarigojn.\n\nLa granduloj konsilis, ke mi flankenlasu desegnojn de boaoj aŭ malfermitaj aŭ ne, kaj prefere interesiĝu pri geografio, historio, kalkularto kaj gramatiko. Kaj tiel, en mia sesjara aĝo, mi rezignis grandiozan pentristan karieron. Mi senkuraĝiĝis pro la fiasko de mia desegno numero Unu kaj de mia desegno numero Du. Neniam la granduloj komprenas tute per si mem kaj al la infanoj estas lacige ĉiam kaj ĉiam donadi al ili klarigojn.\n\nMi do devis elekti alian metion kaj lernis piloti aviadilojn. Mi flugis iom ĉie tra la mondo. Kaj mi tute konsentas, ke geografio multe utilis al mi. Mi scipovis unuavide distingi Ĉinion de Arizono. Tio estas tre taŭga, se oni vojeraris nokte.\n\nKiam mi renkontis inter ili iun, kiu ŝajnis al mi iom klarvida, iam mi provis per mia desegno numero Unu, kiun mi ĉiam konservis. Mi volis scii, ĉu tiu ĉi vere estas komprenema. Sed ĉiam oni respondis al mi: \"Ĝi estas ĉapelo.\" Tiam al tiu mi parolis nek pri boaoj, nek pri praarbaroj, nek pri steloj. Mi adaptiĝis al ties komprenpovo. Mi priparolis briĝon, golfludon, politikon kaj kravatojn. Kaj la grandulo estis ja kontenta koni homon tiel konvenan.",
			source: "La eta princo — Antoine de Saint-Exupéry", rating: 1600, order: 1,
		},
		{
			slug:  "lit-la-sep-kapridoj",
			title: "La Sep Kapridoj (Fratoj Grimm, trad. Kabe)",
			text:  "Estis iam maljuna kaprino, kiu havis sep idojn kaj amis ilin, kiel ĉiu patrino amas siajn infanojn. Iam ŝi volis iri en la arbaron por alporti nutraĵon. Ŝi alvokis ĉiujn sep kaj diris:\n\n\"Karaj infanoj, mi iras en la arbaron, gardu vin bone kontraŭ la lupo; ne enlasu ĝin, ĉar alie ĝi manĝos vin kun haŭto kaj haroj. La fripono ofte aliformigas sin, sed vi tuj rekonos ĝin per ĝiaj nigraj piedoj kaj ĝia raŭka voĉo.\"\n\nLa kapridoj diris: \"Kara patrinjo, vi povas trankvile foriri, ni estos singardaj.\"\n\nLa patrino ekblekis kaj foriris.\n\nBaldaŭ iu ekfrapis la pordon kaj ekkriis: \"Malfermu, karaj infanoj, via panjo alportis ion por ĉiu.\"\n\nSed la kapridoj rekonis la lupon per la raŭka voĉo: \"Ni ne malfermos,\" ili ekkriis, \"vi ne estas nia patrino, ŝi havas delikatan voĉon, kaj la via estas raŭka, vi estas la lupo!\"\n\nLa lupo iris en butikon kaj aĉetis grandan pecon da kreto. Ĝi manĝis ĝin kaj ĝia voĉo fariĝis delikata. Ĝi revenis, frapis la pordon kaj ekkriis: \"Malfermu karaj infanoj, via patrino alportis ion por ĉiu.\"\n\nSed ĉar la lupo metis sian nigran piedon en la fenestron, la infanoj rekonis ĝin kaj diris: \"Ni ne malfermos, nia patrino ne havas nigrajn piedojn, kiajn vi: vi estas la lupo.\"\n\nLa lupo kuris al bakisto kaj diris: \"Mi doloriĝis mian piedon, ŝmiru ĝin per pasto.\" Poste ĝi kuris al la muelisto kaj diris: \"Ŝutu blankan farunon sur mian piedon.\" La muelisto ektimis kaj blankigis ĝian piedon.\n\nLa fripono iris trian fojon al la pordo kaj diris: \"Malfermu, infanoj, jen revenis via kara patrinjo.\" La kapridoj ekkriis: \"Antaŭe montru vian piedon, ni volas vidi, ĉu vi estas nia kara panjo.\" La lupo metis la piedon en la fenestron — ĝi estis blanka. Ili kredis kaj malfermis la pordon. Sed tiu, kiu eniris, estis la lupo.",
			source: "Elektitaj fabeloj — Fratoj Grimm, trad. Kabe (1906)", rating: 1400, order: 2,
		},
		{
			slug:  "lit-pinokjo-komenco",
			title: "La Aventuroj de Pinokjo — Ĉapitro 1 (Carlo Collodi)",
			text:  "—Ne, geknaboj, vi eraras. Estis iam lignopeco.\n\nĜi ne estis io luksa, sed simpla peco el stako, tia, kian vintre oni metas en la fornon aŭ kamenon por bruligi fajron kaj varmigi la ĉambron.\n\nMi ne scias, kiel okazis, sed fakto estas, ke iun tagon tiu lignopeco venis en la laborejon de maljuna lignaĵisto, kies nomo estis majstro Antono, kvankam ĉiuj nomis lin nur Ĉerizo pro makulo sur la nazopinto, ĉiam brila kaj ruĝa, simile al matura ĉerizo!\n\nKiam majstro Ĉerizo ekvidis la lignopecon, li tre ekĝojis, kontente kunfrotis la manojn, kaj murmuris duonlaŭte:\n\n—Ĉi tiu ligno venis ĝustatempe: mi uzos ĝin por fari el ĝi piedon por tablo.\n\nDirite, farite: senplie li prenis akran adzon por senŝeligi ĝin, sed kiam li ĝuste pretis fari la unuan frapon, lia brako restis en la aero, ĉar li ekaŭdis voĉeton tre mallaŭtetan, kiu diris peteme:\n\n—Ne batu min tiel forte!\n\nVi povas imagi, kia miro frapis tiun bonan maljunulon majstron Ĉerizon.\n\nGape li ĉirkaŭokulis en la ejo por vidi, de kie povis veni la voĉeto, sed neniun li vidis! Li rigardis sub la tablon — neniu; rigardis en la ŝrankon — same neniu; malfermis eĉ la pordon de la laborejo por elrigardi al la strato — ankaŭ neniu!\n\n—Mi jam komprenas, — li diris ridetante, kaj gratis sian perukon, — estas klare: mi ĝin nur imagis. Ni reiru al la laboro.\n\nKaj li reprenis la adzon, donis fortegan frapon sur la lignopecon.\n\n—Oj, vi kaŭzas al mi doloron! — kriis plendeme la sama voĉeto.\n\nĈi-foje la kompatinda majstro Ĉerizo falis sur la plankon, kiel fulmofrapito.",
			source: "La aventuroj de Pinokjo — Carlo Collodi", rating: 1500, order: 3,
		},
		{
			slug:  "lit-rego-macxjo-unua",
			title: "Reĝo Maĉjo Unua (Janusz Korczak)",
			text:  "Maturaj homoj prefere ne legu mian rakonton, ĉar estas en ĝi malkonvenaj ĉapitroj, kiujn ili ne komprenos kaj pro tio primokos. Sed se ili nepre volas, ili legu — al plenkreskuloj oni ja nenion povas malpermesi — ili ne obeas.\n\nLa rakonto komenciĝas tiel: La doktoro diris, ke se la reĝo dum tri tagoj ne resaniĝos, estos tre malbone. Ĉiuj tre ĉagreniĝis, kaj la plej maljuna ministro surmetis okulvitrojn kaj demandis: \"Do kio okazos, se la reĝo ne resaniĝos?\"\n\nLa doktoro ne volis klare diri tion, sed ĉiuj komprenis, ke la reĝo mortos.\n\nLa plej maljuna ministro tre malĝojis kaj invitis al konferenco la aliajn ministrojn. Ili kolektiĝis en granda salono, lokiĝis en komfortaj apogseĝoj ĉe longa tablo. Antaŭ ĉiu ministro kuŝis paperfolio kaj du krajonoj; unu krajono estis ordinara, kaj la dua estis ĉe unu flanko blua, ĉe la alia ruĝa.\n\nLa ministroj ŝlosis la pordon, por ke neniu malhelpu la kunsidon, ili lumigis elektrajn lampojn kaj parolis nenion. Poste la plej maljuna ministro eksonorigis la sonorileton kaj diris: \"Nun ni interkonsiliĝos, kion fari, ĉar la reĝo estas malsana kaj ne plu povas regi.\"\n\n\"Mi pensas,\" diris la militministro, \"ke oni devas venigi la kuraciston, por ke li klare diru, ĉu li povas sanigi lin.\"",
			source: "Bonhumoraj rakontoj — Janusz Korczak", rating: 1650, order: 4,
		},
		{
			slug:  "lit-1984-komenco",
			title: "Mil Naŭcent Okdek Kvar — Komenco (George Orwell)",
			text:  "Estis hela malvarma tago en aprilo, kaj la horloĝoj sonigis la dektrian horon. Winston Smith, kun la mentono premita en la bruston, por eskapi de la akrega vento, rapide puŝis sin tra la vitrajn pordojn de la Loĝejoj de la Venko, kvankam ne sufiĉe rapide por neebligi la eniron kun li de nebuleto de eroplena polvo.\n\nLa koridoro fetoris pro boligitaj brasikoj kaj malnovaj ĉifonaj matoj. Ĉe unu finaĵo kolorita afiŝo, tro granda por endoma montrado, estis najlita al la muro. Ĝi montris nur enorman vizaĝon, larĝan pli ol metron: la vizaĝon de viro eble kvardekkvinjaraĝa, kun dikaj nigraj lipharoj kaj neglataj, sed belaj, trajtoj.\n\nWinston paŝis al la ŝtuparo. Ne utilus provi la lifton. Eĉ dum la plej bonaj periodoj, ĝi malofte funkciis, kaj nuntempe la elektro estis malŝaltita dum la taghoroj. Tio estis parto de la ekonomi-kampanjo, prepare por la Semajno da Malamo.\n\nLa apartamento estis sur la sepa etaĝo, kaj Winston, kiu estis trideknaŭjaraĝa, kaj havis varikan ulceron super sia dekstra maleolo, grimpis malrapide, haltante plurfoje por ripozeti. Ĉe ĉiu placeto, kontraŭ la liftejo, la afiŝo kun la enorma vizaĝo rigardis de la muro. GRANDA FRATO RIGARDAS VIN, diris la vortoj sub la bildo.\n\nEkstere, eĉ tra la fermita fenestroglaco, la mondo aspektis malvarmega. Malsupre, en la strato, etaj kirloventoj spirale kirladis polvon kaj ŝiritajn paperpecojn. La vizaĝo kun nigraj lipharoj rigardis de ĉiu grava angulo. GRANDA FRATO RIGARDAS VIN.",
			source: "Mil Naŭcent Okdek Kvar — George Orwell, trad. Donald Broadribb", rating: 1800, order: 5,
		},
		{
			slug:  "lit-regidino-sur-pizo",
			title: "Reĝidino sur Pizo (H. C. Andersen, trad. L. L. Zamenhof)",
			text:  "Estis iam reĝido, kiu volis edziĝi kun reĝidino, sed li nepre volis, ke tio estu vera reĝidino. Li travojaĝis la tutan mondon, por trovi tian, sed ĉie troviĝis ia kontraŭaĵo. Da reĝidinoj estis sufiĉe multe, sed ĉu tio estas veraj reĝidinoj, pri tio li neniel povis konvinkiĝi; ĉiam troviĝis io, kio ne estis tute konforma. Tial li venis returne hejmen kaj estis tre malĝoja, ĉar li tre deziris havi veran reĝidinon.\n\nUnu vesperon fariĝis granda uragano: fulmis kaj tondris, forte pluvegis, estis terure. Subite oni frapetis je la urba pordego, kaj la maljuna reĝo iris, por malfermi. Montriĝis, ke ekstere antaŭ la pordo staras reĝidino. Sed, ho mia Dio, kiel ŝi aspektis pro la pluvo kaj la ventego! La akvo fluis de ŝiaj haroj kaj vestoj, kaj verŝiĝis en ŝiajn ŝuojn kaj elen. Kaj ŝi diris, ke ŝi estas vera reĝidino.\n\n\"Nu, pri tio ni tre baldaŭ konvinkiĝos!\" pensis la maljuna reĝino. Ŝi tamen nenion diris, sed ŝi iris en la dormoĉambron, elprenis ĉiujn litaĵojn kaj metis unu pizon sur la fundon de la lito. Post tio ŝi prenis dudek matracojn, metis ilin sur la pizon, kaj poste ankoraŭ dudek lanugaĵojn sur la matracojn. En tiu lito la reĝidino devis dormi dum la nokto.\n\nMatene oni ŝin demandis, kiel ŝi dormis.\n\n\"Ho, terure malbone!\" diris la reĝidino; \"preskaŭ dum la tuta nokto mi ne povis fermi la okulojn! Dio scias, kio estis en mia lito! Mi kuŝis sur io malmola, kaj mia korpo pro tio fariĝis blua kaj bruna! Estis terure!\"\n\nPer tio oni povis vidi, ke ŝi estas vera reĝidino, ĉar tra la dudek matracoj kaj la dudek lanugaĵojn ŝi sentis la pizon. Tiel delikatsenta povis esti nur vera reĝidino!\n\nTiam la reĝido edziĝis kun ŝi, ĉar nun li sciis, ke li havas veran reĝidinon; kaj la pizon oni metis en la muzeon, kie oni ankoraŭ nun povas ĝin vidi, se neniu ĝin forprenis.\n\nVidu, tio estis vera historio.",
			source: "Fabeloj — H. C. Andersen, trad. L. L. Zamenhof", rating: 1600, order: 7,
		},
		{
			slug:  "lit-vivo-zamenhof-1",
			title: "Vivo de Zamenhof — Infano en Bjalistoko (Edmond Privat)",
			text:  "De la patrino la koro, de la patro la cerbo, de la loko la impreso: jen la tri ĉefaj elementoj en la formado de Zamenhofa genio.\n\nSur la litva tero kvar gentoj malsamaj loĝis en la urboj, kun celoj kontraŭaj, kun lingvoj diversaj, kun kredoj malamikaj. De strato al strato regis malfido, suspekto, sur placoj ofendo ĉiutaga, venĝemo, persekuto kaj malamo. Sur tiu tero malfeliĉa naskiĝis Zamenhof.\n\nLa knabo ja vidis la faktojn ĉirkaŭ si en stratoj Bjalistokoaj. Sur la vendoplaco moviĝis la popolamaso. Bruladis paŝoj kaj paroloj en zumado laŭta. Briladis koloroj inter korboj kaj legomoj: verdaj ŝaloj de virinoj el la kamparo litva, ŝafaj peltoj, grizaj vestoj de soldatoj. Disputis vendistinoj kun germana marĉandulo. Plendis virinoj en dialekto litva. La policanoj ne komprenis. \"Ruse parolu!\" minacis la oficiro, \"nur ruse, ne lingvaĉe! Ĉi tie estas rusa lando!\" Protestis Polo el la amaso — jen lin kaptis la ĝendarmoj. Silentas la vilaĝanoj.\n\nKion scias tiuj homoj unuj pri la aliaj? Ke ankaŭ ili havas koron, konas ĝojon kaj doloron, amas hejmon kun edzino kaj infanoj? Eĉ penso tia ne okazas. Ekzistas nur Hebreoj, Rusoj, Poloj, Germanoj — ne homoj, sole gentoj. En sia domo ĉiu akceptas nur samgentanojn.\n\nPri tiaj kalumnioj indignis jam knabeto Zamenhof en Bjalistoko. Kion fari, por ke la homoj ne eraru tiel abomene? El tiaj kredoj kaj incitoj rezultas iam veraj katastrofoj.\n\nKvardek jarojn pli poste, en 1906, Zamenhof parolis en Ĝenevo pri la Bjalostoka pogromo: \"Ĉu la plej grandaj mensogoj kaj kalumnioj povus doni tiajn terurajn fruktojn, se la gentoj sin reciproke bone konus, se inter ili ne starus altaj kaj dikaj muroj, kiuj malpermesas al ili libere komunikiĝadi inter si kaj vidi, ke la membroj de aliaj gentoj estas tute tiaj samaj homoj kiel la membroj de nia gento? Rompu, rompu la murojn inter la popoloj!\"",
			source: "Vivo de Zamenhof — Edmond Privat", rating: 1850, order: 8,
		},
		{
			slug:  "lit-zamenhof-kongreso-1905",
			title: "Parolado de Zamenhof — Unua Kongreso (Bulonjo, 1905)",
			text:  "Estimataj sinjorinoj kaj sinjoroj! Mi salutas vin, karaj samideanoj, fratoj kaj fratinoj el la granda tutmonda homa familio, kiuj kunvenis el landoj proksimaj kaj malproksimaj, el la plej diversaj regnoj de la mondo, por frate premi al si reciproke la manojn pro la nomo de granda ideo, kiu ĉiujn nin ligas.\n\nSankta estas por ni la hodiaŭa tago. Modesta estas nia kunveno; la mondo ekstera ne multe scias pri ĝi, kaj la vortoj, kiuj estas parolataj en nia kunveno, ne flugos telegrafe al ĉiuj urboj kaj urbetoj de la mondo; ne kunvenis regnestroj, nek ministroj, por ŝanĝi la politikan karton de la mondo, ne brilas luksaj vestoj kaj multego da imponantaj ordenoj en nia salono; sed tra la aero de nia salono flugas misteraj sonoj, sonoj tre mallaŭtaj, ne aŭdeblaj por la orelo, sed senteblaj por ĉiu animo sentema: ĝi estas la sono de io granda, kio nun naskiĝas.\n\nEn la plej malproksima antikveco la homa familio disiĝis kaj ĝiaj membroj ĉesis kompreni unu la alian. Fratoj kreitaj ĉiuj laŭ unu modelo, fratoj, kiuj havis ĉiuj egalan korpon, egalan spiriton, egalajn kapablojn, egalajn idealojn, egalan Dion en siaj koroj — tiuj fratoj fariĝis tute fremdaj unuj al aliaj, disiĝis ŝajne por ĉiam en malamikajn grupetojn.\n\nKaj nun la unuan fojon la revo de miljaroj komencas realiĝi. En la malgrandan urbon de la franca marbordo kunvenis homoj el la plej diversaj landoj kaj nacioj, kaj ili renkontas sin reciproke ne mute kaj surde, sed ili komprenas unu la alian, ili parolas unu kun la alia kiel fratoj, kiel membroj de unu nacio.\n\nNi konsciu bone la tutan gravecon de la hodiaŭa tago, ĉar hodiaŭ inter la gastamaj muroj de Bulonjo-sur-Maro kunvenis ne francoj kun angloj, ne rusoj kun poloj, sed homoj kun homoj. Benata estu la tago, kaj grandaj kaj gloraj estu ĝiaj sekvoj!",
			source: "Paroladoj — L. L. Zamenhof", rating: 1900, order: 9,
		},
	}
	for _, l := range literary {
		items = append(items, &model.ContentItem{
			Slug: l.slug, Type: "reading",
			Content: map[string]interface{}{
				"title": l.title,
				"text":  l.text,
			},
			Tags: []string{"literatura", "legado"}, Source: l.source, Status: "approved",
			Rating: l.rating, RD: 200, Volatility: 0.06,
			SeriesSlug: "literatura", SeriesOrder: l.order,
		})
	}

	// News articles (C-level reading)
	articles := []struct {
		slug, title, text string
		order             int
	}{
		{
			slug:  "artikolo-filmo-finnlando",
			title: "En Finnlando aperas profesia filmo en Esperanto",
			text:  "La finna filmarto atingis novan signifan mejlŝtonon: la unua profesia duonhora filmo tute en Esperanto. \"Patrinoj\", reĝisorita de Aino Suni, prezentos sin en internaciaj festivaloj komencante en 2025.\n\nLa filmo naskiĝis el hazarda renkontiĝo. Suni, konata pro siaj dokumentaj filmoj pri socia justeco, renkontis Esperantiston en helsinka kafejo dum la pandemio. \"Mi neniam pensis pri Esperanto kiel kreiva medio,\" konfesas Suni. \"Sed ju pli mi lernis, des pli mi komprenis ĝian artistikan potencialon.\"\n\nLa rakonto sekvas tri patrinojn el malsamaj generacioj, kiuj komunikas nur tra Esperanto. La filmo esploras temojn de materineco, migrado, kaj intergeneracia kompreno.\n\nLa filmo produktiĝis per miksobendo: Finnish Film Foundation-subvencio, privata investo, kaj amaskolekata financado el la internacia Esperanto-komunumo. La totala buĝeto atingis 180,000 eŭrojn.",
			order: 1,
		},
		{
			slug:  "artikolo-sveda-vortaro",
			title: "Sveda vortaro migras reten – ĉu aliaj sekvos?",
			text:  "Post preskaŭ naŭ jardekoj da silento, la Esperanto-sveda vortaro denove vekiĝas. La Sveda Esperanto-Federacio anoncis la ciferecigon de sia fundamenta vortaro, kiu laste aperis en la 1930-aj jaroj.\n\nDr. Anders Löfgren, ĉefredaktoro de la projekto, klarigas: \"Ni ne simple ciferecigas malnovan libron. Ni rekonstruas tutan lingvan rilaton por la 21-a jarcento.\"\n\nLa cifereciga procezo alfrontas plurajn obstakojn: fidela konservado, semantika ĝisdatigo, kaj teknologia integrado. La nova vortaro devos funkcii en modernaj retaj platformoj.\n\nMalpli tradicie, la nova sveda vortaro estos komunumo-redaktata. Uzantoj povos proponi novajn terminojn, korekti erarojn, kaj diskuti nuancojn. \"Lingvoj vivas tra siaj parolantoj\", komentas Löfgren.",
			order: 2,
		},
	}
	for _, a := range articles {
		items = append(items, &model.ContentItem{
			Slug: a.slug, Type: "reading",
			Content: map[string]interface{}{
				"title": a.title,
				"text":  a.text,
			},
			Tags: []string{"artikolo", "legado", "altnivela"}, Source: "esperanto-kurso.net", Status: "approved",
			Rating: 1900, RD: 200, Volatility: 0.06,
			SeriesSlug: "artikoloj", SeriesOrder: a.order,
		})
	}

	return items
}
// seedFillinItems returns clean fill-in-blank exercises from La Zagreba Metodo.
// This replaces the flawed entries created by seedContentItems().
func seedFillinItems() []*model.ContentItem {
	var items []*model.ContentItem
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Marko estas en la ___.", "answer": "ĉambro"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "___ sidas sur seĝo.", "answer": "Li"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Instruisto instruas. Laboristo ___. Lernanto ___.", "answers": []string{"laboras", "lernas"}},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Marko kaj Aleksandro estas lernant___kaj sportist___.", "answer": "oj"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "La patrino ___ Marko estas instruistino.", "answer": "de"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi06", Type: "fillin",
		Content: map[string]interface{}{"question": "La seĝo estas en ___ ĉambro.", "answer": "la"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 7,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi07", Type: "fillin",
		Content: map[string]interface{}{"question": "La libro estas de li, ĝi estas li___.", "answer": "a"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 8,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l01-fi08", Type: "fillin",
		Content: map[string]interface{}{"question": "Mia nomo est___ Marko.", "answer": "as"},
		Tags: []string{"ekzerco", "zagr-01", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1000.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l01", SeriesOrder: 9,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l02-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Marko havas amik___ kaj amikin___. Ili estas ___amikoj.", "answers": []string{"on", "on", "ge"}},
		Tags: []string{"ekzerco", "zagr-02", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1050.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l02", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l02-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Li kuiras kaf___ kaj ili trinkas ĝi___ en nia hejmo.", "answers": []string{"on", "n"}},
		Tags: []string{"ekzerco", "zagr-02", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1050.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l02", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l02-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Ili iras ___ la hotelo.", "answer": "al"},
		Tags: []string{"ekzerco", "zagr-02", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1050.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l02", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l02-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Ŝi vidas, ___ Marko amas ŝi___.", "answers": []string{"ke", "n"}},
		Tags: []string{"ekzerco", "zagr-02", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1050.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l02", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l02-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Ili havas facil___lernolibroj___.", "answers": []string{"ajn", "n"}},
		Tags: []string{"ekzerco", "zagr-02", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1050.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l02", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Mia amiko skribas mult___.", "answer": "e"},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Bela kelnerino estas bel___o.", "answer": "ulin"},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Ni havas multa___bela___afero___.", "answer": "jn"},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Ni kun___ lernas en lern___o.", "answers": []string{"e", "ej"}},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Miaj ___patroj manĝas en manĝ___.", "answers": []string{"ge", "ejo"}},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l03-fi06", Type: "fillin",
		Content: map[string]interface{}{"question": "La kuko estas manĝ___a.", "answer": "ebl"},
		Tags: []string{"ekzerco", "zagr-03", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1100.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l03", SeriesOrder: 7,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l04-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Ana kaj Marko iris ___ la strato.", "answer": "tra"},
		Tags: []string{"ekzerco", "zagr-04", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1150.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l04", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l04-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Ili venis ___ kafejo, iris ___ tablo kaj vidis la patro___ sidi ___ seĝo.", "answers": []string{"al", "al", "n", "sur"}},
		Tags: []string{"ekzerco", "zagr-04", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1150.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l04", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l04-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Ana metis tri kukojn sur seĝ___.", "answer": "on"},
		Tags: []string{"ekzerco", "zagr-04", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1150.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l04", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l04-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Ŝi ___metis poste la kukojn sur la tabl___.", "answers": []string{"re", "on"}},
		Tags: []string{"ekzerco", "zagr-04", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1150.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l04", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l05-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Dankon, sinjoro. Nedank___.", "answer": "inde"},
		Tags: []string{"ekzerco", "zagr-05", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1200.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l05", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l05-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Horo estas mallonga, tago estas ___ longa, monato estas ___ ___ longa.", "answers": []string{"pli", "la", "plej"}},
		Tags: []string{"ekzerco", "zagr-05", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1200.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l05", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l05-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Mi legas pli multajn librojn ___ mia amiko.", "answer": "ol"},
		Tags: []string{"ekzerco", "zagr-05", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1200.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l05", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l05-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Mi sendis al mia amiko longajn leterojn, ___ li estis en Tokio.", "answer": "dum"},
		Tags: []string{"ekzerco", "zagr-05", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1200.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l05", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l05-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "La libro estas leg___.", "answer": "inda"},
		Tags: []string{"ekzerco", "zagr-05", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1200.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l05", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "La gepatroj ĉiam deziras, ke mi lern___ mult___.", "answers": []string{"u", "on"}},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Mia amiko deziras, ke mi ir___kant___kun li.", "answers": []string{"u", "i"}},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "La patro volas, ke lia infano manĝ___kukon, ki___estas sur la tablo.", "answer": "u"},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Ni loĝas en malgranda ĉambr___.", "answer": "o"},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Mi far___is maltrankvila, kiam mi vidis vin.", "answer": "iĝ"},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l06-fi06", Type: "fillin",
		Content: map[string]interface{}{"question": "Ĉiuj rimarkis, ke ŝi estas bel___a.", "answer": "eg"},
		Tags: []string{"ekzerco", "zagr-06", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1250.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l06", SeriesOrder: 7,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l07-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Li trinkis rapidege sian matenan trink___ por frue ven___ al la lernejo.", "answers": []string{"aĵon", "i"}},
		Tags: []string{"ekzerco", "zagr-07", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1300.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l07", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l07-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Mi ankoraŭ ne aŭdis nova___.", "answer": "ĵon"},
		Tags: []string{"ekzerco", "zagr-07", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1300.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l07", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l07-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "En tiu momento Marko ___kriis.", "answer": "ek"},
		Tags: []string{"ekzerco", "zagr-07", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1300.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l07", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l07-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Knabo, kiu ne amas aliajn geknabojn, havas ___ amikojn ___ amikinojn.", "answer": "nek"},
		Tags: []string{"ekzerco", "zagr-07", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1300.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l07", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l08-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Ni ord___is ĉion kaj transdonis al ili.", "answer": "ig"},
		Tags: []string{"ekzerco", "zagr-08", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1350.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l08", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l08-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Mi petas glason ___ kafo kaj iom ___ akvo.", "answer": "da"},
		Tags: []string{"ekzerco", "zagr-08", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1350.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l08", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l08-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Doloris lin la kapo. Li malsan___is.", "answer": "iĝ"},
		Tags: []string{"ekzerco", "zagr-08", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1350.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l08", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l08-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "La knabino ŝatas esti bela kaj pro tio ŝi bel___as sian vizaĝon.", "answer": "ig"},
		Tags: []string{"ekzerco", "zagr-08", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1350.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l08", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l09-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Se mi havus tiom da mono kiom da ideoj, mi vojaĝ___us ĉirkaŭ la mondo.", "answer": "ad"},
		Tags: []string{"ekzerco", "zagr-09", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1400.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l09", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l09-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "En ĉiu vort___o mankas kelkaj vortoj.", "answer": "ar"},
		Tags: []string{"ekzerco", "zagr-09", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1400.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l09", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l09-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Homo, kiu longe rakont___as ĉe la manĝotablo, restos malsata.", "answer": "ad"},
		Tags: []string{"ekzerco", "zagr-09", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1400.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l09", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l09-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Li devis atenti kaj ne rajtis longe resti ekstere en pluvo. Tial li malvarm___is.", "answer": "um"},
		Tags: []string{"ekzerco", "zagr-09", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1400.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l09", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l09-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Li havas sian klaran ideon, sed li kredos ĉion, kion vi diros, ___ vi estus lia edzino.", "answer": "kvazaŭ"},
		Tags: []string{"ekzerco", "zagr-09", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1400.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l09", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l10-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "En jun___o oni havas ankaŭ bel___on.", "answer": "ec"},
		Tags: []string{"ekzerco", "zagr-10", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1450.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l10", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l10-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Malsimplajn laborojn ĉefoj devus plisimpl___i.", "answer": "ig"},
		Tags: []string{"ekzerco", "zagr-10", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1450.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l10", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l10-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "En tiu aĝo li jam fariĝis klub___o de sporta klubo.", "answer": "estr"},
		Tags: []string{"ekzerco", "zagr-10", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1450.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l10", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l10-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Monon oni metas en mon___on.", "answer": "uj"},
		Tags: []string{"ekzerco", "zagr-10", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1450.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l10", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l10-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Sekvu la ekzemplon de la bonaj famili___oj.", "answer": "an"},
		Tags: []string{"ekzerco", "zagr-10", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1450.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l10", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l11-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "La knabino ĵetis la paper___ en la paper___.", "answers": []string{"on", "ujon"}},
		Tags: []string{"ekzerco", "zagr-11", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1500.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l11", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l11-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Kie estas mia skrib___, mi volas respondi ___ li.", "answers": []string{"ilo", "al"}},
		Tags: []string{"ekzerco", "zagr-11", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1500.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l11", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l11-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "La suno ___aperis kaj nun estas ___ malhele.", "answers": []string{"mal", "preskaŭ"}},
		Tags: []string{"ekzerco", "zagr-11", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1500.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l11", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l11-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "Bonvolu manĝi la panon ___ manĝ___o.", "answers": []string{"per", "il"}},
		Tags: []string{"ekzerco", "zagr-11", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1500.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l11", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l11-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Pardon___min, mi ne rajtas rest___.", "answers": []string{"u", "i"}},
		Tags: []string{"ekzerco", "zagr-11", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1500.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l11", SeriesOrder: 6,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l12-fi01", Type: "fillin",
		Content: map[string]interface{}{"question": "Antaŭ kvar___a horo ili ___venis.", "answers": []string{"on", "re"}},
		Tags: []string{"ekzerco", "zagr-12", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1550.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l12", SeriesOrder: 2,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l12-fi02", Type: "fillin",
		Content: map[string]interface{}{"question": "Post la terura ___feliĉo la homoj foriris kaj la malnovaj urboj ___falis.", "answers": []string{"mal", "dis"}},
		Tags: []string{"ekzerco", "zagr-12", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1550.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l12", SeriesOrder: 3,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l12-fi03", Type: "fillin",
		Content: map[string]interface{}{"question": "Ni devus ___vidi nin, mia amiko.", "answer": "re"},
		Tags: []string{"ekzerco", "zagr-12", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1550.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l12", SeriesOrder: 4,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l12-fi04", Type: "fillin",
		Content: map[string]interface{}{"question": "___ pli ofte mi pripensas, ___ pli mi koleras.", "answers": []string{"Ju", "des"}},
		Tags: []string{"ekzerco", "zagr-12", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1550.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l12", SeriesOrder: 5,
	})
	items = append(items, &model.ContentItem{
		Slug: "zagr-l12-fi05", Type: "fillin",
		Content: map[string]interface{}{"question": "Ŝi loĝas en iu ___ domo.", "answer": "ajn"},
		Tags: []string{"ekzerco", "zagr-12", "plenigi"},
		Source: "La Zagreba Metodo", Status: "approved",
		Rating: 1550.0, RD: 200, Volatility: 0.06,
		SeriesSlug: "zagr-l12", SeriesOrder: 6,
	})
	return items
}
