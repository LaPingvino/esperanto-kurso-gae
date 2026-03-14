package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	localauth "esperanto-kurso-gae/internal/auth"
	"esperanto-kurso-gae/internal/glicko"
	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/recommend"
	"esperanto-kurso-gae/internal/store"
)

// ExerciseHandler handles exercise submission and result rendering.
type ExerciseHandler struct {
	tmpl     Renderer
	content  *store.ContentStore
	users    *store.UserStore
	attempts *store.AttemptStore
}

// NewExerciseHandler creates an ExerciseHandler.
func NewExerciseHandler(
	tmpl Renderer,
	content *store.ContentStore,
	users *store.UserStore,
	attempts *store.AttemptStore,
) *ExerciseHandler {
	return &ExerciseHandler{
		tmpl:     tmpl,
		content:  content,
		users:    users,
		attempts: attempts,
	}
}

// SubmitAttempt handles POST /ekzerco/{slug}/provo.
func (h *ExerciseHandler) SubmitAttempt(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj formularaj datumoj", http.StatusBadRequest)
		return
	}

	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	// Ensure we have an authenticated user; create an anonymous one if not.
	u := UserFromContext(r.Context())
	newToken := ""
	if u == nil {
		u, newToken, err = h.createAnonymousUser(r.Context())
		if err != nil {
			http.Error(w, "Ne eblis krei uzanton", http.StatusInternalServerError)
			return
		}
	}

	answer := strings.TrimSpace(r.FormValue("answer"))
	correct := checkAnswer(item, answer)

	// Record the attempt.
	attempt := &model.Attempt{
		UserID:        u.ID,
		ContentItemID: slug,
		Correct:       correct,
		Answer:        answer,
		TimeMS:        0,
		Timestamp:     time.Now(),
	}
	_ = h.attempts.Create(r.Context(), attempt)

	// Update Glicko-2 ratings.
	score := 0.0
	if correct {
		score = 1.0
	}

	// User's perspective: content is the "opponent".
	userResults := []glicko.Result{{
		OpponentRating: item.Rating,
		OpponentRD:     item.RD,
		Score:          score,
	}}
	newUserR, newUserRD, newUserVol := glicko.Update(u.Rating, u.RD, u.Volatility, userResults)

	// Content's perspective: user is the "opponent", score inverted.
	contentResults := []glicko.Result{{
		OpponentRating: u.Rating,
		OpponentRD:     u.RD,
		Score:          1.0 - score,
	}}
	newContentR, newContentRD, newContentVol := glicko.Update(item.Rating, item.RD, item.Volatility, contentResults)

	// Persist updated ratings.
	_ = h.users.UpdateRating(r.Context(), u.ID, newUserR, newUserRD, newUserVol)
	_ = h.content.UpdateRating(r.Context(), slug, newContentR, newContentRD, newContentVol)

	// Recommend next exercise.
	nextItems, _ := recommend.GetForUser(r.Context(), newUserR, newUserRD, h.content, 5)
	nextSlug := ""
	for _, ni := range nextItems {
		if ni.Slug != slug {
			nextSlug = ni.Slug
			break
		}
	}

	ratingDelta := newUserR - u.Rating

	data := map[string]interface{}{
		"User":          u,
		"Item":          item,
		"Correct":       correct,
		"CorrectAnswer": item.Answer(),
		"YourAnswer":    answer,
		"NextSlug":      nextSlug,
		"UserRating":    newUserR,
		"RatingDelta":   ratingDelta,
		"NewToken":      newToken,
	}

	if newToken != "" {
		w.Header().Set("X-New-Token", newToken)
	}

	if err := h.tmpl.ExecuteTemplate(w, "rezulto.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// createAnonymousUser creates a new user and returns (user, token, error).
func (h *ExerciseHandler) createAnonymousUser(ctx context.Context) (*model.User, string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, "", err
	}
	id := base64.URLEncoding.EncodeToString(b)

	token, err := localauth.GenerateToken()
	if err != nil {
		return nil, "", err
	}
	u := model.NewUser(id, token)
	if err := h.users.Create(ctx, u); err != nil {
		return nil, "", err
	}
	return u, token, nil
}

// checkAnswer validates the submitted answer against the content item.
func checkAnswer(item *model.ContentItem, answer string) bool {
	switch item.Type {
	case "multiplechoice":
		// Answer is the index of the chosen option as a string.
		opts := item.Options()
		if len(opts) == 0 {
			return false
		}
		correct := item.CorrectIndex()
		// Accept the option text itself (case-insensitive) or the index.
		if correct >= 0 && correct < len(opts) {
			if strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(opts[correct])) {
				return true
			}
		}
		return false
	case "vocab":
		return strings.EqualFold(answer, strings.TrimSpace(item.Word()))
	default:
		// fillin, listening, image, reading, phrasebook — compare against answer field.
		return strings.EqualFold(answer, strings.TrimSpace(item.Answer()))
	}
}
