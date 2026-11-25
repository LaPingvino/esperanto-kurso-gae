package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The game's content, read at run time: the course as the game shows it
// (internal/gamedata), built from the same approved content the site shows
// and served as plain text lines the game parses (gamedata/text.go):
//
//	GET /ludo-api/kurso                       the map: levels, series, places
//	GET /ludo-api/leciono/{slug}?lingvo=xx    a place: its text and exercises,
//	                                          meanings in xx (else English)
//	GET /ludo-api/vortoj?lingvo=xx            the vocabulary words, for the
//	                                          bonus games (gamedata/words.go)
//	GET /ludo-api/vorto?v=…                   "1" if v is an Esperanto word
//	                                          (legvortoj.Valid), else "0":
//	                                          Vortludo's guesses
//
// Mounted outside AuthMiddleware: the same for everyone, no user looked up
// or created, no cookie set, so the answers can be cached publicly (ETag by
// the course's version). The course is built from the content store's
// in-memory approved set and built again only when that set reloads: no
// Datastore reads of its own.

type approvedSource interface {
	ApprovedShared(ctx context.Context) ([]*model.ContentItem, error)
}

// GameAPI serves /ludo-api/.
type GameAPI struct {
	src   approvedSource
	cache gamedata.Cache
}

func NewGameAPI(src approvedSource) *GameAPI { return &GameAPI{src: src} }

// Course is the current course (nil when the content can't be read).
func (a *GameAPI) Course(ctx context.Context) *gamedata.Course {
	items, err := a.src.ApprovedShared(ctx)
	if err != nil {
		return nil
	}
	return a.cache.Get(items)
}

// PlaceOf is the game's place showing a site item ("" for none): the
// "play as a game" links.
func (a *GameAPI) PlaceOf(ctx context.Context, itemSlug string) string {
	if c := a.Course(ctx); c != nil {
		return c.PlaceOf(itemSlug)
	}
	return ""
}

// cached sets the headers and answers 304 when the game has this version.
func cached(w http.ResponseWriter, r *http.Request, etag string) bool {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", `"`+etag+`"`)
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, `"`+etag+`"`) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

// Map handles GET /ludo-api/kurso.
func (a *GameAPI) Map(w http.ResponseWriter, r *http.Request) {
	c := a.Course(r.Context())
	if c == nil {
		http.Error(w, "la enhavo ne legeblas", http.StatusServiceUnavailable)
		return
	}
	if cached(w, r, c.Version) {
		return
	}
	gamedata.WriteCourse(w, c)
}

// Lesson handles GET /ludo-api/leciono/{slug}.
func (a *GameAPI) Lesson(w http.ResponseWriter, r *http.Request) {
	c := a.Course(r.Context())
	if c == nil {
		http.Error(w, "la enhavo ne legeblas", http.StatusServiceUnavailable)
		return
	}
	p := c.Place(r.PathValue("slug"))
	if p == nil {
		http.NotFound(w, r)
		return
	}
	lang := r.URL.Query().Get("lingvo")
	if len(lang) > 8 || strings.ContainsAny(lang, `"/ `) {
		lang = "en"
	}
	if cached(w, r, c.Version+"-"+lang) {
		return
	}
	gamedata.WriteLesson(w, c, p, lang)
}

// Words handles GET /ludo-api/vortoj: the words, and the day's (UTC).
func (a *GameAPI) Words(w http.ResponseWriter, r *http.Request) {
	c := a.Course(r.Context())
	if c == nil {
		http.Error(w, "la enhavo ne legeblas", http.StatusServiceUnavailable)
		return
	}
	lang := r.URL.Query().Get("lingvo")
	if len(lang) > 8 || strings.ContainsAny(lang, `"/ `) {
		lang = "en"
	}
	day := time.Now().UTC().Format("2006-01-02")
	if cached(w, r, c.Version+"-"+lang+"-"+day) {
		return
	}
	gamedata.WriteWords(w, c, lang, day)
}

// Word handles GET /ludo-api/vorto?v=…: "1" when v is a word (a ReVo
// headword or its inflection, or known roots and affixes with an ending),
// "0" when not. The answer never changes with the course: cached a day.
func (a *GameAPI) Word(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query().Get("v")
	ans := "0"
	if utf8.RuneCountInString(v) <= 16 && legvortoj.Valid(v) {
		ans = "1"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	io.WriteString(w, ans)
}

// Routes are the API's routes, to mount outside AuthMiddleware.
func (a *GameAPI) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ludo-api/kurso", a.Map)
	mux.HandleFunc("GET /ludo-api/leciono/{slug}", a.Lesson)
	mux.HandleFunc("GET /ludo-api/vortoj", a.Words)
	mux.HandleFunc("GET /ludo-api/vorto", a.Word)
}
