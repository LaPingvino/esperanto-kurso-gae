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
// board; a guess that is no word (the row shakes, no try spent), a row too
// short (refused), two misses (tranĉiloj asked of the site), then the word
// on the third row, in both rooms, every text fitting. Then Nova vorto: another long word with its affix, free.
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
		// bzzrktpln is no word: the row shakes, no try spent
		dir := gameDir(t, c)
		bad := append(append([]string{}, keys...), "text=bzzrktpln:1", ":1", "enter:1", ":3")
		playArgs(t, dir, []string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(bad)), "-keys", strings.Join(bad, ",")},
			`global.scr == 8 && global.vl_state == 0 && global.vl_row == 0 && global.vl_shake > 0 && global.vl_msg != "" && global.vl_cur == "bzzrktpln" && global.vl_wait == 0`)
		// lernejo is too short: not a row, nothing asked
		short := append(append([]string{}, keys...), "text=lernejo:1", ":1", "enter:1", ":3")
		playArgs(t, dir, []string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(short)), "-keys", strings.Join(short, ",")},
			`global.scr == 8 && global.vl_row == 0 && global.vl_shake == 0 && global.vl_msg != "" && global.vl_wait == 0`)
		for _, m := range []string{"kantistoj", "tranĉiloj", "komputilo"} {
			keys = append(keys, "text="+xSpell(m)+":1", ":1", "enter:1", ":3")
		}
		args := append([]string{"play", "-base", base + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 5), "-keys", strings.Join(keys, ",")}, shot("vortludo-long-"+room)...)
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
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4", "enter:1", ":200") // Bonuso: Vetkuro (before Vortpluvo, Konstrui, Memoro and Frazotrajno), Komenci
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

// TestPlayKonstrui: Konstrui from the levels screen, in both rooms, on two
// words (pinned, the bank left in order for the test): lernejo's pieces put
// on the line by their number keys, the first taken back and all put again,
// Kontroli — right; aviadilo with its first two pieces swapped — wrong,
// each placed piece showing which. Every text fits; nothing posted. Then
// the game's pool for every level is the site's (gamedata.BuildPool), each
// word's bank holds its pieces and others of their families, ten at most,
// fitting a phone; and the bonus after a place plays that place's words.
func TestPlayKonstrui(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	ws := c.Words()
	blocks := map[string]string{}
	for _, w := range ws {
		blocks[w.Word] = w.Blocks
	}
	w1, w2 := "lernejo", "aviadilo"
	n1, n2 := len(strings.Split(blocks[w1], "|")), len(strings.Split(blocks[w2], "|"))
	if blocks[w1] == "" || n2 < 3 {
		t.Fatalf("no pieces: %s %q, %s %q", w1, blocks[w1], w2, blocks[w2])
	}
	pinned := func(w1, w2 string) string {
		d := gameDir(t, c)
		f := filepath.Join(d, "game", "scripts", "kb_next.gml")
		b, _ := os.ReadFile(f)
		src := string(b)
		for old, new := range map[string]string{
			"for (k = global.kb_nt - 1; k > 0; k -= 1) { i = floor(random(k + 1)); t = global.kb_t[k]; global.kb_t[k] = global.kb_t[i]; global.kb_t[i] = t; }": "",
			"w = global.kb_p[floor(random(global.kb_pn))];\n": `w = 0; for (i = 0; i < global.nw; i += 1) { if (global.kb_w < 0) { if (global.w_word[i] == "` + w1 + `") w = i; } else { if (global.w_word[i] == "` + w2 + `") w = i; } }` + "\n",
		} {
			if !strings.Contains(src, old) {
				t.Fatalf("kb_next: no %q", old)
			}
			src = strings.Replace(src, old, new, 1)
		}
		os.WriteFile(f, []byte(src), 0o644)
		return d
	}
	digits := func(from, to int) []string {
		var ks []string
		for i := from; i <= to; i++ {
			ks = append(ks, fmt.Sprintf("text=%d:1", i%10), ":1")
		}
		return ks
	}
	start := []string{":30", "down:1", ":1", "down:1", ":1", "down:1", ":1", "enter:1", ":4"} // the Ludoj row's third: Konstrui
	run := func(dir, room, shotName string, keys []string, expect string) {
		args := append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 2), "-keys", strings.Join(keys, ",")}, shot(shotName)...)
		playArgs(t, dir, args, expect)
	}
	for _, room := range []string{"rm_tall", "rm_wide"} {
		dir := pinned(w1, w2)
		run(dir, room, "ludoj-"+room, []string{":30"}, `global.scr == 5 && global.ovf == 0`) // the Ludoj row: three games
		keys := append(append([]string{}, start...), digits(1, 2)...)
		keys = append(keys, "down:1", ":1", "enter:1", ":2") // the line's first piece back to the bank
		run(dir, room, "", keys, `global.scr == 11 && global.kb_n == 1 && global.kb_bp[0] == 1 && global.kb_used[0] == 0 && global.fb == 0 && global.ovf == 0`)
		keys = append(keys, "down:1", ":1", "enter:1", ":2") // and the other (taking back clears the focus)
		keys = append(append(keys, digits(1, n1)...), ":2")
		run(dir, room, "konstrui-"+room, keys, fmt.Sprintf(`global.scr == 11 && global.kb_word == "%s" && kb_built() == "%s" && global.fb == 0 && global.ovf == 0`, w1, w1))
		keys = append(keys, "enter:1", ":2") // Kontroli
		run(dir, room, "konstrui-ghuste-"+room, keys, `global.fb == 1 && global.kb_ok == 1 && global.kb_tries == 1 && global.ovf == 0`)
		keys = append(keys, "enter:1", ":2") // Daŭrigi: the next word
		keys = append(append(append(keys, "text=2:1", ":1", "text=1:1", ":1"), digits(3, n2)...), "enter:1", ":2")
		run(dir, room, "konstrui-ne-"+room, keys, fmt.Sprintf(`global.scr == 11 && global.kb_word == "%s" && global.kb_n == %d && global.fb == 2 && global.kb_ok == 1 && global.kb_tries == 2 && global.ovf == 0`, w2, n2))
	}
	// a word asked by its definition, on a phone, after a wrong try
	def := ""
	for _, w := range ws {
		if w.Blocks != "" && gamedata.PictureKey(w.Word) == "" && def == "" {
			def = w.Word
		}
	}
	if def != "" {
		keys := append(append(append([]string{}, start...), "text=2:1", ":1", "text=1:1", ":1"), "enter:1", ":2")
		run(pinned(def, w2), "rm_tall", "konstrui-difino-rm_tall", keys, fmt.Sprintf(`global.kb_word == "%s" && global.kb_pic < 0 && global.fb == 2 && global.ovf == 0`, def))
	}
	// the pools and banks, as the site makes them
	var b strings.Builder
	b.WriteString("// kbcheck(): 0 when Konstrui's pools are the site's and every bank is sound, else what isn't.\nvar l, k, i, j, f, x, n;\nglobal.kb_place = 0;\n")
	q := func(s string) string { return `"` + s + `"` }
	for _, l := range gamedata.Levels {
		p := gamedata.BuildPool(ws, l)
		fmt.Fprintf(&b, "global.acclevel = %s; kb_pool(); if (global.kb_pn != %d) return %s;\n", q(l), len(p), q(l+" pool size"))
		for _, w := range p {
			fmt.Fprintf(&b, "f = 0; for (k = 0; k < global.kb_pn; k += 1) if (global.w_word[global.kb_p[k]] == %s) f = 1; if (!f) return %s;\n", q(w), q(l+" "+w))
		}
	}
	b.WriteString(`for (i = 0; i < global.nw; i += 1) if (global.w_blk[i] != "") {
  global.kb_pn = 1; global.kb_p[0] = i; global.kb_w = -1; kb_next();
  if (global.kb_nt > 10 || global.kb_nt < global.kb_nb + 4) return global.w_word[i] + " bank";
  x = ""; for (k = 0; k < global.kb_nb; k += 1) { x += string_replace_all(global.kb_b[k], "-", ""); f = 0; for (j = 0; j < global.kb_nt; j += 1) if (global.kb_t[j] == global.kb_b[k]) f = 1; if (!f) return global.w_word[i] + " piece"; }
  if (x != global.w_word[i]) return global.w_word[i] + " spelt " + x;
  for (j = 0; j < global.kb_nt; j += 1) if (tile_w(global.kb_t[j], 20, 0) + 24 > 388 - 24) return global.w_word[i] + " wide";
}
return 0;
`)
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "kbcheck.gml"), []byte(b.String()), 0o644)
	// every word drawn: asked rightly (a translation only for a word with
	// neither picture nor definition), with none, its pieces or the whole
	// bank on the line, before and after a wrong verdict; every text fitting
	os.WriteFile(filepath.Join(dir, "game", "scripts", "kbdraw.gml"), []byte(`// kbdraw(): 0 when every Konstrui word draws whole, else the first that doesn't.
var i, k, j, m;
for (i = 0; i < global.nw; i += 1) if (global.w_blk[i] != "") {
  global.kb_pn = 1; global.kb_p[0] = i; global.kb_w = -1; kb_next();
  if (global.kb_tr != (global.w_pic[i] == "" && global.w_def[i] == "")) return global.w_word[i] + " asked";
  if (global.kb_pic < 0 && !global.kb_tr && global.kb_q != global.w_def[i]) return global.w_word[i] + " definition";
  for (m = 0; m < 3; m += 1) {
    for (k = global.kb_n - 1; k >= 0; k -= 1) kb_take(k);
    if (m == 1) for (k = 0; k < global.kb_nb; k += 1) { for (j = 0; j < global.kb_nt; j += 1) if (!global.kb_used[j] && global.kb_t[j] == global.kb_b[k]) break; kb_place(j); }
    if (m == 2) for (j = 0; j < global.kb_nt; j += 1) kb_place(j);
    global.fb = 0; draw_all(); if (global.ovf) return global.w_word[i] + " " + string(m) + " " + global.ovf_what;
    if (m > 0) { global.fb = 2; draw_all(); global.fb = 0; if (global.ovf) return global.w_word[i] + " " + string(m) + " judged " + global.ovf_what; }
  }
}
return 0;
`), 0o644)
	keys := append(append([]string{}, start...), ":2")
	run(dir, "rm_tall", "", keys, `global.scr == 11 && kbcheck() == 0`)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		run(dir, room, "", keys, `global.scr == 11 && kbdraw() == 0`)
	}
	// the bonus after a place with three or more such words: those words
	slug, want := "", 0
	for _, p := range c.Places {
		n := 0
		for _, e := range p.Ex {
			if e.Kind == 'Q' && len(e.Answers) > 0 && blocks[strings.TrimSpace(e.Answers[0])] != "" {
				n++
			}
		}
		if n >= 3 && slug == "" {
			slug, want = p.Slug, n
		}
	}
	if slug == "" {
		t.Fatal("no place with three words of pieces")
	}
	keys = append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return false })...)
	playArgs(t, gameDir(t, c), append([]string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys[:len(keys)-2]) + 10), "-keys", strings.Join(keys[:len(keys)-2], ",")}, shot("bonuso-rm_tall")...),
		`global.scr == 3 && global.ovf == 0`) // the place done: five bonus games, on a phone too
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4") // Bonuso: Konstrui (before Memoro and Frazotrajno)
	args := append([]string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")}, shot("konstrui-bonuso-rm_tall")...)
	playArgs(t, gameDir(t, c), args, fmt.Sprintf(`global.scr == 11 && global.kb_place == 1 && global.kb_pn == %d && global.ovf == 0`, want))
	if len(s.posts) != 0 {
		t.Errorf("Konstrui posted %q", s.posts)
	}
}

// TestPlayMemoro: Memoro from the levels screen, in both rooms, on a board
// left in order for the test (each word's card before its cue's): a pair
// found by the arrow keys and Enter, two that aren't one shown wrong and
// turned back after a while, the rest found — the board done in seven
// tries, and Denove deals a new one. Then the game's pool for every level
// is the site's (gamedata.MemoPool), every word's two cards draw whole (up,
// wrong and found) with its picture, else its definition; and the bonus
// after a place plays that place's words. Nothing posted.
func TestPlayMemoro(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	ws := c.Words()
	by := map[string]*gamedata.WordEntry{}
	for _, w := range ws {
		if by[w.Word] == nil {
			by[w.Word] = w
		}
	}
	pinned := func() string {
		d := gameDir(t, c)
		f := filepath.Join(d, "game", "scripts", "mm_deal.gml")
		b, _ := os.ReadFile(f)
		src := string(b)
		for old, new := range map[string]string{
			"for (k = global.mm_nc - 1; k > 0; k -= 1) { j = floor(random(k + 1)); t = global.mm_cp[k]; global.mm_cp[k] = global.mm_cp[j]; global.mm_cp[j] = t; t = global.mm_ck[k]; global.mm_ck[k] = global.mm_ck[j]; global.mm_ck[j] = t; }": "",
			"i = global.mm_p[floor(random(global.mm_pn))];": "i = global.mm_p[tries mod global.mm_pn];",
		} {
			if !strings.Contains(src, old) {
				t.Fatalf("mm_deal: no %q", old)
			}
			src = strings.Replace(src, old, new, 1)
		}
		os.WriteFile(f, []byte(src), 0o644)
		return d
	}
	start := []string{":30", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "enter:1", ":4"} // the Ludoj row's fourth: Memoro
	run := func(dir, room, shotName string, keys []string, expect string) {
		args := append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 2), "-keys", strings.Join(keys, ",")}, shot(shotName)...)
		playArgs(t, dir, args, expect)
	}
	press := []string{"enter:1", ":1"}
	down := []string{"down:1", ":1"}
	cat := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	for _, room := range []string{"rm_tall", "rm_wide"} {
		dir := pinned()
		run(dir, room, "ludoj-"+room, []string{":30"}, `global.scr == 5 && global.ovf == 0`) // the Ludoj row: five games
		run(dir, room, "memoro-"+room, cat(start, []string{":2"}), `global.scr == 12 && global.mm_n == 6 && global.mm_nc == 12 && global.mm_found == 0 && global.ovf == 0`)
		keys := cat(start, down, press, down, press) // cards 0 and 1: a pair
		run(dir, room, "", keys, `global.scr == 12 && global.mm_found == 1 && global.mm_tries == 1 && global.mm_st[0] == 2 && global.mm_st[1] == 2 && global.ovf == 0`)
		keys = cat(keys, down, press, down, down, press) // cards 2 and 4: none
		run(dir, room, "memoro-ne-"+room, keys, `global.mm_found == 1 && global.mm_tries == 2 && global.mm_b == 4 && global.mm_st[2] == 1 && global.mm_st[4] == 1 && global.ovf == 0`)
		keys = cat(keys, []string{":50"}) // turned back
		run(dir, room, "", keys, `global.mm_a < 0 && global.mm_b < 0 && global.mm_st[2] == 0 && global.mm_st[4] == 0 && global.focus == 4`)
		keys = cat(keys, []string{"up:1", ":1", "up:1", ":1"}, press, down, press) // cards 2 and 3
		for k := 2; k < 6; k++ {
			keys = cat(keys, down, press, down, press)
		}
		run(dir, room, "memoro-finita-"+room, keys, `global.mm_found == 6 && global.mm_tries == 7 && global.ovf == 0`)
		keys = cat(keys, press) // Denove
		run(dir, room, "", keys, `global.scr == 12 && global.mm_found == 0 && global.mm_tries == 0 && global.mm_st[0] == 0 && global.mm_n == 6`)
	}
	// the pools, as the site makes them; every word's cards drawn
	var b strings.Builder
	b.WriteString("// mmcheck(): 0 when Memoro's pools are the site's, else what isn't.\nvar k, f;\nglobal.mm_place = 0;\n")
	q := func(s string) string { return `"` + s + `"` }
	for _, l := range gamedata.Levels {
		p := gamedata.MemoPool(ws, l)
		fmt.Fprintf(&b, "global.acclevel = %s; mm_pool(); if (global.mm_pn != %d) return %s;\n", q(l), len(p), q(l+" pool size"))
		for _, w := range p {
			fmt.Fprintf(&b, "f = 0; for (k = 0; k < global.mm_pn; k += 1) if (global.w_word[global.mm_p[k]] == %s) f = 1; if (!f) return %s;\n", q(w), q(l+" "+w))
		}
	}
	b.WriteString("return 0;\n")
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "mmcheck.gml"), []byte(b.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "mmdraw.gml"), []byte(`// mmdraw(): 0 when every Memoro word's cards draw whole and ask rightly, else the first that doesn't.
var i, k, m, n;
n = 0;
for (i = 0; i < global.nw; i += 1) if (mm_ok(i)) {
  n += 1;
  global.mm_pn = 1; global.mm_p[0] = i; mm_deal();
  if (global.mm_n != 1) return global.w_word[i] + " dealt";
  if (global.w_pic[i] != "") { if (global.mm_pic[0] < 0) return global.w_word[i] + " picture"; }
  else if (global.mm_q[0] != global.w_def[i]) return global.w_word[i] + " definition";
  for (m = 1; m <= 3; m += 1) {
    for (k = 0; k < 2; k += 1) global.mm_st[k] = 1 + (m == 3);
    global.mm_b = -1; if (m == 2) global.mm_b = 1;
    draw_all(); if (global.ovf) return global.w_word[i] + " " + string(m) + " " + global.ovf_what;
  }
}
if (n < 100) return "only " + string(n);
return 0;
`), 0o644)
	keys := cat(start, []string{":2"})
	run(dir, "rm_tall", "", keys, `global.scr == 12 && mmcheck() == 0`)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		run(dir, room, "", keys, `global.scr == 12 && mmdraw() == 0`)
	}
	// the bonus after a place with four or more such words: those words
	slug, want := "", 0
	for _, p := range c.Places {
		seen := map[string]bool{}
		for _, e := range p.Ex {
			if e.Kind == 'Q' && len(e.Answers) > 0 {
				w := strings.TrimSpace(e.Answers[0])
				if by[w] != nil && gamedata.MemoOK(by[w]) {
					seen[w] = true
				}
			}
		}
		if len(seen) >= 4 && slug == "" {
			slug, want = p.Slug, len(seen)
		}
	}
	if slug == "" {
		t.Fatal("no place with four Memoro words")
	}
	keys = append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return false })...)
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4") // Bonuso: Memoro (before Frazotrajno)
	args := append([]string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")}, shot("memoro-bonuso-rm_tall")...)
	playArgs(t, gameDir(t, c), args, fmt.Sprintf(`global.scr == 12 && global.mm_place == 1 && global.mm_pn == %d && global.ovf == 0`, want))
	if len(s.posts) != 0 {
		t.Errorf("Memoro posted %q", s.posts)
	}
}

// TestPlayFrazotrajno: Frazotrajno from the levels screen, in both rooms,
// on a round left in order for the test (the pool's first five sentences,
// the depot in the text's order): a train coupled by the arrow keys and
// Enter, judged right and gone on after a while; a wagon coupled and taken
// back, a train out of order shown wrong, its right start kept and the
// rest back in the depot, then put right and gone on at Enter; a wagon by
// its number key; the round done in six tries, and Denove plays again. Then
// the game's pool for every level is the site's (gamedata.TrainPool), every
// sentence's train draws whole in every state, its depot never in the
// text's order; and the bonus after a place plays that place's sentences.
// Nothing posted.
func TestPlayFrazotrajno(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	ss := c.Sentences()
	var a0 [][]string // the game's pool for an A0 learner, in its order
	for _, x := range ss {
		if x.Level == "A0" {
			a0 = append(a0, strings.Fields(x.Text))
		}
	}
	if len(a0) < 5 {
		t.Fatalf("A0: %d sentences", len(a0))
	}
	if a0[1][1] == a0[1][2] {
		t.Fatalf("the second sentence's words 1 and 2 alike: %q", a0[1])
	}
	pinned := func() string {
		d := gameDir(t, c)
		for file, subs := range map[string]map[string]string{
			"ft_round.gml": {"i = global.ft_p[floor(random(global.ft_pn))];": "i = global.ft_p[tries mod global.ft_pn];"},
			"ft_next.gml": {
				"for (k = global.ft_nb - 1; k > 0; k -= 1) { j = floor(random(k + 1)); t = global.ft_t[k]; global.ft_t[k] = global.ft_t[j]; global.ft_t[j] = t; }\n": "",
				"if (ft_built(0) == global.ft_s) { t = global.ft_t[0]; for (k = 0; k < global.ft_nb - 1; k += 1) global.ft_t[k] = global.ft_t[k + 1]; global.ft_t[global.ft_nb - 1] = t; }\n": "",
			},
		} {
			f := filepath.Join(d, "game", "scripts", file)
			b, _ := os.ReadFile(f)
			src := string(b)
			for old, new := range subs {
				if !strings.Contains(src, old) {
					t.Fatalf("%s: no %q", file, old)
				}
				src = strings.Replace(src, old, new, 1)
			}
			os.WriteFile(f, []byte(src), 0o644)
		}
		return d
	}
	start := []string{":30", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "enter:1", ":4"} // the Ludoj row's fifth: Frazotrajno
	run := func(dir, room, shotName string, keys []string, expect string) {
		args := append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 2), "-keys", strings.Join(keys, ",")}, shot(shotName)...)
		playArgs(t, dir, args, expect)
	}
	for _, room := range []string{"rm_tall", "rm_wide"} {
		dir := pinned()
		run(dir, room, "ludoj-"+room, []string{":30"}, `global.scr == 5 && global.ovf == 0`) // the Ludoj row: five games
		keys := append([]string{}, start...)
		cur := -1 // the keyboard's focus: the depot's slots come first
		to := func(f int) {
			if cur < 0 {
				keys = append(keys, "down:1", ":1")
				cur = 0
			}
			for ; cur < f; cur++ {
				keys = append(keys, "down:1", ":1")
			}
			for ; cur > f; cur-- {
				keys = append(keys, "up:1", ":1")
			}
		}
		press := func(f int) { to(f); keys = append(keys, "enter:1", ":1") }
		run(dir, room, "frazotrajno-"+room, append(keys, ":2"), `global.scr == 13 && level_me() == "A0" && global.ft_nr == 5 && global.ft_n == 0 && global.ovf == 0`)
		n := len(a0[0])
		for k := 0; k < n; k++ {
			press(k)
		}
		run(dir, room, "frazotrajno-guste-"+room, keys, fmt.Sprintf(`global.ft_st == 2 && global.ft_ok == 1 && global.ft_tries == 1 && global.ft_n == %d && global.ovf == 0`, n))
		keys = append(keys, ":50") // gone on
		cur = -1
		run(dir, room, "", keys, `global.ft_st == 0 && global.ft_i == 1 && global.ft_n == 0`)
		n = len(a0[1])
		press(1)
		press(n) // the train's first wagon: back
		run(dir, room, "", keys, `global.ft_st == 0 && global.ft_n == 0 && global.ft_used[1] == 0`)
		press(0)
		press(2)
		press(1)
		for k := 3; k < n; k++ {
			press(k)
		}
		run(dir, room, "frazotrajno-ne-"+room, keys, fmt.Sprintf(`global.ft_st == 1 && global.ft_tries == 2 && global.ft_good == 1 && global.ft_n == %d && global.ovf == 0`, n))
		keys = append(keys, ":50") // the right start kept
		run(dir, room, "", keys, `global.ft_st == 0 && global.ft_n == 1 && global.ft_used[0] == 1 && global.ft_used[1] == 0 && global.ft_used[2] == 0`)
		for k := 1; k < n; k++ {
			press(k)
		}
		run(dir, room, "", keys, `global.ft_st == 2 && global.ft_ok == 2 && global.ft_tries == 3`)
		keys = append(keys, "enter:1", ":1") // on at once
		cur = -1
		run(dir, room, "", keys, `global.ft_st == 0 && global.ft_i == 2 && global.ft_n == 0`)
		keys = append(keys, "text=1:1", ":1") // the number key
		run(dir, room, "", keys, `global.ft_n == 1 && global.ft_used[0] == 1`)
		for i := 2; i < 5; i++ {
			for k := 0; k < len(a0[i]); k++ {
				if i == 2 && k == 0 {
					continue
				}
				press(k)
			}
			keys = append(keys, ":50")
			cur = -1
		}
		run(dir, room, "frazotrajno-finita-"+room, keys, `global.ft_st == 3 && global.ft_ok == 5 && global.ft_tries == 6 && global.ovf == 0`)
		keys = append(keys, "enter:1", ":1") // Denove
		run(dir, room, "", keys, `global.scr == 13 && global.ft_st == 0 && global.ft_i == 0 && global.ft_tries == 0 && global.ft_n == 0 && global.ft_nr == 5`)
	}
	// the pools, as the site makes them; every sentence's train drawn
	var b strings.Builder
	b.WriteString("// ftcheck(): 0 when Frazotrajno's pools are the site's, else what isn't.\nvar k, f;\nglobal.ft_place = 0;\n")
	q := func(s string) string { return `"` + s + `"` }
	for _, l := range gamedata.Levels {
		p := gamedata.TrainPool(ss, l)
		fmt.Fprintf(&b, "global.acclevel = %s; ft_pool(); if (global.ft_pn != %d) return %s;\n", q(l), len(p), q(l+" pool size"))
		for _, x := range p {
			fmt.Fprintf(&b, "f = 0; for (k = 0; k < global.ft_pn; k += 1) if (global.f_text[global.ft_p[k]] == %s) f = 1; if (!f) return %s;\n", q(x), q(l+" "+x))
		}
	}
	b.WriteString("return 0;\n")
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "ftcheck.gml"), []byte(b.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "ftdraw.gml"), []byte(fmt.Sprintf(`// ftdraw(): 0 when every sentence's train draws whole in every state, its depot shuffled, else the first that doesn't.
var i, k, m;
if (global.nf != %d) return "sentences: " + string(global.nf);
for (i = 0; i < global.nf; i += 1) {
  global.ft_pn = 1; global.ft_p[0] = i; ft_round();
  if (global.ft_nr != 1 || global.ft_s != global.f_text[i] || ft_built(0) == global.ft_s) return global.f_text[i] + " dealt";
  draw_all(); if (global.ovf) return global.f_text[i] + " 0 " + global.ovf_what;
  for (k = 0; k < global.ft_nb; k += 1) { global.ft_used[k] = 1; global.ft_bp[k] = k; }
  global.ft_n = global.ft_nb;
  for (m = 0; m <= 3; m += 1) {
    global.ft_st = m; global.ft_good = 1; global.ft_wait = 20;
    draw_all(); if (global.ovf) return global.f_text[i] + " " + string(m) + " " + global.ovf_what;
  }
}
return 0;
`, len(ss))), 0o644)
	keys := append(append([]string{}, start...), ":2")
	run(dir, "rm_tall", "", keys, `global.scr == 13 && ftcheck() == 0`)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		run(dir, room, "", keys, `global.scr == 13 && ftdraw() == 0`)
	}
	// the bonus after a place with two or more sentences: those
	slug, want := "", 0
	per := map[string]int{}
	for _, x := range ss {
		per[x.Place]++
	}
	for _, p := range c.Places {
		if per[p.Slug] >= 2 && slug == "" {
			slug, want = p.Slug, per[p.Slug]
		}
	}
	if slug == "" {
		t.Fatal("no place with two sentences")
	}
	keys = append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return false })...)
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4") // Bonuso: Frazotrajno
	args := append([]string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")}, shot("frazotrajno-bonuso-rm_tall")...)
	playArgs(t, gameDir(t, c), args, fmt.Sprintf(`global.scr == 13 && global.ft_place == 1 && global.ft_pn == %d && global.ovf == 0`, want))
	if len(s.posts) != 0 {
		t.Errorf("Frazotrajno posted %q", s.posts)
	}
}

// TestPlayVortpluvo: Vortpluvo from the levels screen, in both rooms, its
// words falling in an order pinned for the test: Komenci, a word with ŭ
// typed in the h-system (its ŭ a plain u) and caught, one typed in the
// x-system and caught, the next one falling at once; a hatted letter
// pressed on the screen's keyboard, letters no falling word starts (✗);
// then three words landed, the game done with its score, Denove. Then the
// game's pool for every level is the site's (gamedata.RainPool), every
// word of it falls whole inside a phone's field; the bonus after a place
// rains that place's words. Nothing posted.
func TestPlayVortpluvo(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	ws := c.Words()
	var a0 []string // the game's pool for an A0 learner, in its order
	for _, w := range ws {
		if w.Level == "A0" && gamedata.RainOK(w) {
			a0 = append(a0, w.Word)
		}
	}
	iu, ix := -1, -1
	for i, w := range a0 {
		if strings.Contains(w, "ŭ") && iu < 0 {
			iu = i
		}
		if strings.ContainsAny(w, "ĉĝĥĵŝ") && ix < 0 {
			ix = i
		}
	}
	if iu < 0 || ix < 0 {
		t.Fatalf("A0: no word with ŭ (%d) or another hatted letter (%d)", iu, ix)
	}
	third := 2 // the third word: one that no ĉ starts
	for strings.HasPrefix(a0[third], "ĉ") || third == iu || third == ix {
		third++
	}
	pinned := func() string {
		d := gameDir(t, c)
		f := filepath.Join(d, "game", "scripts", "pv_drop.gml")
		b, _ := os.ReadFile(f)
		old := "i = global.pv_p[floor(random(global.pv_pn))];"
		if !strings.Contains(string(b), old) {
			t.Fatalf("pv_drop: no %q", old)
		}
		seq := fmt.Sprintf("i = global.pv_p[%d]; if (global.pv_k == 0) i = global.pv_p[%d]; if (global.pv_k == 1) i = global.pv_p[%d];", third, iu, ix)
		os.WriteFile(f, []byte(strings.Replace(string(b), old, seq, 1)), 0o644)
		return d
	}
	start := []string{":30", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "enter:1", ":4"} // the Ludoj row's sixth: Vortpluvo
	run := func(dir, room, shotName string, keys []string, expect string) {
		args := append([]string{"play", "-base", s.srv.URL + "/ludo/", "-room", room, "-steps", fmt.Sprint(steps(keys) + 2), "-keys", strings.Join(keys, ",")}, shot(shotName)...)
		playArgs(t, dir, args, expect)
	}
	q := func(s string) string { return `"` + s + `"` }
	for _, room := range []string{"rm_tall", "rm_wide"} {
		dir := pinned()
		run(dir, room, "ludoj-"+room, []string{":30"}, `global.scr == 5 && global.ovf == 0`) // the Ludoj row: six games
		keys := append([]string{}, start...)
		run(dir, room, "vortpluvo-"+room, append(keys, ":2"), `global.scr == 14 && level_me() == "A0" && global.pv_st == 0 && global.pv_pn >= 20 && global.ovf == 0`)
		keys = append(keys, "enter:1", ":3") // Komenci
		run(dir, room, "", keys, fmt.Sprintf(`global.pv_st == 1 && global.pv_n == 1 && global.w_word[global.pv_w[0]] == %s && global.pv_lives == 3`, q(a0[iu])))
		keys = append(keys, "text="+hSpell(a0[iu])+":1", ":2")
		run(dir, room, "vortpluvo-kaptita-"+room, keys, fmt.Sprintf(`global.pv_score == 1 && global.pv_buf == "" && global.pv_n == 1 && global.w_word[global.pv_w[0]] == %s && global.pv_fx > 0 && global.ovf == 0`, q(a0[ix])))
		keys = append(keys, "text="+xSpell(a0[ix])+":1", ":2")
		run(dir, room, "", keys, fmt.Sprintf(`global.pv_score == 2 && global.pv_buf == "" && global.pv_n == 1 && global.w_word[global.pv_w[0]] == %s && global.pv_fall < 270`, q(a0[third])))
		keys = append(keys, "down:1", ":1", "down:1", ":1", "down:1", ":1", "down:1", ":1", "enter:1", ":1") // the screen's ĉ
		run(dir, room, "", keys, `global.pv_buf == "ĉ" && global.pv_score == 2`)
		keys = append(keys, "text=zz:1", ":2")
		run(dir, room, "vortpluvo-ne-"+room, keys, `global.pv_buf == "ĉzz" && global.pv_score == 2 && global.ovf == 0`)
		keys = append(keys, ":800") // three landed
		run(dir, room, "vortpluvo-finita-"+room, keys, `global.pv_st == 2 && global.pv_lives == 0 && global.pv_score == 2 && global.pv_best == 2 && global.pv_n == 0 && global.ovf == 0`)
		keys = append(keys, "enter:1", ":2") // Denove
		run(dir, room, "", keys, `global.scr == 14 && global.pv_st == 1 && global.pv_score == 0 && global.pv_lives == 3 && global.pv_n == 1`)
	}
	// the pools, as the site makes them; every word falling whole in the field
	var b strings.Builder
	b.WriteString("// pvcheck(): 0 when Vortpluvo's pools are the site's, else what isn't.\nvar k, f;\nglobal.pv_place = 0;\n")
	for _, l := range gamedata.Levels {
		p := gamedata.RainPool(ws, l)
		fmt.Fprintf(&b, "global.acclevel = %s; pv_pool(); if (global.pv_pn != %d) return %s;\n", q(l), len(p), q(l+" pool size"))
		for _, w := range p {
			fmt.Fprintf(&b, "f = 0; for (k = 0; k < global.pv_pn; k += 1) if (global.w_word[global.pv_p[k]] == %s) f = 1; if (!f) return %s;\n", q(w), q(l+" "+w))
		}
	}
	b.WriteString("return 0;\n")
	dir := gameDir(t, c)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "pvcheck.gml"), []byte(b.String()), 0o644)
	os.WriteFile(filepath.Join(dir, "game", "scripts", "pvdraw.gml"), []byte(`// pvdraw(): 0 when every word Vortpluvo can drop falls whole in the field, at its right edge, typed or not, else the first that doesn't.
var i, m, n;
n = 0;
for (i = 0; i < global.nw; i += 1) if (pv_ok(i)) {
  n += 1;
  global.pv_pn = 1; global.pv_p[0] = i; global.pv_n = 0; global.pv_buf = "";
  pv_drop();
  if (global.pv_n != 1) return global.w_word[i] + " dropped";
  if (global.pv_x[0] + global.pv_ww[0] > global.px1) return global.w_word[i] + " past the field";
  global.pv_x[0] = global.px1 - global.pv_ww[0];
  global.pv_last = i; global.pv_fx = 8; global.pv_fxw = global.w_word[i]; global.pv_fxx = global.pv_x[0]; global.pv_fxy = global.pt; global.pv_fxww = global.pv_ww[0];
  for (m = 0; m <= 2; m += 1) {
    global.pv_st = 1; global.pv_buf = string_copy(global.w_word[i], 1, m);
    draw_all(); if (global.ovf) return global.w_word[i] + " " + string(m) + " " + global.ovf_what;
  }
}
for (m = 0; m <= 2; m += 1) { global.pv_st = m; draw_all(); if (global.ovf) return "state " + string(m) + " " + global.ovf_what; }
if (n < 500) return "only " + string(n);
return 0;
`), 0o644)
	keys := append(append([]string{}, start...), ":2")
	run(dir, "rm_tall", "", keys, `global.scr == 14 && pvcheck() == 0`)
	for _, room := range []string{"rm_tall", "rm_wide"} {
		run(dir, room, "", keys, `global.scr == 14 && pvdraw() == 0`)
	}
	// the bonus after a place with four or more such words: those words
	by := map[string]*gamedata.WordEntry{}
	for _, w := range ws {
		if by[w.Word] == nil {
			by[w.Word] = w
		}
	}
	slug, want := "", 0
	for _, p := range c.Places {
		seen := map[string]bool{}
		for _, e := range p.Ex {
			if e.Kind == 'Q' && len(e.Answers) > 0 {
				w := strings.TrimSpace(e.Answers[0])
				if by[w] != nil && gamedata.RainOK(by[w]) {
					seen[w] = true
				}
			}
		}
		if len(seen) >= 4 && slug == "" {
			slug, want = p.Slug, len(seen)
		}
	}
	if slug == "" {
		t.Fatal("no place with four Vortpluvo words")
	}
	keys = append([]string{":8"}, placeKeys(t, s.lesson(t, slug), func(int) bool { return false })...)
	keys = append(keys[:len(keys)-2], "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "up:1", ":1", "enter:1", ":4") // Bonuso: Vortpluvo (before Konstrui, Memoro and Frazotrajno)
	args := append([]string{"play", "-base", s.srv.URL + "/ludo/?leciono=" + slug, "-room", "rm_tall", "-steps", fmt.Sprint(steps(keys) + 10), "-keys", strings.Join(keys, ",")}, shot("vortpluvo-bonuso-rm_tall")...)
	playArgs(t, gameDir(t, c), args, fmt.Sprintf(`global.scr == 14 && global.pv_place == 1 && global.pv_pn == %d && global.ovf == 0`, want))
	if len(s.posts) != 0 {
		t.Errorf("Vortpluvo posted %q", s.posts)
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
