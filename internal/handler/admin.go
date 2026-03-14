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

// seedItems returns the built-in bootstrap dataset (Zagreba Metodo + esperanto-kurso.net).
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
			slug: "zagr-01-nom", typ: "vocab",
			content: map[string]interface{}{
				"word": "nomo",
				"definition": "name",
				"definitions": map[string]interface{}{"en": "name", "nl": "naam", "de": "Name", "fr": "nom", "es": "nombre", "pt": "nome, nomear"},
			},
			tags:       []string{"vortaro", "zagr-01"},
			source:     "La Zagreba Metodo, Leciono 1: Amiko Marko",
			rating: 1050, rd: 200,
			seriesSlug:  "zagr-01",
			seriesOrder: 7,
		},
		{
			slug: "zagr-01-respond", typ: "vocab",
			content: map[string]interface{}{
				"word": "respondi",
				"definition": "to answer",
				"definitions": map[string]interface{}{"en": "to answer", "nl": "antwoorden, beantwoorden", "de": "antworten", "fr": "répondre", "es": "responder", "pt": "responder, resposta"},
			},
			tags:       []string{"vortaro", "zagr-01"},
			source:     "La Zagreba Metodo, Leciono 1: Amiko Marko",
			rating: 1050, rd: 200,
			seriesSlug:  "zagr-01",
			seriesOrder: 8,
		},
		{
			slug: "zagr-02-demand", typ: "vocab",
			content: map[string]interface{}{
				"word": "demandi",
				"definition": "to ask",
				"definitions": map[string]interface{}{"en": "to ask", "nl": "vragen", "de": "fragen", "fr": "demander", "es": "preguntar", "pt": "perguntar"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 1,
		},
		{
			slug: "zagr-02-dom", typ: "vocab",
			content: map[string]interface{}{
				"word": "domo",
				"definition": "house",
				"definitions": map[string]interface{}{"en": "house", "nl": "huis", "de": "Haus", "fr": "maison", "es": "casa", "pt": "casa"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 2,
		},
		{
			slug: "zagr-02-facil", typ: "vocab",
			content: map[string]interface{}{
				"word": "facila",
				"definition": "easy",
				"definitions": map[string]interface{}{"en": "easy", "nl": "gemakkelijk", "de": "leicht", "fr": "facile", "es": "fácil", "pt": "fácil"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 3,
		},
		{
			slug: "zagr-02-grand", typ: "vocab",
			content: map[string]interface{}{
				"word": "granda",
				"definition": "big, great",
				"definitions": map[string]interface{}{"en": "big, great", "nl": "groot", "de": "groß", "fr": "grand", "es": "grande, gran", "pt": "grande"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 4,
		},
		{
			slug: "zagr-02-mon", typ: "vocab",
			content: map[string]interface{}{
				"word": "mono",
				"definition": "money",
				"definitions": map[string]interface{}{"en": "money", "nl": "geld", "de": "Geld", "fr": "argent, monnaie", "es": "dinero", "pt": "dinheiro"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 6,
		},
		{
			slug: "zagr-02-nov", typ: "vocab",
			content: map[string]interface{}{
				"word": "nova",
				"definition": "new",
				"definitions": map[string]interface{}{"en": "new", "nl": "nieuw", "de": "neu", "fr": "nouveau", "es": "nuevo", "pt": "novo, nova"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 7,
		},
		{
			slug: "zagr-02-dir", typ: "vocab",
			content: map[string]interface{}{
				"word": "diri",
				"definition": "to say",
				"definitions": map[string]interface{}{"en": "to say", "nl": "zeggen", "de": "sagen", "fr": "dire", "es": "decir", "pt": "dizer"},
			},
			tags:       []string{"vortaro", "zagr-02"},
			source:     "La Zagreba Metodo, Leciono 2: La amikino de Marko",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 8,
		},
		{
			slug: "zagr-03-afer", typ: "vocab",
			content: map[string]interface{}{
				"word": "afero",
				"definition": "matter",
				"definitions": map[string]interface{}{"en": "matter", "nl": "aangelegenheid, zaak", "de": "Angelegenheit, Sache", "fr": "affaire", "es": "asunto, cosa", "pt": "assunto, coisa"},
			},
			tags:       []string{"vortaro", "zagr-03"},
			source:     "La Zagreba Metodo, Leciono 3: En kafejo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-03",
			seriesOrder: 1,
		},
		{
			slug: "zagr-03-sci", typ: "vocab",
			content: map[string]interface{}{
				"word": "scii",
				"definition": "to know",
				"definitions": map[string]interface{}{"en": "to know", "nl": "weten", "de": "wissen", "fr": "savoir", "es": "saber", "pt": "saber, conhecimento"},
			},
			tags:       []string{"vortaro", "zagr-03"},
			source:     "La Zagreba Metodo, Leciono 3: En kafejo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-03",
			seriesOrder: 4,
		},
		{
			slug: "zagr-03-rapid", typ: "vocab",
			content: map[string]interface{}{
				"word": "rapida",
				"definition": "fast",
				"definitions": map[string]interface{}{"en": "fast", "nl": "snel, rap", "de": "schnell", "fr": "rapide", "es": "rápido/a", "pt": "rápido, rápida"},
			},
			tags:       []string{"vortaro", "zagr-03"},
			source:     "La Zagreba Metodo, Leciono 3: En kafejo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-03",
			seriesOrder: 5,
		},
		{
			slug: "zagr-04-auxd", typ: "vocab",
			content: map[string]interface{}{
				"word": "aŭdi",
				"definition": "to hear",
				"definitions": map[string]interface{}{"en": "to hear", "nl": "horen", "de": "hören", "fr": "entendre", "es": "oír", "pt": "ouvir"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 1,
		},
		{
			slug: "zagr-04-kap", typ: "vocab",
			content: map[string]interface{}{
				"word": "kapo",
				"definition": "head",
				"definitions": map[string]interface{}{"en": "head", "nl": "kop", "de": "Kopf", "fr": "tête", "es": "cabeza", "pt": "cabeça"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 2,
		},
		{
			slug: "zagr-04-met", typ: "vocab",
			content: map[string]interface{}{
				"word": "meti",
				"definition": "to put",
				"definitions": map[string]interface{}{"en": "to put", "nl": "zetten, plaatsen, leggen", "de": "setzen, stellen, legen", "fr": "mettre", "es": "poner, meter", "pt": "colocar, pôr"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 3,
		},
		{
			slug: "zagr-04-pied", typ: "vocab",
			content: map[string]interface{}{
				"word": "piedo",
				"definition": "foot",
				"definitions": map[string]interface{}{"en": "foot", "nl": "voet", "de": "Fuß", "fr": "pied", "es": "pie", "pt": "pé"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 4,
		},
		{
			slug: "zagr-04-pren", typ: "vocab",
			content: map[string]interface{}{
				"word": "preni",
				"definition": "to take",
				"definitions": map[string]interface{}{"en": "to take", "nl": "nemen", "de": "nehmen", "fr": "prendre", "es": "coger", "pt": "pegar"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 5,
		},
		{
			slug: "zagr-04-trankvil", typ: "vocab",
			content: map[string]interface{}{
				"word": "trankvila",
				"definition": "calm",
				"definitions": map[string]interface{}{"en": "calm", "nl": "rustig", "de": "ruhig", "fr": "calme", "es": "tranquilo/a", "pt": "calmo, calma"},
			},
			tags:       []string{"vortaro", "zagr-04"},
			source:     "La Zagreba Metodo, Leciono 4: Miaj leteroj",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-04",
			seriesOrder: 6,
		},
		{
			slug: "zagr-05-help", typ: "vocab",
			content: map[string]interface{}{
				"word": "helpi",
				"definition": "to help",
				"definitions": map[string]interface{}{"en": "to help", "nl": "helpen", "de": "helfen", "fr": "aider", "es": "ayudar", "pt": "ajudar, ajuda"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 2,
		},
		{
			slug: "zagr-05-hor", typ: "vocab",
			content: map[string]interface{}{
				"word": "horo",
				"definition": "hour",
				"definitions": map[string]interface{}{"en": "hour", "nl": "uur", "de": "Stunde", "fr": "heure", "es": "horo", "pt": "hora"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 3,
		},
		{
			slug: "zagr-05-jar", typ: "vocab",
			content: map[string]interface{}{
				"word": "jaro",
				"definition": "year",
				"definitions": map[string]interface{}{"en": "year", "nl": "jaar", "de": "Jahr", "fr": "année", "es": "año", "pt": "ano, anual"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 4,
		},
		{
			slug: "zagr-05-komenc", typ: "vocab",
			content: map[string]interface{}{
				"word": "komenci",
				"definition": "to begin",
				"definitions": map[string]interface{}{"en": "to begin", "nl": "beginnen", "de": "anfangen", "fr": "commencer", "es": "empezar", "pt": "começar, começo"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 5,
		},
		{
			slug: "zagr-05-temp", typ: "vocab",
			content: map[string]interface{}{
				"word": "tempo",
				"definition": "time",
				"definitions": map[string]interface{}{"en": "time", "nl": "tijd", "de": "Zeit", "fr": "temps", "es": "tiempo", "pt": "tempo"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 7,
		},
		{
			slug: "zagr-05-viv", typ: "vocab",
			content: map[string]interface{}{
				"word": "vivi",
				"definition": "to live",
				"definitions": map[string]interface{}{"en": "to live", "nl": "leven", "de": "leben", "fr": "vivre", "es": "vivir", "pt": "viver, vida"},
			},
			tags:       []string{"vortaro", "zagr-05"},
			source:     "La Zagreba Metodo, Leciono 5: Nova aŭto",
			rating: 1250, rd: 200,
			seriesSlug:  "zagr-05",
			seriesOrder: 8,
		},
		{
			slug: "zagr-06-bezon", typ: "vocab",
			content: map[string]interface{}{
				"word": "bezoni",
				"definition": "to need",
				"definitions": map[string]interface{}{"en": "to need", "nl": "nodig hebben, benodigen", "de": "brauchen, benötigen", "fr": "avoir besoin", "es": "necesitar", "pt": "precisar"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 1,
		},
		{
			slug: "zagr-06-esper", typ: "vocab",
			content: map[string]interface{}{
				"word": "esperi",
				"definition": "to hope",
				"definitions": map[string]interface{}{"en": "to hope", "nl": "hopen", "de": "hoffen", "fr": "espérer", "es": "esperar (esperanza)", "pt": "ter esperança"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 2,
		},
		{
			slug: "zagr-06-famili", typ: "vocab",
			content: map[string]interface{}{
				"word": "familio",
				"definition": "family",
				"definitions": map[string]interface{}{"en": "family", "nl": "gezin (familie)", "de": "Familie", "fr": "famille", "es": "familiar", "pt": "família"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 3,
		},
		{
			slug: "zagr-06-land", typ: "vocab",
			content: map[string]interface{}{
				"word": "lando",
				"definition": "land",
				"definitions": map[string]interface{}{"en": "land", "nl": "land", "de": "Land", "fr": "pays", "es": "país, territorio", "pt": "terra"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 5,
		},
		{
			slug: "zagr-06-pens", typ: "vocab",
			content: map[string]interface{}{
				"word": "pensi",
				"definition": "to think",
				"definitions": map[string]interface{}{"en": "to think", "nl": "denken", "de": "denken", "fr": "penser", "es": "pensar", "pt": "pensar, pensamento"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 7,
		},
		{
			slug: "zagr-06-util", typ: "vocab",
			content: map[string]interface{}{
				"word": "utila",
				"definition": "useful",
				"definitions": map[string]interface{}{"en": "useful", "nl": "nuttig", "de": "nützlich", "fr": "utile", "es": "útil", "pt": "útil"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 8,
		},
		{
			slug: "zagr-06-zorg", typ: "vocab",
			content: map[string]interface{}{
				"word": "zorgi",
				"definition": "to care",
				"definitions": map[string]interface{}{"en": "to care", "nl": "zorgen", "de": "sorgen", "fr": "se soucier", "es": "preocuparse, encargarse", "pt": "cuidado, cuidar"},
			},
			tags:       []string{"vortaro", "zagr-06"},
			source:     "La Zagreba Metodo, Leciono 6: Maja",
			rating: 1300, rd: 200,
			seriesSlug:  "zagr-06",
			seriesOrder: 9,
		},
		{
			slug: "zagr-07-akv", typ: "vocab",
			content: map[string]interface{}{
				"word": "akvo",
				"definition": "water",
				"definitions": map[string]interface{}{"en": "water", "nl": "water", "de": "Wasser", "fr": "eau", "es": "agua", "pt": "água"},
			},
			tags:       []string{"vortaro", "zagr-07"},
			source:     "La Zagreba Metodo, Leciono 7: Ĉiam malfrue",
			rating: 1350, rd: 200,
			seriesSlug:  "zagr-07",
			seriesOrder: 1,
		},
		{
			slug: "zagr-07-atend", typ: "vocab",
			content: map[string]interface{}{
				"word": "atendi",
				"definition": "to wait",
				"definitions": map[string]interface{}{"en": "to wait", "nl": "wachten", "de": "warten", "fr": "attendre", "es": "esperar", "pt": "esperar"},
			},
			tags:       []string{"vortaro", "zagr-07"},
			source:     "La Zagreba Metodo, Leciono 7: Ĉiam malfrue",
			rating: 1350, rd: 200,
			seriesSlug:  "zagr-07",
			seriesOrder: 2,
		},
		{
			slug: "zagr-07-nokt", typ: "vocab",
			content: map[string]interface{}{
				"word": "nokto",
				"definition": "night",
				"definitions": map[string]interface{}{"en": "night", "nl": "nacht", "de": "Nacht", "fr": "nuit", "es": "noche", "pt": "noite"},
			},
			tags:       []string{"vortaro", "zagr-07"},
			source:     "La Zagreba Metodo, Leciono 7: Ĉiam malfrue",
			rating: 1350, rd: 200,
			seriesSlug:  "zagr-07",
			seriesOrder: 3,
		},
		{
			slug: "zagr-07-rid", typ: "vocab",
			content: map[string]interface{}{
				"word": "ridi",
				"definition": "to laugh",
				"definitions": map[string]interface{}{"en": "to laugh", "nl": "lachen", "de": "lachen", "fr": "rire", "es": "reír", "pt": "rir"},
			},
			tags:       []string{"vortaro", "zagr-07"},
			source:     "La Zagreba Metodo, Leciono 7: Ĉiam malfrue",
			rating: 1350, rd: 200,
			seriesSlug:  "zagr-07",
			seriesOrder: 4,
		},
		{
			slug: "zagr-08-decid", typ: "vocab",
			content: map[string]interface{}{
				"word": "decidi",
				"definition": "to decide",
				"definitions": map[string]interface{}{"en": "to decide", "nl": "beslissen", "de": "entscheiden", "fr": "décider", "es": "decidir", "pt": "decidir"},
			},
			tags:       []string{"vortaro", "zagr-08"},
			source:     "La Zagreba Metodo, Leciono 8: Malbela tago",
			rating: 1400, rd: 200,
			seriesSlug:  "zagr-08",
			seriesOrder: 2,
		},
		{
			slug: "zagr-08-mort", typ: "vocab",
			content: map[string]interface{}{
				"word": "morti",
				"definition": "to die",
				"definitions": map[string]interface{}{"en": "to die", "nl": "sterven, dood", "de": "sterben", "fr": "mourir", "es": "morir", "pt": "morrer, morte"},
			},
			tags:       []string{"vortaro", "zagr-08"},
			source:     "La Zagreba Metodo, Leciono 8: Malbela tago",
			rating: 1400, rd: 200,
			seriesSlug:  "zagr-08",
			seriesOrder: 3,
		},
		{
			slug: "zagr-08-tim", typ: "vocab",
			content: map[string]interface{}{
				"word": "timi",
				"definition": "to fear",
				"definitions": map[string]interface{}{"en": "to fear", "nl": "vrezen, bang zijn", "de": "fürchten", "fr": "crainte", "es": "temer, tener miedo", "pt": "medo, ter medo"},
			},
			tags:       []string{"vortaro", "zagr-08"},
			source:     "La Zagreba Metodo, Leciono 8: Malbela tago",
			rating: 1400, rd: 200,
			seriesSlug:  "zagr-08",
			seriesOrder: 5,
		},
		{
			slug: "zagr-09-plan", typ: "vocab",
			content: map[string]interface{}{
				"word": "plani",
				"definition": "to plan",
				"definitions": map[string]interface{}{"en": "to plan", "nl": "plan", "de": "Plan", "fr": "planifier", "es": "plan", "pt": "planejar"},
			},
			tags:       []string{"vortaro", "zagr-09"},
			source:     "La Zagreba Metodo, Leciono 9: Planoj pri veturado",
			rating: 1450, rd: 200,
			seriesSlug:  "zagr-09",
			seriesOrder: 3,
		},
		{
			slug: "zagr-09-voj", typ: "vocab",
			content: map[string]interface{}{
				"word": "vojo",
				"definition": "way",
				"definitions": map[string]interface{}{"en": "way", "nl": "weg, baan", "de": "Weg", "fr": "voie", "es": "camino", "pt": "caminho"},
			},
			tags:       []string{"vortaro", "zagr-09"},
			source:     "La Zagreba Metodo, Leciono 9: Planoj pri veturado",
			rating: 1450, rd: 200,
			seriesSlug:  "zagr-09",
			seriesOrder: 4,
		},
		{
			slug: "zagr-01-fill-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Marko estas en la ___.",
				"answer":   "ĉambro",
				"hint":     "La loko kie Marko sidas",
			},
			tags:       []string{"plenigi", "gramatiko", "zagr-01"},
			source:     "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-01",
			seriesOrder: 20,
		},
		{
			slug: "zagr-01-fill-02", typ: "fillin",
			content: map[string]interface{}{
				"question": "Li sidas sur ___.",
				"answer":   "seĝo",
				"hint":     "Meblaro por sidi",
			},
			tags:       []string{"plenigi", "gramatiko", "zagr-01"},
			source:     "La Zagreba Metodo",
			rating: 1100, rd: 200,
			seriesSlug:  "zagr-01",
			seriesOrder: 21,
		},
		{
			slug: "zagr-01-fill-03", typ: "fillin",
			content: map[string]interface{}{
				"question": "Instruisto instruas. Laboristo ___.",
				"answer":   "laboras",
				"hint":     "La verbo labori",
			},
			tags:       []string{"plenigi", "gramatiko", "zagr-01"},
			source:     "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-01",
			seriesOrder: 22,
		},
		{
			slug: "zagr-02-fill-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Ili iras ___ la hotelo.",
				"answer":   "al",
				"hint":     "Direkta prepozicio",
			},
			tags:       []string{"plenigi", "gramatiko", "zagr-02"},
			source:     "La Zagreba Metodo",
			rating: 1150, rd: 200,
			seriesSlug:  "zagr-02",
			seriesOrder: 20,
		},
		{
			slug: "zagr-03-fill-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Mia amiko skribas ___.",
				"answer":   "multe",
				"hint":     "Adverba formo de multaj",
			},
			tags:       []string{"plenigi", "gramatiko", "zagr-03"},
			source:     "La Zagreba Metodo",
			rating: 1200, rd: 200,
			seriesSlug:  "zagr-03",
			seriesOrder: 20,
		},
		{
			slug: "saluton-mc-01", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni salutas en Esperanto?",
				"options":       []interface{}{"Saluton!", "Bona tago!", "Ĝis!", "Dankon!"},
				"correct_index": 0,
				"hint":          "Ĝenerala saluto",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 1,
		},
		{
			slug: "saluton-mc-02", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni diras bonan matenon esperante?",
				"options":       []interface{}{"Bonan matenon!", "Bonan tagon!", "Bonan nokton!", "Bonan vesperon!"},
				"correct_index": 0,
				"hint":          "Matena saluto",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 2,
		},
		{
			slug: "saluton-mc-03", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni diras adiauxon en Esperanto?",
				"options":       []interface{}{"Adiaŭ!", "Saluton!", "Dankon!", "Bonvolu!"},
				"correct_index": 0,
				"hint":          "Forira saluto",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 3,
		},
		{
			slug: "saluton-mc-04", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kion signifas la demando Kiel vi fartas?",
				"options":       []interface{}{"Kiel vi estas?", "Kiu vi estas?", "Kie vi estas?", "Kion vi volas?"},
				"correct_index": 0,
				"hint":          "Demando pri sano",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 4,
		},
		{
			slug: "saluton-mc-05", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni esprimas dankon esperante?",
				"options":       []interface{}{"Dankon!", "Bonvolu!", "Pardonu!", "Saluton!"},
				"correct_index": 0,
				"hint":          "Dankesprimo",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 5,
		},
		{
			slug: "saluton-mc-06", typ: "multiplechoice",
			content: map[string]interface{}{
				"question":      "Kiel oni petas ion gentile esperante?",
				"options":       []interface{}{"Bonvolu!", "Dankon!", "Pardonu!", "Jes!"},
				"correct_index": 0,
				"hint":          "Gentila peto",
			},
			tags:       []string{"saluto", "plurelekta", "baza"},
			source:     "esperanto-kurso.net, Salutado kaj Adiauxado",
			rating: 1100, rd: 200,
			seriesSlug:  "salutado",
			seriesOrder: 6,
		},
		{
			slug: "gram-substantivo-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel finiĝas substantivoj en Esperanto?",
				"answer":   "-o",
				"hint":     "Ekz: libro, tablo, domo",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1150, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 1,
		},
		{
			slug: "gram-pluralo-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel fariĝas pluralo de substantivo?",
				"answer":   "-oj",
				"hint":     "Ekz: libroj, tabloj, domoj",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1200, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 2,
		},
		{
			slug: "gram-adjektivo-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel finiĝas adjektivoj en Esperanto?",
				"answer":   "-a",
				"hint":     "Ekz: granda, bela, nova",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1200, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 3,
		},
		{
			slug: "gram-verbo-as-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel finiĝas verboj en prezenco?",
				"answer":   "-as",
				"hint":     "Ekz: estas, havas, iras",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1200, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 4,
		},
		{
			slug: "gram-verbo-is-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel finiĝas verboj en pasinteco?",
				"answer":   "-is",
				"hint":     "Ekz: estis, havis, iris",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1250, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 5,
		},
		{
			slug: "gram-verbo-os-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel finiĝas verboj en futuro?",
				"answer":   "-os",
				"hint":     "Ekz: estos, havos, iros",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1250, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 6,
		},
		{
			slug: "gram-akuzativo-01", typ: "fillin",
			content: map[string]interface{}{
				"question": "Kiel fariĝas la rekta objekto en Esperanto?",
				"answer":   "-n",
				"hint":     "Ekz: Mi vidas libroN, Li amas ŝiN",
			},
			tags:       []string{"gramatiko", "plenigi", "baza"},
			source:     "esperanto-kurso.net",
			rating: 1300, rd: 200,
			seriesSlug:  "gramatiko-baza",
			seriesOrder: 7,
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
