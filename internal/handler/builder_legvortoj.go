package handler

// The cleanup of the old per-text word cards (voc-auto-*), through the
// builder API: the one path by which it changes items that are not the
// builder's own, and only those cards. Like the rest of the API it needs
// the key; it is explicit (a plan, then an apply naming that plan's
// fingerprint), logged (every change, under a run id) and undoable (each
// card keeps what the run changed, and /undo puts it back; nothing is
// deleted — a folded card is hidden with status "merged" and its page
// leads to the card it folded into).
//
//	GET  /builder-api/legvortoj          the plan (legvortoj.Plan) and its fingerprint; changes nothing
//	POST /builder-api/legvortoj/apply    {"fingerprint": "…"[, "run": "…"]}: carry out that plan, at most
//	                                     lvBatch actions a call → {"run", "remaining", "fingerprint"}: while
//	                                     remaining > 0, call again with that fingerprint and the same run
//	POST /builder-api/legvortoj/undo     {"run": "…"}: put back what that run changed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// lvBatch is how many cards one apply call changes (each a few Datastore
// calls; 1,900 cards in one request could outrun App Engine's deadline).
var lvBatch = 400

var lvRunRE = regexp.MustCompile(`^lv-[0-9]{8}T[0-9]{6}-[0-9a-f]{6}$`)

func (b *BuilderAPI) legvortojRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /builder-api/legvortoj", b.legvortojPlan)
	mux.HandleFunc("POST /builder-api/legvortoj/apply", b.legvortojApply)
	mux.HandleFunc("POST /builder-api/legvortoj/undo", b.legvortojUndo)
}

// lvSnapshot is what a run changed on one card, to put back.
type lvSnapshot struct {
	Run        string   `json:"run"`
	Created    bool     `json:"created,omitempty"` // the run made the card
	Status     string   `json:"status,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Rating     float64  `json:"rating,omitempty"`
	RD         float64  `json:"rd,omitempty"`
	Volatility float64  `json:"volatility,omitempty"`
	MergedInto *string  `json:"merged_into,omitempty"` // the content key's value before (nil: absent)
}

func (b *BuilderAPI) currentPlan(r *http.Request) ([]legvortoj.Action, string, map[string]*model.ContentItem, error) {
	all, err := b.content.ListForAdmin(r.Context(), "", 50000)
	if err != nil {
		return nil, "", nil, err
	}
	plan := legvortoj.Plan(all)
	raw, _ := json.Marshal(plan)
	sum := sha256.Sum256(raw)
	bySlug := map[string]*model.ContentItem{}
	for _, it := range all {
		bySlug[it.Slug] = it
	}
	return plan, hex.EncodeToString(sum[:8]), bySlug, nil
}

func lvCounts(plan []legvortoj.Action) map[string]int {
	c := map[string]int{}
	for _, a := range plan {
		c[a.Kind]++
	}
	return c
}

// legvortojPlan: the plan, for review; nothing changes.
func (b *BuilderAPI) legvortojPlan(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	plan, fp, _, err := b.currentPlan(r)
	if err != nil {
		builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	builderJSON(w, http.StatusOK, map[string]any{"fingerprint": fp, "counts": lvCounts(plan), "actions": plan})
}

func copyContent(c map[string]any) map[string]any {
	out := make(map[string]any, len(c)+2)
	for k, v := range c {
		out[k] = v
	}
	return out
}

// snapshots reads a card's undo stack.
func snapshots(c map[string]any) []lvSnapshot {
	var out []lvSnapshot
	if raw, ok := c[legvortoj.UndoKey]; ok {
		b, _ := json.Marshal(raw)
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func setSnapshots(c map[string]any, ss []lvSnapshot) {
	if len(ss) == 0 {
		delete(c, legvortoj.UndoKey)
		return
	}
	b, _ := json.Marshal(ss)
	var v any
	_ = json.Unmarshal(b, &v)
	c[legvortoj.UndoKey] = v
}

// legvortojApply carries out the plan whose fingerprint the caller saw.
func (b *BuilderAPI) legvortojApply(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	var body struct {
		Fingerprint string `json:"fingerprint"`
		Run         string `json:"run"` // go on with this run (the next batch)
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.Fingerprint == "" {
		builderJSON(w, http.StatusBadRequest, map[string]string{"error": `want {"fingerprint": "…"} from GET /builder-api/legvortoj`})
		return
	}
	ctx := r.Context()
	plan, fp, bySlug, err := b.currentPlan(r)
	if err != nil {
		builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if fp != body.Fingerprint {
		builderJSON(w, http.StatusConflict, map[string]any{"error": "the plan changed since; read it again", "fingerprint": fp})
		return
	}
	if len(plan) == 0 {
		builderJSON(w, http.StatusOK, map[string]any{"run": "", "changed": 0})
		return
	}
	run := body.Run
	if run == "" {
		run = "lv-" + time.Now().UTC().Format("20060102T150405") + "-" + fp[:6]
	} else if !lvRunRE.MatchString(run) {
		builderJSON(w, http.StatusBadRequest, map[string]string{"error": "a run id like lv-20261125T120000-abcdef"})
		return
	}
	// new cards first, so every fold has its target
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].Kind == "create" && plan[j].Kind != "create" })
	// a batch a call, well within a request's time; the rest in the next call
	remaining := 0
	if len(plan) > lvBatch {
		remaining, plan = len(plan)-lvBatch, plan[:lvBatch]
	}
	changed := 0
	fail := func(err error) {
		log.Printf("legvortoj %s: stopped after %d changes: %v (undo with this run)", run, changed, err)
		builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "run": run, "changed": changed})
	}
	for _, a := range plan {
		if !legvortoj.IsShared(&model.ContentItem{Type: "vocab", Slug: a.Slug}) {
			continue // the plan holds only voc-auto-* cards; never anything else
		}
		if a.Kind == "create" {
			c := map[string]any{"word": a.Word}
			setSnapshots(c, []lvSnapshot{{Run: run, Created: true}})
			it := &model.ContentItem{Slug: a.Slug, Type: "vocab", Content: c, Tags: a.Tags, Status: "approved",
				AuthorID: BuilderOrigin, Rating: a.Rating, RD: 200, Volatility: 0.06}
			if old := bySlug[a.Slug]; old != nil {
				// made by a run that was undone (hidden since): shown again
				it.AuthorID = old.AuthorID
				err = b.content.UpdateRating(ctx, a.Slug, a.Rating, 200, 0.06)
				if err == nil {
					err = b.content.PatchContentFields(ctx, it)
				}
			} else {
				err = b.content.Create(ctx, it)
			}
			if err != nil {
				fail(err)
				return
			}
			log.Printf("legvortoj %s: create %s (%s) rating %.0f", run, a.Slug, a.Note, a.Rating)
			changed++
			continue
		}
		old, err := b.content.GetBySlug(ctx, a.Slug)
		if err != nil || old == nil {
			fail(err)
			return
		}
		snap := lvSnapshot{Run: run, Status: old.Status, Tags: old.Tags, Rating: old.Rating, RD: old.RD, Volatility: old.Volatility}
		if v, ok := old.Content[legvortoj.MergedIntoKey].(string); ok {
			snap.MergedInto = &v
		}
		c := copyContent(old.Content)
		setSnapshots(c, append(snapshots(c), snap))
		it := *old
		it.Content = c
		switch a.Kind {
		case "keep":
			it.Tags = a.Tags
			if err := b.content.UpdateRating(ctx, a.Slug, a.Rating, 200, 0.06); err != nil {
				fail(err)
				return
			}
		case "core", "lemma", "junk", "refold":
			it.Status = legvortoj.MergedStatus
			c[legvortoj.MergedIntoKey] = a.Into
		default:
			continue
		}
		if err := b.content.PatchContentFields(ctx, &it); err != nil {
			fail(err)
			return
		}
		log.Printf("legvortoj %s: %s %s → %q (was %s, rating %.0f)", run, a.Kind, a.Slug, a.Into, old.Status, old.Rating)
		changed++
	}
	log.Printf("legvortoj %s: %d changes %v, %d to go", run, changed, lvCounts(plan), remaining)
	out := map[string]any{"run": run, "changed": changed, "counts": lvCounts(plan), "remaining": remaining}
	if remaining > 0 {
		if _, next, _, err := b.currentPlan(r); err == nil {
			out["fingerprint"] = next
		}
	}
	builderJSON(w, http.StatusOK, out)
}

// legvortojUndo puts back what a run changed: each card's latest snapshot
// of that run (a card changed by a later run is undone by that run first).
func (b *BuilderAPI) legvortojUndo(w http.ResponseWriter, r *http.Request) {
	if !b.allowed(w, r) {
		return
	}
	var body struct {
		Run string `json:"run"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil || body.Run == "" {
		builderJSON(w, http.StatusBadRequest, map[string]string{"error": `want {"run": "lv-…"}`})
		return
	}
	ctx := r.Context()
	all, err := b.content.ListForAdmin(ctx, "", 50000)
	if err != nil {
		builderJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	undone, blocked := 0, []string{}
	for _, old := range all {
		if !legvortoj.IsShared(old) {
			continue
		}
		ss := snapshots(old.Content)
		if len(ss) == 0 {
			continue
		}
		top := ss[len(ss)-1]
		if top.Run != body.Run {
			for _, s := range ss {
				if s.Run == body.Run {
					blocked = append(blocked, old.Slug)
				}
			}
			continue
		}
		c := copyContent(old.Content)
		setSnapshots(c, ss[:len(ss)-1])
		it := *old
		it.Content = c
		if top.Created {
			// made by the run: hidden again (nothing is deleted)
			it.Status = legvortoj.MergedStatus
			c[legvortoj.MergedIntoKey] = ""
		} else {
			it.Status, it.Tags = top.Status, top.Tags
			if top.MergedInto != nil {
				c[legvortoj.MergedIntoKey] = *top.MergedInto
			} else {
				delete(c, legvortoj.MergedIntoKey)
			}
			if err := b.content.UpdateRating(ctx, old.Slug, top.Rating, top.RD, top.Volatility); err != nil {
				builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "undone": undone})
				return
			}
		}
		if err := b.content.PatchContentFields(ctx, &it); err != nil {
			builderJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "undone": undone})
			return
		}
		log.Printf("legvortoj undo %s: %s back to %s", body.Run, old.Slug, it.Status)
		undone++
	}
	builderJSON(w, http.StatusOK, map[string]any{"run": body.Run, "undone": undone, "blocked_by_a_later_run": blocked})
}
