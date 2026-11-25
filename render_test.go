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
	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
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

// card is a seed item, or, for a shared reading-word card the seed no
// longer has (folded into its core card by the legvortoj cleanup), a card
// of that word without a meaning, as the old ones were.
func card(t *testing.T, slug string) *model.ContentItem {
	t.Helper()
	if w, ok := strings.CutPrefix(slug, "voc-auto-"); ok {
		files, _ := seed.FS.ReadDir(".")
		for _, f := range files {
			raw, _ := seed.FS.ReadFile(f.Name())
			if strings.Contains(string(raw), `"`+slug+`"`) {
				return seedItem(t, slug)
			}
		}
		return &model.ContentItem{Slug: slug, Type: "vocab", Status: "approved", Rating: 1500, RD: 200,
			Content: map[string]interface{}{"word": revo.Normalize(w)}, Tags: []string{"vortaro"}}
	}
	return seedItem(t, slug)
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
	item := card(t, "voc-auto-knabo")
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
		it := card(t, c.slug)
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
		it := card(t, s)
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

// TestRenderReadingWords renders a reading with its words (the tappable
// text, the glossary) and a practice question through the real templates.
// EK_RENDER_DIR=dir writes them there (for screenshots).
func TestRenderReadingWords(t *testing.T) {
	tmpl, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	var all []*model.ContentItem
	files, _ := seed.FS.ReadDir(".")
	for _, f := range files {
		raw, _ := seed.FS.ReadFile(f.Name())
		var items []*model.ContentItem
		if json.Unmarshal(raw, &items) == nil {
			all = append(all, items...)
		}
	}
	reading := seedItem(t, "lit-la-eta-princo-komenco")
	x := legvortoj.NewIndex(all)
	user := &model.User{ID: "test-user-4", Lang: "de", UILang: "eo", Rating: 1500, RD: 200}
	words := handler.Glossary(reading, x, "de")
	var newW, known []*handler.GlossWord
	for _, g := range words {
		if g.New {
			newW = append(newW, g)
		} else {
			known = append(known, g)
		}
	}
	qs := handler.Practice(reading.Slug, words)
	data := map[string]interface{}{
		"User": user, "Item": reading, "UILang": "eo", "Glossary": words, "NewWords": newW, "KnownWords": known,
		"Practice": len(qs), "CommentCount": 0, "CommentCountMyLang": 0,
		"TradukData": map[string]interface{}{"ContentID": reading.Slug, "UserLang": "de", "UILang": "eo", "User": user,
			"TranslationVotes": map[string]int{}, "MyLangTranslations": []*model.Translation{}, "OtherTranslations": []*model.Translation{}},
	}
	var b bytes.Buffer
	if err := tmpl.ExecuteTemplate(&b, "ekzerco.html", data); err != nil {
		t.Fatal(err)
	}
	page := b.String()
	for _, want := range []string{`data-v="v-boao">boaj</a>`, `id="v-boao"`, "/legado/lit-la-eta-princo-komenco/vortoj", "serpent"} {
		if !strings.Contains(page, want) {
			t.Errorf("reading page lacks %q", want)
		}
	}
	if strings.Contains(page, "Schal") {
		t.Error("boao glossed as a scarf")
	}
	if out := os.Getenv("EK_RENDER_DIR"); out != "" {
		os.WriteFile(filepath.Join(out, "legado.html"), b.Bytes(), 0o644)
	}
	if len(qs) == 0 {
		t.Fatal("no practice")
	}
	b.Reset()
	if err := tmpl.ExecuteTemplate(&b, "legvortoj.html", map[string]interface{}{
		"User": user, "UILang": "eo", "Lang": "de", "Reading": reading, "Q": qs[0], "N": 1, "Total": len(qs),
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "gap-slot") || !strings.Contains(b.String(), qs[0].Form) {
		t.Errorf("practice page lacks the gap or the answer tile")
	}
	if out := os.Getenv("EK_RENDER_DIR"); out != "" {
		os.WriteFile(filepath.Join(out, "legvortoj.html"), b.Bytes(), 0o644)
	}
}

// TestRenderPracticeVerdict renders the practice's verdict (rezulto.html)
// with exactly the data the answer handler gives it, with and without a
// card, right and wrong.
func TestRenderPracticeVerdict(t *testing.T) {
	tmpl, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	reading := seedItem(t, "lit-la-eta-princo-komenco")
	words := handler.Glossary(reading, legvortoj.NewIndex(nil), "de")
	qs := handler.Practice(reading.Slug, words)
	if len(qs) < 2 {
		t.Fatal("no practice")
	}
	u := &model.User{ID: "test-user-5", Lang: "de", UILang: "eo", Rating: 1500, RD: 200}
	cardItem := &model.ContentItem{Slug: "voc-auto-x", Type: "vocab", Content: map[string]interface{}{"word": "x"}}
	for _, c := range []struct {
		card   *model.ContentItem
		answer string
	}{{nil, qs[0].Form}, {nil, "nenio"}, {cardItem, qs[0].Form}, {cardItem, "nenio"}} {
		data := handler.PracticeVerdict(u, qs[0], c.answer, c.card, reading.Slug, 1, len(qs), 3, false)
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, "rezulto.html", data); err != nil {
			t.Fatalf("card %v answer %q: %v", c.card != nil, c.answer, err)
		}
		out := b.String()
		if !strings.Contains(out, "/legado/lit-la-eta-princo-komenco/vortoj?n=2") || !strings.Contains(out, qs[0].Form) {
			t.Errorf("verdict lacks the next question or the answer: %s", out)
		}
		if strings.Contains(out, "/alternativo") {
			t.Error("a cloze answer offered as an alternative for the card")
		}
	}
}
