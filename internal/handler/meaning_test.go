package handler

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/lingvoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/seed"
)

func card(word string, defs map[string]interface{}, tags ...string) *model.ContentItem {
	c := map[string]interface{}{"word": word}
	if defs != nil {
		c["definitions"] = defs
	}
	return &model.ContentItem{Slug: "t-" + word, Type: "vocab", Content: c, Tags: tags}
}

func TestMeaningChain(t *testing.T) {
	afer := card("afer", map[string]interface{}{"en": "matter", "fr": "affaire", "ro": "affaire", "de": "Angelegenheit, Sache"}, "radiko")
	cases := []struct {
		name      string
		item      *model.ContentItem
		lang      string
		community string
		text      string // substring
		textLang  string
		source    string
	}{
		{"own definition", afer, "de", "", "Angelegenheit", "de", "course"},
		{"community before ReVo", afer, "lt", "reikalas", "reikalas", "lt", "community"},
		{"ReVo in the learner's language", afer, "lt", "", "dalykas", "lt", "revo"},
		{"a copied definition is not Romanian", afer, "ro", "", "matter", "en", "course"},
		{"no definition: ReVo", card("knabo", nil), "uk", "", "хлоп", "uk", "revo"},
		{"no definition, no ReVo language: ReVo English", card("knabo", nil), "xx", "", "boy", "en", "revo"},
		{"Esperanto learners get the definition", card("knabo", nil), "eo", "", "Virseksa infano", "eo", "revo"},
	}
	for _, c := range cases {
		m := MeaningFor(c.item, c.lang, c.community)
		if !strings.Contains(m.Text, c.text) || m.Lang != c.textLang || m.Source != c.source {
			t.Errorf("%s: got %q (%s, %s), want …%q… (%s, %s)", c.name, m.Text, m.Lang, m.Source, c.text, c.textLang, c.source)
		}
		if m.Fallback != (c.textLang != c.lang) {
			t.Errorf("%s: Fallback = %v", c.name, m.Fallback)
		}
	}
	m := MeaningFor(afer, "lt", "")
	if m.PIVURL != "https://vortaro.net/#afero_kd" {
		t.Errorf("root card PIV link = %q, want the word afero (PIV finds words, not roots)", m.PIVURL)
	}
	if m.ReVoURL != "https://reta-vortaro.de/revo/#afer.0o" {
		t.Errorf("ReVo link = %q", m.ReVoURL)
	}
	none := MeaningFor(card("librojvendejo", nil), "de", "")
	if none.ReVoFound || !strings.Contains(none.ReVoURL, "?q=librojvendejo") || none.PIVURL != "https://vortaro.net/#librojvendejo_kd" {
		t.Errorf("unknown word: %+v", none)
	}
	if got := MeaningFor(card("iom post iom", nil), "de", "").PIVURL; got != "https://vortaro.net/#iom%20post%20iom_kd" {
		t.Errorf("PIV link with spaces = %q", got)
	}
}

func TestEveryVocabLangHasAName(t *testing.T) {
	for _, l := range lingvoj.Vocab() {
		if l.Name == "" || l.Name == l.Code {
			t.Errorf("%s has no name", l.Code)
		}
	}
}

// TestMeaningCoverage reports, per meaning language, how many of the seed's
// vocab cards (the seed files, not the live Datastore) show a meaning in
// that language: before (the course's own definitions, copies of another
// language not counted) and now (with Reta Vortaro). Run with -v.
func TestMeaningCoverage(t *testing.T) {
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
	type row struct {
		lang          string
		before, after int
	}
	var rows []row
	noDef, noDefNow := 0, 0
	for _, it := range cards {
		if len(allDefsOf(it)) == 0 {
			noDef++
			if m := MeaningFor(it, "en", ""); m.Text != "" {
				noDefNow++
			}
		}
	}
	full := 0
	for _, l := range lingvoj.Vocab() {
		if l.Code == "eo" {
			continue
		}
		r := row{lang: l.Code}
		for _, it := range cards {
			if d, copied := courseDef(it.Content, l.Code); d != "" && !copied {
				r.before++
			}
			if m := MeaningFor(it, l.Code, ""); m.Text != "" && !m.Fallback {
				r.after++
			}
		}
		if r.after*2 >= len(cards) {
			full++
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].after > rows[j].after })
	t.Logf("%d vocab cards; %d without any definition, %d of those now have a meaning (in English at least)", len(cards), noDef, noDefNow)
	for _, r := range rows {
		t.Logf("%-4s before %4d  now %4d", r.lang, r.before, r.after)
	}
	t.Logf("%d meaning languages; %d of them cover at least half the cards in that language", len(rows), full)
}

func allDefsOf(it *model.ContentItem) map[string]interface{} {
	d, _ := it.Content["definitions"].(map[string]interface{})
	out := map[string]interface{}{}
	for k, v := range d {
		if s, _ := v.(string); s != "" {
			out[k] = s
		}
	}
	if s, _ := it.Content["definition"].(string); s != "" {
		out["en"] = s
	}
	return out
}
