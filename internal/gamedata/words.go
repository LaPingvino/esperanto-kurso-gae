package gamedata

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/bildoj"
)

// The bonus games (Vortludo, a Wordle; Vetkuro, a race) learn the same list
// in other ways: the course's vocabulary words, each in the level of its
// series, as the map has them.
//
//	GET /ludo-api/vortoj?lingvo=xx
//
//	VORTOJ <version> <day>
//	W <word> <level> <five> <meaning in xx> <eo definition> <picture>
//	T <level> <word>      the day's Vortludo word for a learner of that level
//	FINO
//
// five: 1 when the word is a Vortludo word (five letters, ĉ ĝ ĥ ĵ ŝ ŭ one
// each, small letters only, a whole word rather than a bare root). The eo
// definition (the course's, else ReVo's) has the word itself masked (~), so
// a hint never gives the answer. picture: the word's picture (bildoj), by
// its file's name without folder or extension ("1f415"); the game carries
// the five-letter ones (cmd/game-bildoj), and goes by the definition for
// any it doesn't have.

// WordEntry is one vocabulary word of the course.
type WordEntry struct {
	Word, Level string
	Five        bool
	Ex          *Exercise
}

// Words are the course's vocabulary words, each once (its first place), in map order.
func (c *Course) Words() []*WordEntry {
	var out []*WordEntry
	seen := map[string]bool{}
	for _, s := range c.Series {
		for _, p := range s.Places {
			for _, e := range p.Ex {
				if e.Kind != 'Q' || len(e.Answers) == 0 {
					continue
				}
				w := strings.TrimSpace(e.Answers[0])
				if w == "" || seen[w] {
					continue
				}
				seen[w] = true
				out = append(out, &WordEntry{Word: w, Level: s.Level, Five: !e.Bare && IsFive(w), Ex: e})
			}
		}
	}
	return out
}

// IsFive: five Esperanto letters, small (no names, no spaces or hyphens).
func IsFive(w string) bool {
	rs := []rune(w)
	if len(rs) != 5 {
		return false
	}
	for _, r := range rs {
		if !strings.ContainsRune("abcĉdefgĝhĥijĵklmnoprsŝtuŭvz", r) {
			return false
		}
	}
	return true
}

// FivePool is the Vortludo words for a learner of level: that level's, and
// while there are fewer than ten, the levels below it, then above it; sorted.
// The game builds the same pool for free play (vl_pool.gml).
func FivePool(ws []*WordEntry, level string) []string {
	li := 0
	for i, l := range Levels {
		if l == level {
			li = i
		}
	}
	var out []string
	add := func(l string) {
		for _, w := range ws {
			if w.Five && w.Level == l {
				out = append(out, w.Word)
			}
		}
	}
	add(Levels[li])
	for i := li - 1; i >= 0 && len(out) < 10; i-- {
		add(Levels[i])
	}
	for i := li + 1; i < len(Levels) && len(out) < 10; i++ {
		add(Levels[i])
	}
	sort.Strings(out)
	return out
}

// Daily is the day's word of a pool (day: "2026-11-25"): the same for every
// learner of a level, another the next day.
func Daily(pool []string, day, level string) string {
	if len(pool) == 0 {
		return ""
	}
	return pool[hash(day+"/"+level, 0)%uint64(len(pool))]
}

// eoDef is a word's Esperanto definition with the word masked.
func eoDef(e *Exercise, word string) string {
	d := e.Defs["eo"]
	if d == "" && e.Revo != nil {
		d = e.Revo.Def
	}
	if d == "" {
		return ""
	}
	stem := word
	if n := len([]rune(stem)); n > 3 && strings.ContainsRune("oaie", []rune(stem)[n-1]) {
		stem = string([]rune(stem)[:n-1])
	}
	return maskFold(d, stem)
}

// maskFold replaces every occurrence of stem (capitals alike) and the rest
// of its word with "~".
func maskFold(s, stem string) string {
	rs, st := []rune(s), []rune(strings.ToLower(stem))
	var b strings.Builder
	for i := 0; i < len(rs); {
		if i+len(st) <= len(rs) && strings.ToLower(string(rs[i:i+len(st)])) == string(st) {
			b.WriteRune('~')
			i += len(st)
			for i < len(rs) && unicode.IsLetter(rs[i]) {
				i++
			}
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

// PictureKey is the name the game knows a word's picture by ("" for none).
func PictureKey(word string) string {
	p := bildoj.ForVocab(word, "")
	if p == nil {
		return ""
	}
	f := p.File[strings.LastIndex(p.File, "/")+1:]
	return strings.TrimSuffix(f, f[strings.LastIndex(f, "."):])
}

// WriteWords writes the word list, meanings in lang, the day's words for day.
func WriteWords(w io.Writer, c *Course, lang, day string) {
	ws := c.Words()
	line(w, "VORTOJ", c.Version, day)
	for _, e := range ws {
		line(w, "W", e.Word, e.Level, fmt.Sprint(b2i(e.Five)), Meaning(e.Ex.Defs, e.Ex.Revo, lang), eoDef(e.Ex, e.Word), PictureKey(e.Word))
	}
	for _, l := range Levels {
		if d := Daily(FivePool(ws, l), day, l); d != "" {
			line(w, "T", l, d)
		}
	}
	line(w, "FINO")
}
