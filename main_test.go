package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func render(t *testing.T, name string, data map[string]interface{}) string {
	t.Helper()
	pt, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if data["UILang"] == nil {
		data["UILang"] = "en"
	}
	if data["CommentCount"] == nil {
		data["CommentCount"] = 0
	}
	if data["TradukData"] == nil {
		data["TradukData"] = map[string]interface{}{"UILang": "en", "UserLang": "en", "ContentID": "x"}
	}
	var b bytes.Buffer
	if err := pt.ExecuteTemplate(&b, name, data); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return b.String()
}

var imgTag = regexp.MustCompile(`<img [^>]*>`)

// A picture exercise must not give its answer away: neither the picture's
// URL nor its alt text may contain the word.
func assertNoLeak(t *testing.T, html, word string) {
	t.Helper()
	for _, img := range imgTag.FindAllString(html, -1) {
		if strings.Contains(strings.ToLower(img), word) {
			t.Errorf("picture gives away %q: %s", word, img)
		}
	}
}

func TestPictureExercisesRender(t *testing.T) {
	mc := &model.ContentItem{Slug: "bildo-hundo-elekto", Type: "multiplechoice", Rating: 820,
		Content: map[string]interface{}{"question": "Kio estas tio?", "bildo": "hundo",
			"options": []interface{}{"kato", "hundo", "muso", "urso"}, "correct_index": 1}}
	html := render(t, "ekzerco.html", map[string]interface{}{"Item": mc})
	dog := bildoj.ForVocab("hundo", "").URL()
	if !strings.Contains(html, dog) {
		t.Errorf("picture exercise has no picture %s", dog)
	}
	assertNoLeak(t, html, "hundo")
	// An activity runs on the stage, without the site header and footer.
	if strings.Contains(html, `class="sh"`) || strings.Contains(html, `class="vojo"`) || !strings.Contains(html, `id="dock"`) {
		t.Error("the exercise is not on the stage alone")
	}
	// with definitions, the alt is the word in the learner's language
	mc.Content["definitions"] = map[string]interface{}{"en": "dog"}
	if html = render(t, "ekzerco.html", map[string]interface{}{"Item": mc}); !strings.Contains(html, `alt="dog"`) {
		t.Error("the picture's alt is not the learner-language definition")
	}

	// A vocab word with a picture and no definition in the learner's
	// language: the picture is the prompt, and the word is not shown.
	voc := &model.ContentItem{Slug: "bildo-kato", Type: "vocab", Rating: 860,
		Content: map[string]interface{}{"word": "kato", "definitions": map[string]interface{}{"de": "Katze"}}}
	html = render(t, "ekzerco.html", map[string]interface{}{"Item": voc, "VocabModo": ""})
	if !strings.Contains(html, bildoj.ForVocab("kato", "").URL()) {
		t.Error("vocab item has no picture")
	}
	assertNoLeak(t, html, "kato")
	if !strings.Contains(html, "Write the word in Esperanto") || !strings.Contains(html, `alt="A picture of the word to find"`) {
		t.Error("no picture prompt for a learner without a definition in their language")
	}
	if strings.Contains(html, `class="vocab-word">kato<`) {
		t.Error("the picture prompt shows the word it asks for")
	}
	// A Zagreb root with no definition in the learner's language asks for
	// the root (its answer is "akv", not "akvo").
	bare := &model.ContentItem{Slug: "voc-zagr-akv", Type: "vocab", Tags: []string{"vortaro", "zagr", "radiko"},
		Content: map[string]interface{}{"word": "akv", "definitions": map[string]interface{}{"de": "Wasser"}}}
	html = render(t, "ekzerco.html", map[string]interface{}{"Item": bare, "VocabModo": "", "IsRadiko": true})
	if strings.Contains(html, "Write the word in Esperanto") || !strings.Contains(html, `<span class="tag">radiko</span>`) {
		t.Error("a root's picture prompt does not ask for the root")
	}
	// A Zagreb root gets its word's picture too.
	root := &model.ContentItem{Slug: "voc-zagr-akv", Type: "vocab", Tags: []string{"radiko"},
		Content: map[string]interface{}{"word": "akv", "definitions": map[string]interface{}{"en": "water"}}}
	if html = render(t, "ekzerco.html", map[string]interface{}{"Item": root, "VocabModo": ""}); !strings.Contains(html, bildoj.ForVocab("akvo", "").URL()) {
		t.Error("the root akv has no picture")
	}
}

func TestReadingPictures(t *testing.T) {
	a0 := &model.ContentItem{Slug: "bildoj-bestoj-teksto", Type: "reading", Rating: 810, SeriesSlug: "bildoj-bestoj",
		Tags: []string{"A0"}, Content: map[string]interface{}{"title": "Bildoj: Bestoj", "text": "Jen hundo.\nJen kato."}}
	html := render(t, "ekzerco.html", map[string]interface{}{"Item": a0})
	if !strings.Contains(html, `<ul class="pic-glossary"`) || strings.Contains(html, "pic-glossary-details") {
		t.Error("an A0 reading shows its pictures open")
	}
	zagr := &model.ContentItem{Slug: "zagr-l03-teksto", Type: "reading", Rating: 1050, SeriesSlug: "zagr-l03",
		Content: map[string]interface{}{"title": "Leciono 3: Vojaĝo", "text": "Ili vidis kafejon kaj en ĝi kukojn."}}
	html = render(t, "ekzerco.html", map[string]interface{}{"Item": zagr})
	if !strings.Contains(html, bildoj.Lesson(3)) {
		t.Error("lesson 3 has no place picture")
	}
	if !strings.Contains(html, "pic-glossary-details") {
		t.Error("a Zagreb reading folds its pictures away")
	}
}

func TestPagesRender(t *testing.T) {
	html := render(t, "fontoj.html", map[string]interface{}{"Pictures": bildoj.All()})
	if !strings.Contains(html, "CC BY-SA 4.0") || !strings.Contains(html, "OpenMoji") {
		t.Error("credits page lacks the OpenMoji attribution")
	}
	if n := strings.Count(html, "<tr>"); n != len(bildoj.All())+1 {
		t.Errorf("credits page has %d rows, want %d", n-1, len(bildoj.All()))
	}
	// The pages around the stage render with no user (a first visit).
	render(t, "hejmo.html", map[string]interface{}{})
	render(t, "mapo.html", map[string]interface{}{})
	render(t, "gramatiko.html", map[string]interface{}{})
	render(t, "vojkrucigxo.html", map[string]interface{}{"Angla": "https://angla.org/?de=esperanto-kurso"})
	html = render(t, "hejmo.html", map[string]interface{}{"FromAngla": true})
	if !strings.Contains(html, "angla.org") {
		t.Error("an arrival from angla.org is not greeted")
	}
}

// A page carries the token of the user it was rendered for, so eo.js can
// replace a stale anonymous token in localStorage after a cookie login
// (the cookie itself is HttpOnly). No user or no token, no meta.
func TestPageCarriesUserToken(t *testing.T) {
	u := &model.User{ID: "u-cookie", Token: "tok-from-cookie", UILang: "en", Rating: 1500, RD: 350}
	for _, page := range []string{"hejmo.html", "enskribi.html", "mapo.html"} {
		html := render(t, page, map[string]interface{}{"User": u})
		if !strings.Contains(html, `<meta name="eo-token" content="tok-from-cookie">`) {
			t.Errorf("%s does not carry the cookie user's token", page)
		}
	}
	if html := render(t, "hejmo.html", map[string]interface{}{}); strings.Contains(html, `name="eo-token"`) {
		t.Error("a page without a user carries a token meta")
	}
	if html := render(t, "hejmo.html", map[string]interface{}{"User": &model.User{ID: "gae-admin", Role: "admin"}}); strings.Contains(html, `name="eo-token"`) {
		t.Error("a tokenless user (GAE admin) gets an empty token meta")
	}
}
