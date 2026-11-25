// Command bonalingvo makes content/builder/60-bona-lingvo.json, the "Bona
// lingvo" series (C1–C2), from the word list of La Bona Lingvo
// (https://labonalingvo.org, the project Renato Corsetti started): word pairs
// of a word with a root of its own (hospitalo, mava, stulta) and a word of
// the same meaning built from basic roots (malsanulejo, malbona, malsaĝa).
// Both are Esperanto; the built one is the project's preference.
//
//	curl -sS -o bl.json https://labonalingvo.org/api/all-translations
//	go run ./cmd/bonalingvo bl.json > content/builder/60-bona-lingvo.json
//
// A pair is kept only when (the drops are counted on stderr):
//   - both sides are single plain words, the entry an ordinary word (no
//     affix entry, no context note), and the project has checked it (not one
//     of its grey PIV/ReVo-only suggestions);
//   - the other word is one root with its ending (hospital·o, not
//     de·nov·e), and not a Fundamento word (an Official Addition, as
//     hospitalo and stulta are, or no official root at all);
//   - the preferred word is built: two morphemes or more besides the ending,
//     each an affix or a Fundamento/official root (ReVo's <ofc>);
//   - ReVo has both words, and confirms the meaning: one's definition names
//     the other, or their translations agree in two languages or more.
//
// A typed answer is asked only where it is the one answer in the whole list
// (a fill-in takes one, and another valid word must never be marked wrong);
// the choices of a multiple choice are never another valid counterpart.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/eo"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

const (
	site     = "https://labonalingvo.org/"
	source   = "La Bona Lingvo (labonalingvo.org)"
	series   = "bona-lingvo"
	label    = "Bona lingvo"
	maxPairs = 100
)

type target struct {
	Regular, Divided, Context, Annotation string
	Type                                  string `json:"recommendation_type"`
}

type entry struct {
	Source     string   `json:"source_word"`
	Divided    string   `json:"source_word_divided"`
	Context    *string  `json:"source_word_context"`
	Annotation *string  `json:"source_annotation"`
	Targets    []target `json:"target_words_array"`
}

var (
	prefixes = set("mal", "ge", "re", "dis", "ek", "eks", "bo", "fi", "pra", "mis", "vic")
	suffixes = set("aĉ", "ad", "aĵ", "an", "ar", "ebl", "ec", "eg", "ej", "em", "end", "er", "estr",
		"et", "id", "ig", "iĝ", "il", "in", "ind", "ing", "ism", "ist", "obl", "on", "op", "uj", "ul", "um",
		"ant", "int", "ont", "at", "it", "ot", "ĉj", "nj")
	endings = set("o", "a", "e", "i")
	plain   = regexp.MustCompile(`^[a-pr-vzĉĝĥĵŝŭ]+[oaei]$`)
	langs   = []string{"en", "fr", "de", "nl", "es", "it", "pl", "ru", "pt", "hu"}
)

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func grey(s string) bool { return strings.Contains(s, "PIV") || strings.Contains(s, "ReVo") }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// built: the divided form's pieces are affixes and official roots, two
// besides the ending at least.
func built(divided string) bool {
	ps := strings.Split(divided, "·")
	if len(ps) < 3 || !endings[ps[len(ps)-1]] {
		return false
	}
	for _, p := range ps[:len(ps)-1] {
		if p == "o" || prefixes[p] || suffixes[p] {
			continue
		}
		if revo.RootOfc(p) == "" {
			return false
		}
	}
	return true
}

// roots: how many of the divided form's pieces are roots.
func roots(divided string) int {
	n := 0
	ps := strings.Split(divided, "·")
	for _, p := range ps[:len(ps)-1] {
		if p != "o" && !prefixes[p] && !suffixes[p] {
			n++
		}
	}
	return n
}

// stem: the word without its ending.
func stem(w string) string { return w[:len(w)-1] }

func trWords(e *revo.Entry, lang string) map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(e.Translation(lang)), func(r rune) bool { return r == ',' || r == ';' }) {
		t = strings.TrimSpace(regexp.MustCompile(`\([^)]*\)|\[[^]]*\]`).ReplaceAllString(t, ""))
		if t != "" {
			m[t] = true
		}
	}
	return m
}

// names: the definition gives the word as a synonym, ReVo's way: the
// definition's first word, or a word right after a ";" ("Malsanulejo.",
// "Kuracisto …; haŭtkuracisto."). A word merely used in the definition
// (geologio in terscienco's list of earth sciences) is no synonym.
func names(def, w string) bool {
	def = strings.ToLower(def)
	parts := strings.Split(def, ";")
	for k, part := range parts {
		f := strings.FieldsFunc(part, func(r rune) bool { return !unicode.IsLetter(r) })
		if len(f) == 0 {
			continue
		}
		if k == 0 && len(f) > 2 {
			continue // a definition proper, not a synonym
		}
		if strings.HasPrefix(f[0], stem(w)) && len([]rune(f[0])) <= len([]rune(w))+2 {
			return true
		}
	}
	return false
}

// sameMeaning: ReVo's cross-check of a pair.
func sameMeaning(a, b *revo.Hit, wa, wb string) (bool, string) {
	if names(a.Def, wb) || names(b.Def, wa) {
		return true, "difino"
	}
	n := 0
	var agree []string
	for _, l := range langs {
		ta, tb := trWords(a.Entry, l), trWords(b.Entry, l)
		for t := range ta {
			if tb[t] {
				n++
				agree = append(agree, l+":"+t)
				break
			}
		}
	}
	return n >= 2, strings.Join(agree, " ")
}

// Read by hand against ReVo's definitions: pairs that pass the checks
// above, but whose words ReVo defines differently.
var differ = set(
	"humaneco",  // humane-ness, not homeco (being human)
	"bariero",   // ReVo: a counter's screen; barilo: a fence
	"gemo",      // ReVo's main sense: a treasure, figuratively
	"procesoro", // a computer's processor; datumtraktilo: a program
)

// Identity words are left out: a series that prefers one word over a
// group's own name for itself would not be neutral.
var identity = set("gejo", "lesbanino", "lesbano", "lesbo")

// Joop's own examples go in whatever their rank.
var always = set("hospitalo", "mava", "stulta")

type pair struct {
	Neo, Built, BuiltDiv, Kind, Ofc, Why string
	Weight                               int
}

func main() {
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var d struct{ Results []entry }
	if err := json.Unmarshal(raw, &d); err != nil {
		panic(err)
	}
	// every counterpart, both ways, over the whole list (checked or not)
	fwd, inv := map[string]map[string]bool{}, map[string]map[string]bool{}
	add := func(m map[string]map[string]bool, a, b string) {
		if m[a] == nil {
			m[a] = map[string]bool{}
		}
		m[a][b] = true
	}
	for _, e := range d.Results {
		for _, t := range e.Targets {
			add(fwd, e.Source, t.Regular)
			add(inv, t.Regular, e.Source)
		}
	}
	drop := map[string]int{}
	var pairs []pair
	seen := map[string]bool{}
	for _, e := range d.Results {
		if !plain.MatchString(e.Source) || deref(e.Context) != "" {
			drop["ne simpla vorto (afikso, pluraj vortoj, kunteksto)"]++
			continue
		}
		if ps := strings.Split(e.Divided, "·"); len(ps) != 2 || !endings[ps[1]] {
			drop["la alia vorto ne estas unu radiko kun finaĵo"]++
			continue
		} else if revo.RootOfc(ps[0]) == "*" {
			drop["la alia vorto estas Fundamenta"]++
			continue
		}
		if grey(deref(e.Annotation)) {
			drop["griza (nur PIV/ReVo, ne kontrolita de la projekto)"]++
			continue
		}
		var got *pair
		reason := ""
		for _, t := range e.Targets {
			switch {
			case !plain.MatchString(t.Regular) || t.Context != "":
				reason = "ne simpla vorto (afikso, pluraj vortoj, kunteksto)"
			case grey(t.Annotation):
				reason = "griza (nur PIV/ReVo, ne kontrolita de la projekto)"
			case !built(t.Divided):
				reason = "la preferata ne estas konstruita el oficialaj radikoj"
			default:
				hs, ht := revo.Exact(e.Source), revo.Exact(t.Regular)
				if hs == nil || ht == nil {
					reason = "ReVo ne havas ambaŭ vortojn"
					break
				}
				ok, why := sameMeaning(hs, ht, e.Source, t.Regular)
				if !ok {
					reason = "ReVo ne konfirmas la saman signifon"
					break
				}
				root := strings.Split(e.Divided, "·")[0]
				got = &pair{Neo: e.Source, Built: t.Regular, BuiltDiv: t.Divided, Kind: t.Type,
					Ofc: revo.RootOfc(root), Why: why, Weight: len(hs.Langs())}
			}
			if got != nil {
				break
			}
		}
		if got == nil {
			drop[reason]++
			continue
		}
		if differ[got.Neo] {
			drop["ReVo difinas ilin alie (legite mane)"]++
			continue
		}
		if identity[got.Neo] {
			drop["identeca vorto (lasita ekstere)"]++
			continue
		}
		if seen[got.Built] {
			drop["la preferata jam havas alian paron"]++
			continue
		}
		seen[got.Built] = true
		pairs = append(pairs, *got)
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].Weight != pairs[j].Weight {
			return pairs[i].Weight > pairs[j].Weight
		}
		return pairs[i].Neo < pairs[j].Neo
	})
	fmt.Fprintf(os.Stderr, "%d entries, %d pairs pass\n", len(d.Results), len(pairs))
	for r, n := range drop {
		fmt.Fprintf(os.Stderr, "  drop %5d  %s\n", n, r)
	}
	if len(os.Args) > 2 && os.Args[2] == "-list" {
		for _, p := range pairs {
			fmt.Printf("%s\t%s\t%s\t%s\tofc=%s\tw=%d\t%s\n", p.Neo, p.Built, p.BuiltDiv, p.Kind, p.Ofc, p.Weight, p.Why)
		}
		return
	}
	var keep []pair
	for i, p := range pairs {
		if i < maxPairs || always[p.Neo] {
			keep = append(keep, p)
		}
	}
	pairs = keep
	items := makeItems(pairs, fwd, inv)
	out, _ := json.MarshalIndent(items, "", " ")
	os.Stdout.Write(append(out, '\n'))
	fmt.Fprintf(os.Stderr, "%d pairs → %d items\n", len(pairs), len(items))
}

// hintBuilt: the built word's pieces with its roots hidden (mal·…·ul·ej·o).
func hintBuilt(div string) string {
	ps := strings.Split(div, "·")
	for i, p := range ps[:len(ps)-1] {
		if !prefixes[p] && !suffixes[p] && p != "o" {
			ps[i] = "…"
		}
	}
	return strings.Join(ps, "·")
}

func capital(w string) string {
	r := []rune(w)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func makeItems(pairs []pair, fwd, inv map[string]map[string]bool) []map[string]any {
	var items []map[string]any
	order := 0
	lvl := func(i int) int {
		if i < len(pairs)/2 {
			return 1900 + 2*i // C1
		}
		return 2100 + 2*(i-len(pairs)/2) // C2
	}
	// choices: words of the same side of other pairs, never a counterpart of this one
	pick := func(i int, own string, valid map[string]bool, side func(pair) string) []string {
		var c []string
		for k := 1; len(c) < 3 && k < len(pairs); k++ {
			w := side(pairs[(i+k*7)%len(pairs)])
			if w == own || valid[w] || w[len(w)-1] != own[len(own)-1] {
				continue
			}
			c = append(c, w)
		}
		return c
	}
	for i, p := range pairs {
		url := site + "?vorto=" + p.Neo
		base := func(kind, suffix string, content map[string]any) {
			order++
			content["source_url"] = url
			items = append(items, map[string]any{
				"slug": "bona-lingvo-" + eo.WordToSlug(p.Neo) + "-" + suffix, "type": kind, "rating": lvl(i),
				"content": content, "tags": []string{"vortaro", "bona-lingvo", map[string]string{"multiplechoice": "elekti", "fillin": "plenigi"}[kind]},
				"source": source, "series_slug": series, "series_order": order, "series_label": label,
			})
		}
		mc := func(suffix, q, right string, wrong []string) {
			if len(wrong) < 3 {
				return
			}
			opts := append([]string{}, wrong...)
			ci := (i + len(suffix)) % 4
			opts = append(opts[:ci], append([]string{right}, opts[ci:]...)...)
			base("multiplechoice", suffix, map[string]any{"question": q, "options": opts, "correct_index": ci})
		}
		// the preferred word → the word with its own root
		mc("al-radiko", fmt.Sprintf("„%s“ estas konstruita el bazaj radikoj. Kiu Esperanta vorto kun propra radiko signifas la samon?", capital(p.Built)),
			p.Neo, pick(i, p.Neo, inv[p.Built], func(q pair) string { return q.Neo }))
		// the word with its own root → the preferred built word
		mc("al-konstruita", fmt.Sprintf("Anstataŭ „%s“ La Bona Lingvo proponas vorton el bazaj radikoj. Kiun?", p.Neo),
			p.Built, pick(i, p.Built, fwd[p.Neo], func(q pair) string { return q.Built }))
		// the project proposes only this word for it, and it has one root (a
		// compound may take a linking o or not: dent·o·karn·o, dent·karn·o)
		if len(fwd[p.Neo]) == 1 && roots(p.BuiltDiv) == 1 {
			base("fillin", "tajpu-konstruitan", map[string]any{
				"question": fmt.Sprintf("%s = ___ (la sama signifo, el bazaj radikoj)", capital(p.Neo)),
				"answer":   p.Built, "hint": hintBuilt(p.BuiltDiv)})
		}
		if len(inv[p.Built]) == 1 { // and only this word has it as its proposal
			base("fillin", "tajpu-radikan", map[string]any{
				"question": fmt.Sprintf("%s = ___ (la sama signifo, per propra radiko)", capital(p.Built)),
				"answer":   p.Neo, "hint": fmt.Sprintf("%s… (%d literoj)", string([]rune(p.Neo)[0]), len([]rune(p.Neo)))})
		}
	}
	return items
}
