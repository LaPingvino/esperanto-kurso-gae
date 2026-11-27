package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/ripetoj"
)

// Ripetoj in the pages: the header's link and count, the meta eo-ripetoj.js
// reads (owner, trace size, rating), the page itself; on a card of a
// session (?ripeto=1) its forms say so; a verdict names its card and grade;
// the choices of a multiple-choice item come in its fixed shuffle.
func TestRipetojRender(t *testing.T) {
	u := &model.User{ID: "u-srs", Token: "tok", UILang: "eo", Rating: 1234, RD: 100,
		Spuro: ripetoj.Note(ripetoj.Note(nil, "a", 3, time.Now()), "b", 1, time.Now())}
	html := render(t, "hejmo.html", map[string]interface{}{"User": u})
	if !strings.Contains(html, `<meta name="eo-ripetoj" content="`+handler.SRSOwner(u)+` 2 1234">`) {
		t.Error("the page lacks the repetition meta")
	}
	if !strings.Contains(html, `href="/ripetoj"`) || !strings.Contains(html, "data-ripetoj-nombro") {
		t.Error("the header lacks Ripetoj and its count")
	}
	for _, f := range handler.AssetFiles {
		if !strings.Contains(html, "/static/"+f) && !strings.HasSuffix(f, ".css") {
			t.Errorf("the page does not load %s", f)
		}
	}
	unplaced := &model.User{ID: "n", RD: 350, Rating: 1500}
	if html := render(t, "hejmo.html", map[string]interface{}{"User": unplaced}); !strings.Contains(html, ` 0 900">`) {
		t.Error("an unplaced learner's cards are not picked near A0")
	}
	page := render(t, "ripetoj.html", map[string]interface{}{"User": u, "Plu": true})
	for _, want := range []string{`id="ripetoj"`, "data-plu", `data-r="komenci"`, `data-r="elsxuti"`, `data-r="restarigi"`, `data-r="forgesi"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the Ripetoj page lacks %s", want)
		}
	}

	opts := []string{"unu", "du", "tri", "kvar"}
	item := &model.ContentItem{Slug: "mc-a1-001", Type: "multiplechoice", Status: "approved", Rating: 1000,
		Content: map[string]interface{}{"question": "?", "options": []interface{}{"unu", "du", "tri", "kvar"}, "correct_index": 0}}
	ex := render(t, "ekzerco.html", map[string]interface{}{"User": u, "Item": item, "Ripeto": true})
	if n := strings.Count(ex, `<input type="hidden" name="ripeto" value="1">`); n != 2 {
		t.Errorf("%d forms carry ripeto, want 2 (answer, ne scias)", n)
	}
	got := regexp.MustCompile(`name="answer" value="([^"]+)"`).FindAllStringSubmatch(ex, -1)
	var order []string
	for _, m := range got {
		order = append(order, m[1])
	}
	var want []string
	for _, j := range ripetoj.Shuffle(4, item.Slug) {
		want = append(want, opts[j])
	}
	if strings.Join(order, ",") != strings.Join(want, ",") || strings.Join(order, ",") == strings.Join(opts, ",") {
		t.Errorf("choices %v, want the shuffle %v", order, want)
	}
	if plain := render(t, "ekzerco.html", map[string]interface{}{"User": u, "Item": item}); strings.Contains(plain, `name="ripeto"`) {
		t.Error("a card outside a session says ripeto")
	}

	v := render(t, "rezulto.html", map[string]interface{}{"User": u, "Item": item, "Correct": true, "SRSSlug": item.Slug, "Grade": 3, "ContinueHref": "/ripetoj?plu=1"})
	if !strings.Contains(v, `data-srs="mc-a1-001" data-grade="3"`) {
		t.Errorf("the verdict lacks its card and grade: %s", v)
	}
	if v := render(t, "rezulto.html", map[string]interface{}{"User": u, "Correct": true}); strings.Contains(v, "data-srs") {
		t.Error("a verdict of no card names one")
	}
}
