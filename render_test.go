package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/seed"
)

// seedItem reads one item from the seed files by slug.
func seedItem(t *testing.T, slug string) *model.ContentItem {
	t.Helper()
	files, _ := seed.FS.ReadDir(".")
	for _, f := range files {
		raw, _ := seed.FS.ReadFile(f.Name())
		var items []*model.ContentItem
		if json.Unmarshal(raw, &items) != nil {
			continue
		}
		for _, it := range items {
			if it.Slug == slug {
				return it
			}
		}
	}
	t.Fatalf("no seed item %s", slug)
	return nil
}

// TestRenderVocab renders a vocab card and the vortaro list through the real
// templates with the dictionary meanings. EK_RENDER_DIR=dir writes the pages
// there (for screenshots).
func TestRenderVocab(t *testing.T) {
	tmpl, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	out := os.Getenv("EK_RENDER_DIR")
	write := func(name string, b []byte) {
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	// A card the course has no meaning for, for a Lithuanian learner: Lithuanian
	// is new, from ReVo; the card is one of the 335 without a definition.
	item := seedItem(t, "voc-auto-knabo")
	user := &model.User{ID: "test-user-1", Lang: "lt", UILang: "en", Rating: 1100, RD: 200}
	m := handler.MeaningFor(item, "lt", "")
	data := map[string]interface{}{
		"User": user, "Item": item, "UILang": "en", "Meaning": m,
		"TradukData": map[string]interface{}{
			"ContentID": item.Slug, "UserLang": "lt", "UILang": "en", "User": user, "TranslationVotes": map[string]int{},
			"MyLangTranslations": []*model.Translation{}, "OtherTranslations": []*model.Translation{},
			"Suggestions": m.Suggestions,
		},
		"CommentCount": 0, "CommentCountMyLang": 0, "VocabModo": "",
	}
	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, "ekzerco.html", data); err != nil {
		t.Fatal(err)
	}
	page := b.String()
	for _, want := range []string{"berniukas", "Reta Vortaro", "https://vortaro.net/#knabo_kd", "https://reta-vortaro.de/revo/#knab.0o"} {
		if !strings.Contains(page, want) {
			t.Errorf("vocab page lacks %q", want)
		}
	}
	write("ekzerco.html", b.Bytes())

	// Every vocab branch of the template: three modes, with and without a
	// user, a related-word hit, no ReVo hit, a community translation, an
	// affix card, an Esperanto learner.
	cases := []struct {
		slug, lang, community string
		user                  bool
	}{
		{"voc-auto-knabo", "lt", "", true},
		{"voc-auto-knabo", "lt", "", false},
		{"voc-auto-trankvile", "de", "", true},
		{"voc-auto-librojvendejo", "de", "", true},
		{"voc-auto-librojvendejo", "xx", "", false},
		{"voc-auto-amiko", "nl", "vriend", true},
		{"vortaro-sufikso-ad", "ro", "", true},
		{"vortaro-radiko-hom", "eo", "", true},
	}
	for _, c := range cases {
		it := seedItem(t, c.slug)
		m := handler.MeaningFor(it, c.lang, c.community)
		var u *model.User
		if c.user {
			u = &model.User{ID: "test-user-3", Lang: c.lang, UILang: "eo", Rating: 1100, RD: 200}
		}
		var mine []*model.Translation
		if c.community != "" {
			mine = append(mine, &model.Translation{Language: c.lang, Text: c.community})
		}
		for _, modo := range []string{"", "karto-eo", "karto-def"} {
			d := map[string]interface{}{
				"User": u, "Item": it, "UILang": "eo", "Meaning": m, "VocabModo": modo,
				"VocabKind": map[bool]string{true: "sufikso", false: ""}[c.slug == "vortaro-sufikso-ad"],
				"TradukData": map[string]interface{}{
					"ContentID": it.Slug, "UserLang": c.lang, "UILang": "eo", "User": u, "TranslationVotes": map[string]int{},
					"MyLangTranslations": mine, "OtherTranslations": []*model.Translation{}, "Suggestions": m.Suggestions,
				},
				"CommentCount": 0, "CommentCountMyLang": 0,
			}
			b.Reset()
			if err := tmpl.ExecuteTemplate(&b, "ekzerco.html", d); err != nil {
				t.Errorf("%s %s modo=%q user=%v: %v", c.slug, c.lang, modo, c.user, err)
				continue
			}
			if m.Text != "" && !strings.Contains(b.String(), template.HTMLEscapeString(m.Text)) {
				t.Errorf("%s %s modo=%q: meaning %q not on the page", c.slug, c.lang, modo, m.Text)
			}
		}
	}

	// The list, for a Romanian learner: the course's Romanian column is
	// the French text copied; ReVo's Romanian replaces it where it has one.
	var items []*model.ContentItem
	meanings := map[string]handler.Meaning{}
	for _, s := range []string{"voc-auto-afero", "vortaro-radiko-afer", "voc-auto-amiko", "voc-auto-trankvile", "vortaro-koloro-verda", "voc-auto-kato", "voc-auto-loko", "vortaro-radiko-hom", "voc-auto-librojvendejo", "voc-auto-vorto"} {
		it := seedItem(t, s)
		items = append(items, it)
		meanings[s] = handler.MeaningFor(it, "ro", "")
	}
	b.Reset()
	ro := &model.User{ID: "test-user-2", Lang: "ro", UILang: "en", Rating: 1100, RD: 200}
	if err := tmpl.ExecuteTemplate(&b, "vortaro.html", map[string]interface{}{
		"User": ro, "Items": items, "Meanings": meanings, "Page": 1, "TotalPages": 1,
		"Total": len(items), "UILang": "en",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "ReVo") || !strings.Contains(b.String(), "vortaro.net/#") {
		t.Errorf("vortaro page lacks the ReVo marks or PIV links")
	}
	write("vortaro.html", b.Bytes())
}
