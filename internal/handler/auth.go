package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	localauth "esperanto-kurso-gae/internal/auth"
	"esperanto-kurso-gae/internal/model"
	"esperanto-kurso-gae/internal/store"

	"github.com/go-webauthn/webauthn/webauthn"
)

// AuthHandler bundles all authentication-related HTTP handlers.
type AuthHandler struct {
	tmpl     Renderer
	users    *store.UserStore
	sessions *localauth.SessionStore
	wa       *webauthn.WebAuthn
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(
	tmpl Renderer,
	users *store.UserStore,
	sessions *localauth.SessionStore,
	wa *webauthn.WebAuthn,
) *AuthHandler {
	return &AuthHandler{tmpl: tmpl, users: users, sessions: sessions, wa: wa}
}

// generateUserID creates a random URL-safe 16-byte user ID.
func generateUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetOrCreateUser handles POST /auth/token.
// If a valid X-Auth-Token is supplied, the existing user is returned.
// Otherwise a new user is created and its token returned.
func (h *AuthHandler) GetOrCreateUser(w http.ResponseWriter, r *http.Request) {
	existingToken := r.Header.Get("X-Auth-Token")
	if existingToken != "" {
		u, err := h.users.GetByToken(r.Context(), existingToken)
		if err == nil && u != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"token":  u.Token,
				"userID": u.ID,
			})
			return
		}
	}

	// Create a new anonymous user.
	id, err := generateUserID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	token, err := localauth.GenerateToken()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	u := model.NewUser(id, token)
	if err := h.users.Create(r.Context(), u); err != nil {
		http.Error(w, "could not create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":  token,
		"userID": id,
	})
}

// ShowEnskribi handles GET /enskribi — the "save progress" page.
func (h *AuthHandler) ShowEnskribi(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())

	var magicURL string
	if u != nil {
		magicURL = fmt.Sprintf("%s://%s/auth/verify?token=%s",
			scheme(r), r.Host, u.Token)
	}

	data := map[string]interface{}{
		"User":     u,
		"MagicURL": magicURL,
	}
	if err := h.tmpl.ExecuteTemplate(w, "enskribi.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// VerifyToken handles GET /auth/verify?token=XXX.
// It sets the token in a cookie and redirects to /.
func (h *AuthHandler) VerifyToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "mankas token", http.StatusBadRequest)
		return
	}
	// Validate the token.
	u, err := h.users.GetByToken(r.Context(), token)
	if err != nil || u == nil {
		http.Error(w, "nevalida token", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// BeginPasskeyRegistration handles POST /auth/passkey/register/begin.
func (h *AuthHandler) BeginPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	options, sessionData, err := h.wa.BeginRegistration(u)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.sessions.Set("reg:"+u.ID, sessionData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// FinishPasskeyRegistration handles POST /auth/passkey/register/finish.
func (h *AuthHandler) FinishPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	sessionData, ok := h.sessions.Get("reg:" + u.ID)
	if !ok {
		http.Error(w, "session expired", http.StatusBadRequest)
		return
	}
	h.sessions.Delete("reg:" + u.ID)

	cred, err := h.wa.FinishRegistration(u, *sessionData, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.users.AddPasskey(r.Context(), u.ID, *cred); err != nil {
		http.Error(w, "could not save passkey", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// BeginPasskeyLogin handles POST /auth/passkey/login/begin.
func (h *AuthHandler) BeginPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	options, sessionData, err := h.wa.BeginDiscoverableLogin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Store session data keyed by challenge string.
	h.sessions.Set("login:"+sessionData.Challenge, sessionData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(options)
}

// FinishPasskeyLogin handles POST /auth/passkey/login/finish.
func (h *AuthHandler) FinishPasskeyLogin(w http.ResponseWriter, r *http.Request) {
	// Read the body so we can extract the challenge before the webauthn library consumes it.
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Extract challenge from clientDataJSON to look up the session.
	var parsed struct {
		Response struct {
			ClientDataJSON string `json:"clientDataJSON"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var clientData struct {
		Challenge string `json:"challenge"`
	}
	if parsed.Response.ClientDataJSON != "" {
		cdRaw, decErr := base64.RawURLEncoding.DecodeString(parsed.Response.ClientDataJSON)
		if decErr == nil {
			_ = json.Unmarshal(cdRaw, &clientData)
		}
	}

	sessionData, ok := h.sessions.Get("login:" + clientData.Challenge)
	if !ok {
		http.Error(w, "session not found or expired", http.StatusBadRequest)
		return
	}
	h.sessions.Delete("login:" + clientData.Challenge)

	// Re-inject the body for the webauthn library.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// userHandler finds the user matching the credential.
	userHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		userID := string(userHandle)
		u, err := h.users.GetByID(r.Context(), userID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, fmt.Errorf("user not found")
		}
		return u, nil
	}

	_, err = h.wa.FinishDiscoverableLogin(userHandler, *sessionData, r)
	if err != nil {
		http.Error(w, "authentication failed: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Decode userHandle to get userID, then return their token.
	var credResp struct {
		Response struct {
			UserHandle string `json:"userHandle"`
		} `json:"response"`
	}
	_ = json.Unmarshal(bodyBytes, &credResp)

	var userID string
	if credResp.Response.UserHandle != "" {
		raw, decErr := base64.RawURLEncoding.DecodeString(credResp.Response.UserHandle)
		if decErr == nil {
			userID = string(raw)
		}
	}

	if userID == "" {
		http.Error(w, "could not determine user", http.StatusInternalServerError)
		return
	}

	u, err := h.users.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":  u.Token,
		"userID": u.ID,
	})
}

// scheme returns "https" or "http" based on request headers.
func scheme(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}
