package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// static/tokens.css is kurso-design's contract, copied verbatim. When the
// design repo sits beside this one (Joop's machine), the copy must match.
func TestTokensAreTheDesigns(t *testing.T) {
	ours, err := os.ReadFile("static/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	theirs, err := os.ReadFile(filepath.Join(home, "Projects", "kurso-design", "tokens.css"))
	if err != nil {
		t.Skip("kurso-design not beside this repo")
	}
	if string(ours) != string(theirs) {
		t.Error("static/tokens.css differs from kurso-design/tokens.css: copy it again")
	}
}

func tokenBlock(t *testing.T, css, selector string) map[string]string {
	t.Helper()
	m := regexp.MustCompile(selector + `\s*\{((?s).*?)\n\s*\}`).FindStringSubmatch(css)
	if m == nil {
		t.Fatalf("tokens.css: no block %s", selector)
	}
	out := map[string]string{}
	for _, kv := range regexp.MustCompile(`--([\w-]+):\s*(#[0-9A-Fa-f]{6})\s*;`).FindAllStringSubmatch(m[1], -1) {
		out[kv[1]] = kv[2]
	}
	return out
}

func luminance(hex string) float64 {
	var c [3]float64
	for i := range c {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		x := float64(v) / 255
		if x <= 0.03928 {
			c[i] = x / 12.92
		} else {
			c[i] = math.Pow((x+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
}

func contrast(a, b string) float64 {
	x, y := luminance(a), luminance(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}

// Every colour pair the stylesheet uses, by the numbers (the method of
// kurso-design/tools/contrast.py): 7:1 for all text, 3:1 for control edges
// and meaningful icons, in both themes. The design's own pairs for the
// Esperanto site first, then the pairs only stilo.css adds.
func TestContrast(t *testing.T) {
	const text, edge = 7.0, 3.0
	b, err := os.ReadFile("static/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	light := tokenBlock(t, css, `(?:^|\n):root`)
	dark := map[string]string{}
	for k, v := range light {
		dark[k] = v
	}
	for k, v := range tokenBlock(t, css, `:root\[data-theme="dark"\]`) {
		dark[k] = v
	}
	pairs := []struct {
		fg, bg string
		need   float64
		use    string
	}{
		// kurso-design's pairs (DESIGN.md "Contrast"), the Esperanto side
		{"ink", "paper", text, "body text"}, {"ink", "surface", text, "cards"}, {"ink", "sunk", text, "wells, inputs"},
		{"ink", "eo-tint", text, "on the tint"}, {"ink", "en-tint", text, "on the angla tint"},
		{"ink", "right-tint", text, "right panel"}, {"ink", "wrong-tint", text, "wrong panel"}, {"ink", "note-tint", text, "note"},
		{"ink-muted", "paper", text, "meta"}, {"ink-muted", "surface", text, "meta in cards"}, {"ink-muted", "sunk", text, "meta in wells"},
		{"ink-muted", "right-tint", text, "why, right"}, {"ink-muted", "wrong-tint", text, "why, wrong"},
		{"eo-text", "paper", text, "links"}, {"eo-text", "surface", text, "links in cards"}, {"eo-text", "eo-tint", text, "level chip, nav place"},
		{"eo-on-fill", "eo-fill", text, "primary button"}, {"en-text", "surface", text, "angla links"},
		{"en-on-fill", "en-fill", text, "Komenci la anglan"},
		{"right-text", "right-tint", text, "Ĝuste!"}, {"right-text", "surface", text, "Farita"},
		{"wrong-text", "wrong-tint", text, "Ne tute."}, {"wrong-text", "surface", text, "wrong mark"},
		{"note-text", "note-tint", text, "missing definition, pending"},
		{"edge", "paper", edge, "borders"}, {"edge", "surface", edge, "borders in cards"}, {"edge", "sunk", edge, "done tile edge"},
		{"eo-fill", "paper", edge, "button shape"}, {"eo-fill", "surface", edge, "button shape in cards"},
		{"eo-line", "paper", edge, "map line"}, {"eo-line", "surface", edge, "progress in cards"}, {"eo-line", "sunk", edge, "progress"},
		{"en-line", "surface", edge, "the angla line in the footer strip"},
		{"eo-on-fill", "eo-line", edge, "tick in a done station"},
		{"right-edge", "surface", edge, "right border"}, {"wrong-edge", "surface", edge, "wrong border"},
		{"surface", "right-edge", edge, "tick on a right badge"}, {"surface", "wrong-edge", edge, "cross on a wrong badge"},
		{"star-edge", "paper", edge, "streak icon"}, {"star-edge", "surface", edge, "star in cards"},
		// pairs only this stylesheet adds
		{"eo-fill", "eo-on-fill", text, "the next card's inverted button and icon"},
		{"right-tint", "right-text", text, "Daŭrigi on the right dock"},
		{"wrong-tint", "wrong-text", text, "Daŭrigi on the wrong dock, the reject button"},
		{"paper", "ink", text, "the pressed theme button, tooltips"},
		{"right-text", "sunk", text, "Farita on a done line"},
		{"wrong-text", "paper", text, "outline reject button"},
		{"en-text", "paper", text, "the way to angla.org on the page"},
		{"eo-text", "sunk", text, "the series picker's current item"},
		{"eo-fill", "eo-tint", edge, "the practice card's icon tile"},
		{"en-fill", "surface", edge, "the ghost button to angla.org"},
		{"edge", "right-tint", edge, "inputs in the right dock"},
		{"edge", "wrong-tint", edge, "inputs in the wrong dock"},
	}
	for _, p := range pairs {
		for name, th := range map[string]map[string]string{"light": light, "dark": dark} {
			fg, bg := th[p.fg], th[p.bg]
			if fg == "" || bg == "" {
				t.Fatalf("no token --%s or --%s", p.fg, p.bg)
			}
			if r := contrast(fg, bg); r < p.need {
				t.Errorf("%s: --%s on --%s is %.2f < %.0f (%s)", name, p.fg, p.bg, r, p.need, p.use)
			}
		}
	}
	// No colour literal may slip into the components: colours come from the tokens.
	sb, _ := os.ReadFile("static/stilo.css")
	for _, m := range regexp.MustCompile(`#[0-9A-Fa-f]{3,6}\b`).FindAllString(string(sb), -1) {
		if m != "#fff" { // the QR code's white backing
			t.Errorf("stilo.css has a colour literal %s: use a token", m)
		}
	}
}

// The locales that have the newest keys must all have every key the
// Esperanto file has (the others fall back to Esperanto).
func TestLocaleParity(t *testing.T) {
	read := func(code string) map[string]string {
		b, err := os.ReadFile("internal/locale/" + code + ".json")
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	eo := read("eo")
	for _, code := range []string{"en", "de", "es", "fr", "it", "nl", "pt", "tet"} {
		m := read(code)
		var missing []string
		for k := range eo {
			if m[k] == "" {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s lacks %s", code, strings.Join(missing, ", "))
		}
	}
	// Emoji show as empty boxes on machines without a colour-emoji font.
	files, _ := filepath.Glob("internal/locale/*.json")
	files2, _ := filepath.Glob("templates/*.html")
	files3, _ := filepath.Glob("templates/*/*.html")
	emoji := regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{26FF}]|&#1[0-9]{5};`)
	for _, f := range append(append(files, files2...), files3...) {
		b, _ := os.ReadFile(f)
		if m := emoji.Find(b); m != nil {
			t.Errorf("%s has an emoji %q: use a drawn icon (templates/partials/ikonoj.html)", f, m)
		}
	}
}
