package config

import (
	"os"
	"strings"
)

type Config struct {
	ProjectID      string
	Port           string
	WebAuthnRPID   string
	WebAuthnOrigin string
	// BuilderKeyHash: the SHA-256 (hex) of the builder's key, from
	// EK_BUILDER_KEY_SHA256 ("" the builder API is off; handler/builder.go).
	BuilderKeyHash string
}

func Load() *Config {
	projectID := os.Getenv("FIRESTORE_PROJECT_ID")
	if projectID == "" {
		projectID = "esperanto-kurso"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	rpID := os.Getenv("WEBAUTHN_RPID")
	if rpID == "" {
		rpID = "localhost"
	}
	origin := os.Getenv("WEBAUTHN_ORIGIN")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	return &Config{
		ProjectID:      projectID,
		Port:           port,
		WebAuthnRPID:   rpID,
		WebAuthnOrigin: origin,
		BuilderKeyHash: strings.ToLower(strings.TrimSpace(os.Getenv("EK_BUILDER_KEY_SHA256"))),
	}
}
