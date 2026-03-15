package main

import (
	"fmt"
	"context"
	"html"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	localauth "github.com/LaPingvino/esperanto-kurso-gae/internal/auth"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/config"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/locale"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

func main() {
	cfg := config.Load()
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

	// --- Auth ---
	sessionStore := localauth.NewSessionStore(db)
	wa, err := localauth.NewWebAuthn(cfg.WebAuthnRPID, cfg.WebAuthnOrigin)
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	// --- Handlers ---
	authH := handler.NewAuthHandler(tmpl, userStore, sessionStore, wa)
	contentH := handler.NewContentHandler(tmpl, contentStore, commentStore, voteStore, translationStore, userStore)
	exerciseH := handler.NewExerciseHandler(tmpl, contentStore, userStore, attemptStore)
	communityH := handler.NewCommunityHandler(tmpl, contentStore, voteStore, commentStore, translationStore, modMessageStore)
	adminH := handler.NewAdminHandler(tmpl, contentStore, commentStore, userStore, modMessageStore, translationStore)

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
	mux.HandleFunc("GET /honorlisto", contentH.ShowHonorListo)
	mux.HandleFunc("GET /vorto", contentH.RandomVocab)
	mux.HandleFunc("POST /ekzerco/{slug}/provo", exerciseH.SubmitAttempt)
	mux.HandleFunc("POST /ekzerco/{slug}/jugxo", exerciseH.JudgeExercise)
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
	mux.HandleFunc("POST /komentoj/{contentID}", communityH.AddComment)
	mux.HandleFunc("POST /tradukoj/{contentID}", communityH.AddTranslation)
	mux.HandleFunc("POST /tradukoj/{contentID}/vochdoni/{id}", communityH.VoteTranslation)

	// Admin routes — registered directly with method+path to avoid ServeMux conflicts.
	ra := func(h http.HandlerFunc) http.Handler { return handler.RequireAdmin(h) }
	rm := func(h http.HandlerFunc) http.Handler { return handler.RequireMod(h) }
	mux.HandleFunc("GET /admin/initial", adminH.InitialSetup)
	mux.Handle("GET /admin", rm(adminH.Dashboard))
	// Content editing: mods and admins
	mux.Handle("GET /admin/enhavo", rm(adminH.ListContent))
	mux.Handle("GET /admin/enhavo/nova", rm(adminH.NewContentForm))
	mux.Handle("POST /admin/enhavo", rm(adminH.CreateContent))
	mux.Handle("GET /admin/enhavo/{slug}/redakti", rm(adminH.EditContentForm))
	mux.Handle("POST /admin/enhavo/{slug}", rm(adminH.UpdateContent))
	mux.Handle("POST /admin/enhavo/{slug}/forigi", ra(adminH.DeleteContent))
	// Moderation queue: mods and admins
	mux.Handle("GET /admin/moderigo", rm(adminH.ModerationQueue))
	mux.Handle("POST /admin/moderigo/{id}", rm(adminH.ModerateComment))
	mux.Handle("POST /admin/tradukoj/{id}/aprobi", rm(adminH.ApproveTranslation))
	mux.Handle("POST /admin/mesagxoj/{id}/legita", rm(adminH.MarkModMessageRead))
	// Translation deletion: admin only
	mux.Handle("POST /admin/tradukoj/{id}/forigi", ra(adminH.DeleteTranslation))
	// Vocabulary generator: mods and admins
	mux.Handle("GET /admin/enhavo/{slug}/vortaro", rm(adminH.VocabFromReading))
	mux.Handle("POST /admin/enhavo/{slug}/vortaro", rm(adminH.CreateVocabFromReading))
	// Destructive / bulk operations: admin only
	mux.Handle("POST /admin/seed", ra(adminH.SeedContent))
	mux.Handle("POST /admin/patch-seed", ra(adminH.PatchSeedContent))
	mux.Handle("GET /admin/eksporti", ra(adminH.ExportContent))
	mux.Handle("POST /admin/importi", ra(adminH.ImportContent))
	mux.Handle("POST /admin/forigi-cion", ra(adminH.NukeContent))
	// User management: admin only
	mux.Handle("GET /admin/uzantoj", ra(adminH.ListUsers))
	mux.Handle("POST /admin/uzantoj/kunfandi", ra(adminH.MergeUsers))
	mux.Handle("POST /admin/uzantoj/{id}/rolo", ra(adminH.SetUserRole))
	mux.Handle("POST /admin/uzantoj/{id}/nomo-forigi", ra(adminH.UnlinkUsername))
	mux.HandleFunc("GET /admin/purigxi", adminH.CleanupInactiveUsers)

	// Profile routes (logged-in users).
	mux.HandleFunc("POST /profilo/nomo", authH.SetUsername)
	mux.HandleFunc("POST /profilo/nomo/forigi", authH.ClearUsername)
	mux.HandleFunc("POST /profilo/konservado", authH.UpdateKeepDataDays)
	mux.HandleFunc("POST /kontaktu", communityH.SendModMessage)

	// Apply auth middleware to all routes.
	root := handler.AuthMiddleware(userStore, mux)

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
		"hejmo.html":            "templates/hejmo.html",
		"ekzerco.html":          "templates/ekzerco.html",
		"vortaro.html":          "templates/vortaro.html",
		"enskribi.html":         "templates/enskribi.html",
		"admin_dashboard.html":  "templates/admin/dashboard.html",
		"listo.html":            "templates/admin/listo.html",
		"redaktilo.html":        "templates/admin/redaktilo.html",
		"moderigo.html":         "templates/admin/moderigo.html",
		"admin_uzantoj.html":    "templates/admin/uzantoj.html",
		"admin_vortaro_gen.html": "templates/admin/vortaro-gen.html",
		"sercxi.html":           "templates/sercxi.html",
		"etikedoj.html":         "templates/etikedoj.html",
		"steloj.html":           "templates/steloj.html",
		"honorlisto.html":       "templates/honorlisto.html",
	}

	// Standalone partials (returned as HTMX fragments — no base layout).
	partials := map[string]string{
		"rezulto.html":              "templates/rezulto.html",
		"vochdonado.html":           "templates/partials/vochdonado.html",
		"komentoj.html":             "templates/partials/komentoj.html",
		"traduko.html":              "templates/partials/traduko.html",
		"alternativo-konfirmo.html": "templates/partials/alternativo-konfirmo.html",
		"flag-konfirmo.html":        "templates/partials/flag-konfirmo.html",
	}

	pt := &pageTemplates{m: make(map[string]*template.Template)}

	for name, path := range pages {
		// Use the file's basename as the root template name so that
		// t.Execute() runs the right template even when the map key
		// (e.g. "admin_uzantoj.html") differs from the basename ("uzantoj.html").
		t, err := template.New(filepath.Base(path)).Funcs(funcs).ParseFiles(
			"templates/base.html",
			"templates/partials/vochdonado.html",
			"templates/partials/komentoj.html",
			"templates/partials/traduko.html",
			path,
		)
		if err != nil {
			return nil, fmt.Errorf("parseTemplates %s: %w", name, err)
		}
		pt.m[name] = t
	}

	for name, path := range partials {
		t, err := template.New(name).Funcs(funcs).ParseFiles(path)
		if err != nil {
			return nil, fmt.Errorf("parseTemplates partial %s: %w", name, err)
		}
		pt.m[name] = t
	}

	return pt, nil
}

func templateFuncs() template.FuncMap {
	typeNames := map[string]string{
		"multiplechoice": "Plurelekta",
		"fillin":         "Plenigi blankon",
		"listening":      "Aŭskulti",
		"vocab":          "Vortaro",
		"reading":        "Legado",
		"phrasebook":     "Frazaro",
		"image":          "Bildo",
		"video":          "Video",
	}
	return template.FuncMap{
		"t": func(lang, key string) string {
			return locale.T(lang, key)
		},
		"isRTL": func(lang string) bool {
			return locale.IsRTL(lang)
		},
		"tipNomo": func(t string) string {
			if name, ok := typeNames[t]; ok {
				return name
			}
			return t
		},
		"cefr": func(rating float64) string {
			return model.RatingToCEFR(rating)
		},
		// canContribute returns true for admins and for established learners (B1+ with stable rating).
		"canContribute": func(u *model.User) bool {
			if u == nil {
				return false
			}
			if u.Role == "admin" || u.Role == "mod" {
				return true
			}
			return u.Rating >= 1500 && u.RD < 150
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
		"seriesBar": func(current, total int) string {
			if total <= 0 {
				return ""
			}
			bar := ""
			for i := 1; i <= total; i++ {
				if i < current {
					bar += "▪"
				} else if i == current {
					bar += "◆"
				} else {
					bar += "▫"
				}
			}
			return bar
		},
		// defForLang returns a vocab definition in the requested language only.
		// Checks content["definitions"]["lang"] first (multilingual map), then falls
		// back to the legacy content["definition"] flat string as English.
		"defForLang": func(content map[string]interface{}, lang string) string {
			if content == nil {
				return ""
			}
			if defsRaw, ok := content["definitions"]; ok {
				if defs, ok := defsRaw.(map[string]interface{}); ok {
					if v, ok := defs[lang].(string); ok && v != "" {
						return v
					}
				}
			}
			// Legacy fallback: flat "definition" field treated as English.
			if lang == "en" {
				if v, ok := content["definition"].(string); ok && v != "" {
					return v
				}
			}
			return ""
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
		// linkifyText wraps known Esperanto words in reading texts with links to
		// their vocab exercise. Returns safe HTML. vocabItems should be the vocab
		// items tagged with this reading's slug.
		"linkifyText": func(text string, vocabItems []*model.ContentItem, lang string) template.HTML {
			// Build lookup: base_form → (slug, best_def)
			type vocabEntry struct {
				slug string
				def  string
			}
			lookup := map[string]vocabEntry{}
			for _, item := range vocabItems {
				if item.Type != "vocab" {
					continue
				}
				word := ""
				if w, ok := item.Content["word"].(string); ok {
					word = strings.ToLower(strings.TrimSpace(w))
				}
				if word == "" {
					continue
				}
				def := ""
				if defs, ok := item.Content["definitions"].(map[string]interface{}); ok {
					if v, ok := defs[lang].(string); ok {
						def = v
					}
					if def == "" {
						if v, ok := defs["en"].(string); ok {
							def = v
						}
					}
				}
				if def == "" {
					if v, ok := item.Content["definition"].(string); ok {
						def = v
					}
				}
				entry := vocabEntry{slug: item.Slug, def: def}
				lookup[word] = entry
				// Register root forms and all inflected variants so any word form
				// in a reading text matches, regardless of whether the stored
				// word is a full form (amiko) or a bare root (amik).
				// Strip common endings to get the root, then re-register.
				root := word
				for _, sfx := range []string{
					// Noun/adj plural+acc, singular+acc
					"ojn", "ajn", "oj", "aj", "on", "an",
					// Verb tenses/moods
					"as", "is", "os", "us",
					// Single-vowel endings
					"en", "o", "a", "e", "i", "u",
				} {
					if strings.HasSuffix(word, sfx) && len(word)-len(sfx) >= 2 {
						root = word[:len(word)-len(sfx)]
						break
					}
				}
				// Register all grammatical forms under the same entry.
				nounAdj := []string{"o", "oj", "on", "ojn", "a", "aj", "an", "ajn", "e"}
				verbForms := []string{"i", "as", "is", "os", "us", "u"}
				for _, ending := range append(nounAdj, verbForms...) {
					form := root + ending
					if _, exists := lookup[form]; !exists {
						lookup[form] = entry
					}
				}
				// Also register the bare root itself.
				if _, exists := lookup[root]; !exists {
					lookup[root] = entry
				}
			}

			// Tokenize text preserving whitespace and punctuation.
			var out strings.Builder
			// Process character by character, collecting word tokens.
			runes := []rune(text)
			i := 0
			for i < len(runes) {
				// Collect a word (Esperanto letters only).
				j := i
				for j < len(runes) && isEoLetter(runes[j]) {
					j++
				}
				if j > i {
					token := string(runes[i:j])
					base := eo.ToBaseForm(strings.ToLower(token))
					if entry, ok := lookup[base]; ok && entry.slug != "" {
						escaped := html.EscapeString(token)
						link := `/ekzerco/` + entry.slug
						if entry.def != "" {
							out.WriteString(`<a href="` + link + `" class="eo-word" data-def="` + html.EscapeString(entry.def) + `">` + escaped + `</a>`)
						} else {
							out.WriteString(`<a href="` + link + `" class="eo-word">` + escaped + `</a>`)
						}
					} else {
						out.WriteString(html.EscapeString(token))
					}
					i = j
				} else {
					// Non-word character: emit as-is (escaped).
					ch := runes[i]
					if ch == '\n' {
						out.WriteString("<br>")
					} else {
						out.WriteString(html.EscapeString(string(ch)))
					}
					i++
				}
			}
			return template.HTML(out.String())
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
		"langName": func(code string) string {
			names := map[string]string{
				"ar": "العربية", "be": "Беларуская", "bn": "বাংলা", "ca": "Català",
				"cs": "Čeština", "da": "Dansk", "de": "Deutsch", "el": "Ελληνικά",
				"en": "English", "eo": "Esperanto", "es": "Español", "fa": "فارسی",
				"fr": "Français", "frp": "Arpitan", "ga": "Gaeilge", "he": "עברית",
				"hi": "हिन्दी", "hr": "Hrvatski", "hu": "Magyar", "id": "Bahasa Indonesia",
				"it": "Italiano", "ja": "日本語", "kk": "Қазақша", "km": "ខ្មែរ",
				"ko": "한국어", "ku": "Kurdî", "lo": "ລາວ", "mg": "Malagasy",
				"ms": "Bahasa Melayu", "my": "မြန်မာဘာသာ", "nl": "Nederlands",
				"pl": "Polski", "pt": "Português", "ro": "Română", "ru": "Русский",
				"sk": "Slovenčina", "sl": "Slovenščina", "sv": "Svenska", "sw": "Kiswahili",
				"th": "ไทย", "tok": "Toki Pona", "tr": "Türkçe", "uk": "Українська",
				"ur": "اردو", "vi": "Tiếng Việt", "yo": "Yorùbá", "zh": "中文",
				"zh-tw": "繁體中文",
			}
			if n, ok := names[code]; ok {
				return n
			}
			return code
		},
	}
}

// isEoLetter reports whether r is an Esperanto letter (Latin + special chars).
func isEoLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		r == 'ĉ' || r == 'Ĉ' || r == 'ĝ' || r == 'Ĝ' ||
		r == 'ĥ' || r == 'Ĥ' || r == 'ĵ' || r == 'Ĵ' ||
		r == 'ŝ' || r == 'Ŝ' || r == 'ŭ' || r == 'Ŭ'
}
