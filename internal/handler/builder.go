package handler

// The builder API (Joop 2026-11-25: "enable adding to the course from the VPS
// … with a key like with the WE feedback cycle"). The builder — Claude,
// working on the course from the VPS — holds a key and, through
// /builder-api/…, reads the course and adds to it. Modelled on WEquilibrium's
// web/builder.go: only the key's SHA-256 is known to the site
// (EK_BUILDER_KEY_SHA256 in app.yaml; empty: the API is off); the key itself
// lives on the VPS in ~/.config/esperanto-kurso-builder/key. The API takes no
// cookies, so no form token either.
//
// What it may touch: only its own items. Everything it writes carries the
// origin mark AuthorID "builder" (kept by the admin editor too); it refuses to
// overwrite or change the status of any item without that mark (seed, admin and
// community content), so it can never clobber or publish someone else's work.
//
// The approval flow: new items go in "pending" — on the admin's list
// (/admin/enhavo?status=pending) and the dashboard's count, invisible in every
// public list, series and recommendation — until an admin approves them there,
// or the builder publishes them ("trusted": true on add, or /publish). Every
// publish is undone in one call: POST /builder-api/unpublish {"all": true}
// sends every published builder item back to pending (or {"slugs": […]},
// {"series": "…"} for some). Limit: like every unapproved item on the site, a
// pending item still opens by its direct URL /ekzerco/{slug}; it is listed
// nowhere.
//
//	GET  /builder-api/content[?status=&type=&series=&level=&origin=builder]  the outline, plus counts
//	GET  /builder-api/content/{slug}                                       one item, whole
//	POST /builder-api/content    one item or an array (≤ 500); add, or update the builder's own
//	POST /builder-api/publish    {"slugs"|"series"|"all"} → approved   (builder items only)
//	POST /builder-api/unpublish  {"slugs"|"series"|"all"} → pending    (builder items only)
//	POST /builder-api/review     the same as unpublish, by its other name: "mark for review"

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// BuilderOrigin is the AuthorID every builder-made item carries.
const BuilderOrigin = "builder"

// builderStore is what the API needs of the content store (store.ContentStore).
type builderStore interface {
	GetBySlug(ctx context.Context, slug string) (*model.ContentItem, error)
	Create(ctx context.Context, item *model.ContentItem) error
	PatchContentFields(ctx context.Context, item *model.ContentItem) error
	ListForAdmin(ctx context.Context, statusFilter string, limit int) ([]*model.ContentItem, error)
	UpdateRating(ctx context.Context, slug string, rating, rd, volatility float64) error
}

// BuilderAPI serves /builder-api/….
type BuilderAPI struct {
	content builderStore
	keyHash string // lower-case hex SHA-256 of the key; "" the API is off
	tries   *builderLimiter
}

// NewBuilderAPI makes the API; keyHash is the key's SHA-256 in hex ("" off).
func NewBuilderAPI(content builderStore, keyHash string) *BuilderAPI {
	return &BuilderAPI{
		content: content,
		keyHash: strings.ToLower(strings.TrimSpace(keyHash)),
		tries:   &builderLimiter{max: 20, window: 15 * time.Minute, seen: map[string][]time.Time{}},
	}
}

// Routes registers the API on mux.
func (b *BuilderAPI) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /builder-api/content", b.list)
	mux.HandleFunc("GET /builder-api/content/{slug}", b.get)
	mux.HandleFunc("POST /builder-api/content", b.put)
	mux.HandleFunc("POST /builder-api/publish", b.setStatus("approved"))
	mux.HandleFunc("POST /builder-api/unpublish", b.setStatus("pending"))
	mux.HandleFunc("POST /builder-api/review", b.setStatus("pending"))
}

// builderLimiter counts wrong keys per address.
type builderLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	seen   map[string][]time.Time
}

func (l *builderLimiter) allow(addr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.seen[addr][:0]
	for _, t := range l.seen[addr] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.seen[addr] = kept
		return false
	}
	l.seen[addr] = append(kept, now)
	return true
}

func builderClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Appengine-User-Ip"); ip != "" {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func builderSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// allowed checks the request's key (Authorization: Bearer …): 401 without one
// or with a wrong one, 403 while the API is off, 429 after too many wrong keys.
func (b *BuilderAPI) allowed(w http.ResponseWriter, r *http.Request) bool {
	if b.keyHash == "" {
		builderJSON(w, http.StatusForbidden, map[string]string{"error": "the builder API is off (no EK_BUILDER_KEY_SHA256)"})
		return false
	}
	key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(builderSHA256(strings.TrimSpace(key))), []byte(b.keyHash)) != 1 {
		if !b.tries.allow(builderClientIP(r)) {
			builderJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many tries"})
			return false
		}
		builderJSON(w, http.StatusUnauthorized, map[string]string{"error": "a builder key is needed"})
		return false
	}
	return true
}

func builderJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

// BuilderItem is an item as the API reads and writes it.
type BuilderItem struct {
	Slug         string         `json:"slug"`
	Type         string         `json:"type"`
	Content      map[string]any `json:"content"`
	Tags         []string       `json:"tags,omitempty"`
	Source       string         `json:"source,omitempty"`
	ImageURL     string         `json:"image_url,omitempty"`
	SeriesSlug   string         `json:"series_slug,omitempty"`
	SeriesOrder  int            `json:"series_order,omitempty"`
	SeriesLabel  string         `json:"series_label,omitempty"`
	SeriesParent string         `json:"series_parent,omitempty"`
	// Level (A0…C2) or Rating sets the starting difficulty of a new item
	// (the level's middle); an update keeps the learned rating.
	Level  string  `json:"level,omitempty"`
	Rating float64 `json:"rating,omitempty"`
	// Trusted: publish at once (approved) instead of pending.
	Trusted bool `json:"trusted,omitempty"`
	// Rerate: on an update, start the rating again from Level or Rating
	// (RD 200), for an item whose first level was wrong; Glicko then
	// settles it again.
	Rerate bool `json:"rerate,omitempty"`
	// read back only
	Status    string    `json:"status,omitempty"`
	AuthorID  string    `json:"author_id,omitempty"`
	Votes     int       `json:"vote_score,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

func builderItemOf(it *model.ContentItem) BuilderItem {
	return BuilderItem{
		Slug: it.Slug, Type: it.Type, Content: it.Content, Tags: it.Tags, Source: it.Source,
		ImageURL: it.ImageURL, SeriesSlug: it.SeriesSlug, SeriesOrder: it.SeriesOrder,
		SeriesLabel: it.SeriesLabel, SeriesParent: it.SeriesParent,
		Level: model.RatingToCEFR(it.Rating), Rating: it.Rating,
		Status: it.Status, AuthorID: it.AuthorID, Votes: it.VoteScore, UpdatedAt: it.UpdatedAt,
	}
}

// builderLevelRating: the rating a new item starts at, inside its level's band
// (content.go cefrToRatingRange; C2 has no top, so 2100).
var builderLevelRating = map[string]float64{
	"A0": 900, "A1": 1100, "A2": 1300, "B1": 1500, "B2": 1700, "C1": 1900, "C2": 2100,
}

// builderStartRating: the rating an item asks to start at (Rating, else its
// Level's), and whether it asks.
func builderStartRating(it *BuilderItem) (float64, bool) {
	if it.Rating > 0 {
		return it.Rating, true
	}
	r, ok := builderLevelRating[it.Level]
	return r, ok
}

var builderSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,119}$`)

var builderTypes = map[string]bool{
	"reading": true, "multiplechoice": true, "fillin": true, "vocab": true,
	"phrasebook": true, "video": true, "listening": true, "image": true,
}

func builderStr(c map[string]any, k string) string {
	s, _ := c[k].(string)
	return strings.TrimSpace(s)
}

func builderStrs(c map[string]any, k string) ([]string, bool) {
	raw, ok := c[k]
	if !ok {
		return nil, false
	}
	xs, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		s, ok := x.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// ValidateBuilderItem says what is wrong with an item ("" nothing), by the
// site's own answer rules (exercise.go checkAnswer): a multiple choice's
// options differ even ignoring case, its correct_index is among them; a
// fill-in has one answer per ___ (or one for all), none empty. The content
// files in content/builder are checked by the same function in a test.
func ValidateBuilderItem(it *BuilderItem) string {
	if !builderSlugRE.MatchString(it.Slug) {
		return "a slug of lower-case letters, digits and hyphens (up to 120)"
	}
	if !builderTypes[it.Type] {
		return "a type: reading, multiplechoice, fillin, vocab, phrasebook, video, listening or image"
	}
	if it.Level != "" {
		if _, ok := builderLevelRating[it.Level]; !ok {
			return "a level: A0, A1, A2, B1, B2, C1 or C2"
		}
	}
	if it.Rating < 0 || it.Rating > 3500 {
		return "a rating between 0 and 3500"
	}
	if it.SeriesSlug != "" && !builderSlugRE.MatchString(it.SeriesSlug) {
		return "a series_slug like a slug"
	}
	if it.SeriesParent != "" && !builderSlugRE.MatchString(it.SeriesParent) {
		return "a series_parent like a slug"
	}
	if len(it.Tags) > 20 {
		return "at most 20 tags"
	}
	c := it.Content
	if c == nil {
		return "a content object"
	}
	if b, _ := json.Marshal(c); len(b) > 200<<10 {
		return "content of at most 200 kB"
	}
	switch it.Type {
	case "reading":
		if builderStr(c, "title") == "" || builderStr(c, "text") == "" {
			return "a reading: content.title and content.text"
		}
	case "multiplechoice":
		opts, ok := builderStrs(c, "options")
		if builderStr(c, "question") == "" || !ok || len(opts) < 2 || len(opts) > 6 {
			return "a multiple choice: content.question and 2 to 6 content.options"
		}
		seen := map[string]bool{}
		for _, o := range opts {
			k := strings.ToLower(strings.TrimSpace(o))
			if k == "" || seen[k] {
				return "a multiple choice: options that differ (ignoring case), none empty"
			}
			seen[k] = true
		}
		ci, ok := c["correct_index"].(float64)
		if !ok || ci != float64(int(ci)) || int(ci) < 0 || int(ci) >= len(opts) {
			return "a multiple choice: content.correct_index, the number of the right option from 0"
		}
	case "fillin":
		q := builderStr(c, "question")
		gaps := strings.Count(q, "___")
		if gaps == 0 {
			return "a fill-in: content.question with ___ for each gap"
		}
		if strings.Contains(q, "____") {
			return "a fill-in: gaps of exactly three underscores"
		}
		answers, ok := builderStrs(c, "answers")
		if !ok {
			if a := builderStr(c, "answer"); a != "" {
				answers = []string{a}
			}
		}
		if len(answers) != gaps && len(answers) != 1 {
			return fmt.Sprintf("a fill-in: one answer per gap (%d), or one for all", gaps)
		}
		for _, a := range answers {
			if strings.TrimSpace(a) == "" {
				return "a fill-in: no empty answer"
			}
		}
	case "vocab":
		if builderStr(c, "word") == "" {
			return "a vocab item: content.word"
		}
		if builderStr(c, "definition") == "" {
			if d, ok := c["definitions"].(map[string]any); !ok || len(d) == 0 {
				return "a vocab item: content.definition or content.definitions"
			}
		}
	case "phrasebook", "listening", "image":
		if builderStr(c, "answer") == "" {
			return "a " + it.Type + " item: content.answer"
		}
	case "video":
		if !strings.HasPrefix(builderStr(c, "video_url"), "https://") {
			return "a video: content.video_url (https://…)"
		}
	}
	for _, k := range []string{"title", "question", "text"} {
		if s := builderStr(c, k); s != "" && !utf8.ValidString(s) {
			return "content." + k + " in UTF-8"
		}
	}
	return ""
}

// list: the outline of the course — every item's slug, type, level, status
// and place, without the content — and counts by level, type and status.
func (b *BuilderAPI) list(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	q := r.URL.Query()
	items, err := b.content.ListForAdmin(r.Context(), q.Get("status"), 50000)
	if err != nil {
		builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type line struct {
		Slug, Type, Level, Status, Series, Parent, Label, Source, Author, Title string
		Order                                                                   int
		Rating                                                                  float64
		Tags                                                                    []string
	}
	lines := []line{}
	counts := map[string]map[string]int{"level": {}, "type": {}, "status": {}, "series": {}}
	for _, it := range items {
		lvl := model.RatingToCEFR(it.Rating)
		switch {
		case q.Get("type") != "" && it.Type != q.Get("type"),
			q.Get("series") != "" && it.SeriesSlug != q.Get("series"),
			q.Get("level") != "" && lvl != q.Get("level"),
			q.Get("origin") == BuilderOrigin && it.AuthorID != BuilderOrigin:
			continue
		}
		title := it.Title()
		if title == "" {
			title = it.Word()
		}
		if title == "" {
			title = it.Question()
		}
		if utf8.RuneCountInString(title) > 80 {
			title = string([]rune(title)[:80]) + "…"
		}
		lines = append(lines, line{it.Slug, it.Type, lvl, it.Status, it.SeriesSlug, it.SeriesParent, it.SeriesLabel, it.Source, it.AuthorID, title, it.SeriesOrder, it.Rating, it.Tags})
		counts["level"][lvl]++
		counts["type"][it.Type]++
		counts["status"][it.Status]++
		if it.SeriesSlug != "" {
			counts["series"][it.SeriesSlug]++
		}
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].Series != lines[j].Series {
			return lines[i].Series < lines[j].Series
		}
		if lines[i].Order != lines[j].Order {
			return lines[i].Order < lines[j].Order
		}
		return lines[i].Slug < lines[j].Slug
	})
	builderJSON(w, http.StatusOK, map[string]any{"count": len(lines), "counts": counts, "items": lines})
}

// get: one item, whole.
func (b *BuilderAPI) get(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	it, err := b.content.GetBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if it == nil {
		builderJSON(w, http.StatusNotFound, map[string]string{"error": "no item " + r.PathValue("slug")})
		return
	}
	builderJSON(w, http.StatusOK, builderItemOf(it))
}

// put: one item or an array — each added (pending, or approved if trusted),
// or, if it is already the builder's, updated (its status kept unless trusted
// publishes it; its learned rating kept unless rerate). The whole batch is checked first: one
// bad item and nothing is written.
func (b *BuilderAPI) put(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&raw); err != nil {
		builderJSON(w, http.StatusBadRequest, map[string]string{"error": "the body isn't JSON: " + err.Error()})
		return
	}
	var batch []BuilderItem
	if t := strings.TrimSpace(string(raw)); strings.HasPrefix(t, "[") {
		if err := json.Unmarshal(raw, &batch); err != nil {
			builderJSON(w, http.StatusBadRequest, map[string]string{"error": "the items aren't right: " + err.Error()})
			return
		}
	} else {
		var one BuilderItem
		if err := json.Unmarshal(raw, &one); err != nil {
			builderJSON(w, http.StatusBadRequest, map[string]string{"error": "the item isn't right: " + err.Error()})
			return
		}
		batch = []BuilderItem{one}
	}
	if len(batch) == 0 || len(batch) > 500 {
		builderJSON(w, http.StatusBadRequest, map[string]string{"error": "1 to 500 items"})
		return
	}
	ctx := r.Context()
	type problem struct {
		Slug, Error string
	}
	var problems []problem
	existing := make([]*model.ContentItem, len(batch))
	inBatch := map[string]bool{}
	for i := range batch {
		it := &batch[i]
		if p := ValidateBuilderItem(it); p != "" {
			problems = append(problems, problem{it.Slug, "needs " + p})
			continue
		}
		if inBatch[it.Slug] {
			problems = append(problems, problem{it.Slug, "is twice in the batch"})
			continue
		}
		inBatch[it.Slug] = true
		old, err := b.content.GetBySlug(ctx, it.Slug)
		if err != nil {
			builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if old != nil && old.AuthorID != BuilderOrigin {
			problems = append(problems, problem{it.Slug, "is not the builder's (author " + fmt.Sprintf("%q", old.AuthorID) + "): the builder changes only its own items"})
			continue
		}
		existing[i] = old
	}
	for i := range batch { // parents: in the store or in this batch
		if p := batch[i].SeriesParent; p != "" && !inBatch[p] {
			if par, _ := b.content.GetBySlug(ctx, p); par == nil {
				problems = append(problems, problem{batch[i].Slug, "has series_parent " + p + ", which doesn't exist"})
			}
		}
	}
	if len(problems) > 0 {
		builderJSON(w, http.StatusBadRequest, map[string]any{"error": "nothing was written", "problems": problems})
		return
	}
	type done struct {
		Slug, Action, Status string
	}
	var results []done
	for i := range batch {
		it := &batch[i]
		status := "pending"
		if it.Trusted {
			status = "approved"
		}
		item := &model.ContentItem{
			Slug: it.Slug, Type: it.Type, Content: it.Content, Tags: it.Tags, Source: it.Source,
			AuthorID: BuilderOrigin, ImageURL: it.ImageURL,
			SeriesSlug: it.SeriesSlug, SeriesOrder: it.SeriesOrder, SeriesLabel: it.SeriesLabel, SeriesParent: it.SeriesParent,
		}
		if old := existing[i]; old != nil {
			if !it.Trusted {
				status = old.Status
			}
			item.Status = status
			if it.Rerate { // before the patch, whose version bump makes every instance reload
				r, ok := builderStartRating(it)
				if !ok {
					builderJSON(w, http.StatusBadRequest, map[string]any{"error": it.Slug + ": rerate needs a level or a rating", "written": results})
					return
				}
				if err := b.content.UpdateRating(ctx, it.Slug, r, 200, 0.06); err != nil {
					builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "written": results})
					return
				}
			}
			if err := b.content.PatchContentFields(ctx, item); err != nil {
				builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "written": results})
				return
			}
			results = append(results, done{it.Slug, "updated", status})
			continue
		}
		item.Status = status
		item.Rating, item.RD, item.Volatility = 1500, 200, 0.06
		if r, ok := builderStartRating(it); ok {
			item.Rating = r
		}
		if err := b.content.Create(ctx, item); err != nil {
			builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "written": results})
			return
		}
		results = append(results, done{it.Slug, "added", status})
	}
	builderJSON(w, http.StatusOK, map[string]any{"written": results})
}

// setStatus: builder items — named, a series, or all — to a status.
// "approved" publishes; "pending" unpublishes (back on the admin's queue).
func (b *BuilderAPI) setStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !b.allowed(w, r) {
			return
		}
		var body struct {
			Slugs  []string `json:"slugs"`
			Series string   `json:"series"`
			All    bool     `json:"all"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil ||
			(len(body.Slugs) == 0 && body.Series == "" && !body.All) {
			builderJSON(w, http.StatusBadRequest, map[string]string{"error": `want {"slugs": [...]}, {"series": "..."} or {"all": true}`})
			return
		}
		ctx := r.Context()
		var targets []*model.ContentItem
		var refused []string
		if len(body.Slugs) > 0 {
			for _, s := range body.Slugs {
				it, err := b.content.GetBySlug(ctx, s)
				if err != nil {
					builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				if it == nil || it.AuthorID != BuilderOrigin {
					refused = append(refused, s)
					continue
				}
				targets = append(targets, it)
			}
		} else {
			all, err := b.content.ListForAdmin(ctx, "", 50000)
			if err != nil {
				builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for _, it := range all {
				if it.AuthorID == BuilderOrigin && (body.All || it.SeriesSlug == body.Series) {
					targets = append(targets, it)
				}
			}
		}
		if len(refused) > 0 {
			builderJSON(w, http.StatusConflict, map[string]any{"error": "not builder items (or missing); nothing was changed", "refused": refused})
			return
		}
		changed := []string{}
		for _, it := range targets {
			if it.Status == status {
				continue
			}
			it.Status = status
			if err := b.content.PatchContentFields(ctx, it); err != nil {
				builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "changed": changed})
				return
			}
			changed = append(changed, it.Slug)
		}
		builderJSON(w, http.StatusOK, map[string]any{"status": status, "changed": changed, "matched": len(targets)})
	}
}
