package handler

import (
	"net/url"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/lingvoj"
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

// courseDefs is the card's definitions per language (the flat legacy
// "definition" as English).
func courseDefs(content map[string]interface{}) map[string]string {
	out := map[string]string{}
	defs, _ := content["definitions"].(map[string]interface{})
	for l, v := range defs {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			out[l] = strings.TrimSpace(s)
		}
	}
	if out["en"] == "" {
		if s, _ := content["definition"].(string); strings.TrimSpace(s) != "" {
			out["en"] = strings.TrimSpace(s)
		}
	}
	return out
}

// courseDef is the card's own definition in lang and whether it is not
// really in lang: a copy of another language's definition, or text in
// another script (English on a Hindi card, Greek on a German one).
func courseDef(content map[string]interface{}, lang string) (def string, copied bool) {
	defs := courseDefs(content)
	def = defs[lang]
	return def, def != "" && (lingvoj.CopySource(defs, lang) != "" || lingvoj.WrongScript(def, lang))
}

// MeaningFor runs the fallback chain for a vocab card in the learner's
// language: the course's own definition → a community translation →
// ReVo's translation → English, the course's or ReVo's → ReVo's Esperanto
// definition. A course definition copied from another language (see
// lingvoj.CopiedFrom) does not count as one in the learner's language.
//
// A card or a reading's word may name the sense it means (content
// "senco", a ReVo sense marker: boa.0o.KOMUNE, the snake, not the scarf):
// then the definition and the translations are that sense's.
func MeaningFor(item *model.ContentItem, lang, community string) Meaning {
	word := item.Word()
	hit := revo.Lookup(word, isRootCard(item))
	m := Meaning{PIVURL: pivURL(word, hit)}
	var sense *revo.Sense
	if hit != nil {
		m.ReVoURL, m.ReVoFound = hit.URL(), true
		m.Via = hit.Via
		m.EoDef, m.EoDefRevo = hit.Def, hit.Def != ""
		if mrk, _ := item.Content["senco"].(string); mrk != "" {
			if sense = hit.Sense(mrk); sense != nil {
				m.ReVoURL = revo.ArticleURL(sense.Mrk)
				if sense.Def != "" {
					m.EoDef = sense.Def
				}
			}
		}
	} else {
		m.ReVoURL = revo.SearchURL(word)
	}
	// ReVo's translation: the sense's own; for the main sense, else the
	// word's (whose translations are the main sense's).
	revoTr := func(l string) string {
		if sense != nil {
			if t := sense.Translation(l); t != "" {
				return t
			}
			if hit.Senses[hit.Main()].Mrk != sense.Mrk {
				return ""
			}
		}
		return hit.Translation(l)
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
	if t := revoTr(lang); t != "" && !lingvoj.WrongScript(t, lang) {
		m.Suggestions = splitSuggestions(t)
		return set(t, lang, "revo")
	}
	if en, _ := courseDef(item.Content, "en"); en != "" {
		return set(en, "en", "course")
	}
	if t := revoTr("en"); t != "" {
		return set(t, "en", "revo")
	}
	if src := lingvoj.CopySource(courseDefs(item.Content), lang); src != "" {
		// a copy, shown as what it is: the other language's text
		return set(own, src, "course")
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
