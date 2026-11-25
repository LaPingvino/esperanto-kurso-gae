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

// isStreakMilestone reports whether a streak length deserves a celebration:
// early wins (3, 7, 14 days), then round marks up to every 100 days.
func isStreakMilestone(days int) bool {
	switch days {
	case 3, 7, 14, 30, 50, 365:
		return true
	}
	return days > 0 && days%100 == 0
}

// ExerciseHandler handles exercise submission and result rendering.
type ExerciseHandler struct {
	tmpl     Renderer
	content  *store.ContentStore
	users    *store.UserStore
	attempts *store.AttemptStore
	votes    *store.VoteStore
}

// NewExerciseHandler creates an ExerciseHandler.
func NewExerciseHandler(
	tmpl Renderer,
	content *store.ContentStore,
	users *store.UserStore,
	attempts *store.AttemptStore,
	votes *store.VoteStore,
) *ExerciseHandler {
	return &ExerciseHandler{
		tmpl:     tmpl,
		content:  content,
		users:    users,
		attempts: attempts,
		votes:    votes,
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

	score := 0.0
	if correct {
		score = 1.0
	}
	h.verdict(w, r, item, u, newToken, score, correct, answer, "")
}

// rate records an answer (or a self-judgement) on an item: the attempt,
// both Glicko-2 ratings and the streak. It returns the learner's new
// rating, the streak and whether it reached a milestone just now.
func (h *ExerciseHandler) rate(ctx context.Context, u *model.User, item *model.ContentItem, score float64, correct bool, recorded string) (float64, float64, int, bool) {
	slug := item.Slug
	_ = h.attempts.Create(ctx, &model.Attempt{
		UserID:        u.ID,
		ContentItemID: slug,
		Correct:       correct,
		Answer:        recorded,
		Timestamp:     time.Now(),
	})

	// The user's opponent is the content, and the content's the user (score inverted).
	newUserR, newUserRD, newUserVol := glicko.Update(u.Rating, u.RD, u.Volatility,
		[]glicko.Result{{OpponentRating: item.Rating, OpponentRD: item.RD, Score: score}})
	newContentR, newContentRD, newContentVol := glicko.Update(item.Rating, item.RD, item.Volatility,
		[]glicko.Result{{OpponentRating: u.Rating, OpponentRD: u.RD, Score: 1.0 - score}})
	_ = h.users.UpdateRating(ctx, u.ID, newUserR, newUserRD, newUserVol)
	_ = h.content.UpdateRating(ctx, slug, newContentR, newContentRD, newContentVol)

	// Milestones celebrate only the attempt that extends the streak, so check
	// the pre-update practice day before it is overwritten below.
	now := time.Now().UTC()
	firstToday := u.LastPracticeAt.UTC().Truncate(24 * time.Hour).Before(now.Truncate(24 * time.Hour))
	streak, _ := h.users.UpdateStreakAndSeen(ctx, u.ID)
	u.StreakDays = streak
	u.LastSeenAt = now
	u.LastPracticeAt = now

	return newUserR, newUserRD, streak, firstToday && isStreakMilestone(streak)
}

// verdict records an answer or a self-judgement, updates both Glicko-2
// ratings and the streak, and renders the verdict into the stage's dock.
func (h *ExerciseHandler) verdict(w http.ResponseWriter, r *http.Request, item *model.ContentItem, u *model.User, newToken string, score float64, correct bool, answer, judgment string) {
	ctx := r.Context()
	slug := item.Slug
	recorded := answer
	if judgment != "" {
		recorded, answer = judgment, "" // a self-judgement is no answer to show
	}
	newUserR, newUserRD, streak, milestone := h.rate(ctx, u, item, score, correct, recorded)
	after := &model.User{Rating: newUserR, RD: newUserRD}
	// The interface switches itself to Esperanto once the learner is established.
	if after.Established() && u.UILang != "eo" && u.UILang != "" {
		_ = h.users.UpdateUILang(ctx, u.ID, "eo")
	}

	nextSlug, harderSlug, easierSlug := nextSlugs(ctx, newUserR, newUserRD, slug, h.content)
	nextInSeries := nextSeriesItem(ctx, item, h.content)
	levelUp := model.RatingToCEFR(newUserR) != model.RatingToCEFR(u.Rating) && newUserR > u.Rating && after.Placed()

	// Daŭrigi: on through the series, else the recommendation, else home. The
	// step that reaches B1 leads to the crossroads with angla.org, once.
	cont := "/"
	switch {
	case levelUp && after.ReachedB1() && !u.ReachedB1():
		cont = "/vojkrucigxo"
	case nextInSeries != nil:
		cont = "/ekzerco/" + nextInSeries.Slug
	case nextSlug != "":
		cont = "/ekzerco/" + nextSlug
	}

	correctAnswer := item.Answer()
	switch item.Type {
	case "fillin":
		correctAnswer = strings.Join(item.GapAnswers(), " / ")
	case "vocab":
		correctAnswer = item.Word()
	case "multiplechoice":
		if opts, i := item.Options(), item.CorrectIndex(); i >= 0 && i < len(opts) {
			correctAnswer = opts[i]
		}
	}
	// The gloss: the word in the learner's language, for words and pictures.
	gloss := ""
	if item.Type == "vocab" || ItemKind(item) == "bildo" {
		gloss = DefinitionFor(item, u.Lang)
	}

	currentVote, _ := h.votes.GetByUserAndContent(ctx, u.ID, slug)
	data := map[string]interface{}{
		"User":            u,
		"Item":            item,
		"Correct":         correct,
		"Judgment":        judgment,
		"CorrectAnswer":   correctAnswer,
		"YourAnswer":      answer,
		"Gloss":           gloss,
		"Why":             item.Hint(),
		"ContinueHref":    cont,
		"NextSlug":        nextSlug,
		"HarderSlug":      harderSlug,
		"EasierSlug":      easierSlug,
		"NextInSeries":    nextInSeries,
		"CEFRLevel":       model.RatingToCEFR(newUserR),
		"LevelUp":         levelUp,
		"StreakDays":      streak,
		"StreakMilestone": milestone,
		"UILang":          u.UILangOrDefault(),
		"CurrentVote":     currentVote,
		"VoteScore":       item.VoteScore,
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
	judgment := r.FormValue("judgment")
	switch judgment {
	case "facile":
		score, correct = 1.0, true
	case "bone":
		score, correct = 1.0, true
	case "malfacile":
		score, correct = 0.5, false
	default: // "ne-sciis"
		judgment = "ne-sciis"
		score, correct = 0.0, false
	}

	h.verdict(w, r, item, u, newToken, score, correct, "", judgment)
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
