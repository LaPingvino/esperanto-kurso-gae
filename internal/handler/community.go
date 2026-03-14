package handler

import (
	"math"
	"net/http"

	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/store"
)

// CommunityHandler handles voting and commenting.
type CommunityHandler struct {
	tmpl     Renderer
	content  *store.ContentStore
	votes    *store.VoteStore
	comments *store.CommentStore
}

// NewCommunityHandler creates a CommunityHandler.
func NewCommunityHandler(
	tmpl Renderer,
	content *store.ContentStore,
	votes *store.VoteStore,
	comments *store.CommentStore,
) *CommunityHandler {
	return &CommunityHandler{
		tmpl:     tmpl,
		content:  content,
		votes:    votes,
		comments: comments,
	}
}

// Vote handles POST /vochdonado/{contentID}.
func (h *CommunityHandler) Vote(w http.ResponseWriter, r *http.Request) {
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
	}
	if err := h.tmpl.ExecuteTemplate(w, "vochdonado.html", data); err != nil {
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
	}
	if err := h.comments.Create(r.Context(), comment); err != nil {
		http.Error(w, "Ne eblis konservi komenton", http.StatusInternalServerError)
		return
	}

	comments, _ := h.comments.ListApprovedByContent(r.Context(), contentID)
	if autoApprove {
		comments = append(comments, comment)
	}

	data := map[string]interface{}{
		"ContentID": contentID,
		"Comments":  comments,
		"User":      u,
	}
	if err := h.tmpl.ExecuteTemplate(w, "komentoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
