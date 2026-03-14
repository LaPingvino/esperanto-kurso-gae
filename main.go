package main

import (
	"fmt"
	"context"
	"html/template"
	"io"
	"log"
	"net/http"

	localauth "esperanto-kurso-gae/internal/auth"
	"esperanto-kurso-gae/internal/config"
	"esperanto-kurso-gae/internal/handler"
	"esperanto-kurso-gae/internal/store"
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

	// --- Templates ---
	tmpl, err := parseTemplates()
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	// --- Auth ---
	sessionStore := localauth.NewSessionStore()
	wa, err := localauth.NewWebAuthn(cfg.WebAuthnRPID, cfg.WebAuthnOrigin)
	if err != nil {
		log.Fatalf("webauthn: %v", err)
	}

	// --- Handlers ---
	authH := handler.NewAuthHandler(tmpl, userStore, sessionStore, wa)
	contentH := handler.NewContentHandler(tmpl, contentStore, commentStore, voteStore)
	exerciseH := handler.NewExerciseHandler(tmpl, contentStore, userStore, attemptStore)
	communityH := handler.NewCommunityHandler(tmpl, contentStore, voteStore, commentStore)
	adminH := handler.NewAdminHandler(tmpl, contentStore, commentStore, userStore)

	// --- Router ---
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("GET /", contentH.ShowHome)
	mux.HandleFunc("GET /ekzerco/{slug}", contentH.ShowExercise)
	mux.HandleFunc("POST /ekzerco/{slug}/provo", exerciseH.SubmitAttempt)
	mux.HandleFunc("GET /enskribi", authH.ShowEnskribi)

	// Auth routes.
	mux.HandleFunc("POST /auth/token", authH.GetOrCreateUser)
	mux.HandleFunc("GET /auth/verify", authH.VerifyToken)
	mux.HandleFunc("POST /auth/passkey/register/begin", authH.BeginPasskeyRegistration)
	mux.HandleFunc("POST /auth/passkey/register/finish", authH.FinishPasskeyRegistration)
	mux.HandleFunc("POST /auth/passkey/login/begin", authH.BeginPasskeyLogin)
	mux.HandleFunc("POST /auth/passkey/login/finish", authH.FinishPasskeyLogin)
	mux.HandleFunc("POST /auth/magic", authH.ShowEnskribi) // alias

	// Community routes.
	mux.HandleFunc("POST /vochdonado/{contentID}", communityH.Vote)
	mux.HandleFunc("POST /komentoj/{contentID}", communityH.AddComment)

	// Admin routes — registered directly with method+path to avoid ServeMux conflicts.
	ra := func(h http.HandlerFunc) http.Handler { return handler.RequireAdmin(h) }
	mux.Handle("GET /admin", ra(adminH.Dashboard))
	mux.Handle("GET /admin/enhavo", ra(adminH.ListContent))
	mux.Handle("GET /admin/enhavo/nova", ra(adminH.NewContentForm))
	mux.Handle("POST /admin/enhavo", ra(adminH.CreateContent))
	mux.Handle("GET /admin/enhavo/{slug}/redakti", ra(adminH.EditContentForm))
	mux.Handle("POST /admin/enhavo/{slug}", ra(adminH.UpdateContent))
	mux.Handle("GET /admin/moderigo", ra(adminH.ModerationQueue))
	mux.Handle("POST /admin/moderigo/{id}", ra(adminH.ModerateComment))
	mux.Handle("POST /admin/seed", ra(adminH.SeedContent))

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
	return t.ExecuteTemplate(w, name, data)
}

// parseTemplates builds one *template.Template per page/partial.
func parseTemplates() (*pageTemplates, error) {
	funcs := templateFuncs()

	// Pages that use base.html layout.
	// key = name used in ExecuteTemplate calls; value = template file path.
	pages := map[string]string{
		"hejmo.html":            "templates/hejmo.html",
		"ekzerco.html":          "templates/ekzerco.html",
		"enskribi.html":         "templates/enskribi.html",
		"admin_dashboard.html":  "templates/admin/dashboard.html",
		"listo.html":            "templates/admin/listo.html",
		"redaktilo.html":        "templates/admin/redaktilo.html",
		"moderigo.html":         "templates/admin/moderigo.html",
	}

	// Standalone partials (returned as HTMX fragments — no base layout).
	partials := map[string]string{
		"rezulto.html":    "templates/rezulto.html",
		"vochdonado.html": "templates/partials/vochdonado.html",
		"komentoj.html":   "templates/partials/komentoj.html",
		"traduko.html":    "templates/partials/traduko.html",
	}

	pt := &pageTemplates{m: make(map[string]*template.Template)}

	for name, path := range pages {
		t, err := template.New(name).Funcs(funcs).ParseFiles(
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
	return template.FuncMap{
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
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
		"not": func(v interface{}) bool {
			if v == nil {
				return true
			}
			if b, ok := v.(bool); ok {
				return !b
			}
			return false
		},
	}
}
