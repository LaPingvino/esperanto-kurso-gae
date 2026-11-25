package handler

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// TestGameParity plays the game's parity variant (game/parity.toml) against
// the real /ludo-api/ (over the seed): it fetches the map and every place's
// lesson as the game does, answers every exercise through its own GML
// checks, and the site's checkAnswer must judge the same answers alike —
// every fill-in, multiple choice, vocabulary word and typed answer, right
// and wrong; every gap's tiles must hold its answer once; the game's own
// word cards and sentences must take their answer and refuse another.
// Needs the gameviber command (GAMEVIBER, or on PATH); skipped without it.
func TestGameParity(t *testing.T) {
	gv := os.Getenv("GAMEVIBER")
	if gv == "" {
		var err error
		if gv, err = exec.LookPath("gameviber"); err != nil {
			t.Skip("no gameviber command (set GAMEVIBER)")
		}
	}
	root, _ := filepath.Abs("../..")
	items := seedSet(t)
	api := NewGameAPI(&seedSource{items: items})
	mux := http.NewServeMux()
	api.Routes(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	course := api.Course(t.Context())

	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	ui, _ := gamedata.ReadLocale(filepath.Join(root, "internal", "locale", "eo.json"))
	os.WriteFile(filepath.Join(dir, "game", "scripts", "course_data.gml"), []byte(gamedata.GML(course, ui)), 0o644)
	cmd := exec.Command(gv, "play", "-variant", "parity", "-base", srv.URL+"/ludo/", "-steps", "1000", "-expect", "global.par_done", "game")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil || strings.Contains(string(out), "error") {
		t.Fatalf("%v\n%s", err, out)
	}

	byslug := map[string]*model.ContentItem{}
	for _, it := range items {
		byslug[it.Slug] = it
	}
	want := map[string]bool{}
	for _, p := range course.Places {
		for _, e := range p.Ex {
			if e.Slug != "" {
				want[e.Slug] = true
			}
		}
	}
	f, err := os.Open(filepath.Join(dir, "parity.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	counts := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		p := strings.Split(sc.Text(), "\t")
		counts[p[0]]++
		switch p[0] {
		case "X":
			t.Errorf("not loaded: %q", p)
		case "R", "W":
			it := byslug[p[1]]
			if it == nil {
				t.Errorf("%s: not in the content", p[1])
				continue
			}
			game, site := p[2] == "1", checkAnswer(it, p[3:])
			if game != site {
				t.Errorf("%s %s (%s) %q: the game says %v, the site %v", p[0], p[1], it.Type, p[3:], game, site)
			}
			if p[0] == "R" {
				delete(want, p[1])
				if !site {
					t.Errorf("%s (%s): the right answer %q is not accepted", p[1], it.Type, p[3:])
				}
			} else if site {
				t.Errorf("%s (%s): the wrong answer %q is accepted", p[1], it.Type, p[3:])
			}
		case "T":
			if p[3] != "1" {
				t.Errorf("%s gap %s: %s of its tiles are right, want 1", p[1], p[2], p[3])
			}
		case "B":
			if p[2] != "1" {
				t.Errorf("sentence %s: in order, not accepted", p[1])
			}
		case "O":
			if p[2] != "0" {
				t.Errorf("sentence %s: in the tiles' order, accepted", p[1])
			}
		case "C":
			if p[2] != "1" || p[3] != "0" {
				t.Errorf("word card %s: its own meaning %s, another %s (want 1, 0)", p[1], p[2], p[3])
			}
		}
	}
	for slug := range want {
		t.Errorf("%s (%s): in the course, never answered by the parity run", slug, byslug[slug].Type)
	}
	t.Logf("checked over %d places: %d right / %d wrong answers, %d gaps' tiles, %d sentences, %d word cards",
		len(course.Places), counts["R"], counts["W"], counts["T"], counts["B"], counts["C"])
}
