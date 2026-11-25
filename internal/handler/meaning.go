package handler

import (
	"net/url"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

// Meaning is what a vocab card shows as the meaning of its word, found by
// the fallback chain in MeaningFor, with links into the dictionaries.
type Meaning struct {
	Text     string // the meaning shown
	Lang     string // language of Text
	Source   string // "course", "community", "revo" or "" (none)
	Fallback bool   // Text is not in the learner's language (English or Esperanto)
	Via      string // ReVo had a related word, not this one ("trankvila" for "trankvile")

	// EoDef is ReVo's Esperanto definition. (A card's flat "definition"
	// field is its legacy English meaning, not an Esperanto definition.)
	EoDef     string
	EoDefRevo bool
	ReVoURL   string // the ReVo article (or ReVo's search when it has none)
	ReVoFound bool
	PIVURL    string // vortaro.net, PIV: Esperanto only, linked, never copied
	// Suggestions are ReVo's translations into the learner's language, split
	// into single words, offered to contributors adding a translation.
	Suggestions []Suggestion
}

// Suggestion is one translation a contributor can pick.
type Suggestion struct {
	Text, Source string
}

// copiedLangs are the languages whose generated course definitions are
// copies of another language's, not translations, and that language:
// measured on the seed, each has the same text as its source on 680 to
// 1,011 of its ~1,011 cards (ur: 199 English + 64 Persian). Elsewhere a
// definition equal to the English one is a cognate (de Hand, nl water), so
// the rule applies only to these.
var copiedLangs = map[string][]string{
	"ro": {"fr"}, "sw": {"fr"}, "frp": {"fr"}, "be": {"ru"}, "ku": {"fa"},
	"hi": {"en"}, "lo": {"en"}, "my": {"en"}, "yo": {"en"}, "km": {"en"},
	"tok": {"en"}, "kk": {"en"}, "ur": {"en", "fa"},
}

// copiedFrom names the language a copied definition was taken from.
func copiedFrom(content map[string]interface{}, lang, def string) string {
	for _, src := range copiedLangs[lang] {
		if d, _ := courseDef(content, src); src != lang && strings.EqualFold(d, def) {
			return src
		}
	}
	return lang
}

// courseDef is the card's own definition in lang and whether it is a copy
// of another language's definition rather than a translation.
func courseDef(content map[string]interface{}, lang string) (def string, copied bool) {
	defs, _ := content["definitions"].(map[string]interface{})
	str := func(l string) string {
		s, _ := defs[l].(string)
		if s == "" && l == "en" {
			s, _ = content["definition"].(string)
		}
		return strings.TrimSpace(s)
	}
	def = str(lang)
	if def == "" || lang == "en" {
		return def, false
	}
	for _, src := range copiedLangs[lang] {
		if strings.EqualFold(def, str(src)) {
			return def, true
		}
	}
	return def, false
}

// MeaningFor runs the fallback chain for a vocab card in the learner's
// language: the course's own definition → a community translation →
// ReVo's translation → English, the course's or ReVo's → ReVo's Esperanto
// definition. A course definition copied from another language (see
// copiedLangs) does not count as one in the learner's language.
func MeaningFor(item *model.ContentItem, lang, community string) Meaning {
	word := item.Word()
	hit := revo.Lookup(word, isRootCard(item))
	m := Meaning{PIVURL: pivURL(word, hit)}
	if hit != nil {
		m.ReVoURL, m.ReVoFound = hit.URL(), true
		m.Via = hit.Via
		m.EoDef, m.EoDefRevo = hit.Def, hit.Def != ""
	} else {
		m.ReVoURL = revo.SearchURL(word)
	}
	set := func(text, l, src string) Meaning {
		m.Text, m.Lang, m.Source, m.Fallback = text, l, src, l != lang
		if src != "revo" {
			m.Via = ""
		}
		return m
	}
	if lang == "eo" {
		if m.EoDef != "" {
			return set(m.EoDef, "eo", sourceOf(m.EoDefRevo))
		}
	}
	own, copied := courseDef(item.Content, lang)
	if own != "" && !copied {
		return set(own, lang, "course")
	}
	if community != "" {
		return set(community, lang, "community")
	}
	if t := hit.Translation(lang); t != "" {
		m.Suggestions = splitSuggestions(t)
		return set(t, lang, "revo")
	}
	if en, _ := courseDef(item.Content, "en"); en != "" {
		return set(en, "en", "course")
	}
	if t := hit.Translation("en"); t != "" {
		return set(t, "en", "revo")
	}
	if own != "" { // a copy, shown as what it is: the other language's text
		return set(own, copiedFrom(item.Content, lang, own), "course")
	}
	if m.EoDef != "" {
		return set(m.EoDef, "eo", sourceOf(m.EoDefRevo))
	}
	m.Via = ""
	return m
}

func sourceOf(fromRevo bool) string {
	if fromRevo {
		return "revo"
	}
	return "course"
}

func isRootCard(item *model.ContentItem) bool {
	for _, t := range item.Tags {
		if t == "radiko" || t == "sufikso" || t == "prefikso" {
			return true
		}
	}
	return false
}

// splitSuggestions turns "w1, w2; w3" into single suggestions.
func splitSuggestions(t string) []Suggestion {
	var out []Suggestion
	for _, sense := range strings.Split(t, ";") {
		for _, w := range strings.Split(sense, ",") {
			if w = strings.TrimSpace(w); w != "" && len(out) < 6 {
				out = append(out, Suggestion{Text: w, Source: "Reta Vortaro"})
			}
		}
	}
	return out
}

// pivURL links to the word in PIV at vortaro.net. Its address is the
// search state after "#": "<word>_kd" searches headwords (k) and
// derivations (d) (vortaro.net/js/piv.js, KreiStatonElAdreso). A root,
// an inflected form or a related-word hit is looked up by the headword
// ReVo matched ("hom" → homo), since PIV finds words, not roots; spaces are %20, as piv.js decodes with decodeURI.
func pivURL(word string, hit *revo.Hit) string {
	w := revo.Normalize(word)
	if hit != nil && len(hit.Heads) > 0 && !hit.HasHead(w) {
		w = hit.Heads[0] // knabojn → knabo, hom → homo, trankvile → trankvila
	}
	if w == "" {
		return "https://vortaro.net/"
	}
	return "https://vortaro.net/#" + strings.ReplaceAll(url.PathEscape(w), "+", "%2B") + "_kd"
}
