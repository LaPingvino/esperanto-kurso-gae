package gamedata

import (
	"sort"
	"strings"
	"sync"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/revo"
)

// Konstrui's pool beyond the course's own words: words built from the
// roots and affixes the course teaches. A part is taught at the first level
// that has it — as a bare root or affix card, or as a piece of a course
// word (A0's hundo teaches hund, its knabino -in-). A derived word is a
// ReVo headword whose split is beyond doubt (legvortoj.Blocks), every piece
// of it taught; its level is that of its hardest piece. Its prompt is the
// course words' chain (kb_next): its picture, else ReVo's definition of
// that derivation, else a marked translation.

// PerAffix is how many derived words with one affix a level takes at most
// (the commonest first), so its pool is a mix and not mostly -in-.
const PerAffix = 12

// MinLangs: a derived word ReVo translates into fewer languages is too rare
// for a learner, unless it is official (Fundamento, Official Additions).
const MinLangs = 5

// lowerHead: the entry's headword h is written small (not a name).
func lowerHead(e *revo.Entry, h string) bool {
	for _, x := range e.Heads {
		if revo.Normalize(x) == h && x == strings.ToLower(x) {
			return true
		}
	}
	return false
}

// blockable: ReVo's headwords with Blocks, once (the course doesn't change them).
var (
	blockOnce sync.Once
	blockable []string
)

func blockWords() []string {
	blockOnce.Do(func() {
		for _, h := range revo.Headwords() {
			if letters(h) && legvortoj.Blocks(h) != nil {
				blockable = append(blockable, h)
			}
		}
	})
	return blockable
}

// taught is the level index each root and affix is first taught at.
func taught(course []*WordEntry) map[string]int {
	lv := map[string]int{}
	for i, l := range Levels {
		lv[l] = i
	}
	out := map[string]int{}
	add := func(part string, li int) {
		if old, ok := out[part]; !ok || li < old {
			out[part] = li
		}
	}
	for _, w := range course {
		li, ok := lv[w.Level]
		if !ok {
			continue
		}
		word := revo.Normalize(w.Word)
		if w.Ex.Bare {
			add(word, li)
			continue
		}
		if ps := legvortoj.Parts(word); ps != nil {
			for _, p := range ps {
				if p.Kind != "finaĵo" && p.Kind != "ligilo" {
					add(p.Text, li)
				}
			}
			continue
		}
		for _, e := range []string{"oj", "o", "a", "e", "i"} { // hundo: hund
			if r, cut := strings.CutSuffix(word, e); cut && revo.IsRoot(r) {
				add(r, li)
				break
			}
		}
	}
	return out
}

// Derived are Konstrui's words built from the course's roots and affixes,
// not among its own words (course: Words without them); by level, then
// word. The same course gives the same list.
func Derived(course []*WordEntry) []*WordEntry {
	at := taught(course)
	have := map[string]bool{}
	for _, w := range course {
		have[revo.Normalize(w.Word)] = true
	}
	type cand struct {
		w        *WordEntry
		affixes  []string
		li, rank int
	}
	var cs []cand
	for _, h := range blockWords() {
		if have[h] {
			continue
		}
		hit := revo.Exact(h)
		if hit == nil {
			continue
		}
		if m := hit.Main(); m < len(hit.Senses) {
			if s := hit.Senses[m]; s.Fak || s.Tax || s.Stl {
				continue // a special field's word, a taxon, a styled use: not for a learner
			}
		}
		if hit.Def == "" && hit.Translation("en") == "" || !lowerHead(hit.Entry, h) {
			continue // no prompt, or a name (Julino, Klaranino)
		}
		ps := legvortoj.Blocks(h)
		li, ok := 0, true
		var affixes []string
		for _, p := range ps[:len(ps)-1] {
			l, known := at[p.Text]
			if !known || p.Kind == "radiko" && legvortoj.AffixKind(p.Text) != "" {
				ok = false
				break
			}
			if l > li {
				li = l
			}
			if p.Kind != "radiko" {
				affixes = append(affixes, p.Text)
			}
		}
		if !ok {
			continue
		}
		rank := strings.Count(hit.Trd, "\n") + 1 // the more languages translate it, the commoner
		if rank < MinLangs && hit.Ofc == "" {
			continue // a rare word (komputebla)
		}
		if PictureKey(h) != "" {
			rank += 1000
		}
		if hit.Ofc != "" {
			rank += 100
		}
		e := &Exercise{Kind: 'Q', Answers: []string{h}, Revo: hit}
		cs = append(cs, cand{&WordEntry{Word: h, Level: Levels[li], Blocks: BlockLabels(ps), Derived: true, Ex: e}, affixes, li, rank})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].rank != cs[j].rank {
			return cs[i].rank > cs[j].rank
		}
		return cs[i].w.Word < cs[j].w.Word
	})
	var out []*WordEntry
	n := map[string]int{}
next:
	for _, c := range cs {
		for _, a := range c.affixes {
			if n[c.w.Level+"/"+a] >= PerAffix {
				continue next
			}
		}
		for _, a := range c.affixes {
			n[c.w.Level+"/"+a]++
		}
		out = append(out, c.w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level < out[j].Level
		}
		return out[i].Word < out[j].Word
	})
	return out
}
