package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// The game played headless (gameviber play -base) against a stub of the
// site: the real /ludo-api/ and /ludo-progreso handlers over the seed and an
// account in memory, and POST /ekzerco/{slug}/provo judging with the site's
// checkAnswer (refusing the first ones it is told to, so the game retries).

type stubSite struct {
	mu       sync.Mutex
	items    map[string]*model.ContentItem
	stars    map[string]int
	attempts []*model.Attempt
	posts    []string
	refuse   int
	api      *GameAPI
	srv      *httptest.Server
}

func (s *stubSite) MergeGameStars(_ context.Context, _ string, in map[string]int) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stars = model.MergeGameStars(s.stars, in)
	out := map[string]int{}
	for k, v := range s.stars {
		out[k] = v
	}
	return out, nil
}

func (s *stubSite) ListByUser(context.Context, string, int) ([]*model.Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*model.Attempt(nil), s.attempts...), nil
}

func newStubSite(t *testing.T) *stubSite {
	items := seedSet(t)
	s := &stubSite{items: map[string]*model.ContentItem{}, api: NewGameAPI(&seedSource{items: items})}
	for _, it := range items {
		s.items[it.Slug] = it
	}
	mux := http.NewServeMux()
	s.api.Routes(mux)
	gh := NewGameHandler(s, s)
	gh.Course = s.api.Course
	withUser := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r.WithContext(context.WithValue(r.Context(), UserContextKey, &model.User{ID: "u1", Lang: "en"})))
		}
	}
	mux.HandleFunc("GET /ludo-progreso", withUser(gh.Progress))
	mux.HandleFunc("POST /ludo-progreso", withUser(gh.Progress))
	mux.HandleFunc("POST /ekzerco/{slug}/provo", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.refuse > 0 {
			s.refuse--
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		r.ParseForm()
		it := s.items[r.PathValue("slug")]
		if it == nil {
			http.NotFound(w, r)
			return
		}
		s.posts = append(s.posts, it.Slug+" "+strings.Join(r.Form["answer"], "|"))
		s.attempts = append(s.attempts, &model.Attempt{ContentItemID: it.Slug, Correct: checkAnswer(it, r.Form["answer"])})
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func gameviber(t *testing.T) string {
	if gv := os.Getenv("GAMEVIBER"); gv != "" {
		return gv
	}
	gv, err := exec.LookPath("gameviber")
	if err != nil {
		t.Skip("no gameviber command (set GAMEVIBER)")
	}
	return gv
}

// gameDir is a copy of game/ with its data script, in a folder of its own
// (where the game keeps its progress and cache files).
func gameDir(t *testing.T, c *gamedata.Course) string {
	root, _ := filepath.Abs("../..")
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"esperanto-kurso-ludo.txt", "parity.txt", "ludo-kurso.txt"} {
		os.Remove(filepath.Join(dir, "game", f))
	}
	ui, _ := gamedata.ReadLocale(filepath.Join(root, "internal", "locale", "eo.json"))
	os.WriteFile(filepath.Join(dir, "game", "scripts", "course_data.gml"), []byte(gamedata.GML(c, ui)), 0o644)
	return dir
}

func play(t *testing.T, dir, base, room string, keys []string, extra int, expect string) {
	t.Helper()
	steps := extra
	for _, k := range keys {
		n, _ := strconv.Atoi(k[strings.LastIndex(k, ":")+1:])
		steps += n
	}
	args := []string{"play", "-base", base, "-steps", fmt.Sprint(steps), "-keys", strings.Join(keys, ","), "-expect", expect}
	if room != "" {
		args = append(args, "-room", room)
	}
	cmd := exec.Command(gameviber(t), append(args, "game")...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "pass") || strings.Contains(string(out), "error") {
		t.Fatalf("%s %s: %v\n%s", base, room, err, out)
	}
}

// placeKeys are the keys answering every exercise of a place rightly, from
// its lesson as the site serves it: choices and tiles by their number keys;
// gaps typed when typed(exercise index) says so (answers a key script can't
// type go by their tile), then Kontroli and Daŭrigi. Ends back on the
// series' map.
func placeKeys(t *testing.T, lesson string, typed func(i int) bool) []string {
	digit := func(n int) []string { return []string{fmt.Sprintf("text=%d:1", n+1), ":2"} }
	enter := []string{"enter:1", ":2"}
	var ks []string
	var hasText, video bool
	type ex struct {
		kind  string
		f     []string
		ans   []string
		tiles [][]string
		ntoks int
	}
	var exs []*ex
	for _, l := range strings.Split(lesson, "\n") {
		f := strings.Split(l, "\t")
		switch f[0] {
		case "X":
			hasText = true
		case "U":
			video = true
		case "C", "F", "M", "Q", "H", "B":
			exs = append(exs, &ex{kind: f[0], f: f})
		case "A":
			exs[len(exs)-1].ans = append(exs[len(exs)-1].ans, f[1])
		case "K":
			exs[len(exs)-1].tiles = append(exs[len(exs)-1].tiles, f[1:])
		case "O":
			if f[2] != "-1" {
				exs[len(exs)-1].ntoks++
			}
		}
	}
	if video {
		return append(append(ks, enter...), enter...) // watch (on the site), then the result
	}
	if hasText {
		ks = append(ks, "down:1", ":2", "enter:1", ":2") // Komenci: the first thing on the reading screen
	}
	if len(exs) == 0 {
		return append(ks, enter...)
	}
	for i, e := range exs {
		switch e.kind {
		case "C", "M":
			n, _ := strconv.Atoi(e.f[3])
			ks = append(ks, digit(n)...)
		case "F", "Q", "H":
			gaps := 1
			if e.kind == "F" {
				gaps = e.ntoks
			}
			for g := 0; g < gaps; g++ {
				a := e.ans[g%len(e.ans)]
				if e.kind != "F" {
					a = e.ans[0]
				}
				if typed(i) && !strings.ContainsAny(a, ",:") && !(len(a) == 1 && a >= "0" && a <= "9") {
					ks = append(ks, "text="+a+":1", ":1", "enter:1", ":2")
					continue
				}
				found := false
				for k, x := range e.tiles[g] {
					if x == a {
						ks = append(ks, digit(k)...)
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("%v: answer %q not among its tiles %q", e.f, a, e.tiles[g])
				}
			}
		case "B":
			tiles := e.tiles[0]
			used := make([]bool, len(tiles))
			for _, w := range strings.Fields(e.f[1]) {
				for k, x := range tiles {
					if !used[k] && x == w {
						used[k] = true
						ks = append(ks, digit(k)...)
						break
					}
				}
			}
		}
		ks = append(ks, enter...) // Kontroli
		ks = append(ks, enter...) // Daŭrigi
	}
	return append(ks, enter...) // the result: Daŭrigi, to the map
}

func (s *stubSite) lesson(t *testing.T, slug string) string {
	r, err := http.Get(s.srv.URL + "/ludo-api/leciono/" + slug + "?lingvo=en")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return string(b)
}

// TestPlayEachPart plays a place of every kind the course has, each through
// the game's real keys, loaded from the site: a Zagreb lesson (cards,
// fill-ins, sentences), a reading of another series, vocabulary,
// multiple choice, the phrasebook's typed answers, a video — each ends with
// three stars (a video: one) on the map, every text having fitted its box
// on the way (global.ovf). The Zagreb lesson in the tall room too.
func TestPlayEachPart(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	picks := map[string]string{"zagreb": "zagr-l01-teksto"}
	for _, p := range c.Places {
		kinds := map[byte]bool{}
		for _, e := range p.Ex {
			kinds[e.Kind] = true
		}
		set := func(name string, ok bool) {
			if ok && picks[name] == "" {
				picks[name] = p.Slug
			}
		}
		set("reading", p.Kind == 'L' && !kinds['F'] && kinds['C'] && kinds['B'] && !strings.HasPrefix(p.Slug, "zagr-"))
		set("vocabulary", kinds['Q'])
		set("choice", kinds['M'])
		set("phrase", kinds['H'])
		set("video", p.Kind == 'V')
	}
	if len(picks) != 6 {
		t.Fatalf("not every part found: %v", picks)
	}
	never := func(int) bool { return false }
	for name, slug := range picks {
		dir := gameDir(t, c)
		want := 3
		if name == "video" {
			want = 1
		}
		keys := append([]string{":8"}, placeKeys(t, s.lesson(t, slug), never)...)
		play(t, dir, s.srv.URL+"/ludo/?leciono="+slug, "", keys, 10,
			fmt.Sprintf(`stars_get("%s") == %d && global.scr == 0 && global.ovf == 0`, slug, want))
		t.Logf("%s: %s played, %d stars", name, slug, want)
	}
	dir := gameDir(t, c)
	keys := append([]string{":8"}, placeKeys(t, s.lesson(t, "zagr-l01-teksto"), never)...)
	play(t, dir, s.srv.URL+"/ludo/?leciono=zagr-l01-teksto", "rm_tall", keys, 10, `stars_get("zagr-l01-teksto") == 3 && global.ovf == 0`)
}

// TestSyncWithSite: progress made on the site comes into the game (a place
// whose exercises were all answered rightly there: done; the account's
// stars); a Zagreb lesson played with its first fill-in by tiles and the
// others typed sends exactly the typed ones to the site (the first refused
// once: retried), all judged right there by checkAnswer; the stars end on
// the account; the game's queue is empty.
func TestSyncWithSite(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	mc := c.Place("gramatiko-mc.1")
	for _, e := range mc.Ex { // gramatiko-mc done on the site
		s.attempts = append(s.attempts, &model.Attempt{ContentItemID: e.Slug, Correct: true})
	}
	s.stars = map[string]int{"vortaro-koloro.1": 2}
	s.refuse = 1
	lesson := s.lesson(t, "zagr-l01-teksto")
	var fills []int // exercise indexes of the fill-ins
	for i, e := range c.Place("zagr-l01-teksto").Ex {
		if e.Kind == 'F' {
			fills = append(fills, i)
		}
	}
	typed := func(i int) bool { return i != fills[0] && c.Place("zagr-l01-teksto").Ex[i].Kind == 'F' }
	dir := gameDir(t, c)
	keys := append([]string{":8"}, placeKeys(t, lesson, typed)...)
	play(t, dir, s.srv.URL+"/ludo/?leciono=zagr-l01-teksto", "", keys, 300,
		`stars_get("zagr-l01-teksto") == 3 && stars_get("gramatiko-mc.1") == 1 && stars_get("vortaro-koloro.1") == 2 && global.ob_n == 0 && global.sync_ok && global.acclang == "en"`)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.posts) != len(fills)-1 {
		t.Errorf("%d attempts posted, want the %d typed fill-ins: %q", len(s.posts), len(fills)-1, s.posts)
	}
	for _, a := range s.attempts[len(mc.Ex):] {
		if !a.Correct {
			t.Errorf("%s: the site judged the game's typed answer wrong", a.ContentItemID)
		}
	}
	if s.stars["zagr-l01-teksto"] != 3 || s.stars["vortaro-koloro.1"] != 2 || s.stars["gramatiko-mc.1"] != 1 {
		t.Errorf("the account's stars: %v", s.stars)
	}
}

// TestSyncOffline: played with the site unreachable — from the built-in
// part — the typed answers wait in the progress file; opened again with the
// site there, they go, and the stars too. And the old progress file (version
// 2: stars by lesson number, a queue) is read into the new one.
func TestSyncOffline(t *testing.T) {
	gameviber(t)
	s := newStubSite(t)
	c := s.api.Course(t.Context())
	dir := gameDir(t, c)
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	// an old (version 2) progress file: lesson 3 two stars, German (index 3), one answer queued
	old := "2\r\n3\r\n12\r\n0\r\n0\r\n2\r\n0\r\n0\r\n0\r\n0\r\n0\r\n0\r\n0\r\n0\r\n0\r\n1\r\nzagr-l02-fi01\r\n1\r\nn\r\n"
	os.WriteFile(filepath.Join(dir, "esperanto-kurso-ludo.txt"), []byte(old), 0o644)
	lesson := s.lesson(t, "zagr-l01-teksto")
	all := func(int) bool { return true }
	typed := 0
	for _, e := range c.Place("zagr-l01-teksto").Ex {
		if e.Kind == 'F' {
			typed++
		}
	}
	play(t, dir, deadURL+"/ludo/?leciono=zagr-l01-teksto", "", append([]string{":8"}, placeKeys(t, lesson, all)...), 100,
		fmt.Sprintf(`stars_get("zagr-l01-teksto") == 3 && stars_get("zagr-l03-teksto") == 2 && global.mylang == "de" && global.ob_n == %d && !global.sync_ok`, typed+1))
	play(t, dir, s.srv.URL+"/ludo/", "", nil, 400, `global.ob_n == 0 && global.sync_ok && stars_get("zagr-l01-teksto") == 3`)
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.posts) != typed+1 || s.stars["zagr-l01-teksto"] != 3 || s.stars["zagr-l03-teksto"] != 2 {
		t.Errorf("after reopening: %d attempts (want %d), stars %v", len(s.posts), typed+1, s.stars)
	}
}
