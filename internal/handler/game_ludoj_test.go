package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
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

// wordsServer is the stub site with the day's word of level made word (for
// A0 a word with a picture, so the picture hint shows).
func wordsServer(t *testing.T, s *stubSite, level, word string) string {
	u, _ := url.Parse(s.srv.URL)
	proxy := httputil.NewSingleHostReverseProxy(u)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ludo-api/vortoj", func(w http.ResponseWriter, r *http.Request) {
		var out []string
		for _, l := range strings.Split(s.get(t, "/ludo-api/vortoj?"+r.URL.RawQuery), "\n") {
			if strings.HasPrefix(l, "T\t"+level+"\t") {
				l = "T\t" + level + "\t" + word
			}
			out = append(out, l)
		}
		io.WriteString(w, strings.Join(out, "\n"))
	})
	mux.Handle("/", proxy)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestPlayVortludo: from the levels screen, the day's word of the learner's
// level (a word with a picture): the first letter pressed on the screen's
// keyboard, three misses typed in the x-system, the picture hint taken, the
// word typed in the h-system — found on the fourth row; in both rooms, every
// text fitting, nothing posted. Then the day's word comes from the site's
// list, never from yesterday's kept copy; without the site, free play.
func TestPlayVortludo(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	pool := gamedata.Pool(c.Words(), "A0")
	word, def := "", false
	for _, l := range strings.Split(s.get(t, "/ludo-api/vortoj?lingvo=en"), "\n") {
		if f := strings.Split(l, "\t"); f[0] == "W" && f[3] == "1" && f[2] == "A0" && f[6] != "" && word == "" && f[1] != "abelo" {
			word, def = f[1], f[5] != ""
		}
	}
	if word == "" {
		t.Fatal("no A0 word with a picture")
	}
	base := wordsServer(t, s, "A0", word)
	misses := []string{"abelo"}
	for _, w := range pool {
		if w != word && w != "abelo" && len(misses) < 3 {
			misses = append(misses, w)
		}
	}
	if !strings.Contains(strings.Join(pool, " "), "abelo") {
		t.Fatal("abelo not in the A0 pool")
	}
	var dir string
	for _, room := range []string{"rm_tall", "rm_wide"} {
		keys := []string{":30", "down:1", ":1", "enter:1", ":4", "down:1", ":1", "enter:1", ":1"} // Vortludo; the screen's a
		for i, m := range misses {
			if i == 0 {
				m = m[1:]
			}
			keys = append(keys, "text="+xSpell(m)+":1", ":1", "enter:1", ":2")
		}
		keys = append(keys, "up:1", ":1", "up:1", ":1")
		if def {
			keys = append(keys, "up:1", ":1")
		}
		keys = append(keys, "enter:1", ":2") // Bildo
		keys = append(keys, "text="+strings.ReplaceAll(hSpell(word), "u", "ux")+":1", ":1", "enter:1", ":4")
		dir = gameDir(t, c)
		args := append([]string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 5), "-keys", strings.Join(keys, ",")}, shot("vortludo-"+room)...)
		playArgs(t, dir, args, fmt.Sprintf(`global.scr == 8 && global.vl_word == "%s" && global.vl_daily && global.vl_state == 1 && global.vl_row == 4 && global.vl_hint == 1 && global.vl_pic >= 0 && global.ovf == 0`, word))
	}
	// the kept copy says another word is the day's: the site's list wins
	kept := filepath.Join(dir, "ludo-vortoj-en.txt") // the game keeps its files in its working folder
	b, err := os.ReadFile(kept)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(kept, []byte(strings.ReplaceAll(string(b), "T\tA0\t"+word, "T\tA0\t"+misses[1])), 0o644)
	open := []string{":30", "down:1", ":1", "enter:1", ":6"}
	playArgs(t, dir, []string{"play", "-base", base + "/ludo/", "-steps", "50", "-keys", strings.Join(open, ",")},
		fmt.Sprintf(`global.scr == 8 && global.vl_daily && global.vl_word == "%s"`, word))
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	os.WriteFile(kept, []byte(strings.ReplaceAll(string(b), "T\tA0\t"+word, "T\tA0\t"+misses[1])), 0o644)
	playArgs(t, dir, []string{"play", "-base", deadURL + "/ludo/", "-steps", "50", "-keys", strings.Join(open, ",")},
		`global.scr == 8 && !global.vl_daily && global.vl_state == 0`)
	if len(s.posts) != 0 {
		t.Errorf("Vortludo posted %q", s.posts)
	}
}

// TestPlayVortludoLong: a B1 learner's Vortludo — the day's word long
// (komputilo, nine letters: the widest board), its affix (il) beside the
// board; two misses (and a row too short, refused), then the word on the third row, in both rooms, every
// text fitting. Then Nova vorto: another long word with its affix, free.
func TestPlayVortludoLong(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	s.rating = 1500 // B1
	c := s.api.Course(t.Context())
	word := "komputilo"
	if !strings.Contains(" "+strings.Join(gamedata.Pool(c.Words(), "B1"), " ")+" ", " "+word+" ") {
		t.Fatalf("%s not in the B1 pool", word)
	}
	base := wordsServer(t, s, "B1", word)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		keys := []string{":30", "down:1", ":1", "enter:1", ":4"}
		for _, m := range []string{"kantistoj", "lernejo", "tranĉiloj", "komputilo"} {
			keys = append(keys, "text="+xSpell(m)+":1", ":1", "enter:1", ":2")
		}
		dir := gameDir(t, c)
		args := append([]string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 5), "-keys", strings.Join(keys, ",")}, shot("vortludo-long-"+room)...)
		// lernejo is too short: not a row
		playArgs(t, dir, args, fmt.Sprintf(`global.scr == 8 && global.vl_word == "%s" && global.vl_daily && global.vl_long && global.vl_n == 9 && global.vl_aff == "il" && global.vl_state == 1 && global.vl_row == 3 && global.ovf == 0`, word))
		keys = append(keys, "enter:1", ":2") // Nova vorto
		playArgs(t, dir, []string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 5), "-keys", strings.Join(keys, ",")},
			`global.scr == 8 && !global.vl_daily && global.vl_long && global.vl_aff != "" && global.vl_n >= 6 && global.vl_n <= 9 && global.vl_state == 0 && global.ovf == 0`)
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
	playArgs(t, gameDir(t, c), []string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")},
		`global.scr == 9 && global.rc_place == 1 && global.rp_n >= 4 && global.rc_state >= 1 && global.ovf == 0`)
	// every word of the list Vetkuro takes fits a phone's gate
	dir = gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "rccheck.gml"), []byte(`// rccheck(): the words rc_pool_add takes that don't fit a phone's gate.
var i, n;
n = 0;
for (i = 0; i < global.nw; i += 1) {
  global.rp_n = 0;
  rc_pool_add(global.w_word[i], global.w_mean[i]);
  if (global.rp_n == 1) { font(17, 1, 0); if (string_width(global.w_word[i]) > 97 || !fits(global.w_word[i], 97, 17, 1)) n += 1; }
}
return n;
`), 0o644)
	keys = append(append([]string{}, start...), ":5")
	playArgs(t, dir, []string{"play", "-base", s.srv.URL + "/ludo/", "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys)), "-keys", strings.Join(keys, ",")},
		`global.nw > 500 && rccheck() == 0 && global.ovf == 0`)
	// the questions go by the word, not the learner's language: a picture,
	// else the masked definition, else the meaning marked with its language
	os.WriteFile(filepath.Join(dir, "game", "scripts", "rccues.gml"), []byte(`// rccues(): 1 when the race's pool asks rightly — pictures among them, a
// translation only where a word has neither picture nor definition, and
// marked; else 0.
var i, j, k, pics;
pics = 0;
for (k = 0; k < global.rp_n; k += 1) {
  j = -1; for (i = 0; i < global.nw; i += 1) if (global.w_word[i] == global.rp_word[k]) j = i;
  if (global.rp_pic[k] >= 0) { pics += 1; continue; }
  if (j >= 0 && global.w_def[j] != "") { if (global.rp_tr[k] || global.rp_q[k] != global.w_def[j]) return 0; continue; }
  if (!global.rp_tr[k] || string_char_at(global.rp_q[k], 1) != "(") return 0;
}
return pics > 0;
`), 0o644)
	playArgs(t, dir, []string{"play", "-base", s.srv.URL + "/ludo/", "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys)), "-keys", strings.Join(keys, ",")},
		`global.scr == 9 && rccues() && global.ovf == 0`)
	// a picture question, a definition one: shown, every text fitting
	for name, pick := range map[string]string{"bildo": "global.rp_pic[q] >= 0", "difino": "global.rp_pic[q] < 0 && !global.rp_tr[q]"} {
		d := gameDir(t, c)
		f := filepath.Join(d, "game", "scripts", "rc_next.gml")
		b, _ := os.ReadFile(f)
		os.WriteFile(f, []byte(strings.Replace(string(b), "q = floor(random(global.rp_n));", "for (q = 0; q < global.rp_n - 1; q += 1) if ("+pick+") break;", 1)), 0o644)
		keys := append(append([]string{}, start...), ":40")
		playArgs(t, d, append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys)), "-keys", strings.Join(keys, ",")}, shot("vetkuro-"+name+"-rm_tall")...),
			`global.scr == 9 && global.rc_state == 1 && (`+strings.ReplaceAll(pick, "[q]", "[global.rc_q]")+`) && global.ovf == 0`)
	}
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

// TestTypedHSystemPosts: a vocabulary place with a ŭ word typed in the
// h-system (ankaŭ as "ankau", which no conversion makes ankaŭ): the game takes the answers by both readings and
// sends the site the answers themselves, which checkAnswer judges right —
// the ratings' guard.
func TestTypedHSystemPosts(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	slug, want := "", ""
	for _, p := range c.Places {
		for _, e := range p.Ex {
			if e.Kind == 'Q' && slug == "" && strings.Contains(e.Answers[0], "ŭ") && !strings.ContainsAny(e.Answers[0], ",: ") && p.Kind == 'E' {
				slug, want = p.Slug, e.Answers[0]
			}
		}
	}
	if slug == "" {
		t.Fatal("no vocabulary place with a ŭ word")
	}
	keys := append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return true })...)
	playArgs(t, gameDir(t, c), []string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-steps", fmt.Sprint(steps(keys) + 200), "-keys", strings.Join(keys, ",")},
		fmt.Sprintf(`stars_get("%s") == 3 && global.ob_n == 0`, slug))
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for _, p := range s.posts {
		found = found || strings.HasSuffix(p, " "+want)
	}
	if !found {
		t.Errorf("%s: %q not posted as itself: %q", slug, want, s.posts)
	}
	for _, a := range s.attempts {
		if !a.Correct {
			t.Errorf("%s: the site judged the game's typed answer wrong", a.ContentItemID)
		}
	}
	t.Logf("%s: %d typed answers posted, %q among them", slug, len(s.posts), want)
}
