package legvortoj

import (
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func TestLemma(t *testing.T) {
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"knabojn", "knabo", true},
		{"homoj", "homo", true}, // the singular, though ReVo lists homoj too
		{"aĉetita", "aĉeti", true},
		{"bele", "bela", true},
		{"alte", "alta", true},
		{"akrega", "akrega", true}, // akr|eg|a, not ReVo's "akreo" (acre)
		{"gelernanto", "gelernanto", true},
		{"abonpago", "abonpago", true},
		{"malsanulejo", "malsanulejo", true},
		{"boa", "boa", true}, // a word on its own: bo|a, of the in-laws
		{"ami→malami", "", false},
		{"bol", "", false},
		{"kv", "", false},
	} {
		got, _, _, ok := Lemma(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Lemma(%q) = %q %v, want %q %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParts(t *testing.T) {
	for w, want := range map[string]string{
		"malsanulejo": "mal san ul ej o",
		"lernolibro":  "lern o libr o",
		"abonpago":    "abon pag o",
		"gefratoj":    "ge frat oj",
		"knabo":       "knab o",
	} {
		var got []string
		for _, p := range Parts(w) {
			got = append(got, p.Text)
		}
		if strings.Join(got, " ") != want {
			t.Errorf("Parts(%q) = %v, want %s", w, got, want)
		}
	}
	if ps := Parts("malsanulejo"); ps[0].Kind != "prefikso" || ps[1].Kind != "radiko" || ps[2].Kind != "sufikso" || ps[4].Kind != "finaĵo" {
		t.Errorf("malsanulejo kinds: %+v", ps)
	}
}

// La eta princo: "la boaj" is the snakes (boao), as boaon and boaoj in the
// same text say, not the adjective boa; the snake, not the fur scarf.
const etaPrinco = "La batanto iris supren kaj elen kun kvanto da akvo. La diamanto brilis. Tiu bildo prezentis boaon, kiu glutas rabobeston. En la libro oni diris: \"La boaj glutas sian rabaĵon unuglute, sen-maĉe.\" La granduloj konsilis, ke mi flankenlasu desegnojn de boaoj. Marko havas amikon."

func vocab(slug, word string, tags ...string) *model.ContentItem {
	return &model.ContentItem{Slug: slug, Type: "vocab", Status: "approved", Content: map[string]interface{}{"word": word}, Tags: tags}
}

func TestWordsBoao(t *testing.T) {
	x := NewIndex([]*model.ContentItem{vocab("voc-zagr-amik", "amik", "zagr", "radiko"), vocab("voc-auto-boao", "boao")})
	ws := Words("lit-la-eta-princo-komenco", etaPrinco, x)
	var boao *Use
	for _, u := range ws {
		if u.Lemma == "boa" {
			t.Errorf("a word boa from %v", u.Forms)
		}
		if u.Lemma == "boao" {
			boao = u
		}
		if u.Lemma == "amiko" && (u.Card == nil || u.Card.Slug != "voc-zagr-amik" || !u.Core) {
			t.Errorf("amiko: card %+v core %v, want the root card amik", u.Card, u.Core)
		}
		if u.Lemma == "marko" {
			t.Errorf("the name Marko as a word")
		}
	}
	if boao == nil {
		t.Fatal("no boao")
	}
	if strings.Join(boao.Forms, " ") != "boaon boaj boaoj" {
		t.Errorf("boao forms %v", boao.Forms)
	}
	if boao.Card == nil || boao.Card.Slug != "voc-auto-boao" || boao.Core {
		t.Errorf("boao card %+v", boao.Card)
	}
	if boao.Sense != "boa.0o.KOMUNE" || !boao.Checked {
		t.Errorf("boao sense %q checked %v, want the snake, reviewed (sencoj.json)", boao.Sense, boao.Checked)
	}
	if !strings.Contains(boao.EoDef(), "serpent") {
		t.Errorf("boao definition %q", boao.EoDef())
	}
}

func TestIndexPreference(t *testing.T) {
	x := NewIndex([]*model.ContentItem{
		vocab("voc-zagr-knab", "knab", "radiko"),
		vocab("vortaro-knabo", "knabo"),
		vocab("bildo-knabo", "knabo"),
		vocab("voc-auto-knabo", "knabo"),
		{Slug: "voc-auto-domo", Type: "vocab", Status: MergedStatus, Content: map[string]interface{}{"word": "domo"}},
	})
	if it, core := x.Card("knabo"); it.Slug != "bildo-knabo" || !core {
		t.Errorf("knabo → %s %v, want the picture card", it.Slug, core)
	}
	if it, _ := x.Card("domo"); it != nil {
		t.Errorf("a merged card is no card: %s", it.Slug)
	}
}

func TestRating(t *testing.T) {
	for w, want := range map[string]float64{"sana": 1500, "abomena": 1700, "malsanulejo": 1500, "abonpago": 1700} {
		l, h, _, _ := Lemma(w)
		if got := Rating(l, h); got != want {
			t.Errorf("Rating(%s) = %v, want %v", w, got, want)
		}
	}
}

func TestPlan(t *testing.T) {
	items := []*model.ContentItem{
		{Slug: "lit-la-eta-princo-komenco", Type: "reading", Status: "approved", Content: map[string]interface{}{"title": "La eta princo", "text": etaPrinco}},
		vocab("bildo-libro", "libro", "bildvortaro"),
		vocab("voc-zagr-amik", "amik", "radiko"),
		vocab("voc-auto-libro", "libro", "vortaro", "lit-la-eta-princo-komenco"),       // core duplicate
		vocab("voc-auto-amiko", "amiko", "vortaro", "lit-la-eta-princo-komenco"),       // the core root card's word
		vocab("voc-auto-boa", "boa", "vortaro", "lit-la-eta-princo-komenco"),           // made of "la boaj"
		vocab("voc-auto-boao", "boao", "vortaro", "lit-la-eta-princo-komenco"),         // kept
		vocab("voc-auto-desegnoj", "desegnoj", "vortaro", "lit-la-eta-princo-komenco"), // an inflected form
		vocab("voc-auto-ami→malami", "ami→malami", "vortaro", "pmeg-vortfarado"),       // no word
		vocab("voc-auto-bat", "bat", "vortaro", "lit-la-eta-princo-komenco"),           // cut from batanto: the root's verb
		vocab("voc-auto-kv", "kv", "vortaro", "lit-la-eta-princo-komenco"),             // cut from kvanto: no word
		{Slug: "voc-auto-diam", Type: "vocab", Status: MergedStatus, Tags: []string{"vortaro", "lit-la-eta-princo-komenco"}, // retired by a run; cut from diamanto
			Content: map[string]interface{}{"word": "diam", MergedIntoKey: "", UndoKey: []interface{}{map[string]interface{}{"run": "lv-1"}}}},
		{Slug: "voc-auto-metr", Type: "vocab", Status: MergedStatus, Tags: []string{"vortaro", "lit-la-eta-princo-komenco"}, // retired; its text has no metro…: stays
			Content: map[string]interface{}{"word": "metr", MergedIntoKey: "", UndoKey: []interface{}{map[string]interface{}{"run": "lv-1"}}}},
		vocab("voc-auto-kilom", "kilom", "vortaro", "lit-la-eta-princo-komenco"), // not in its text: retired, for review
		vocab("voc-auto-supre", "supre", "vortaro", "lit-la-eta-princo-komenco"), // a word, though the text has supren
		vocab("voc-auto-ele", "ele", "vortaro", "lit-la-eta-princo-komenco"),     // cut from elen
		{Slug: "voc-auto-glutas", Type: "vocab", Status: "approved", AuthorID: "someone", Content: map[string]interface{}{"word": "glutas", "definitions": map[string]interface{}{"en": "swallows"}}},
	}
	got := map[string]Action{}
	for _, a := range Plan(items) {
		got[a.Slug] = a
	}
	want := map[string]struct{ kind, into string }{
		"voc-auto-libro":      {"core", "bildo-libro"},
		"voc-auto-amiko":      {"core", "voc-zagr-amik"},
		"voc-auto-boa":        {"lemma", "voc-auto-boao"},
		"voc-auto-boao":       {"keep", ""},
		"voc-auto-desegnoj":   {"lemma", "voc-auto-desegno"},
		"voc-auto-desegno":    {"create", ""},
		"voc-auto-ami→malami": {"junk", ""},
		"voc-auto-glutas":     {"keep", ""}, // a meaning someone gave: kept
		"voc-auto-bat":        {"lemma", "voc-auto-bati"},
		"voc-auto-bati":       {"create", ""},
		"voc-auto-kv":         {"lemma", "voc-auto-kvanto"},
		"voc-auto-kvanto":     {"create", ""},
		"voc-auto-kilom":      {"junk", ""},
		"voc-auto-diam":       {"refold", "voc-auto-diamanto"},
		"voc-auto-diamanto":   {"create", ""},
		"voc-auto-supre":      {"keep", ""},
		"voc-auto-ele":        {"lemma", "voc-auto-elen"},
		"voc-auto-elen":       {"create", ""},
	}
	for s, w := range want {
		if a := got[s]; a.Kind != w.kind || a.Into != w.into {
			t.Errorf("%s: %s → %q, want %s → %q", s, a.Kind, a.Into, w.kind, w.into)
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d actions, want %d: %+v", len(got), len(want), got)
	}
	if b := got["voc-auto-boao"]; b.Rating != 1500 || strings.Join(b.Tags, ",") != "vortaro,legvorto,lit-la-eta-princo-komenco" {
		t.Errorf("boao kept with rating %v tags %v", b.Rating, b.Tags)
	}
	// a card a run already handled is not planned again
	items[6].Content[UndoKey] = []interface{}{map[string]interface{}{"run": "lv-1"}}
	for _, a := range Plan(items) {
		if a.Slug == "voc-auto-boao" {
			t.Errorf("planned again: %+v", a)
		}
		if a.Slug == "voc-auto-boa" && a.Into != "voc-auto-boao" {
			t.Errorf("boa now → %q", a.Into)
		}
	}
}

func TestSentences(t *testing.T) {
	s := Sentences("Unu. Du! Ĉu tri?\nKvar")
	if len(s) != 4 || s[2] != "Ĉu tri?" {
		t.Errorf("%q", s)
	}
}

// TestOneAffix: a root, one affix and the ending, read alike by ReVo and by
// the pieces alone; anything else has none.
func TestOneAffix(t *testing.T) {
	for w, want := range map[string]string{
		"lernejo": "ej", "malbona": "mal", "lernanto": "ant", "malsanulejo": "", "knabo": "", "lernolibro": "", "gefratoj": "",
	} {
		if got := OneAffix(w); got != want {
			t.Errorf("OneAffix(%q) = %q, want %q", w, got, want)
		}
	}
}
