package auth

import (
	"github.com/go-webauthn/webauthn/webauthn"
)

// NewWebAuthn creates a configured webauthn.WebAuthn instance.
// rpID should be the domain (e.g. "esperanto-kurso.net" or "localhost").
// rpOrigin should be the full origin (e.g. "https://esperanto-kurso.net").
func NewWebAuthn(rpID, rpOrigin string) (*webauthn.WebAuthn, error) {
	cfg := &webauthn.Config{
		RPDisplayName: "Esperanto-Kurso",
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	}
	return webauthn.New(cfg)
}
