package gamedata

import (
	"fmt"
	"io"
	"strings"
)

// The game reads the course as lines of tab-separated fields (GML has no
// JSON reader; this it splits with string_pos). A field never holds a tab
// or a line break; '#', GML's line break when drawn, is written \#.
//
// The map (GET /ludo-api/kurso):
//
//	KURSO <version>
//	N <lang>                          a language the meanings can be shown in
//	S <series slug> <level> <label>   a series, in map order
//	P <place slug> <kind> <exercises> <title>   its places in order; kind L, E or V
//	FINO
//
// A place (GET /ludo-api/leciono/<slug>?lingvo=<lang>):
//
//	LECIONO <version> <slug> <kind>
//	T <title>
//	X <paragraph>                     the text, a line per paragraph
//	U <item slug>                     a video: watched at /ekzerco/<item slug>
//	then the exercises, each a head line and its own lines:
//	C <show> <seen> <right> <m0> <m1> <m2> <m3>   word card: meanings, right = index
//	F <slug> <question> <hint>        fill-in; O <text> <gap> <space> per token,
//	                                  A <answer> per gap answer, K <tiles…> per gap
//	M <slug> <question> <right>       multiple choice; K <options…>, Y <alternative>…
//	Q <slug> <meaning> <hint>         vocabulary: the word for the meaning;
//	                                  A <word>, Y <alternative>…, K <tiles…>
//	H <slug> <question> <hint>        a typed answer; A, Y, K as Q
//	B <sentence>                      build the sentence; K <tiles…>
//	FINO
//
// "FINO" closes each: a text cut short is never taken for a whole one.

// field makes s safe as a field.
func field(s string) string {
	s = strings.NewReplacer("\t", " ", "\r", "", "\n", " ", "#", `\#`).Replace(s)
	return strings.TrimSpace(s)
}

func line(w io.Writer, fs ...string) {
	for i, f := range fs {
		if i > 0 {
			io.WriteString(w, "\t")
		}
		io.WriteString(w, field(f))
	}
	io.WriteString(w, "\n")
}

// WriteCourse writes the map.
func WriteCourse(w io.Writer, c *Course) {
	line(w, "KURSO", c.Version)
	for _, l := range c.Langs {
		line(w, "N", l)
	}
	for _, s := range c.Series {
		line(w, "S", s.Slug, s.Level, s.Label)
		for _, p := range s.Places {
			line(w, "P", p.Slug, string(p.Kind), fmt.Sprint(len(p.Ex)), p.Title)
		}
	}
	line(w, "FINO")
}

// WriteLesson writes a place, its meanings in lang (else English).
func WriteLesson(w io.Writer, c *Course, p *Place, lang string) {
	line(w, "LECIONO", c.Version, p.Slug, string(p.Kind))
	line(w, "T", p.Title)
	for _, para := range strings.Split(strings.ReplaceAll(p.Text, "\r", ""), "\n") {
		if strings.TrimSpace(para) != "" {
			line(w, "X", para)
		}
	}
	if p.Video != "" {
		line(w, "U", p.Video)
	}
	for _, e := range p.Ex {
		switch e.Kind {
		case 'C':
			show, seen := e.Word.Shown()
			ms := []string{Meaning(e.Word.Defs, lang)}
			for _, o := range e.Word.Others {
				ms = append(ms, Meaning(o.Defs, lang))
			}
			order := shuffled(ms, e.Word.Root)
			right := 0
			for i, m := range order {
				if m == ms[0] {
					right = i
					break
				}
			}
			line(w, append([]string{"C", show, seen, fmt.Sprint(right)}, order...)...)
		case 'F':
			line(w, "F", e.Slug, e.Question, e.Hint)
			for _, t := range tokens(e.Question) {
				line(w, "O", t.s, fmt.Sprint(t.gap), fmt.Sprint(b2i(t.space)))
			}
			for _, a := range e.Answers {
				line(w, "A", a)
			}
			for _, ts := range e.Tiles {
				line(w, append([]string{"K"}, ts...)...)
			}
		case 'M':
			line(w, "M", e.Slug, e.Question, fmt.Sprint(e.Correct))
			line(w, append([]string{"K"}, e.Options...)...)
			for _, a := range e.Alts {
				line(w, "Y", a)
			}
		case 'Q', 'H':
			q := e.Question
			if e.Kind == 'Q' {
				q = Meaning(e.Defs, lang)
			}
			line(w, string(e.Kind), e.Slug, q, e.Hint)
			line(w, "A", e.Answers[0])
			for _, a := range e.Alts {
				line(w, "Y", a)
			}
			line(w, append([]string{"K"}, e.Tiles[0]...)...)
		case 'B':
			line(w, "B", e.Sentence)
			line(w, append([]string{"K"}, e.Tiles[0]...)...)
		}
	}
	line(w, "FINO")
}
