package gamedata

import "sort"

// Vortpluvo, the word rain: the course's words fall, the learner types each
// before it lands. It plays from the word list's W lines; nothing of its own
// goes into the list.

// RainMin and RainMax are the letters a falling word has, at least and at
// most (RainMax: a phone's field holds it at its size).
const (
	RainMin = 2
	RainMax = 12
)

// RainOK: a word Vortpluvo drops — the course's own (not Derived), one word
// of the alphabet's small letters (no name, space or hyphen to type), two to
// twelve letters (pv_ok.gml).
func RainOK(w *WordEntry) bool {
	n := len([]rune(w.Word))
	return !w.Derived && letters(w.Word) && n >= RainMin && n <= RainMax
}

// RainPool is Vortpluvo's words for a learner of level: those RainOK of
// that level and, while there are fewer than twenty, of the levels below —
// never above (a word not yet taught); sorted. The game builds the same
// (pv_pool.gml).
func RainPool(ws []*WordEntry, level string) []string {
	li := 0
	for i, l := range Levels {
		if l == level {
			li = i
		}
	}
	var out []string
	for i := li; i >= 0 && (i == li || len(out) < 20); i-- {
		for _, w := range ws {
			if w.Level == Levels[i] && RainOK(w) {
				out = append(out, w.Word)
			}
		}
	}
	sort.Strings(out)
	return out
}
