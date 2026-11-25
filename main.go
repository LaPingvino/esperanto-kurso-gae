package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	localauth "github.com/LaPingvino/esperanto-kurso-gae/internal/auth"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/config"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/lingvoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/locale"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

func main() {
	cfg := config.Load()
	go revo.Warm() // the embedded Reta Vortaro extract takes about a second to load
	ctx := context.Background()

	// --- Datastore ---
	db, err := store.NewDatastoreClient(ctx, cfg.ProjectID)
	if err != nil {
		log.Fatalf("datastore: %v", err)
	}
	defer db.Close()

	// --- Stores ---
	userStore := store.NewUserStore(db)
	contentStore := store.NewContentStore(db)
	attemptStore := store.NewAttemptStore(db)
	voteStore := store.NewVoteStore(db)
	commentStore := store.NewCommentStore(db)
	translationStore := store.NewTranslationStore(db)
	modMessageStore := store.NewModMessageStore(db)

	// --- Templates ---
	tmpl, err := parseTemplates()
	if err != nil {
		log.Fatalf("templates: %v", err)
	}
	handler.AssetVersion = assetVersion()

	// --- Auth ---
	sessionStore := localauth.NewSessionStore(db)
	wa, err := localauth.NewWebAuthn(cfg.WebAuthnRPID, cfg.WebAuthnOrigin)
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	// --- Handlers ---
	authH := handler.NewAuthHandler(tmpl, userStore, sessionStore, wa)
	contentH := handler.NewContentHandler(tmpl, contentStore, commentStore, voteStore, translationStore, userStore, attemptStore)
	exerciseH := handler.NewExerciseHandler(tmpl, contentStore, userStore, attemptStore, voteStore)
	communityH := handler.NewCommunityHandler(tmpl, contentStore, voteStore, commentStore, translationStore, modMessageStore, userStore)
	adminH := handler.NewAdminHandler(tmpl, contentStore, commentStore, userStore, modMessageStore, translationStore)
	gameH := handler.NewGameHandler(userStore, attemptStore)
	gameAPI := handler.NewGameAPI(contentStore)
	contentH.GamePlace = gameAPI.PlaceOf
	gameH.Course = gameAPI.Course

	// --- Router ---
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("GET /", contentH.ShowHome)
	mux.HandleFunc("GET /ekzerco/{slug}", contentH.ShowExercise)
	mux.HandleFunc("GET /vortaro", contentH.ShowVortaro)
	mux.HandleFunc("GET /sercxi", contentH.Browse)
	mux.HandleFunc("GET /etikedoj", contentH.ShowEtikedoj)
	mux.HandleFunc("GET /steloj", contentH.ShowFavorites)
	mux.HandleFunc("POST /ekzerco/{slug}/steli", contentH.ToggleFavorite)
	mux.HandleFunc("POST /serio/{series_slug}/steli", contentH.ToggleFavoriteSeries)
	mux.HandleFunc("POST /etikedo/{tag}/steli", contentH.ToggleFavoriteTag)
	mux.HandleFunc("GET /mapo", contentH.ShowMap)
	mux.HandleFunc("GET /gramatiko", contentH.ShowGrammarIndex)
	mux.HandleFunc("GET /gramatiko/{topic}", contentH.ShowGrammar)
	mux.HandleFunc("GET /vojkrucigxo", contentH.ShowHandoff)
	// The map replaces the series browser; the honour list is dropped (a
	// rating leaderboard serves neither the course nor the path to English).
	mux.Handle("GET /serioj", http.RedirectHandler("/mapo", http.StatusMovedPermanently))
	mux.Handle("GET /honorlisto", http.RedirectHandler("/mapo", http.StatusMovedPermanently))
	mux.HandleFunc("GET /vorto", contentH.RandomVocab)
	mux.HandleFunc("GET /fontoj", contentH.ShowCredits)
	// The course as a game (game/; built into static/ludo by `make ludo`). On
	// App Engine app.yaml serves /ludo/ as static files; these serve them when
	// the app runs by itself (go run). The progress endpoint is the app's.
	mux.HandleFunc("GET /ludo", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ludo/", http.StatusMovedPermanently)
	})
	mux.Handle("GET /ludo/", http.StripPrefix("/ludo/", http.FileServer(http.Dir("static/ludo"))))
	// app.yaml serves /static/ on App Engine; this serves it when the app runs
	// by itself (go run), so a local run has its stylesheet and pictures.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /ludo-progreso", gameH.Progress)
	mux.HandleFunc("POST /ludo-progreso", gameH.Progress)
	mux.HandleFunc("POST /ekzerco/{slug}/provo", exerciseH.SubmitAttempt)
	mux.HandleFunc("POST /ekzerco/{slug}/jugxo", exerciseH.JudgeExercise)
	mux.HandleFunc("GET /legado/{slug}/vortoj", contentH.ShowReadingPractice)
	mux.HandleFunc("POST /legado/{slug}/vortoj", exerciseH.SubmitReadingPractice)
	mux.HandleFunc("POST /ekzerco/{slug}/alternativo", communityH.SuggestAlternative)
	mux.HandleFunc("POST /ekzerco/{slug}/flagi", communityH.FlagExercise)
	mux.HandleFunc("GET /enskribi", authH.ShowEnskribi)

	// Auth routes.
	mux.HandleFunc("POST /auth/token", authH.GetOrCreateUser)
	mux.HandleFunc("GET /auth/verify", authH.VerifyToken)
	mux.HandleFunc("POST /auth/passkey/register/begin", authH.BeginPasskeyRegistration)
	mux.HandleFunc("POST /auth/passkey/register/finish", authH.FinishPasskeyRegistration)
	mux.HandleFunc("POST /auth/passkey/login/begin", authH.BeginPasskeyLogin)
	mux.HandleFunc("POST /auth/passkey/login/finish", authH.FinishPasskeyLogin)
	mux.HandleFunc("POST /auth/magic", authH.ShowEnskribi) // alias
	mux.HandleFunc("POST /lingvo", authH.SetLang)
	mux.HandleFunc("POST /uilingvo", authH.SetUILang)

	// Community routes.
	mux.HandleFunc("POST /vochdonado/{contentID}", communityH.Vote)
	mux.HandleFunc("POST /vochdonado/{contentID}/kompakta", communityH.VoteCompact)
	mux.HandleFunc("POST /komentoj/{contentID}", communityH.AddComment)
	mux.HandleFunc("POST /tradukoj/{contentID}", communityH.AddTranslation)
	mux.HandleFunc("POST /tradukoj/{contentID}/vochdoni/{id}", communityH.VoteTranslation)
	mux.HandleFunc("POST /tradukoj/{contentID}/redakti/{id}", communityH.EditTranslation)

	// Admin routes — registered directly with method+path to avoid ServeMux conflicts.
	ra := func(h http.HandlerFunc) http.Handler { return handler.RequireAdmin(h) }
	rm := func(h http.HandlerFunc) http.Handler { return handler.RequireMod(h) }
	rc := func(h http.HandlerFunc) http.Handler { return handler.RequireCreator(h) }
	mux.HandleFunc("GET /admin/initial", adminH.InitialSetup)
	mux.Handle("GET /admin", rc(adminH.Dashboard))
	// Content editing: creators, mods and admins
	mux.Handle("GET /admin/enhavo", rc(adminH.ListContent))
	mux.Handle("GET /admin/enhavo/nova", rc(adminH.NewContentForm))
	mux.Handle("POST /admin/enhavo", rc(adminH.CreateContent))
	mux.Handle("GET /admin/enhavo/{slug}/redakti", rc(adminH.EditContentForm))
	mux.Handle("POST /admin/enhavo/{slug}", rc(adminH.UpdateContent))
	mux.Handle("POST /admin/enhavo/{slug}/forigi", rm(adminH.DeleteContent))
	mux.Handle("POST /admin/enhavo/bulk-forigi", rm(adminH.BulkDeleteContent))
	mux.Handle("POST /admin/enhavo/auto-gepatro", rm(adminH.AutoAssignParents))
	// Series edit routes: creators, mods and admins
	mux.Handle("GET /admin/serio/{series_slug}/redakti", rc(adminH.SeriesEditForm))
	mux.Handle("POST /admin/serio/{series_slug}", rc(adminH.UpdateSeries))
	mux.Handle("POST /admin/serio/{series_slug}/renomi", rc(adminH.RenameSeries))
	// Moderation queue: mods and admins
	mux.Handle("GET /admin/moderigo", rm(adminH.ModerationQueue))
	mux.Handle("POST /admin/moderigo/{id}", rm(adminH.ModerateComment))
	mux.Handle("GET /admin/komentoj", rm(adminH.ShowAllComments))
	mux.Handle("POST /admin/komentoj/{id}/forigi", rm(adminH.DeleteApprovedComment))
	mux.Handle("POST /admin/tradukoj/{id}/aprobi", rm(adminH.ApproveTranslation))
	mux.Handle("POST /admin/mesagxoj/{id}/legita", rm(adminH.MarkModMessageRead))
	// Translation deletion: admin only
	mux.Handle("POST /admin/tradukoj/{id}/forigi", ra(adminH.DeleteTranslation))
	// Vocabulary generator: creators, mods and admins
	// Destructive / bulk operations: admin only
	mux.Handle("GET /admin/seed/{filename}/rigardi", ra(adminH.PeekSeed))
	mux.Handle("POST /admin/seed/{filename}", ra(adminH.SeedContent))
	mux.Handle("GET /admin/eksporti", ra(adminH.ExportContent))
	mux.Handle("POST /admin/importi", ra(adminH.ImportContent))
	mux.Handle("POST /admin/forigi-cion", ra(adminH.NukeContent))
	// User management: admin only
	mux.Handle("GET /admin/uzantoj", ra(adminH.ListUsers))
	mux.Handle("POST /admin/uzantoj/kunfandi", ra(adminH.MergeUsers))
	mux.Handle("POST /admin/uzantoj/{id}/rolo", ra(adminH.SetUserRole))
	mux.Handle("POST /admin/uzantoj/{id}/nomo-forigi", ra(adminH.UnlinkUsername))
	mux.HandleFunc("GET /admin/purigxi", adminH.CleanupInactiveUsers)

	// The builder's API, by key (handler/builder.go; scripts/builder.sh).
	handler.NewBuilderAPI(contentStore, cfg.BuilderKeyHash).Routes(mux)

	// Profile routes (logged-in users).
	mux.HandleFunc("POST /profilo/nomo", authH.SetUsername)
	mux.HandleFunc("POST /profilo/nomo/forigi", authH.ClearUsername)
	mux.HandleFunc("POST /profilo/konservado", authH.UpdateKeepDataDays)
	mux.HandleFunc("POST /kontaktu", communityH.SendModMessage)

	// Apply auth middleware to all routes — but the game's content API, the
	// same for everyone and publicly cacheable, needs no user (game_api.go).
	root := http.NewServeMux()
	gameAPI.Routes(root)
	// The site as an installable app (internal/handler/pwa.go, sw.js): the
	// same for everyone, so no user either.
	root.HandleFunc("GET /sw.js", handler.ServiceWorker)
	root.HandleFunc("GET /manifest.webmanifest", handler.Manifest)
	root.Handle("/", handler.AuthMiddleware(userStore, mux))

	addr := ":" + cfg.Port
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, root); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// pageTemplates maps a template name (e.g. "hejmo.html") to an isolated
// *template.Template that contains base.html + that page only.
// This prevents the "last define wins" problem when all pages define "content".
type pageTemplates struct {
	m map[string]*template.Template
}

// ExecuteTemplate satisfies handler.Renderer.
// Pages are rendered via their own template set so "content" / "title" blocks
// are unique to each page. Partials are executed directly.
func (pt *pageTemplates) ExecuteTemplate(w io.Writer, name string, data interface{}) error {
	t, ok := pt.m[name]
	if !ok {
		return fmt.Errorf("template %q not found", name)
	}
	// t.Execute runs the root template, which is named after the file's
	// basename (set via template.New(filepath.Base(path)) below).
	// This correctly handles admin templates whose map key differs from basename.
	return t.Execute(w, data)
}

// parseTemplates builds one *template.Template per page/partial.
func parseTemplates() (*pageTemplates, error) {
	funcs := templateFuncs()

	// Pages that use base.html layout.
	// key = name used in ExecuteTemplate calls; value = template file path.
	pages := map[string]string{
		"hejmo.html":              "templates/hejmo.html",
		"ekzerco.html":            "templates/ekzerco.html",
		"legvortoj.html":          "templates/legvortoj.html",
		"vortaro.html":            "templates/vortaro.html",
		"enskribi.html":           "templates/enskribi.html",
		"admin_dashboard.html":    "templates/admin/dashboard.html",
		"listo.html":              "templates/admin/listo.html",
		"redaktilo.html":          "templates/admin/redaktilo.html",
		"moderigo.html":           "templates/admin/moderigo.html",
		"admin_uzantoj.html":      "templates/admin/uzantoj.html",
		"sercxi.html":             "templates/sercxi.html",
		"etikedoj.html":           "templates/etikedoj.html",
		"steloj.html":             "templates/steloj.html",
		"mapo.html":               "templates/mapo.html",
		"gramatiko.html":          "templates/gramatiko.html",
		"gramatikero.html":        "templates/gramatikero.html",
		"vojkrucigxo.html":        "templates/vojkrucigxo.html",
		"fontoj.html":             "templates/fontoj.html",
		"admin-komentoj.html":     "templates/admin/komentoj.html",
		"admin-seed-rigardi.html": "templates/admin/seed-rigardi.html",
	}

	// Standalone partials (returned as HTMX fragments — no base layout).
	partials := map[string]string{
		"rezulto.html":              "templates/rezulto.html",
		"vochdonado.html":           "templates/partials/vochdonado.html",
		"komentoj.html":             "templates/partials/komentoj.html",
		"traduko.html":              "templates/partials/traduko.html",
		"alternativo-konfirmo.html": "templates/partials/alternativo-konfirmo.html",
		"flag-konfirmo.html":        "templates/partials/flag-konfirmo.html",
		"vote-kompakta.html":        "templates/partials/vote-kompakta.html",
	}

	pt := &pageTemplates{m: make(map[string]*template.Template)}

	for name, path := range pages {
		// Use the file's basename as the root template name so that
		// t.Execute() runs the right template even when the map key
		// (e.g. "admin_uzantoj.html") differs from the basename ("uzantoj.html").
		t, err := template.New(filepath.Base(path)).Funcs(funcs).ParseFiles(
			"templates/base.html",
			"templates/partials/ikonoj.html",
			"templates/partials/vochdonado.html",
			"templates/partials/komentoj.html",
			"templates/partials/traduko.html",
			"templates/partials/vortaroj.html",
			"templates/partials/glosaro.html",
			path,
		)
		if err != nil {
			return nil, fmt.Errorf("parseTemplates %s: %w", name, err)
		}
		pt.m[name] = t
	}

	for name, path := range partials {
		files := []string{path}
		if name == "rezulto.html" { // the verdict carries the compact vote
			files = append(files, "templates/partials/vote-kompakta.html")
		}
		t, err := template.New(name).Funcs(funcs).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("parseTemplates partial %s: %w", name, err)
		}
		pt.m[name] = t
	}

	return pt, nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// Drawn icons (templates/partials/ikonoj.html), never emoji.
		"ico": handler.Icon,
		// kindIco / tipNomo: an item's kind as icon + word (DESIGN.md 8).
		"kindIco": func(item *model.ContentItem) template.HTML {
			return handler.Icon(handler.KindIcon(item))
		},
		"typeIco": func(typ string) template.HTML {
			return handler.Icon(handler.KindIcon(&model.ContentItem{Type: typ}))
		},
		"tipNomo": handler.KindName,
		"itemKind": func(item *model.ContentItem) string {
			return handler.KindName(handler.ItemKind(item))
		},
		// levelIndex: a KER level's place on the line (A0 = 0 … C2 = 6).
		"levelIndex": func(l string) int {
			for i, x := range handler.Levels {
				if x == l {
					return i
				}
			}
			return -1
		},
		// versio busts the browser cache of the static files on each deploy.
		"versio": func() string { return handler.AssetVersion },
		// vojo: the seven Esperanto stations of the footer strip, as the
		// class of each dot ("" passed, "me" here, "off" ahead).
		"vojo": func(here string) []string {
			var out []string
			passed := true
			for _, l := range handler.Levels {
				switch {
				case l == here:
					out = append(out, "me")
					passed = false
				case passed:
					out = append(out, "")
				default:
					out = append(out, "off")
				}
			}
			return out
		},
		// Pictures (internal/bildoj): through data, never by hand here.
		// vortBildo: a vocab item's word picture, or a picture exercise's
		// (multiplechoice with content "bildo"); nil when there is none.
		"vortBildo": func(item *model.ContentItem) *bildoj.Picture {
			if item == nil {
				return nil
			}
			switch item.Type {
			case "vocab":
				return bildoj.ForVocab(item.Word(), handler.VocabKind(item))
			case "multiplechoice":
				if w, _ := item.Content["bildo"].(string); w != "" {
					return bildoj.ForVocab(w, "")
				}
			}
			return nil
		},
		"tekstBildoj": bildoj.InText,
		"hasTag": func(item *model.ContentItem, tag string) bool {
			for _, t := range item.Tags {
				if t == tag {
					return true
				}
			}
			return false
		},
		"serioBildo":  bildoj.ForSeries,
		"legadoBildo": bildoj.ForReading,
		"t": func(lang, key string) string {
			return locale.T(lang, key)
		},
		"isRTL": func(lang string) bool {
			return locale.IsRTL(lang)
		},
		"cefr": func(rating float64) string {
			return model.RatingToCEFR(rating)
		},
		// canContribute returns true for admins and for established learners (B1+ with stable rating).
		"canContribute": func(u *model.User) bool {
			if u == nil {
				return false
			}
			if u.Role == "admin" || u.Role == "mod" || u.Role == "creator" {
				return true
			}
			return u.Established()
		},
		"slice": func(s string, i, j int) string {
			if i < 0 {
				i = 0
			}
			if j > len(s) {
				j = len(s)
			}
			if i >= j {
				return ""
			}
			return s[i:j]
		},
		// defForLang: a vocab definition in the requested language only.
		"defForLang": handler.DefinitionIn,
		// percent: a part of a whole as a whole percentage (progress bars).
		"percent": func(part, whole int) int {
			if whole <= 0 {
				return 0
			}
			return part * 100 / whole
		},
		// longest: the length in characters of the longest string (tile layout).
		"longest": func(ss []string) int {
			n := 0
			for _, s := range ss {
				if l := len([]rune(s)); l > n {
					n = l
				}
			}
			return n
		},
		// allDefs returns all available definitions from content["definitions"] as a map.
		// Also includes the legacy flat content["definition"] as "en" if no multilingual map exists.
		"allDefs": func(content map[string]interface{}) map[string]interface{} {
			if content == nil {
				return nil
			}
			if defsRaw, ok := content["definitions"]; ok {
				if defs, ok := defsRaw.(map[string]interface{}); ok && len(defs) > 0 {
					return defs
				}
			}
			// Legacy fallback.
			if v, ok := content["definition"].(string); ok && v != "" {
				return map[string]interface{}{"en": v}
			}
			return nil
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		// splitGaps splits a fill-in question by "___" returning text segments.
		// len(segments) == number-of-gaps + 1.
		"splitGaps": func(q string) []string {
			return strings.Split(q, "___")
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "…"
		},
		"map": func(pairs ...interface{}) map[string]interface{} {
			m := make(map[string]interface{}, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				key, ok := pairs[i].(string)
				if !ok {
					continue
				}
				m[key] = pairs[i+1]
			}
			return m
		},
		// glossText: a reading's text with its words tappable (handler.GlossText).
		"glossText": handler.GlossText,
		"jsStr": func(s string) template.JS {
			b, _ := json.Marshal(s)
			return template.JS(b)
		},
		"not": func(v interface{}) bool {
			if v == nil {
				return true
			}
			if b, ok := v.(bool); ok {
				return !b
			}
			return false
		},
		"ne": func(a, b string) bool { return a != b },
		// kCount formats a count with a 'k' suffix when ≥ 1000.
		// The full number is placed in a title attribute for mouseover.
		"kCount": func(n int) template.HTML {
			if n < 1000 {
				return template.HTML(fmt.Sprintf("%d", n))
			}
			k := float64(n) / 1000.0
			var short string
			if k == float64(int(k)) {
				short = fmt.Sprintf("%dk", int(k))
			} else {
				short = fmt.Sprintf("%.1fk", k)
			}
			return template.HTML(fmt.Sprintf(`<span title="%d">%s</span>`, n, short))
		},
		"langName":   lingvoj.Name,
		"vocabLangs": lingvoj.Vocab,
	}
}

// assetVersion is a short hash of the static files the pages load, so their
// URLs change whenever their contents do.
func assetVersion() string {
	h := sha256.New()
	for _, f := range []string{"static/tokens.css", "static/stilo.css", "static/eo.js"} {
		if b, err := os.ReadFile(f); err == nil {
			h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

