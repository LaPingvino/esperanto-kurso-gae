package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/store"
)

// AdminHandler bundles all admin HTTP handlers.
type AdminHandler struct {
	tmpl     Renderer
	content  *store.ContentStore
	comments *store.CommentStore
	users    *store.UserStore
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	users *store.UserStore,
) *AdminHandler {
	return &AdminHandler{
		tmpl:     tmpl,
		content:  content,
		comments: comments,
		users:    users,
	}
}

// Dashboard handles GET /admin.
func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	approved, _ := h.content.ListForAdmin(r.Context(), "approved", 1000)
	pending, _ := h.content.ListForAdmin(r.Context(), "pending", 1000)
	pendingComments, _ := h.comments.ListPending(r.Context(), 100)

	data := map[string]interface{}{
		"User":            u,
		"ApprovedCount":   len(approved),
		"PendingCount":    len(pending),
		"CommentCount":    len(pendingComments),
		"SeedResult":      r.URL.Query().Get("seed"),
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
		"IsNew": true,
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
		"User":  u,
		"Item":  item,
		"IsNew": false,
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

// ModerationQueue handles GET /admin/moderigo.
func (h *AdminHandler) ModerationQueue(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	comments, err := h.comments.ListPending(r.Context(), 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"User":     u,
		"Comments": comments,
	}
	if err := h.tmpl.ExecuteTemplate(w, "moderigo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
		"question":      r.FormValue("question"),
		"answer":        r.FormValue("answer"),
		"hint":          r.FormValue("hint"),
		"audio_url":     r.FormValue("audio_url"),
		"word":          r.FormValue("word"),
		"definition":    r.FormValue("definition"),
		"title":         r.FormValue("title"),
		"text":          r.FormValue("text"),
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
	for _, item := range seedItems() {
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

// seedItems returns the built-in bootstrap dataset of approved exercises.
func seedItems() []*model.ContentItem {
	type si struct {
		slug       string
		typ        string
		content    map[string]interface{}
		tags       []string
		source     string
		rating, rd float64
	}
	items := []si{
		{
			slug: "saluton-mondo", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni diras 'Hello' en Esperanto?",
				"options":       []interface{}{"Saluton", "Dankon", "Bonvolu", "Ĝis"},
				"correct_index": 0,
			},
			tags: []string{"saluto", "baza"}, source: "Baza Esperanto-kurso", rating: 1200, rd: 200,
		},
		{
			slug: "kiel-vi-fartas", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kion signifas 'Kiel vi fartas?'",
				"options":       []interface{}{"Kiel vi fartas?", "Kie vi loĝas?", "Kiam vi venas?", "Kion vi volas?"},
				"correct_index": 0,
			},
			tags: []string{"saluto", "demando", "baza"}, source: "Baza Esperanto-kurso", rating: 1250, rd: 200,
		},
		{
			slug: "mi-parolas-esperante", typ: "fillin",
			content: map[string]interface{}{
				"question": "Plenigi la blankon: Mi ___ Esperanton.",
				"answer":   "parolas",
				"hint":     "La verbo por paroli",
			},
			tags: []string{"gramatiko", "verbo", "baza"}, source: "Baza Esperanto-kurso", rating: 1300, rd: 200,
		},
		{
			slug: "la-domo-estas-granda", typ: "fillin",
			content: map[string]interface{}{
				"question": "Plenigi la blankon: La domo estas ___.",
				"answer":   "granda",
				"hint":     "La kontraŭo de 'malgranda'",
			},
			tags: []string{"adjektivo", "baza"}, source: "Baza Esperanto-kurso", rating: 1350, rd: 200,
		},
		{
			slug: "vorto-akvo", typ: "vocab",
			content: map[string]interface{}{
				"word":       "akvo",
				"definition": "likvaĵo, kiu konsistas el H₂O; baza trinkaĵo",
				"definitions": map[string]interface{}{
					"en": "water",
					"nl": "water",
					"de": "Wasser",
					"fr": "eau",
					"es": "agua",
					"pt": "água",
					"eo": "likvaĵo H₂O",
				},
			},
			tags: []string{"vortaro", "baza", "substantivo"}, source: "PIV", rating: 1200, rd: 200,
		},
		{
			slug: "vorto-lerni", typ: "vocab",
			content: map[string]interface{}{
				"word":       "lerni",
				"definition": "akiri scion aŭ kapablon per studo aŭ praktiko",
				"definitions": map[string]interface{}{
					"en": "to learn",
					"nl": "leren",
					"de": "lernen",
					"fr": "apprendre",
					"es": "aprender",
					"pt": "aprender",
					"eo": "akiri scion per studo",
				},
			},
			tags: []string{"vortaro", "verbo", "baza"}, source: "PIV", rating: 1250, rd: 200,
		},
		{
			slug: "frazo-bonvolu", typ: "phrasebook",
			content: map[string]interface{}{
				"question": "Kiel oni diras «please» en Esperanto?",
				"answer":   "bonvolu",
				"hint":     "Uzata por gentila peto",
			},
			tags: []string{"frazaro", "etiketo", "baza"}, source: "Praktika Esperanto", rating: 1200, rd: 200,
		},
		{
			slug: "frazo-dankon", typ: "phrasebook",
			content: map[string]interface{}{
				"question": "Kiel oni diras «thank you» en Esperanto?",
				"answer":   "dankon",
				"hint":     "Uzata por esprimi dankemon",
			},
			tags: []string{"frazaro", "etiketo", "baza"}, source: "Praktika Esperanto", rating: 1200, rd: 200,
		},
		{
			slug: "legado-la-stelo", typ: "reading",
			content: map[string]interface{}{
				"title":    "La Verda Stelo",
				"text":     "Esperanto estas internacia lingvo. Ĝia simbolo estas verda stelo. La stelo havas kvin pintojn. Ĉiu pinto reprezentas unu kontinenton. La lingvo celas unuigi la homojn de la tuta mondo.",
				"question": "Kiom da pintojn havas la Esperanto-stelo?",
				"answer":   "kvin",
			},
			tags: []string{"legado", "historio", "baza"}, source: "Enkonduko en Esperanton", rating: 1400, rd: 200,
		},
		{
			slug: "legado-zamenhof", typ: "reading",
			content: map[string]interface{}{
				"title":    "Ludoviko Zamenhof",
				"text":     "Ludoviko Lazaro Zamenhof naskiĝis en 1859 en Bjalistoko. Li estis okulisto kaj lingvisto. En 1887 li publikigis la unuan libron de Esperanto sub la pseŭdonimo 'Doktoro Esperanto', kiu signifas 'unu kiu esperas'.",
				"question": "En kiu jaro Zamenhof publikigis la unuan Esperanto-libron?",
				"answer":   "1887",
			},
			tags: []string{"legado", "historio", "meznivelulo"}, source: "Historio de Esperanto", rating: 1500, rd: 200,
		},
	}

	out := make([]*model.ContentItem, len(items))
	for i, s := range items {
		out[i] = &model.ContentItem{
			Slug:       s.slug,
			Type:       s.typ,
			Content:    s.content,
			Tags:       s.tags,
			Source:     s.source,
			AuthorID:   "seed",
			Status:     "approved",
			Rating:     s.rating,
			RD:         s.rd,
			Volatility: 0.06,
			Version:    1,
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
