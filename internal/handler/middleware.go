package handler

import (
	"context"
	"net/http"

	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/store"
	gaeuser "google.golang.org/appengine/v2/user"
)

type contextKey string

const UserContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from the request context.
// Returns nil if no user is present.
func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(UserContextKey).(*model.User)
	return u
}

// AuthMiddleware extracts the bearer token from the X-Auth-Token header or
// the "token" cookie, looks up the corresponding user, and stores it in the
// request context. The next handler is always called, even when no user is found.
//
// GAE admins (as reported by appengine/user.IsAdmin) are automatically granted
// the "admin" role even without a token — this mirrors the native GAE pattern.
func AuthMiddleware(us *store.UserStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// If the requester is a GAE-authenticated admin, synthesise a superuser.
		if gaeuser.IsAdmin(ctx) {
			gaeU := gaeuser.Current(ctx)
			email := ""
			if gaeU != nil {
				email = gaeU.Email
			}
			admin := &model.User{
				ID:     "gae-admin",
				Token:  "",
				Role:   "admin",
				Rating: 1500,
				RD:     350,
			}
			_ = email // available for logging if needed
			ctx = context.WithValue(ctx, UserContextKey, admin)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
			return
		}

		// Normal token-based auth.
		token := r.Header.Get("X-Auth-Token")
		if token == "" {
			if c, err := r.Cookie("token"); err == nil {
				token = c.Value
			}
		}
		if token != "" {
			u, err := us.GetByToken(ctx, token)
			if err == nil && u != nil {
				ctx = context.WithValue(ctx, UserContextKey, u)
				r = r.WithContext(ctx)
				go func() { _ = us.UpdateLastSeen(context.Background(), u.ID) }()
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin checks the user has "admin" role.
// For /admin/* routes, GAE already enforces Google Account admin login via app.yaml
// (login: admin). This middleware provides an extra check for the role field,
// and also allows tokens with role=admin (e.g., for mod tooling).
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || u.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireMod returns 403 if the authenticated user does not have at least the "mod" role.
func RequireMod(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || (u.Role != "mod" && u.Role != "admin") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
