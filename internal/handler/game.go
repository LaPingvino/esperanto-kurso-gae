package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The game (game/, "la ludo": the course played as a game, served at
// /ludo/) keeps its progress on the player's account through two endpoints:
//
//   - POST /ekzerco/{slug}/provo — the site's own, for a fill-in the player
//     TYPED in the game. Only typed answers go there, so only they move the
//     ratings: picking among the game's tiles is easier than typing, and the
//     Glicko ratings are calibrated on typing.
//   - GET / POST /ludo-progreso — the game's stars per lesson. A POST
//     carries the game's stars ("steloj": "slug=n", comma-joined), merged into the
//     account by taking the larger per lesson (model.MergeGameStars); both
//     answer with the account's state, so one request syncs both ways:
//
//     S <reading slug> <stars>   the game's stars on the account
//     F <exercise slug>          an exercise answered rightly on the site
//
// The game merges that into its own (the larger again), so progress made on
// the site — every fill-in of a lesson answered rightly — shows in the game,
// and nothing is lost in either direction. Every visitor has an account
// (an anonymous one until they sign in, AuthMiddleware); signing in merges
// accounts (UserStore.MergeUsers, the stars by the same rule).

type gameStarStore interface {
	MergeGameStars(ctx context.Context, userID string, stars map[string]int) (map[string]int, error)
}

type attemptLister interface {
	ListByUser(ctx context.Context, userID string, limit int) ([]*model.Attempt, error)
}

// GameHandler serves /ludo-progreso.
type GameHandler struct {
	users    gameStarStore
	attempts attemptLister
}

func NewGameHandler(users gameStarStore, attempts attemptLister) *GameHandler {
	return &GameHandler{users: users, attempts: attempts}
}

// Progress handles GET and POST /ludo-progreso (see above).
func (h *GameHandler) Progress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	u := UserFromContext(r.Context())
	if u == nil { // a crawler: no account, nothing kept
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	stars, err := h.users.MergeGameStars(r.Context(), u.ID, ParseGameStars(r.Form["steloj"]))
	if err != nil {
		http.Error(w, "ne eblis konservi", http.StatusInternalServerError)
		return
	}
	var right []string
	if as, err := h.attempts.ListByUser(r.Context(), u.ID, 2000); err == nil {
		seen := map[string]bool{}
		for _, a := range as {
			if a.Correct && !seen[a.ContentItemID] {
				seen[a.ContentItemID] = true
				right = append(right, a.ContentItemID)
			}
		}
	}
	fmt.Fprint(w, FormatGameProgress(stars, right))
}

// ParseGameStars reads the game's "slug=n" values (n 1–3), each value
// possibly several of them joined by commas; others are skipped.
func ParseGameStars(vals []string) map[string]int {
	out := map[string]int{}
	var parts []string
	for _, v := range vals {
		parts = append(parts, strings.Split(v, ",")...)
	}
	for _, v := range parts {
		slug, n, ok := strings.Cut(v, "=")
		k, err := strconv.Atoi(n)
		if !ok || err != nil || slug == "" || k < 1 || k > 3 || strings.ContainsAny(slug, " \n\t") {
			continue
		}
		if k > out[slug] {
			out[slug] = k
		}
	}
	return out
}

// FormatGameProgress is /ludo-progreso's answer: S lines, then F lines, sorted.
func FormatGameProgress(stars map[string]int, right []string) string {
	var b strings.Builder
	keys := make([]string, 0, len(stars))
	for k := range stars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "S %s %d\n", k, stars[k])
	}
	right = append([]string(nil), right...)
	sort.Strings(right)
	for _, s := range right {
		fmt.Fprintf(&b, "F %s\n", s)
	}
	return b.String()
}

// GameLesson is the game's lesson for an exercise page: the Zagreb reading
// itself, or the reading a fill-in belongs to; "" when the game has none.
func GameLesson(item *model.ContentItem) string {
	if item == nil {
		return ""
	}
	// By the series where an item has them, else by the Zagreb slugs
	// (zagr-l01-teksto, zagr-l01-fi01: items seeded without series fields).
	lesson := func(s string) bool {
		return len(s) == len("zagr-l01-teksto") && strings.HasPrefix(s, "zagr-l") && strings.HasSuffix(s, "-teksto")
	}
	switch {
	case item.Type == "reading" && (item.SeriesSlug == "zagr-metodo" || lesson(item.Slug)):
		return item.Slug
	case item.Type == "fillin" && lesson(item.SeriesParent):
		return item.SeriesParent
	case item.Type == "fillin" && strings.HasPrefix(item.Slug, "zagr-l") && strings.Contains(item.Slug, "-fi") && lesson(item.Slug[:len("zagr-l01")]+"-teksto"):
		return item.Slug[:len("zagr-l01")] + "-teksto"
	}
	return ""
}
