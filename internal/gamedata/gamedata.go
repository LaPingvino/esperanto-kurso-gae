// Package gamedata turns the course's own content (the site's approved
// model.ContentItems) into the game's: the map of levels, series and places
// (course.go), each place's lesson — its text, the site's exercises with
// their answer data, and the game's own derived ones (word cards, sentences
// to build) — written as plain text lines the game parses (text.go), served
// by the site at /ludo-api/ (handler/game_api.go). Nothing is retyped: every
// string comes from the content, read through the site's model and eo code.
// fallback.go writes a built-in copy of part of it into the game for when
// the site can't be reached.
package gamedata

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// seedItem is a seed file's row (the restructured files use the Go field names).
type seedItem struct {
	Slug, Type, SeriesSlug, SeriesParent, SeriesLabel, Status, Source, ImageURL string
	SeriesOrder                                                                 int
	Rating                                                                      float64
	Content                                                                     map[string]any
	Tags                                                                        []string
}

// ReadSeed reads a seed file into the site's content items.
func ReadSeed(path string) ([]*model.ContentItem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []seedItem
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out := make([]*model.ContentItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, &model.ContentItem{Slug: r.Slug, Type: r.Type, Content: r.Content, Tags: r.Tags, Status: r.Status, Source: r.Source,
			SeriesSlug: r.SeriesSlug, SeriesOrder: r.SeriesOrder, SeriesParent: r.SeriesParent, SeriesLabel: r.SeriesLabel,
			Rating: r.Rating, ImageURL: r.ImageURL})
	}
	return out, nil
}

// ReadSeedDir reads every seed file in dir into one set, by slug: the files
// in name order, the restructured ones (with the series fields) last, a
// later file's item replacing an earlier one's.
func ReadSeedDir(dir string) ([]*model.ContentItem, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.SliceStable(files, func(i, j int) bool { // the restructured files last: they win
		ri, rj := strings.Contains(files[i], "restructured"), strings.Contains(files[j], "restructured")
		if ri != rj {
			return rj
		}
		return files[i] < files[j]
	})
	by := map[string]*model.ContentItem{}
	var order []string
	for _, f := range files {
		items, err := ReadSeed(f)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it.Slug == "" {
				continue
			}
			if by[it.Slug] == nil {
				order = append(order, it.Slug)
			}
			by[it.Slug] = it
		}
	}
	out := make([]*model.ContentItem, 0, len(order))
	for _, s := range order {
		out = append(out, by[s])
	}
	return out, nil
}

// ReadLocale reads one of the site's interface-string files.
func ReadLocale(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	return m, json.Unmarshal(b, &m)
}

// token is a piece of a fill-in's question as the game lays it out: a word
// (with its punctuation) or a gap; space: a space comes before it.
type token struct {
	s     string
	gap   int // -1: text; else the gap's number
	space bool
}

// tokens splits a question into words and gaps, keeping a gap glued to the
// letters around it (lernant___kaj: "lernant", gap, "kaj", no spaces).
func tokens(q string) []token {
	var out []token
	gap := 0
	for wi, word := range strings.Fields(q) {
		first := true // the word's first piece takes the space before it
		add := func(t token) {
			t.space = first && wi > 0
			first = false
			out = append(out, t)
		}
		for pi, p := range strings.Split(word, "___") {
			if pi > 0 {
				add(token{"", gap, false})
				gap++
			}
			if p != "" {
				add(token{p, -1, false})
			}
		}
	}
	return out
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// gapIsSuffix: the g-th gap follows a letter (lernant___): an ending.
func gapIsSuffix(q string, g int) bool {
	parts := strings.Split(q, "___")
	if g >= len(parts) {
		return false
	}
	rs := []rune(parts[g])
	return len(rs) > 0 && unicode.IsLetter(rs[len(rs)-1])
}

// matchCase writes s with its first letter as like's is.
func matchCase(s, like string) string {
	rs, lr := []rune(s), []rune(like)
	if len(rs) == 0 || len(lr) == 0 {
		return s
	}
	if unicode.IsUpper(lr[0]) {
		rs[0] = unicode.ToUpper(rs[0])
	} else {
		rs[0] = unicode.ToLower(rs[0])
	}
	return string(rs)
}

// sentences are the lesson text's plain sentences of 3 to 7 words (no
// quotes, dashes or digits), as word lists without the final stop.
func sentences(text string) [][]string {
	var out [][]string
	start := 0
	rs := []rune(text)
	for i, r := range rs {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		s := strings.TrimSpace(string(rs[start:i]))
		start = i + 1
		if r != '.' || strings.ContainsAny(s, "\"'„“”«»—–-:;()0123456789\n") {
			continue
		}
		ws := strings.Fields(strings.ReplaceAll(s, ",", ""))
		if len(ws) >= 3 && len(ws) <= 7 {
			out = append(out, ws)
		}
	}
	return out
}

// shuffled is a fixed shuffle of xs (by key), never the order given when
// another order exists.
func shuffled(xs []string, key string) []string {
	out := append([]string(nil), xs...)
	h := hash(key, 0)
	for i := len(out) - 1; i > 0; i-- {
		h = h*6364136223846793005 + 1442695040888963407
		j := int((h >> 33) % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	if strings.Join(out, "\x00") == strings.Join(xs, "\x00") && len(out) > 1 {
		for i := range out { // rotate by one
			out[i] = xs[(i+1)%len(xs)]
		}
	}
	return out
}

func hash(s string, n int) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s#%d", s, n)
	return h.Sum64()
}

func containsFold(xs []string, s string) bool {
	for _, x := range xs {
		if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(s)) {
			return true
		}
	}
	return false
}

// findRoot finds the vocabulary entry for a base form from eo.ExtractWords:
// the word itself, or its root without the ending, or with common suffixes
// taken off (instruistino → instru). DERIVED: the course links no words to
// lessons, so this is the game's own matching, not the site's.
func findRoot(base string, vocab map[string]*model.ContentItem) (string, *model.ContentItem) {
	if v := vocab[base]; v != nil {
		return base, v
	}
	stem := base
	if n := len(stem); n > 1 && strings.ContainsRune("oaie", rune(stem[n-1])) {
		stem = stem[:n-1]
	}
	for {
		if v := vocab[stem]; v != nil && len([]rune(stem)) >= 2 {
			return stem, v
		}
		cut := false
		for _, suf := range []string{"in", "ist", "ej", "ul", "ar", "et", "eg", "il", "an", "ec", "aĵ", "ad", "ig", "iĝ", "ebl", "ind", "em", "er", "estr", "id"} {
			if strings.HasSuffix(stem, suf) && len([]rune(stem))-len([]rune(suf)) >= 2 {
				stem = strings.TrimSuffix(stem, suf)
				cut = true
				break
			}
		}
		if !cut {
			return "", nil
		}
	}
}

func definitions(v *model.ContentItem) map[string]string {
	out := map[string]string{}
	if m, ok := v.Content["definitions"].(map[string]any); ok {
		for k, d := range m {
			if s, ok := d.(string); ok && strings.TrimSpace(s) != "" {
				out[k] = strings.TrimSpace(s)
			}
		}
	}
	return out
}

// gmlStr writes s as a GML string expression: GameMaker 5's strings have no
// escapes, so a double quote is spliced in from a single-quoted one; '#' is
// GML's line break, so a real one is written \#, and a newline becomes '#'.
func gmlStr(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "#", `\#`)
	s = strings.ReplaceAll(s, "\n", "#")
	parts := strings.Split(s, `"`)
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, `+'"'+`)
}
