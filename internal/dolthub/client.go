// Package dolthub provides a read-only client for the DoltHub SQL API
// against the lapingvino/esperantaj-vortaroj dictionary repository.
package dolthub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiBase = "https://www.dolthub.com/api/v1alpha1/lapingvino/esperantaj-vortaroj/main"

// langMap translates app language codes to DoltHub codes where they differ.
var langMap = map[string]string{
	"zh":    "cmn",
	"zh-tw": "zh-tw",
}

// sourceNames maps internal fonto values to display names.
var sourceNames = map[string]string{
	"komputeko": "Komputeko",
	"revo":      "Reta Vortaro",
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

type apiResponse struct {
	Rows []map[string]json.RawMessage `json:"rows"`
}

// VocabResult holds the result of a single DoltHub vocab lookup.
type VocabResult struct {
	// Suggestions are translation strings in the requested user language.
	Suggestions []string
	// EoDefinition is the Esperanto-language definition of the word.
	EoDefinition string
	// Source is the human-readable source name ("Komputeko" or "Reta Vortaro").
	Source string
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func mapLang(lang string) string {
	if mapped, ok := langMap[lang]; ok {
		return mapped
	}
	return lang
}

// LookupVocab fetches the Esperanto definition (with source) and up to 5
// translation suggestions in userLang for the given Esperanto word, in a
// single DoltHub query. Returns nil result (no error) when nothing is found.
func LookupVocab(word, userLang string) (*VocabResult, error) {
	if word == "" {
		return nil, nil
	}
	dbLang := mapLang(userLang)

	// Build the IN clause — always include eo; add userLang if different.
	langs := "'eo'"
	if dbLang != "eo" {
		langs += fmt.Sprintf(", '%s'", escapeSQL(dbLang))
	}

	q := fmt.Sprintf(
		`SELECT t.traduko, t.lingvo, v.fonto FROM vortoj v JOIN tradukoj t ON v.id = t.vorto_id WHERE v.vorto = '%s' AND t.lingvo IN (%s) LIMIT 15`,
		escapeSQL(word), langs,
	)

	resp, err := httpClient.Get(apiBase + "?q=" + url.QueryEscape(q))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	out := &VocabResult{}
	for _, row := range result.Rows {
		traduko := jsonStr(row["traduko"])
		lingvo := jsonStr(row["lingvo"])
		fonto := jsonStr(row["fonto"])
		if traduko == "" {
			continue
		}
		if lingvo == "eo" && out.EoDefinition == "" {
			out.EoDefinition = traduko
			if name, ok := sourceNames[fonto]; ok {
				out.Source = name
			} else {
				out.Source = fonto
			}
		} else if lingvo == dbLang && len(out.Suggestions) < 5 {
			out.Suggestions = append(out.Suggestions, traduko)
		}
	}

	if out.EoDefinition == "" && len(out.Suggestions) == 0 {
		return nil, nil
	}
	return out, nil
}

func jsonStr(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
