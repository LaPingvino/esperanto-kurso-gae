package handler

import (
	"net/http"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// VocabKind is a vocab item's word kind as far as pictures care: "sufikso",
// "prefikso" or "tabelvorto" (these never get a picture), else "".
func VocabKind(item *model.ContentItem) string {
	for _, t := range item.Tags {
		switch t {
		case "sufikso", "prefikso", "tabelvorto":
			return t
		}
	}
	return ""
}

// vocabPicture is a vocab item's word picture, or nil.
func vocabPicture(item *model.ContentItem) *bildoj.Picture {
	if item == nil || item.Type != "vocab" {
		return nil
	}
	return bildoj.ForVocab(item.Word(), VocabKind(item))
}

// ShowCredits handles GET /fontoj: where the site's pictures come from, as
// their licences require (OpenMoji's CC BY-SA asks for attribution).
func (h *ContentHandler) ShowCredits(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	data := map[string]interface{}{
		"User":     u,
		"UILang":   UILangFor(u),
		"Pictures": bildoj.All(),
	}
	if err := h.tmpl.ExecuteTemplate(w, "fontoj.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
