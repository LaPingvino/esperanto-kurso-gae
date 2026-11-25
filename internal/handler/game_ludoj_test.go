package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
)

// The bonus games (Vortludo, Vetkuro) played headless against the stub site,
// and the typed-answer input's x- and h-systems over every typed answer of
// the course. Neither game sends anything to the site (only a course
// exercise's typed answer moves the ratings).

// hSpell writes s in the h-system (ĉ → ch, ŭ → u), ASCII for a key script.
func hSpell(s string) string {
	return strings.NewReplacer("ĉ", "ch", "ĝ", "gh", "ĥ", "hh", "ĵ", "jh", "ŝ", "sh", "ŭ", "u",
		"Ĉ", "Ch", "Ĝ", "Gh", "Ĥ", "Hh", "Ĵ", "Jh", "Ŝ", "Sh", "Ŭ", "U").Replace(s)
}

// xSpell writes s in the x-system (ĉ → cx).
func xSpell(s string) string {
	return strings.NewReplacer("ĉ", "cx", "ĝ", "gx", "ĥ", "hx", "ĵ", "jx", "ŝ", "sx", "ŭ", "ux").Replace(s)
}

// shot: with GAME_SHOTS set, where the run's last frame goes.
func shot(name string) []string {
	d := os.Getenv("GAME_SHOTS")
	if d == "" {
		return nil
	}
	return []string{"-shot", filepath.Join(d, name+".png")}
}

func playArgs(t *testing.T, dir string, args []string, expect string) {
	t.Helper()
	cmd := exec.Command(gameviber(t), append(append(args, "-expect", expect), "game")...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "pass") || strings.Contains(string(out), "error") {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
}

func (s *stubSite) get(t *testing.T, path string) string {
	r, err := http.Get(s.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

func steps(keys []string) int {
	n := 0
	for _, k := range keys {
		var m int
		fmt.Sscan(k[strings.LastIndex(k, ":")+1:], &m)
		n += m
	}
	return n
}

// TestPlayVortludo: from the levels screen, the day's word of the learner's
// level (the site's T line): three misses typed in the x-system, the hint
// taken, the word typed in the h-system — found on the fourth row; in both
// rooms, every text fitting, nothing posted.
func TestPlayVortludo(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	ws := c.Words()
	pool := gamedata.FivePool(ws, "A0")
	word := ""
	for _, l := range strings.Split(s.get(t, "/ludo-api/vortoj?lingvo=en"), "\n") {
		if f := strings.Split(l, "\t"); f[0] == "T" && f[1] == "A0" {
			word = f[2]
		}
	}
	if word == "" {
		t.Fatal("no day's word for A0")
	}
	var misses []string
	for _, w := range pool {
		if w != word && len(misses) < 3 {
			misses = append(misses, w)
		}
	}
	for _, room := range []string{"rm_tall", "rm_wide"} {
		keys := []string{":30", "down:1", ":1", "enter:1", ":4"}
		for _, m := range misses {
			keys = append(keys, "text="+xSpell(m)+":1", ":1", "enter:1", ":2")
		}
		keys = append(keys, "up:1", ":1", "up:1", ":1", "enter:1", ":2") // the last hint button
		keys = append(keys, "text="+strings.ReplaceAll(hSpell(word), "u", "ux")+":1", ":1", "enter:1", ":4")
		if strings.Contains(word, "ŭ") {
			keys[len(keys)-4] = "text=" + xSpell(word) + ":1"
		}
		dir := gameDir(t, c)
		args := append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 5), "-keys", strings.Join(keys, ",")}, shot("vortludo-"+room)...)
		playArgs(t, dir, args, fmt.Sprintf(`global.scr == 8 && global.vl_word == "%s" && global.vl_daily && global.vl_state == 1 && global.vl_row == 4 && global.vl_hint > 0 && global.ovf == 0`, word))
	}
	if len(s.posts) != 0 {
		t.Errorf("Vortludo posted %q", s.posts)
	}
}

// TestPlayVetkuro: the race from the levels screen — driven to the right
// lane each time (the AUTO variant, steering with the keys' rc_steer) it
// scores and keeps its lives; held in the left lane, the wrong gates are
// crashes, each a life, and the count adds up. Then the bonus after a
// place: the race with that place's words. Every text fits; nothing posted.
func TestPlayVetkuro(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	start := []string{":30", "down:1", ":1", "down:1", ":1", "enter:1", ":4", "enter:1", ":2"} // Vetkuro, Komenci
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "auto.toml"), []byte("[constants]\n  AUTO = \"1\"\n"), 0o644)
	keys := append(append([]string{}, start...), ":1500")
	playArgs(t, dir, append([]string{"play", "-variant", "auto", "-base", s.srv.URL + "/ludo/", "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys)), "-keys", strings.Join(keys, ",")}, shot("vetkuro-rm_tall")...),
		`global.scr == 9 && global.rc_score >= 4 && global.rc_lives == 3 && global.rc_crash == 0 && global.ovf == 0`)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		keys = append(append([]string{}, start...), "left:1", ":3000")
		playArgs(t, gameDir(t, c), []string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys)), "-keys", strings.Join(keys, ",")},
			`global.scr == 9 && global.rc_lane == 0 && global.rc_crash >= 1 && global.rc_n == global.rc_score + global.rc_crash && global.rc_lives == 3 - global.rc_crash && global.ovf == 0`)
	}
	// the bonus after a vocabulary place: its words
	slug := ""
	for _, p := range c.Places {
		n := 0
		for _, e := range p.Ex {
			if e.Kind == 'Q' {
				n++
			}
		}
		if n >= 4 && slug == "" {
			slug = p.Slug
		}
	}
	keys = append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return false })...)
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4", "enter:1", ":200") // Bonuso: Vetkuro, Komenci
	playArgs(t, gameDir(t, c), []string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")},
		`global.scr == 9 && global.rc_place == 1 && global.rp_n >= 4 && global.rc_state >= 1 && global.ovf == 0`)
	if len(s.posts) != 0 {
		t.Errorf("Vetkuro posted %q", s.posts)
	}
}

// TestTypingSystems: every typed answer of the course (fill-ins' gaps,
// vocabulary words, typed answers), written in the h-system or the
// x-system and converted as the game's input does (xsys), is the same
// answer by the game's judging (hfold) — flughaveno, shown fluĝaveno, too.
func TestTypingSystems(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	seen := map[string]bool{"flughaveno": true, "dishaki": true, "ĉashundo": true, "aŭto": true}
	for _, p := range c.Places {
		for _, e := range p.Ex {
			if e.Kind == 'F' || e.Kind == 'Q' || e.Kind == 'H' {
				for _, a := range append(append([]string{}, e.Answers...), e.Alts...) {
					seen[a] = true
				}
			}
		}
	}
	var b strings.Builder
	b.WriteString("// hcheck(): the typed answers that don't come through the input (0: all do).\nvar n;\nn = 0;\n")
	q := func(s string) string { return `"` + strings.ReplaceAll(s, `"`, `"+'"'+"`) + `"` }
	for a := range seen {
		if strings.ContainsAny(a, "#\n") {
			continue
		}
		for _, typed := range []string{a, hSpell(a), xSpell(a)} {
			fmt.Fprintf(&b, "if (hfold(xsys(%s)) != hfold(%s)) { n += 1; show_debug_message(%s); }\n", q(typed), q(a), q(typed))
		}
	}
	b.WriteString("return n;\n")
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "hcheck.gml"), []byte(b.String()), 0o644)
	playArgs(t, dir, []string{"play", "-steps", "2"}, `hcheck() == 0 && hfold(xsys("flughaveno")) == hfold("flughaveno") && xsys("cxu sxi") == "ĉu ŝi" && xsys("chu shi") == "ĉu ŝi"`)
	t.Logf("%d answers, each typed three ways", len(seen))
}
