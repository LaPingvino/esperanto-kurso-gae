package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	localauth "github.com/LaPingvino/esperanto-kurso-gae/internal/auth"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"

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
		http.Error(w, "Interna eraro", http.StatusInternalServerError)
		return
	}
	token, err := localauth.GenerateToken()
	if err != nil {
		http.Error(w, "Interna eraro", http.StatusInternalServerError)
		return
	}
	u := model.NewUser(id, token)
	if err := h.users.Create(r.Context(), u); err != nil {
		http.Error(w, "Ne eblis krei uzanton", http.StatusInternalServerError)
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
		"User":        u,
		"MagicURL":    magicURL,
		"SentMsg":     r.URL.Query().Get("sendita") == "1",
		"NomForigita": r.URL.Query().Get("nomo-forigita") == "1",
		"UILang":      UILangFor(u),
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
		http.Error(w, "Ne aŭtentikigita", http.StatusUnauthorized)
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
		http.Error(w, "Ne aŭtentikigita", http.StatusUnauthorized)
		return
	}

	sessionData, ok := h.sessions.Get("reg:" + u.ID)
	if !ok {
		http.Error(w, "Sesio eksvalidiĝis", http.StatusBadRequest)
		return
	}
	h.sessions.Delete("reg:" + u.ID)

	cred, err := h.wa.FinishRegistration(u, *sessionData, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := h.users.AddPasskey(r.Context(), u.ID, *cred); err != nil {
		http.Error(w, "Ne eblis konservi la ensalutŝlosilon", http.StatusInternalServerError)
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
		http.Error(w, "Malĝusta peto", http.StatusBadRequest)
		return
	}

	// Extract challenge from clientDataJSON to look up the session.
	var parsed struct {
		Response struct {
			ClientDataJSON string `json:"clientDataJSON"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		http.Error(w, "Malĝusta peto", http.StatusBadRequest)
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
		http.Error(w, "Sesio ne trovita aŭ eksvalidiĝis", http.StatusBadRequest)
		return
	}
	h.sessions.Delete("login:" + clientData.Challenge)

	// Re-inject the body for the webauthn library.
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// userHandler finds the user matching the credential.
	// Follows UserAlias redirects so that passkeys registered under a merged
	// (deleted) account still resolve to the surviving account.
	userHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
		userID := h.users.ResolveAlias(r.Context(), string(userHandle))
		u, err := h.users.GetByID(r.Context(), userID)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, fmt.Errorf("uzanto ne trovita")
		}
		return u, nil
	}

	_, err = h.wa.FinishDiscoverableLogin(userHandler, *sessionData, r)
	if err != nil {
		http.Error(w, "Aŭtentikigo malsukcesis: "+err.Error(), http.StatusUnauthorized)
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
		http.Error(w, "Ne eblis identigi uzanton", http.StatusInternalServerError)
		return
	}
	userID = h.users.ResolveAlias(r.Context(), userID)

	u, err := h.users.GetByID(r.Context(), userID)
	if err != nil || u == nil {
		http.Error(w, "Uzanto ne trovita", http.StatusInternalServerError)
		return
	}

	// Set the token cookie so that full-page loads (GET requests) also use the
	// passkey account — not just HTMX requests that carry X-Auth-Token.
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    u.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":  u.Token,
		"userID": u.ID,
	})
}

// SetLang handles POST /lingvo — sets the user's preferred language cookie and updates the user record.
func (h *AuthHandler) SetLang(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta peto", http.StatusBadRequest)
		return
	}
	lang := r.FormValue("lang")
	if lang == "" {
		lang = "en"
	}
	// Sanitise: only allow short alphanumeric lang codes.
	if len(lang) > 8 {
		lang = lang[:8]
	}
	// Set lang cookie (works for anonymous users too).
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
	})
	// Persist to user record if logged in.
	u := UserFromContext(r.Context())
	if u != nil {
		_ = h.users.UpdateLang(r.Context(), u.ID, lang)
	}
	// Redirect back to referrer or home.
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// SetUILang handles POST /uilingvo — sets the user's preferred interface language.
func (h *AuthHandler) SetUILang(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝusta peto", http.StatusBadRequest)
		return
	}
	lang := r.FormValue("ui_lang")
	if lang == "" || len(lang) > 8 {
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "ui_lang",
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(365 * 24 * time.Hour),
	})
	u := UserFromContext(r.Context())
	if u != nil {
		_ = h.users.UpdateUILang(r.Context(), u.ID, lang)
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// scheme returns "https" or "http" based on request headers.
func scheme(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}

// SetUsername handles POST /profilo/nomo — sets a unique display name.
func (h *AuthHandler) SetUsername(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" || len(username) < 3 || len(username) > 30 {
		http.Error(w, "Uzantnomo devas havi 3–30 signojn", http.StatusBadRequest)
		return
	}
	// Allow letters, digits, hyphens, underscores only.
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			http.Error(w, "Uzantnomo povas enhavi nur literojn, ciferojn, streketon, substrekon", http.StatusBadRequest)
			return
		}
	}
	if err := h.users.SetUsername(r.Context(), u.ID, username); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusConflict)
		return
	}
	// Optionally update retention preference at the same time.
	if keepStr := r.FormValue("keep_days"); keepStr != "" {
		if days, err := strconv.Atoi(keepStr); err == nil {
			_ = h.users.UpdateKeepDataDays(r.Context(), u.ID, days)
		}
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/enskribi"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// ClearUsername handles POST /profilo/nomo/forigi — removes the username, making the account anonymous.
func (h *AuthHandler) ClearUsername(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := h.users.ClearUsername(r.Context(), u.ID); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/enskribi?nomo-forigita=1", http.StatusSeeOther)
}

// UpdateKeepDataDays handles POST /profilo/konservado — updates data retention preference.
func (h *AuthHandler) UpdateKeepDataDays(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "Ne ensalutita", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Malĝustaj datumoj", http.StatusBadRequest)
		return
	}
	days, err := strconv.Atoi(r.FormValue("keep_days"))
	if err != nil {
		http.Error(w, "Nevalida valoro", http.StatusBadRequest)
		return
	}
	if err := h.users.UpdateKeepDataDays(r.Context(), u.ID, days); err != nil {
		http.Error(w, "Eraro: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/enskribi", http.StatusSeeOther)
}
