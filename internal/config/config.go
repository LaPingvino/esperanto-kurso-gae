package config

import "os"

type Config struct {
	ProjectID    string
	Port         string
	WebAuthnRPID string
	WebAuthnOrigin string
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
	}
}
