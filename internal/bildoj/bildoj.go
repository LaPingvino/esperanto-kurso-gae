// Package bildoj gives the site's pictures: a picture per word (through
// vortbildoj.json, see README.md), a picture per Zagreb lesson (the lesson's
// place on the game's map) and a picture per series where one fits.
//
// Pictures map to words through data, never by hand in templates: a word gets
// a picture by a row in vortbildoj.json, and every vocab item, reading and
// picture exercise with that word shows it.
package bildoj

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Base is the URL every picture file is served under (static/bildoj/).
const Base = "/static/bildoj/"

// Picture is one row of vortbildoj.json.
type Picture struct {
	Word    string `json:"word"`            // the Esperanto word, in its dictionary form (hundo, ruĝa, manĝi)
	Topic   string `json:"topic"`           // the A0 topic it belongs to (bestoj, koloroj, …)
	File    string `json:"file"`            // path under static/bildoj/
	Source  string `json:"source"`          // where the picture comes from
	Author  string `json:"author"`          // its author
	License string `json:"license"`         // its licence
	Exact   bool   `json:"exact,omitempty"` // match the word only as written (numbers): no root matching
	Group   string `json:"group,omitempty"` // pictures too alike to be told apart: never options of one exercise
	Set     string `json:"set,omitempty"`   // pictures of one drawing (the family tree): their exercises take options only from the set
}

// URL is where the picture is served.
func (p *Picture) URL() string { return Base + p.File }

//go:embed vortbildoj.json
var data []byte

var (
	all   []*Picture
	exact = map[string]*Picture{} // word as written
	roots = map[string]*Picture{} // root (word less its ending), for words that are not Exact
)

func init() {
	if err := json.Unmarshal(data, &all); err != nil {
		panic("bildoj: vortbildoj.json: " + err.Error())
	}
	for _, p := range all {
		w := strings.ToLower(p.Word)
		exact[w] = p
		if p.Exact {
			continue
		}
		if r, ok := root(w); ok {
			roots[r] = p
		}
	}
}

// All returns every word picture, in the file's order (by topic).
func All() []*Picture { return all }

// endings are the grammatical endings a root can carry, longest first.
var endings = []string{
	"ojn", "ajn", "oj", "aj", "on", "an", "en",
	"as", "is", "os", "us",
	"o", "a", "e", "i", "u",
}

// root strips a grammatical ending from w; ok is false when w has none or
// the root left would be shorter than two letters.
func root(w string) (string, bool) {
	for _, e := range endings {
		if r, found := strings.CutSuffix(w, e); found && len([]rune(r)) >= 2 {
			return r, true
		}
	}
	return "", false
}

// ForToken returns the picture for a word as it stands in a text (domon,
// manĝis, ruĝaj), or nil. A word with no grammatical ending matches only as
// written, so "vin" never shows wine and "tre" never shows three.
func ForToken(tok string) *Picture {
	w := strings.ToLower(strings.TrimSpace(tok))
	if p := exact[w]; p != nil {
		return p
	}
	if r, ok := root(w); ok {
		return roots[r]
	}
	return nil
}

// ForVocab returns the picture for a vocab item's word, or nil. The word may
// be a full word (akvo) or a bare root as the Zagreb vocabulary keeps them
// (akv). kind is the item's word kind tag when it has one: affixes
// (sufikso, prefikso) and correlatives (tabelvorto) never get a picture.
func ForVocab(word, kind string) *Picture {
	switch kind {
	case "sufikso", "prefikso", "tabelvorto":
		return nil
	}
	w := strings.ToLower(strings.TrimSpace(word))
	if strings.ContainsAny(w, " -") {
		return exact[w]
	}
	if p := ForToken(w); p != nil {
		return p
	}
	if _, hasEnding := root(w); !hasEnding {
		return roots[w]
	}
	return nil
}

// InText returns the pictures of the words in text, each once, in the order
// they first appear.
func InText(text string) []*Picture {
	var out []*Picture
	seen := map[*Picture]bool{}
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool { return !isLetter(r) }) {
		if p := ForToken(tok); p != nil && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || strings.ContainsRune("ĉĝĥĵŝŭĈĜĤĴŜŬ", r)
}

// Lesson returns the picture of Zagreb lesson n (1–12): its place on the
// game's map. "" for any other n.
func Lesson(n int) string {
	if n < 1 || n > 12 {
		return ""
	}
	return fmt.Sprintf("%slokoj/l%02d.png", Base, n)
}

// ForSeries returns a picture for a series, or "": the lesson's place for a
// Zagreb lesson (zagr-l01 … zagr-l12), the first word's picture for an A0
// picture series (bildoj-<topic>).
func ForSeries(slug string) string {
	var n int
	if _, err := fmt.Sscanf(slug, "zagr-l%02d", &n); err == nil && len(slug) == len("zagr-l01") {
		return Lesson(n)
	}
	if topic, ok := strings.CutPrefix(slug, "bildoj-"); ok {
		for _, p := range all {
			if p.Topic == topic {
				return p.URL()
			}
		}
	}
	return ""
}

// ForReading returns the lesson picture of a Zagreb lesson's reading text
// (slug zagr-lNN-teksto, or series zagr-lNN), or "".
func ForReading(slug, series string) string {
	if strings.HasPrefix(series, "zagr-l") {
		if u := ForSeries(series); u != "" {
			return u
		}
	}
	if s, ok := strings.CutSuffix(slug, "-teksto"); ok && strings.HasPrefix(s, "zagr-l") {
		return ForSeries(s)
	}
	return ""
}
