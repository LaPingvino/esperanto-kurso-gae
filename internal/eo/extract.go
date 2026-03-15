// Package eo provides Esperanto-specific text utilities.
package eo

import (
	"sort"
	"strings"
	"unicode"
)

// stopwords are common Esperanto words that should not become vocabulary items.
var stopwords = map[string]bool{
	// pronouns
	"mi": true, "vi": true, "li": true, "ŝi": true, "ĝi": true,
	"ni": true, "ili": true, "oni": true, "si": true,
	// article
	"la": true,
	// prepositions
	"en": true, "de": true, "da": true, "al": true, "kun": true, "per": true,
	"por": true, "sur": true, "sub": true, "tra": true, "post": true,
	"antaŭ": true, "ĝis": true, "je": true, "el": true, "apud": true,
	"anstataŭ": true, "ekde": true, "preter": true, "kontraŭ": true,
	"inter": true, "pri": true, "pro": true, "sen": true, "laŭ": true,
	"malgraŭ": true, "dum": true, "ĉe": true,
	// conjunctions
	"kaj": true, "aŭ": true, "sed": true, "ĉar": true, "tial": true,
	"ke": true, "kvankam": true, "kiam": true, "se": true, "ĉu": true,
	"do": true, "tamen": true, "ankaŭ": true, "eĉ": true, "ja": true,
	"nur": true, "jam": true, "ankoraŭ": true, "nun": true,
	"baldaŭ": true, "hodiaŭ": true, "morgaŭ": true, "hieraŭ": true,
	"tre": true, "tro": true, "ho": true, "pli": true, "plej": true,
	"certe": true, "eble": true, "vere": true, "bone": true, "ree": true,
	// correlatives (tio-, ĉio-, kio-, io-, nenio- series)
	"tio": true, "tiu": true, "tiuj": true, "tiom": true, "tie": true,
	"tiel": true, "tiam": true, "tia": true,
	"ĉio": true, "ĉiu": true, "ĉiuj": true, "ĉiom": true, "ĉie": true,
	"ĉiel": true, "ĉiam": true, "ĉia": true,
	"kio": true, "kiu": true, "kiuj": true, "kiom": true, "kie": true,
	"kiel": true, "kia": true,
	"io": true, "iu": true, "iuj": true, "iom": true, "ie": true,
	"iel": true, "iam": true, "ia": true,
	"nenio": true, "neniu": true, "neniuj": true, "neniom": true, "nenie": true,
	"neniel": true, "neniam": true, "nenia": true,
	// numbers
	"unu": true, "du": true, "tri": true, "kvar": true, "kvin": true,
	"ses": true, "sep": true, "ok": true, "naŭ": true, "dek": true,
	"cent": true, "mil": true, "nul": true,
	// common verbs (infinitive forms — conjugated forms are handled by normaliser)
	"esti": true, "havi": true, "fari": true, "iri": true, "veni": true,
	"diri": true, "povi": true, "devi": true, "voli": true, "scii": true,
	// common discourse words
	"kompreneble": true, "pardonu": true, "dankon": true, "bonvolu": true,
	"saluton": true, "adiaŭ": true, "jes": true, "ne": true,
	"tuj": true, "poste": true, "fine": true, "denove": true,
	"plu": true, "krom": true, "mem": true, "ajn": true, "almenaŭ": true,
	"kvazaŭ": true, "preskaŭ": true, "sufiĉe": true, "ĉefe": true,
	"finfine": true, "subite": true, "apenaŭ": true, "cetere": true,
}

// suffixes maps grammatical endings to canonical form.
// Ordered longest-first so the longest match wins.
var suffixes = []struct {
	suffix  string
	replace string
	keep    bool // if true, this IS already the base form — no action needed
}{
	// Verb participles (skip — they become separate vocab entries if meaningful)
	{"-antan", "", false}, {"-antaj", "", false}, {"-antajn", "", false},
	{"-anta", "", false}, {"-anto", "", false}, {"-antoj", "", false},
	{"-intan", "", false}, {"-intaj", "", false}, {"-inta", "", false},
	{"-ontan", "", false}, {"-onta", "", false},
	// Noun plural accusative → base noun
	{"ojn", "o", false},
	// Noun plural → base noun
	{"oj", "o", false},
	// Noun accusative → base noun
	{"on", "o", false},
	// Adj plural accusative → base adj
	{"ajn", "a", false},
	// Adj plural → base adj
	{"aj", "a", false},
	// Adj accusative → base adj
	{"an", "a", false},
	// Adverb accusative
	{"en", "e", false},
	// Verb conjugated → infinitive
	{"as", "i", false},
	{"is", "i", false},
	{"os", "i", false},
	{"us", "i", false},
	// Imperative — could be verb or keep (e.g. "iru")
	// We keep -u words only if they have a long enough root
	// Base forms: already correct
	{"o", "o", true},
	{"a", "a", true},
	{"i", "i", true},
	{"e", "e", true}, // adverb — include
}

// ToBaseForm strips Esperanto grammatical endings and returns the canonical
// (dictionary) form: noun→-o, verb→-i, adj→-a, adverb→-e.
// Returns empty string if the word can't be normalised (too short, proper noun, etc.).
func ToBaseForm(word string) string {
	word = strings.ToLower(strings.TrimSpace(word))

	// Strip punctuation from both ends
	word = strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r) && r != 'ĉ' && r != 'ĝ' && r != 'ĥ' &&
			r != 'ĵ' && r != 'ŝ' && r != 'ŭ'
	})

	if len([]rune(word)) < 3 {
		return ""
	}

	for _, s := range suffixes {
		suf := s.suffix
		if strings.HasPrefix(suf, "-") {
			suf = suf[1:]
		}
		if strings.HasSuffix(word, suf) {
			root := word[:len(word)-len(suf)]
			if len([]rune(root)) < 2 {
				continue
			}
			if s.keep {
				return word
			}
			return root + s.replace
		}
	}
	return ""
}

// ExtractWords returns a deduplicated, sorted list of base-form Esperanto
// content words extracted from text. Stopwords and proper nouns are excluded.
func ExtractWords(text string) []string {
	seen := make(map[string]bool)
	// Track which positions start a sentence so we can detect true capitalisation.
	// Simple heuristic: a word is a proper noun if it appears capitalised but
	// its lowercase form doesn't appear anywhere in the text.
	uppercaseOnly := make(map[string]int) // word → times seen capitalised
	total := make(map[string]int)         // word → total times seen

	// First pass: collect capitalisation stats
	for _, raw := range tokenize(text) {
		lower := strings.ToLower(raw)
		total[lower]++
		if len(raw) > 0 && unicode.IsUpper([]rune(raw)[0]) {
			uppercaseOnly[lower]++
		}
	}

	var out []string
	for _, raw := range tokenize(text) {
		lower := strings.ToLower(raw)
		if len([]rune(lower)) < 3 {
			continue
		}
		// Likely a proper noun: always starts with uppercase
		if total[lower] > 0 && uppercaseOnly[lower] == total[lower] {
			continue
		}
		base := ToBaseForm(lower)
		if base == "" {
			continue
		}
		if stopwords[base] || stopwords[lower] {
			continue
		}
		if seen[base] {
			continue
		}
		seen[base] = true
		out = append(out, base)
	}
	sort.Strings(out)
	return out
}

// WordToSlug converts an Esperanto word to an ASCII-safe slug fragment.
func WordToSlug(word string) string {
	r := strings.NewReplacer(
		"ĉ", "cx", "ĝ", "gx", "ĥ", "hx", "ĵ", "jx", "ŝ", "sx", "ŭ", "ux",
	)
	return r.Replace(strings.ToLower(word))
}

func tokenize(text string) []string {
	var tokens []string
	for _, line := range strings.Fields(text) {
		// Split on common punctuation
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == '.' || r == '!' || r == '?' ||
				r == ';' || r == ':' || r == '"' || r == '\'' ||
				r == '(' || r == ')' || r == '[' || r == ']' ||
				r == '–' || r == '—' || r == '…' || r == '«' || r == '»' ||
				r == '\n' || r == '\r' || r == '\t'
		})
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				tokens = append(tokens, p)
			}
		}
	}
	return tokens
}
