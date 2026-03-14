package handler

import (
	"net/http"

	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/recommend"
	"esperanto-kurso-gae/internal/store"
)

// ContentHandler handles exercise display and the home page.
type ContentHandler struct {
	tmpl     Renderer
	content  *store.ContentStore
	comments *store.CommentStore
	votes    *store.VoteStore
}

// NewContentHandler creates a ContentHandler.
func NewContentHandler(
	tmpl Renderer,
	content *store.ContentStore,
	comments *store.CommentStore,
	votes *store.VoteStore,
) *ContentHandler {
	return &ContentHandler{
		tmpl:     tmpl,
		content:  content,
		comments: comments,
		votes:    votes,
	}
}

// ShowHome handles GET /.
func (h *ContentHandler) ShowHome(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	var rating, rd float64 = 1500, 350
	if u != nil {
		rating = u.Rating
		rd = u.RD
	}

	items, err := recommend.GetForUser(r.Context(), rating, rd, h.content, 10)
	if err != nil {
		// Fall back to any approved items.
		items, err = h.content.ListApproved(r.Context(), 10)
		if err != nil {
			http.Error(w, "Eraro ĉe ŝarĝado de ekzercoj", http.StatusInternalServerError)
			return
		}
	}

	data := map[string]interface{}{
		"User":  u,
		"Items": items,
	}
	if err := h.tmpl.ExecuteTemplate(w, "hejmo.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// ShowVortaro handles GET /vortaro — lists all vocab items as a dictionary.
func (h *ContentHandler) ShowVortaro(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	items, err := h.content.ListByType(r.Context(), "vocab", 500)
	if err != nil {
		items, _ = h.content.ListApproved(r.Context(), 500)
		var vocabOnly []*model.ContentItem
		for _, it := range items {
			if it.Type == "vocab" {
				vocabOnly = append(vocabOnly, it)
			}
		}
		items = vocabOnly
	}
	data := map[string]interface{}{
		"User":  u,
		"Items": items,
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

	var currentVote *model.Vote
	if u != nil {
		currentVote, _ = h.votes.GetByUserAndContent(r.Context(), u.ID, slug)
	}

	data := map[string]interface{}{
		"User":        u,
		"Item":        item,
		"Comments":    comments,
		"CurrentVote": currentVote,
	}
	if err := h.tmpl.ExecuteTemplate(w, "ekzerco.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
