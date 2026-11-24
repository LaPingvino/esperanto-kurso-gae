package gamedata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlayFirstLessons plays the game's lessons headless through its
// real input — the arrow keys move the focus, Enter presses — answering
// every exercise rightly, in the wide room and in the tall (phone) one, and
// expects the lesson finished with three stars and the next one open. The
// keys are worked out from the same course data the game is built from.
// Lessons 1, 1–2, and all of them in one go.
// Needs the gameviber command (GAMEVIBER, or on PATH); skipped without it.
func TestPlayFirstLessons(t *testing.T) {
	gv := os.Getenv("GAMEVIBER")
	if gv == "" {
		var err error
		if gv, err = exec.LookPath("gameviber"); err != nil {
			t.Skip("no gameviber command (set GAMEVIBER)")
		}
	}
	root, _ := filepath.Abs("../..")
	items, err := ReadSeed(filepath.Join(root, "seed", "ekzercoj-zagr-restructured.json"))
	if err != nil {
		t.Fatal(err)
	}
	ui, err := ReadLocale(filepath.Join(root, "internal", "locale", "eo.json"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := MakeCourse(items)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "game", "esperanto-kurso-ludo.txt"))
	if err := os.WriteFile(filepath.Join(dir, "game", "scripts", "course_data.gml"), []byte(GML(c, ui)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, room := range []string{"rm_wide", "rm_tall"} {
		for _, n := range []int{1, 2, len(c.Lessons)} { // lessons 1 … n, one after the other, from the map
			var keys []string
			for l := 0; l < n; l++ {
				keys = append(keys, lessonKeys(c, l)...)
			}
			steps := 0
			for _, k := range keys {
				var s int
				fmt.Sscanf(k[strings.Index(k, ":")+1:], "%d", &s)
				steps += s
			}
			os.Remove(filepath.Join(dir, "esperanto-kurso-ludo.txt")) // a new player each run
			expect := fmt.Sprintf("global.scr == 0 && global.stars[%d] == 3", n-1)
			if n < len(c.Lessons) {
				expect += fmt.Sprintf(" && global.stars[%d] == 0", n)
			}
			cmd := exec.Command(gv, "play", "-room", room, "-steps", fmt.Sprint(steps+2), "-keys", strings.Join(keys, ","), "-expect", expect, "game")
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "pass") || strings.Contains(string(out), "error") {
				t.Errorf("%s, lessons 1–%d: %v\n%s", room, n, err, out)
			}
		}
	}
}

// lessonKeys are the keys that open lesson l from the map and answer each of
// its exercises rightly, in the order the game asks them, back to the map.
func lessonKeys(c *Course, l int) []string {
	press := func(downs int) []string { // move the focus to the downs-th thing, press it
		var ks []string
		for i := 0; i < downs; i++ {
			ks = append(ks, "down:1", ":2")
		}
		return append(ks, "enter:1", ":2")
	}
	ks := []string{":2", "enter:1", ":2"} // the map's main button: the next lesson
	ks = append(ks, press(1)...)          // its text: Komenci, the first thing on the screen
	les := c.Lessons[l]
	for k, wi := range les.Words {
		if k == 5 {
			break
		}
		for o, x := range c.Words[wi].Options {
			if x == wi {
				ks = append(ks, press(o+1)...)
			}
		}
		ks = append(ks, "enter:1", ":2") // Daŭrigi
	}
	for _, f := range les.Fill {
		for g, tiles := range f.Tiles {
			for t, x := range tiles {
				if x == strings.TrimSpace(f.Answers[g%len(f.Answers)]) {
					ks = append(ks, press(t+1)...)
					break
				}
			}
		}
		ks = append(ks, "enter:1", ":2")
	}
	for _, b := range les.Build {
		used := make([]bool, len(b.Tiles))
		for placed, word := range strings.Fields(b.Sentence) {
			for t, x := range b.Tiles {
				if !used[t] && x == word {
					pos := placed // the placed words come first, then the tiles left, in order
					for u := 0; u < t; u++ {
						if !used[u] {
							pos++
						}
					}
					used[t] = true
					ks = append(ks, press(pos+1)...)
					break
				}
			}
		}
		ks = append(ks, "enter:1", ":2")
	}
	return append(ks, "enter:1", ":2") // the lesson's result: Daŭrigi, to the map
}
