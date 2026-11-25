package handler

import (
	"html/template"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// Levels are the KER levels in order: the stations of the map and the
// footer's path strip.
var Levels = []string{"A0", "A1", "A2", "B1", "B2", "C1", "C2"}

// AssetVersion goes on the static files' URLs (?v=) so a deploy reaches
// browsers at once; main sets it from the files' contents.
var AssetVersion = "1"

// AnglaArrival is angla.org's arrival URL: links carry their origin so the
// other side can greet the learner (kurso-design DESIGN.md, the hand-off).
const AnglaArrival = "https://angla.org/?de=esperanto-kurso"

// Icon is one of the drawn icons of templates/partials/ikonoj.html.
func Icon(name string) template.HTML {
	return template.HTML(`<svg class="ico" aria-hidden="true"><use href="#i-` + template.HTMLEscapeString(name) + `"/></svg>`)
}

// ItemKind is the activity kind of an item as the learner meets it: its
// type, except that a multiple choice with a picture is a picture choice.
func ItemKind(item *model.ContentItem) string {
	if item == nil {
		return ""
	}
	if item.Type == "multiplechoice" {
		if w, _ := item.Content["bildo"].(string); w != "" {
			return "bildo"
		}
	}
	return item.Type
}

var kindIcons = map[string]string{
	"bildo":          "image",
	"image":          "image",
	"multiplechoice": "choose",
	"fillin":         "pen",
	"listening":      "sound",
	"vocab":          "card",
	"reading":        "book",
	"phrasebook":     "chat",
	"video":          "video",
}

// kindNames are in Esperanto on purpose: the kind is part of the course's
// own vocabulary, shown beside its icon (DESIGN.md 9: Esperanto chrome).
var kindNames = map[string]string{
	"bildo":          "Bildo",
	"image":          "Bildo",
	"multiplechoice": "Elekti",
	"fillin":         "Plenigi",
	"listening":      "Aŭskulti",
	"vocab":          "Vorto",
	"reading":        "Legi",
	"phrasebook":     "Frazo",
	"video":          "Spekti",
}

// KindIcon names the icon of an item's kind.
func KindIcon(item *model.ContentItem) string {
	if n, ok := kindIcons[ItemKind(item)]; ok {
		return n
	}
	return "blocks"
}

// KindName is the word for a kind (an item type or "bildo").
func KindName(kind string) string {
	if n, ok := kindNames[kind]; ok {
		return n
	}
	return kind
}

// DefinitionIn is a vocab definition in lang only: content["definitions"][lang],
// else the legacy flat content["definition"] as English.
func DefinitionIn(content map[string]interface{}, lang string) string {
	if content == nil {
		return ""
	}
	if defs, ok := content["definitions"].(map[string]interface{}); ok {
		if v, ok := defs[lang].(string); ok && v != "" {
			return v
		}
	}
	if lang == "en" {
		if v, ok := content["definition"].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// DefinitionFor is DefinitionIn for an item.
func DefinitionFor(item *model.ContentItem, lang string) string {
	if item == nil {
		return ""
	}
	return DefinitionIn(item.Content, lang)
}
