package gamedata

import (
	"sort"
	"strings"
)

// Frazotrajno, the sentence train: a sentence of the course's readings, its
// words as wagons to couple in the text's order. The sentences come with the
// word list (WriteWords), one line each:
//
//	F <level> <place> <sentence>
//
// level: its reading's series' level; place: the reading's place slug (the
// bonus after that place plays its own sentences). The sentence is written
// as the text has it, without the final stop; its words are split at spaces.

// TrainMin and TrainMax are the words a train has, at least and at most.
const (
	TrainMin = 3
	TrainMax = 7
)

// Sentence is one train: a sentence of a reading.
type Sentence struct {
	Text, Level, Place string
}

// Sentences are the trains of the course, each sentence once (its first
// place), in map order: every TrainOK sentence of every reading.
func (c *Course) Sentences() []*Sentence {
	var out []*Sentence
	seen := map[string]bool{}
	for _, s := range c.Series {
		for _, p := range s.Places {
			if p.Kind != 'L' {
				continue
			}
			for _, t := range plainSentences(p.Text) {
				if !TrainOK(t) || seen[t] {
					continue
				}
				seen[t] = true
				out = append(out, &Sentence{Text: t, Level: s.Level, Place: p.Slug})
			}
		}
	}
	return out
}

// TrainOK: a sentence the train judges safely by the text's order alone —
// three to seven words, no comma (its wagons would lose it), and no kaj or
// aŭ: "patro kaj frato" is as right as "frato kaj patro", and the game
// can't tell which orders the grammar allows, so it plays only sentences
// where the text's order is the one to find.
func TrainOK(s string) bool {
	if strings.Contains(s, ",") {
		return false
	}
	ws := strings.Fields(s)
	if len(ws) < TrainMin || len(ws) > TrainMax {
		return false
	}
	for _, w := range ws {
		if l := strings.ToLower(w); l == "kaj" || l == "aŭ" {
			return false
		}
	}
	return true
}

// TrainPool is Frazotrajno's sentences for a learner of level: that
// level's and, while there are fewer than ten, the levels below — never
// above (words not yet taught); sorted. The game builds the same
// (ft_pool.gml) and plays five of it in a round.
func TrainPool(ss []*Sentence, level string) []string {
	li := 0
	for i, l := range Levels {
		if l == level {
			li = i
		}
	}
	var out []string
	for i := li; i >= 0 && (i == li || len(out) < 10); i-- {
		for _, s := range ss {
			if s.Level == Levels[i] {
				out = append(out, s.Text)
			}
		}
	}
	sort.Strings(out)
	return out
}
