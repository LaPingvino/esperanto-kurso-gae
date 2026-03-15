// Package locale provides UI string lookup and Accept-Language detection.
package locale

import (
	"embed"
	"encoding/json"
	"path/filepath"
	"strings"
)

//go:embed *.json
var localeFiles embed.FS

// translations maps lang code → key → translated string.
var translations map[string]map[string]string

func init() {
	translations = make(map[string]map[string]string)
	entries, err := localeFiles.ReadDir(".")
	if err != nil {
		panic("locale: failed to list files: " + err.Error())
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		b, err := localeFiles.ReadFile(e.Name())
		if err != nil {
			panic("locale: failed to read " + e.Name() + ": " + err.Error())
		}
		m := make(map[string]string)
		if err := json.Unmarshal(b, &m); err != nil {
			panic("locale: failed to parse " + e.Name() + ": " + err.Error())
		}
		translations[code] = m
	}
}

// Supported reports whether lang is a supported UI language.
func Supported(lang string) bool {
	_, ok := translations[lang]
	return ok
}

// T looks up key in the given lang. Falls back to "eo", then returns the key itself.
func T(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	// Fallback to Esperanto.
	if m, ok := translations["eo"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

// rtlLangs is the set of supported languages written right-to-left.
var rtlLangs = map[string]bool{
	"ar": true, // Arabic
	"he": true, // Hebrew
	"fa": true, // Persian / Farsi
	"ur": true, // Urdu
}

// IsRTL reports whether lang uses right-to-left text direction.
func IsRTL(lang string) bool {
	return rtlLangs[lang]
}

// DetectLang parses an Accept-Language header value and returns the best
// supported language. Defaults to "en" if no match is found.
func DetectLang(acceptLang string) string {
	if acceptLang == "" {
		return "en"
	}
	// Parse comma-separated language tags (ignoring quality values for simplicity).
	for _, part := range strings.Split(acceptLang, ",") {
		// Strip quality value (e.g. "en-US;q=0.9" → "en-US").
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		tag = strings.ToLower(tag)
		// Exact match.
		if Supported(tag) {
			return tag
		}
		// Match on primary subtag (e.g. "en-US" → "en").
		primary := strings.SplitN(tag, "-", 2)[0]
		if Supported(primary) {
			return primary
		}
	}
	return "en"
}
