package legvortoj

import (
	"math/rand"
	"testing"
)

func TestValid(t *testing.T) {
	for _, w := range []string{
		"domo", "lernejo", "komputilo", "ankaŭ", // dictionary words
		"domojn", "lernejoj", "kantis", "skribus", "bele", // inflected
		"domŝipo", "domŝipojn", "abonpago", // compounds ReVo lacks
		"malbonulejo", "malbonulejoj", "gelernantoj", "rekantos", // derivations
		"tiujn", "kiuj", "ĉiujn", "ilin", "kiun", "kion", "tien", // correlatives, pronouns
		"tranĉiloj", "kantistoj", "trancxiloj",
	} {
		if !Valid(w) {
			t.Errorf("%s: not valid", w)
		}
	}
	for _, w := range []string{
		"", "xqzvb", "bzzrktpln", "qwrtzp", "aeiou", // random letters
		"domok", "kantistok", "lernejt", "komputilk", // a bad ending
		"kajn", "dom-o", "domo1",
	} {
		if Valid(w) {
			t.Errorf("%s: valid", w)
		}
	}
}

// TestValidRandom: random letters of a guess's length ending in a vowel
// are mostly no word (permissive, but not any letters).
func TestValidRandom(t *testing.T) {
	letters := []rune("abcĉdefgĝhĥijĵklmnoprsŝtuŭvz")
	r := rand.New(rand.NewSource(1))
	for _, n := range []int{5, 7, 9} {
		ok := 0
		const tries = 2000
		for i := 0; i < tries; i++ {
			w := make([]rune, n)
			for k := range w {
				w[k] = letters[r.Intn(len(letters))]
			}
			w[n-1] = []rune("oaei")[r.Intn(4)]
			if Valid(string(w)) {
				ok++
			}
		}
		t.Logf("%d letters: %d of %d random strings valid", n, ok, tries)
		if ok*5 > tries {
			t.Errorf("%d letters: %d of %d random strings valid", n, ok, tries)
		}
	}
}
