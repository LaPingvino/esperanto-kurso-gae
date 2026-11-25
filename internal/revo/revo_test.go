package revo

import (
	"encoding/json"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/esperanto-kurso-gae/seed"
)

func TestLoadCost(t *testing.T) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	once.Do(load)
	took := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("%d derivations, %d headwords; load %v, heap +%.1f MB", len(data.Drv), len(heads), took, float64(after.HeapAlloc-before.HeapAlloc)/1e6)
	if len(data.Drv) < 30000 {
		t.Fatalf("only %d derivations: is revo.json.gz the full extract?", len(data.Drv))
	}
}

func TestLookup(t *testing.T) {
	cases := []struct {
		word string
		root bool
		mrk  string // prefix of the expected marker
		en   string // substring of the English translation
	}{
		{"knabo", false, "knab.0o", "boy"},
		{"knabojn", false, "knab.0o", "boy"},
		{"verda", false, "verd.0a", "green"},
		{"signifas", false, "signif.0i", "signify"},
		{"kies", false, "kies.", "whose"},
		{"kiujn", false, "kiu.", ""},
		{"iom post iom", false, "iom.0post0", "little by little"},
		{"hom", true, "hom.", "human"},
		{"ŝanĝ", true, "sxangx.", "change"},
		{"sxangxi", false, "sxangx.0i", "change"},
		{"-ec-", true, "ec.", ""},
		{"mal-", true, "mal.", ""},
		{"vorto", false, "vort.0o", "word"}, // not "Vorto", the Logos
		{"do", false, "do.", ""},            // not the letter d
	}
	for _, c := range cases {
		h := Lookup(c.word, c.root)
		if h == nil {
			t.Errorf("%q: no hit", c.word)
			continue
		}
		if !strings.HasPrefix(h.Mrk, c.mrk) {
			t.Errorf("%q: marker %q, want %q…", c.word, h.Mrk, c.mrk)
		}
		if c.en != "" && !strings.Contains(h.Translation("en"), c.en) {
			t.Errorf("%q: en %q, want …%q…", c.word, h.Translation("en"), c.en)
		}
	}
	if h := Lookup("trajnbiletejestrinoj", false); h != nil {
		t.Errorf("compound trajnbiletejestrinoj matched %q: a compound should fall through to the search link", h.Mrk)
	}
}

func TestParseGlitches(t *testing.T) {
	if en := Lookup("signifi", false).Translation("en"); strings.Count(en, "signify") != 1 {
		t.Errorf("signifi en = %q: the ambiguity note came along", en)
	}
	homo := Lookup("homo", false)
	if strings.Contains(homo.Def, "()") {
		t.Errorf("homo def has empty brackets: %q", homo.Def)
	}
	if strings.HasPrefix(homo.Translation("en"), "Homo") {
		t.Errorf("homo en starts with the zoological genus: %q", homo.Translation("en"))
	}
}

func TestURLs(t *testing.T) {
	if got := ArticleURL("hom.0o"); got != "https://reta-vortaro.de/revo/#hom.0o" {
		t.Errorf("ArticleURL = %q", got)
	}
	if got := SearchURL("popola maso"); got != "https://reta-vortaro.de/revo/?q=popola+maso" {
		t.Errorf("SearchURL = %q", got)
	}
}

type seedCard struct {
	slug, word string
	root       bool
	defs       map[string]string
}

func seedVocab(t *testing.T) []seedCard {
	t.Helper()
	files, _ := fs.Glob(seed.FS, "*.json")
	seen := map[string]bool{}
	var out []seedCard
	for _, f := range files {
		raw, _ := seed.FS.ReadFile(f)
		var items []struct {
			Slug    string
			Type    string
			Tags    []string
			Content map[string]any
		}
		if json.Unmarshal(raw, &items) != nil {
			continue
		}
		for _, it := range items {
			if it.Type != "vocab" || seen[it.Slug] {
				continue
			}
			seen[it.Slug] = true
			c := seedCard{slug: it.Slug, defs: map[string]string{}}
			c.word, _ = it.Content["word"].(string)
			for _, tg := range it.Tags {
				if tg == "radiko" || tg == "sufikso" || tg == "prefikso" {
					c.root = true
				}
			}
			if d, ok := it.Content["definitions"].(map[string]any); ok {
				for l, v := range d {
					if s, _ := v.(string); s != "" {
						c.defs[l] = s
					}
				}
			}
			if s, _ := it.Content["definition"].(string); s != "" && c.defs["en"] == "" {
				c.defs["en"] = s
			}
			out = append(out, c)
		}
	}
	return out
}

// TestDumpMatches lists matches for reading by eye: go test -run Dump -v -args dump
func TestDumpMatches(t *testing.T) {
	if !dumpFlag() {
		t.Skip("pass -args dump")
	}
	for _, c := range seedVocab(t) {
		h := Lookup(c.word, c.root)
		tag := "def"
		if len(c.defs) == 0 {
			tag = "NODEF"
		}
		if h == nil {
			t.Logf("%s %-28s %-20q -", tag, c.slug, c.word)
			continue
		}
		alt := ""
		if r := Lookup(c.word, !c.root); r != nil && r.Mrk != h.Mrk {
			alt = " (other: " + r.Mrk + ")"
		}
		t.Logf("%s %-28s %-20q %s root=%v via=%q en=%q%s", tag, c.slug, c.word, h.Mrk, h.Root, h.Via, h.Translation("en"), alt)
	}
}

func dumpFlag() bool {
	for _, a := range os.Args {
		if a == "dump" {
			return true
		}
	}
	return false
}

// TestBoaoMainSense: La eta princo's boao is the snake. Its translations
// are the main sense's, not the fur scarf's (TEKS) as well; the senses are
// kept apart for a text to name the one it means.
func TestBoaoMainSense(t *testing.T) {
	h := Lookup("boaon", false)
	if h == nil || !h.HasHead("boao") {
		t.Fatalf("boaon: %+v", h)
	}
	for lang, bad := range map[string]string{"de": "Schal", "ru": "шарф", "sk": "kožušina", "pl": "szal"} {
		if tr := h.Translation(lang); strings.Contains(tr, bad) {
			t.Errorf("%s: %q has the scarf", lang, tr)
		}
	}
	if len(h.Senses) != 3 || h.Sense("boa.0o.TEKS") == nil || !h.Sense("boa.0o.TEKS").Fak {
		t.Fatalf("senses: %+v", h.Senses)
	}
	if tr := h.Sense("boa.0o.TEKS").Translation("de"); tr != "Boa (Schal)" {
		t.Errorf("TEKS de %q", tr)
	}
	if o := h.Ofc; o != "*" {
		t.Errorf("ofc %q", o)
	}
}
