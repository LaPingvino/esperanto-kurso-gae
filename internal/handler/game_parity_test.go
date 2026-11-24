package handler

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// TestGameParity plays the game's parity variant (game/parity.toml), which
// answers every exercise of the game through its own GML checks, and judges
// the same answers with the site's checkAnswer: every fill-in's right answer
// must be accepted by both, a wrong tile rejected by both, every gap must
// offer its right answer exactly once among its tiles, and the game's own
// exercises (sentences, word cards) must accept their answer and reject
// another. Needs the gameviber command (GAMEVIBER, or on PATH); skipped
// without it.
func TestGameParity(t *testing.T) {
	gv := os.Getenv("GAMEVIBER")
	if gv == "" {
		var err error
		if gv, err = exec.LookPath("gameviber"); err != nil {
			t.Skip("no gameviber command (set GAMEVIBER)")
		}
	}
	root, _ := filepath.Abs("../..")
	seed := filepath.Join(root, "seed", "ekzercoj-zagr-restructured.json")
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	run := func(in, name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = in
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run(root, "go", "run", "./cmd/export-game", "-seed", seed,
		"-locale", filepath.Join(root, "internal", "locale", "eo.json"), "-out", filepath.Join(dir, "game", "scripts", "course_data.gml"))
	run(dir, gv, "play", "-variant", "parity", "-steps", "1", "game")

	b, err := os.ReadFile(seed)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Slug, Type, SeriesSlug, SeriesParent string
		Content                              map[string]any
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	items := map[string]*model.ContentItem{}
	readings := map[string]bool{}
	for _, r := range rows {
		items[r.Slug] = &model.ContentItem{Slug: r.Slug, Type: r.Type, Content: r.Content}
		if r.SeriesSlug == "zagr-metodo" && r.Type == "reading" {
			readings[r.Slug] = true
		}
	}
	want := map[string]bool{} // every fill-in of the lessons the game has
	for _, r := range rows {
		if r.Type == "fillin" && readings[r.SeriesParent] {
			want[r.Slug] = true
		}
	}

	f, err := os.Open(filepath.Join(dir, "parity.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	counts := map[string]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		p := strings.Split(sc.Text(), "\t")
		counts[p[0]]++
		switch p[0] {
		case "F", "W":
			it := items[p[1]]
			if it == nil {
				t.Errorf("%s: not in the seed", p[1])
				continue
			}
			game, site := p[2] == "1", checkAnswer(it, p[3:])
			if game != site {
				t.Errorf("%s %s %q: the game says %v, the site %v", p[0], p[1], p[3:], game, site)
			}
			if p[0] == "F" {
				delete(want, p[1])
				if !site {
					t.Errorf("%s: the right answers %q are not accepted", p[1], p[3:])
				}
			} else if site {
				t.Errorf("%s: the wrong answer %q is accepted", p[1], p[3:])
			}
		case "T":
			if p[3] != "1" {
				t.Errorf("%s gap %s: %s of its tiles are right, want 1", p[1], p[2], p[3])
			}
		case "B":
			if p[2] != "1" {
				t.Errorf("sentence %s: placed in order (%s words), not accepted", p[1], p[3])
			}
		case "R":
			if p[2] != "0" {
				t.Errorf("sentence %s: placed in the wrong order, accepted", p[1])
			}
		case "C":
			if p[2] != "1" || p[3] != "0" {
				t.Errorf("word card %s: its own meaning %s, another %s (want 1, 0)", p[1], p[2], p[3])
			}
		}
	}
	for slug := range want {
		t.Errorf("%s: in the course, not in the game", slug)
	}
	if counts["F"] == 0 || counts["B"] == 0 || counts["C"] == 0 {
		t.Errorf("an empty parity run: %v", counts)
	}
	t.Logf("checked: %d fill-ins (and as many wrong answers), %d gaps' tiles, %d sentences, %d word cards", counts["F"], counts["T"], counts["B"], counts["C"])
}
