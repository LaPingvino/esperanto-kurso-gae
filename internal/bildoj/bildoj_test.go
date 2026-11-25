package bildoj

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func word(p *Picture) string {
	if p == nil {
		return ""
	}
	return p.Word
}

func TestForToken(t *testing.T) {
	cases := map[string]string{
		"domo": "domo", "domon": "domo", "Domoj": "domo",
		"hundojn": "hundo", "ruĝaj": "ruĝa", "manĝis": "manĝi", "manĝu": "manĝi",
		"kukojn": "kuko", "pluvas": "pluvo", "du": "du", "tri": "tri",
		// no grammatical ending: only as written
		"vin": "", "tre": "", "tr": "",
		// other roots that merely start alike
		"amiko": "", "virino": "virino", "viro": "viro", "kafejon": "", "lundo": "",
		"knabaĉo": "", "skribotablo": "", "arbaro": "", "buŝo": "buŝo", "buso": "buso",
		"revido": "", "monato": "",
		// family words: the short roots and the ge- plurals
		"patrinon": "patrino", "patroj": "patro", "gepatrojn": "gepatroj", "avinoj": "avino",
		"avon": "avo", "filinon": "filino", "filoj": "filo", "nepinoj": "nepino", "nevon": "nevo",
		"edzino": "edzino", "edzon": "edzo", "kuzinoj": "kuzino", "gefratoj": "gefratoj",
		"onklinon": "onklino", "filmo": "", "nepre": "", "aventuro": "",
	}
	for tok, want := range cases {
		if got := word(ForToken(tok)); got != want {
			t.Errorf("ForToken(%q) = %q, want %q", tok, got, want)
		}
	}
}

func TestInTextLeavesOutLookalikes(t *testing.T) {
	for _, text := range []string{"Mi amas vin.", "Ĝi estas tre bona.", "Li estas mia amiko."} {
		var got []string
		for _, p := range InText(text) {
			got = append(got, p.Word)
		}
		want := map[string]string{"Mi amas vin.": "ami"}[text]
		if strings.Join(got, ",") != want {
			t.Errorf("InText(%q) = %v, want %q", text, got, want)
		}
	}
}

func TestForVocab(t *testing.T) {
	cases := []struct{ word, kind, want string }{
		{"akv", "radiko", "akvo"}, // a Zagreb root
		{"akvo", "", "akvo"},
		{"dom", "radiko", "domo"},
		{"blanka", "", "blanka"},
		{"dek", "cifero", "dek"},
		{"dek du", "cifero", ""},
		{"an", "sufikso", ""},
		{"ul", "sufikso", ""},
		{"ĉiu", "tabelvorto", ""},
		{"te", "radiko", "teo"},
	}
	for _, c := range cases {
		if got := word(ForVocab(c.word, c.kind)); got != c.want {
			t.Errorf("ForVocab(%q, %q) = %q, want %q", c.word, c.kind, got, c.want)
		}
	}
}

// Every row names a file that exists, has its source, author and licence,
// and no file name spells its word (a picture exercise must not give the
// answer away in its URL).
func TestDataIsComplete(t *testing.T) {
	if len(All()) == 0 {
		t.Fatal("no pictures")
	}
	words := map[string]bool{}
	files := map[string]string{}
	for _, p := range All() {
		if words[p.Word] {
			t.Errorf("%s: twice", p.Word)
		}
		words[p.Word] = true
		if w, ok := files[p.File]; ok {
			t.Errorf("%s and %s share %s: one picture, two words", w, p.Word, p.File)
		}
		files[p.File] = p.Word
		if p.Topic == "" || p.Source == "" || p.Author == "" || p.License == "" {
			t.Errorf("%s: topic, source, author or licence missing: %+v", p.Word, p)
		}
		if _, err := os.Stat(filepath.Join("..", "..", "static", "bildoj", p.File)); err != nil {
			t.Errorf("%s: %v", p.Word, err)
		}
		if strings.Contains(strings.ToLower(p.File), strings.ToLower(p.Word)) {
			t.Errorf("%s: file name %s spells the word", p.Word, p.File)
		}
	}
	for n := 1; n <= 12; n++ {
		if _, err := os.Stat(filepath.Join("..", "..", strings.TrimPrefix(Lesson(n), "/"))); err != nil {
			t.Errorf("lesson %d: %v", n, err)
		}
	}
}

func TestForSeries(t *testing.T) {
	if got := ForSeries("zagr-l03"); got != Base+"lokoj/l03.png" {
		t.Errorf("zagr-l03: %q", got)
	}
	if got := ForSeries("zagr-l13"); got != "" {
		t.Errorf("zagr-l13: %q", got)
	}
	if got := ForSeries("bildoj-bestoj"); got == "" {
		t.Error("bildoj-bestoj: no picture")
	}
	if got := ForReading("bildoj-bestoj-teksto", "bildoj-bestoj"); got != "" {
		t.Errorf("an A0 picture reading has a lesson picture: %q", got)
	}
	if got := ForReading("zagr-l05-teksto", ""); got != Base+"lokoj/l05.png" {
		t.Errorf("zagr-l05-teksto: %q", got)
	}
}

// The A0 picture series (seed/bildvortaro.json, made by tools/make_seed.py)
// stay in step with the mapping: every picture exercise and vocab word has
// its picture, the right option is the pictured word, and no option looks
// like it (same group).
func TestSeedPicturesResolve(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "seed", "bildvortaro.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Slug, Type string
		Content    map[string]interface{}
	}
	if err := json.Unmarshal(b, &items); err != nil {
		t.Fatal(err)
	}
	byWord := map[string]*Picture{}
	for _, p := range All() {
		byWord[p.Word] = p
	}
	for _, it := range items {
		switch it.Type {
		case "vocab":
			w, _ := it.Content["word"].(string)
			if ForVocab(w, "") == nil {
				t.Errorf("%s: %q has no picture", it.Slug, w)
			}
		case "multiplechoice":
			w, _ := it.Content["bildo"].(string)
			p := ForVocab(w, "")
			if p == nil {
				t.Errorf("%s: bildo %q has no picture", it.Slug, w)
				continue
			}
			opts, _ := it.Content["options"].([]interface{})
			ci, _ := it.Content["correct_index"].(float64)
			if int(ci) >= len(opts) || opts[int(ci)] != w {
				t.Errorf("%s: the right option is not %q", it.Slug, w)
			}
			for _, o := range opts {
				q := byWord[o.(string)]
				if q == nil {
					t.Errorf("%s: option %q has no picture row", it.Slug, o)
				} else if q != p && p.Group != "" && q.Group == p.Group {
					t.Errorf("%s: options %q and %q look alike (group %s)", it.Slug, w, o, p.Group)
				} else if p.Set != "" && q.Set != p.Set {
					t.Errorf("%s: option %q is not of the set %s", it.Slug, o, p.Set)
				}
			}
		}
	}
}
