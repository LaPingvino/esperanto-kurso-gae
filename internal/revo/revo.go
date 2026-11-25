// Package revo looks up Esperanto words in an embedded extract of Reta
// Vortaro (https://reta-vortaro.de): per derivation its marker, headwords,
// Esperanto definition and translations into some 130 languages.
//
// The extract (revo.json.gz) is built by cmd/revo-import from ReVo's own
// sources, github.com/revuloj/revo-fonto, which are licensed under the GNU
// GPL 2.0; the extract keeps that licence (see LICENSE-REVO in this
// directory). Pages that show its text credit ReVo and link to the article.
package revo

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode"
)

//go:embed revo.json.gz
var raw []byte

// Data is the file format of revo.json.gz.
type Data struct {
	Source string         `json:"source"`
	Commit string         `json:"commit,omitempty"`
	Date   string         `json:"date,omitempty"`
	Drv    []Entry        `json:"drv"`
	Roots  map[string]int `json:"roots"` // root → index of its article's first drv
}

// Entry is one ReVo derivation (drv).
type Entry struct {
	Mrk   string   `json:"m"`           // e.g. "abak.0o": file "abak", anchor "abak.0o"
	Heads []string `json:"h"`           // headwords, e.g. ["homo", "homoj"]
	Def   string   `json:"d,omitempty"` // first Esperanto definition
	// Trd holds the translations as "lang\ttext" records separated by
	// "\n" (text: "w1, w2; sense2"): one string per entry instead of a
	// map keeps the loaded extract small. Use Translation.
	Trd string `json:"t,omitempty"`
}

// Translation returns the entry's translation in a ReVo language code, or "".
func (e *Entry) Translation(lang string) string {
	if e == nil || lang == "" {
		return ""
	}
	rest := e.Trd
	for rest != "" {
		var rec string
		rec, rest, _ = strings.Cut(rest, "\n")
		if l, text, ok := strings.Cut(rec, "\t"); ok && l == lang {
			return text
		}
	}
	return ""
}

// Langs lists the languages the entry has translations in.
func (e *Entry) Langs() []string {
	var out []string
	for _, rec := range strings.Split(e.Trd, "\n") {
		if l, _, ok := strings.Cut(rec, "\t"); ok {
			out = append(out, l)
		}
	}
	return out
}

// EncodeTrd builds an Entry.Trd from a lang → text map, sorted by language.
func EncodeTrd(m map[string]string) string {
	langs := make([]string, 0, len(m))
	for l := range m {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	var b strings.Builder
	for _, l := range langs {
		t := strings.NewReplacer("\n", " ", "\t", " ").Replace(m[l])
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l + "\t" + t)
	}
	return b.String()
}

// MaxSenses and MaxWords bound a translation line: enough to show the
// range of a word without turning a flashcard into an article.
const (
	MaxSenses = 3
	MaxWords  = 4
)

// JoinSenses formats translations: words of a sense joined by ", ",
// senses by "; ". Words repeated in an earlier sense are left out.
func JoinSenses(senses [][]string) string {
	var parts []string
	seen := map[string]bool{}
	for _, ws := range senses {
		var keep []string
		for _, w := range ws {
			if w == "" || seen[w] {
				continue
			}
			seen[w] = true
			if len(keep) < MaxWords {
				keep = append(keep, w)
			}
		}
		if len(keep) > 0 {
			parts = append(parts, strings.Join(keep, ", "))
		}
		if len(parts) == MaxSenses {
			break
		}
	}
	return strings.Join(parts, "; ")
}

var xsys = strings.NewReplacer(
	"cx", "ĉ", "gx", "ĝ", "hx", "ĥ", "jx", "ĵ", "sx", "ŝ", "ux", "ŭ",
	"Cx", "ĉ", "Gx", "ĝ", "Hx", "ĥ", "Jx", "ĵ", "Sx", "ŝ", "Ux", "ŭ",
	"CX", "ĉ", "GX", "ĝ", "HX", "ĥ", "JX", "ĵ", "SX", "ŝ", "UX", "ŭ",
)

// Normalize lower-cases a word, turns x-system spelling into the
// diacritics and trims spaces, hyphens, slashes and stars.
func Normalize(w string) string {
	w = strings.ToLower(xsys.Replace(strings.TrimSpace(w)))
	return strings.Trim(w, " -/*.!?")
}

var (
	once  sync.Once
	data  *Data
	heads map[string][]int // normalized headword → drv indices
)

func load() {
	data = &Data{}
	heads = map[string][]int{}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		log.Printf("revo: %v", err)
		return
	}
	if err := json.NewDecoder(zr).Decode(data); err != nil {
		log.Printf("revo: %v", err)
		data = &Data{}
		return
	}
	for i, e := range data.Drv {
		for _, h := range e.Heads {
			k := Normalize(h)
			heads[k] = append(heads[k], i)
		}
	}
}

// Warm loads the extract ahead of the first lookup (it takes about a
// second); main starts it in the background.
func Warm() { once.Do(load) }

// Info returns the extract's source line, ReVo commit and date.
func Info() (source, commit, date string) {
	once.Do(load)
	return data.Source, data.Commit, data.Date
}

// Hit is the result of a lookup.
type Hit struct {
	*Entry
	Root bool // matched the word as a root (e.g. "hom" → homo)
	// Via is set when the word itself has no entry and a related one was
	// taken instead: "trankvile" → "trankvila", "leonino" → "leono". Pages
	// show it, so the learner knows the meaning is of that word.
	Via string
}

// URL is the ReVo page of the derivation, opened at its anchor.
func (h *Hit) URL() string { return ArticleURL(h.Mrk) }

// ArticleURL opens ReVo's own interface at a derivation or sense marker:
// reta-vortaro.de/revo/#hund.0o loads art/hund.html at #hund.0o, with
// ReVo's search and index beside it (voko-grundo jsc/rv/shargo.ts hash2art).
// ReVo lists the translations in the reader's preferred languages first;
// it takes those from the browser, so no language parameter is needed.
func ArticleURL(mrk string) string {
	return "https://reta-vortaro.de/revo/#" + url.PathEscape(mrk)
}

// SearchURL is ReVo's search (Esperanto and all its translation
// languages) for a word with no article here: /revo/?q= (kadro.ts).
func SearchURL(word string) string {
	return "https://reta-vortaro.de/revo/?q=" + url.QueryEscape(strings.TrimSpace(word))
}

// Translation returns the translation in one of the site's language
// codes, or "".
func (h *Hit) Translation(lang string) string {
	if h == nil || h.Entry == nil {
		return ""
	}
	return h.Entry.Translation(ReVoLang(lang))
}

// ReVoLang maps the site's language codes to ReVo's where they differ.
func ReVoLang(lang string) string {
	switch lang {
	case "zh-tw":
		return "zh" // ReVo has one Chinese, mostly simplified
	case "nb", "nn":
		return "no"
	}
	return lang
}

// endings an inflected or derived word may carry, longest first, each
// with the base forms to try: a verb form looks for the infinitive first,
// so "estas" finds esti, not a noun "esto".
var endings = []struct {
	end   string
	bases []string
}{
	{"ojn", []string{"o"}}, {"ajn", []string{"a"}},
	{"oj", []string{"o"}}, {"aj", []string{"a"}},
	{"on", []string{"o"}}, {"an", []string{"a"}}, {"en", []string{"e"}},
	{"as", []string{"i"}}, {"is", []string{"i"}}, {"os", []string{"i"}},
	{"us", []string{"i"}}, {"u", []string{"i"}},
	{"o", []string{"o"}}, {"a", []string{"a"}}, {"e", []string{"e"}}, {"i", []string{"i"}},
}

// Lookup finds the ReVo derivation for a course word: a headword
// ("knabo", "iom post iom"), an inflected form ("knabojn" → knabo), a
// root ("hom" → its article's first derivation) or an affix ("-ec-",
// whose ReVo headword "-ec" normalizes to the same key). With root set
// (cards tagged radiko, sufikso, prefikso) the word is tried as a root
// first, so the root "post" is not taken for the preposition.
// It returns nil when ReVo has nothing for it.
func Lookup(word string, root bool) *Hit {
	once.Do(load)
	w := Normalize(word)
	if w == "" {
		return nil
	}
	byRoot := func() *Hit {
		if i, ok := data.Roots[w]; ok {
			return &Hit{Entry: &data.Drv[i], Root: true}
		}
		return nil
	}
	if root {
		if h := byRoot(); h != nil {
			return h
		}
	}
	if is, ok := heads[w]; ok {
		return &Hit{Entry: &data.Drv[is[0]]}
	}
	if h := byRoot(); h != nil {
		return h
	}
	if strings.ContainsFunc(w, unicode.IsSpace) {
		return nil
	}
	// Accusative and plural of words without a noun/adjective ending
	// (kiujn → kiu, tiun → tiu).
	for _, end := range []string{"jn", "n", "j"} {
		if stem, ok := strings.CutSuffix(w, end); ok {
			if is, ok := heads[stem]; ok {
				return &Hit{Entry: &data.Drv[is[0]]}
			}
		}
	}
	// Inflected forms: strip an ending, then try the base form.
	for _, e := range endings {
		stem, ok := strings.CutSuffix(w, e.end)
		if !ok || len([]rune(stem)) < 2 {
			continue
		}
		for _, b := range e.bases {
			if is, ok := heads[stem+b]; ok {
				return &Hit{Entry: &data.Drv[is[0]]}
			}
		}
		return related(stem)
	}
	return nil
}

// related finds a nearby headword for a stem that has none of its own:
// a participle's verb (kre-it-a → krei, before the noun kreito), another
// word class (trankvil-e → trankvila), or the word without a common affix (leon-in-o →
// leono, ek-bleki → bleki). The hit's Via names the word it found.
func related(stem string) *Hit {
	try := func(cands ...string) *Hit {
		for _, c := range cands {
			if is, ok := heads[c]; ok {
				return &Hit{Entry: &data.Drv[is[0]], Via: data.Drv[is[0]].Heads[0]}
			}
		}
		return nil
	}
	for _, p := range []string{"ant", "int", "ont", "at", "it", "ot"} {
		if v, ok := strings.CutSuffix(stem, p); ok && len([]rune(v)) >= 2 {
			if h := try(v + "i"); h != nil {
				return h
			}
		}
	}
	if h := try(stem+"a", stem+"aj", stem+"o", stem+"oj", stem+"i", stem+"e"); h != nil {
		return h
	}
	for _, sfx := range []string{"in", "et", "eg", "ad", "ul"} {
		if v, ok := strings.CutSuffix(stem, sfx); ok && len([]rune(v)) >= 2 {
			if h := try(v+"o", v+"i", v+"a"); h != nil {
				return h
			}
		}
	}
	if v, ok := strings.CutPrefix(stem, "ek"); ok && len([]rune(v)) >= 2 {
		if h := try(v+"i", v+"o", v+"a"); h != nil {
			return h
		}
	}
	return nil
}
