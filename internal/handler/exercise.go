package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	localauth "github.com/LaPingvino/esperanto-kurso-gae/internal/auth"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/glicko"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/recommend"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
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

	// Collect all submitted gap answers (multi-gap fillin posts multiple "answer" values).
	rawAnswers := r.Form["answer"]
	answers := make([]string, len(rawAnswers))
	for i, a := range rawAnswers {
		answers[i] = strings.TrimSpace(a)
	}
	answer := strings.Join(answers, " / ") // display string
	correct := checkAnswer(item, answers)

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
	streak, _ := h.users.UpdateStreakAndSeen(r.Context(), u.ID)
	streakDeadline := time.Now().UTC().Truncate(24 * time.Hour).Add(48 * time.Hour).Format("02 Jan 15:04 UTC")

	// Auto-upgrade UI language to Esperanto once user reaches B1 stability.
	if newUserRD < 150 && newUserR >= 1500 && u.UILang != "eo" && u.UILang != "" {
		_ = h.users.UpdateUILang(r.Context(), u.ID, "eo")
	}

	// Recommend next exercises (normal, harder, easier).
	nextSlug, harderSlug, easierSlug := nextSlugs(r.Context(), newUserR, newUserRD, slug, h.content)
	nextInSeries := nextSeriesItem(r.Context(), item, h.content)

	ratingDelta := newUserR - u.Rating
	levelUp := model.RatingToCEFR(newUserR) != model.RatingToCEFR(u.Rating)

	correctAnswer := item.Answer()
	switch item.Type {
	case "fillin":
		correctAnswer = strings.Join(item.GapAnswers(), " / ")
	case "vocab":
		correctAnswer = item.Word()
	}

	data := map[string]interface{}{
		"User":           u,
		"Item":           item,
		"Correct":        correct,
		"CorrectAnswer":  correctAnswer,
		"YourAnswer":     answer,
		"NextSlug":       nextSlug,
		"HarderSlug":     harderSlug,
		"EasierSlug":     easierSlug,
		"NextInSeries":   nextInSeries,
		"UserRating":     newUserR,
		"RatingDelta":    ratingDelta,
		"CEFRLevel":      model.RatingToCEFR(newUserR),
		"LevelUp":        levelUp,
		"StreakDays":     streak,
		"StreakDeadline": streakDeadline,
		"NewToken":       newToken,
		"UILang":         u.UILangOrDefault(),
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

// checkAnswer validates submitted gap answers against the content item.
// answers[i] corresponds to the i-th "___" in the question.
// For single-answer types (multiplechoice, vocab, etc.) only answers[0] is used.
func checkAnswer(item *model.ContentItem, answers []string) bool {
	first := ""
	if len(answers) > 0 {
		first = strings.TrimSpace(answers[0])
	}

	switch item.Type {
	case "multiplechoice":
		opts := item.Options()
		if len(opts) == 0 {
			return false
		}
		correct := item.CorrectIndex()
		if correct >= 0 && correct < len(opts) {
			return strings.EqualFold(first, strings.TrimSpace(opts[correct]))
		}
		return false

	case "fillin":
		gapAnswers := item.GapAnswers()
		if len(gapAnswers) == 0 || len(answers) == 0 {
			return false
		}
		for i, submitted := range answers {
			// Cycle single-answer exercises (same suffix for every gap).
			expected := strings.TrimSpace(gapAnswers[i%len(gapAnswers)])
			if !strings.EqualFold(strings.TrimSpace(submitted), expected) {
				return false
			}
		}
		return true

	case "vocab":
		if strings.EqualFold(first, strings.TrimSpace(item.Word())) {
			return true
		}

	default:
		// listening, image, phrasebook — compare against primary answer.
		if strings.EqualFold(first, strings.TrimSpace(item.Answer())) {
			return true
		}
	}

	// Check community-accepted alternative answers (single-answer types).
	for _, alt := range item.Alternatives() {
		if strings.EqualFold(first, strings.TrimSpace(alt)) {
			return true
		}
	}
	return false
}

// nextSlugs returns the recommended, harder, and easier next exercise slugs.
func nextSlugs(ctx context.Context, userR, userRD float64, currentSlug string, cs *store.ContentStore) (next, harder, easier string) {
	if items, _ := recommend.GetForUser(ctx, userR, userRD, cs, 6); len(items) > 0 {
		for _, it := range items {
			if it.Slug != currentSlug {
				next = it.Slug
				break
			}
		}
	}
	if items, _ := recommend.GetHarder(ctx, userR, cs, 3, currentSlug); len(items) > 0 {
		harder = items[0].Slug
	}
	if items, _ := recommend.GetEasier(ctx, userR, cs, 3, currentSlug); len(items) > 0 {
		easier = items[0].Slug
	}
	return
}

// JudgeExercise handles POST /ekzerco/{slug}/juĝo.
// Used for passive exercises (reading, phrasebook) and the "Mi ne scias" shortcut.
// Accepts form field "judgment": ne-sciis (0.0), malfacile (0.5), bone (1.0), facile (1.0).
func (h *ExerciseHandler) JudgeExercise(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}

	item, err := h.content.GetBySlug(r.Context(), slug)
	if err != nil || item == nil {
		http.NotFound(w, r)
		return
	}

	u := UserFromContext(r.Context())
	newToken := ""
	if u == nil {
		u, newToken, err = h.createAnonymousUser(r.Context())
		if err != nil {
			http.Error(w, "Ne eblis krei uzanton", http.StatusInternalServerError)
			return
		}
	}

	var score float64
	var correct bool
	switch r.FormValue("judgment") {
	case "facile":
		score, correct = 1.0, true
	case "bone":
		score, correct = 1.0, true
	case "malfacile":
		score, correct = 0.5, false
	default: // "ne-sciis"
		score, correct = 0.0, false
	}

	attempt := &model.Attempt{
		UserID:        u.ID,
		ContentItemID: slug,
		Correct:       correct,
		Answer:        r.FormValue("judgment"),
		Timestamp:     time.Now(),
	}
	_ = h.attempts.Create(r.Context(), attempt)

	userResults := []glicko.Result{{OpponentRating: item.Rating, OpponentRD: item.RD, Score: score}}
	newUserR, newUserRD, newUserVol := glicko.Update(u.Rating, u.RD, u.Volatility, userResults)

	contentResults := []glicko.Result{{OpponentRating: u.Rating, OpponentRD: u.RD, Score: 1.0 - score}}
	newContentR, newContentRD, newContentVol := glicko.Update(item.Rating, item.RD, item.Volatility, contentResults)

	_ = h.users.UpdateRating(r.Context(), u.ID, newUserR, newUserRD, newUserVol)
	_ = h.content.UpdateRating(r.Context(), slug, newContentR, newContentRD, newContentVol)
	streak, _ := h.users.UpdateStreakAndSeen(r.Context(), u.ID)
	streakDeadline := time.Now().UTC().Truncate(24 * time.Hour).Add(48 * time.Hour).Format("02 Jan 15:04 UTC")

	// Auto-upgrade UI language to Esperanto once user reaches B1 stability.
	if newUserRD < 150 && newUserR >= 1500 && u.UILang != "eo" && u.UILang != "" {
		_ = h.users.UpdateUILang(r.Context(), u.ID, "eo")
	}

	nextSlug, harderSlug, easierSlug := nextSlugs(r.Context(), newUserR, newUserRD, slug, h.content)
	nextInSeries := nextSeriesItem(r.Context(), item, h.content)
	levelUp := model.RatingToCEFR(newUserR) != model.RatingToCEFR(u.Rating)

	correctAnswer := item.Answer()
	switch item.Type {
	case "fillin":
		correctAnswer = strings.Join(item.GapAnswers(), " / ")
	case "vocab":
		correctAnswer = item.Word()
	}

	data := map[string]interface{}{
		"User":           u,
		"Item":           item,
		"Correct":        correct,
		"Judgment":       r.FormValue("judgment"),
		"CorrectAnswer":  correctAnswer,
		"NextSlug":       nextSlug,
		"HarderSlug":     harderSlug,
		"EasierSlug":     easierSlug,
		"NextInSeries":   nextInSeries,
		"UserRating":     newUserR,
		"RatingDelta":    newUserR - u.Rating,
		"CEFRLevel":      model.RatingToCEFR(newUserR),
		"LevelUp":        levelUp,
		"StreakDays":     streak,
		"StreakDeadline": streakDeadline,
		"NewToken":       newToken,
		"UILang":         u.UILangOrDefault(),
	}
	if newToken != "" {
		w.Header().Set("X-New-Token", newToken)
	}
	if err := h.tmpl.ExecuteTemplate(w, "rezulto.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// nextSeriesItem returns the next item in the series after the given item, or nil.
func nextSeriesItem(ctx context.Context, item *model.ContentItem, cs *store.ContentStore) *model.ContentItem {
	if item.SeriesSlug == "" {
		return nil
	}
	seriesItems, err := cs.ListBySeries(ctx, item.SeriesSlug)
	if err != nil {
		return nil
	}
	for i, si := range seriesItems {
		if si.Slug == item.Slug && i < len(seriesItems)-1 {
			return seriesItems[i+1]
		}
	}
	return nil
}
