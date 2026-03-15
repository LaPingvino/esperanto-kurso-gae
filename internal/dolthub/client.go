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

// langMap translates our app's language codes to the codes used in DoltHub where they differ.
var langMap = map[string]string{
	"zh":    "cmn",
	"zh-tw": "zh-tw",
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

type apiResponse struct {
	Rows []map[string]json.RawMessage `json:"rows"`
}

// LookupTranslations returns up to 5 translations of an Esperanto word in the given language
// from the DoltHub esperantaj-vortaroj repository. Returns nil (no error) if nothing is found.
func LookupTranslations(word, lang string) ([]string, error) {
	if word == "" || lang == "" {
		return nil, nil
	}
	dbLang := lang
	if mapped, ok := langMap[lang]; ok {
		dbLang = mapped
	}

	q := fmt.Sprintf(
		`SELECT DISTINCT t.traduko FROM vortoj v JOIN tradukoj t ON v.id = t.vorto_id WHERE v.vorto = '%s' AND t.lingvo = '%s' LIMIT 5`,
		strings.ReplaceAll(word, "'", "''"),
		strings.ReplaceAll(dbLang, "'", "''"),
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

	var out []string
	for _, row := range result.Rows {
		raw, ok := row["traduko"]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil || s == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}
