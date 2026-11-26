package gamedata

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/lingvoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
	"github.com/LaPingvino/esperanto-kurso-gae/seed"
)

var markRe = regexp.MustCompile(`^\(([a-z-]+)\) `)

// TestMeaningsInTheirLanguage: every meaning the game API serves for
// ?lingvo=X is in X's script, or starts with the language it is in,
// "(en) …", and is in that one's. The bug: ?lingvo=de gave "multe" as
// "πολύ …", the course's only (Greek) definition, unmarked.
func TestMeaningsInTheirLanguage(t *testing.T) {
	files, _ := fs.Glob(seed.FS, "*.json")
	seen := map[string]bool{}
	var cards []*model.ContentItem
	for _, f := range files {
		raw, _ := seed.FS.ReadFile(f)
		var items []*model.ContentItem
		if json.Unmarshal(raw, &items) != nil {
			continue
		}
		for _, it := range items {
			if it.Type == "vocab" && !seen[it.Slug] {
				seen[it.Slug] = true
				cards = append(cards, it)
			}
		}
	}
	bad := 0
	for _, l := range lingvoj.Vocab() {
		for _, it := range cards {
			m := Meaning(defsOf(it), lookup(it), l.Code)
			if m == "" {
				continue
			}
			lang, text := l.Code, m
			if g := markRe.FindStringSubmatch(m); g != nil {
				lang, text = g[1], strings.TrimPrefix(m, g[0])
				if lang == l.Code {
					t.Errorf("%s %q: marked with its own language: %q", l.Code, it.Word(), m)
				}
			}
			if lingvoj.WrongScript(text, lang) {
				bad++
				if bad <= 30 {
					t.Errorf("?lingvo=%s card %q: %q is not in %s's script", l.Code, it.Word(), m, lang)
				}
			}
		}
	}
	if bad > 30 {
		t.Errorf("… %d in all", bad)
	}
}

func TestMeaningRegression(t *testing.T) {
	multe := &model.ContentItem{Type: "vocab", Content: map[string]any{
		"word": "multe", "definitions": map[string]any{"el": "πολύ (προηγείται ή έπεται, μόνο ρήματα)"},
	}}
	if got := Meaning(defsOf(multe), lookup(multe), "de"); strings.ContainsAny(got, "πολύ") || got == "" {
		t.Errorf("multe for de = %q: want ReVo's German, never the Greek", got)
	}
	if got := Meaning(defsOf(multe), nil, "de"); got != "(el) πολύ (προηγείται ή έπεται, μόνο ρήματα)" {
		t.Errorf("without ReVo the Greek must carry its language: %q", got)
	}
}

// TestLessonsInTheirLanguage runs the whole seed course through the API's
// writer (/ludo-api/leciono/{slug}?lingvo=X) and checks every meaning on
// the word cards (C) and vocab questions (Q).
func TestLessonsInTheirLanguage(t *testing.T) {
	files, _ := fs.Glob(seed.FS, "*.json")
	var all []*model.ContentItem
	seen := map[string]bool{}
	for _, f := range files {
		raw, _ := seed.FS.ReadFile(f)
		var items []*model.ContentItem
		if json.Unmarshal(raw, &items) != nil {
			continue
		}
		for _, it := range items {
			if !seen[it.Slug] {
				seen[it.Slug] = true
				it.Status = "approved"
				all = append(all, it)
			}
		}
	}
	c := Build(all)
	checked := 0
	for _, lang := range []string{"de", "lt", "ro", "ru", "hi", "ja"} {
		for _, p := range c.Places {
			var b strings.Builder
			WriteLesson(&b, c, p, lang)
			for _, ln := range strings.Split(b.String(), "\n") {
				f := strings.Split(ln, "\t")
				var ms []string
				switch {
				case f[0] == "C" && len(f) > 4:
					ms = f[4:]
				case f[0] == "Q" && len(f) > 2:
					ms = f[2:3]
				}
				for _, m := range ms {
					checked++
					l, text := lang, m
					if g := markRe.FindStringSubmatch(m); g != nil {
						l, text = g[1], strings.TrimPrefix(m, g[0])
					}
					if lingvoj.WrongScript(text, l) {
						t.Errorf("%s ?lingvo=%s: %q is not in %s's script", p.Slug, lang, m, l)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no meanings checked: did the course build?")
	}
	t.Logf("%d meanings checked", checked)
}

// TestMeaningCourseEoBeforeRevo: falling back to Esperanto, the course's
// own definition comes before ReVo's; ReVo's stays the fallback.
func TestMeaningCourseEoBeforeRevo(t *testing.T) {
	hit := &revo.Hit{Entry: &revo.Entry{Def: "ReVo-difino."}}
	own := "Malgranda vira homo."
	if got := Meaning(map[string]string{"eo": own}, hit, "xx"); got != "(eo) "+own {
		t.Errorf("Meaning = %q, want the course's Esperanto definition", got)
	}
	if got := Meaning(nil, hit, "xx"); got != "(eo) ReVo-difino." {
		t.Errorf("Meaning without one = %q, want ReVo's", got)
	}
}
