package gamedata

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// siteStub is the site as the game sees it: the real /ludo-progreso handler
// over an account in memory, and POST /ekzerco/{slug}/provo recording the
// attempts (the first one refused with 503, so the game has to retry).
type siteStub struct {
	mu       sync.Mutex
	stars    map[string]int
	attempts []*model.Attempt
	posts    []string // "slug answers…", as they came
	refused  int
	items    map[string]*model.ContentItem
}

func (s *siteStub) MergeGameStars(_ context.Context, _ string, in map[string]int) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stars = model.MergeGameStars(s.stars, in)
	out := map[string]int{}
	for k, v := range s.stars {
		out[k] = v
	}
	return out, nil
}

func (s *siteStub) ListByUser(context.Context, string, int) ([]*model.Attempt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*model.Attempt(nil), s.attempts...), nil
}

func (s *siteStub) handler() http.Handler {
	mux := http.NewServeMux()
	gh := handler.NewGameHandler(s, s)
	withUser := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r.WithContext(context.WithValue(r.Context(), handler.UserContextKey, &model.User{ID: "u1"})))
		}
	}
	mux.HandleFunc("GET /ludo-progreso", withUser(gh.Progress))
	mux.HandleFunc("POST /ludo-progreso", withUser(gh.Progress))
	mux.HandleFunc("POST /ekzerco/{slug}/provo", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.refused == 0 { // the site is briefly down
			s.refused++
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		r.ParseForm()
		slug, ans := r.PathValue("slug"), r.Form["answer"]
		it := s.items[slug]
		if it == nil {
			http.NotFound(w, r)
			return
		}
		gaps := it.GapAnswers()
		right := len(ans) > 0
		for i, a := range ans {
			right = right && strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(gaps[i%len(gaps)]))
		}
		s.posts = append(s.posts, slug+" "+strings.Join(ans, "|"))
		s.attempts = append(s.attempts, &model.Attempt{ContentItemID: slug, Correct: right})
	})
	return mux
}

// TestSyncWithSite plays the game against a stub of the site, headless, at
// the page /ludo/ on it:
//   - progress made on the site comes into the game at the start: every
//     fill-in of lesson 2 answered rightly there opens it with a star, and the
//     account's 2 stars for lesson 3 show;
//   - lesson 1 played in the game, its first fill-in by a tile, the others
//     typed: only the typed ones go to the site as attempts (only they move
//     ratings), all accepted there, the first after the site refused it once;
//   - the game's stars end on the account (merged by the larger), and the
//     game's queue is empty.
func TestSyncWithSite(t *testing.T) {
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
	ui, _ := ReadLocale(filepath.Join(root, "internal", "locale", "eo.json"))
	c, err := MakeCourse(items)
	if err != nil {
		t.Fatal(err)
	}
	st := &siteStub{stars: map[string]int{c.Lessons[2].Slug: 2}, items: map[string]*model.ContentItem{}}
	for _, it := range items {
		st.items[it.Slug] = it
	}
	for _, f := range c.Lessons[1].Fill { // lesson 2 done on the site
		st.attempts = append(st.attempts, &model.Attempt{ContentItemID: f.Slug, Correct: true})
	}
	srv := httptest.NewServer(st.handler())
	defer srv.Close()

	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "game", "esperanto-kurso-ludo.txt"))
	if err := os.WriteFile(filepath.Join(dir, "game", "scripts", "course_data.gml"), []byte(GML(c, ui)), 0o644); err != nil {
		t.Fatal(err)
	}
	keys := append([]string{":4"}, typedLessonKeys(c, 0)...)
	keys = append(keys, ":300") // time for the queue to drain and the stars to go
	steps := 0
	for _, k := range keys {
		var n int
		fmt.Sscanf(k[strings.LastIndex(k, ":")+1:], "%d", &n)
		steps += n
	}
	expect := "global.stars[0] == 3 && global.stars[1] == 1 && global.stars[2] == 2 && global.ob_n == 0 && global.sync_ok"
	cmd := exec.Command(gv, "play", "-base", srv.URL+"/ludo/", "-steps", fmt.Sprint(steps), "-keys", strings.Join(keys, ","), "-expect", expect, "game")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "pass") || strings.Contains(string(out), "error") {
		t.Fatalf("%v\n%s", err, out)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	fills := c.Lessons[0].Fill
	if len(st.posts) != len(fills)-1 || st.refused != 1 {
		t.Errorf("attempts posted %d (refused %d), want %d typed ones: %q", len(st.posts), st.refused, len(fills)-1, st.posts)
	}
	for i, p := range st.posts {
		if !strings.HasPrefix(p, fills[i+1].Slug+" ") {
			t.Errorf("post %d: %q, want %s", i, p, fills[i+1].Slug)
		}
	}
	for _, a := range st.attempts[len(c.Lessons[1].Fill):] {
		if !a.Correct {
			t.Errorf("%s: the site judged the game's typed answer wrong", a.ContentItemID)
		}
	}
	if st.stars[c.Lessons[0].Slug] != 3 || st.stars[c.Lessons[1].Slug] != 1 || st.stars[c.Lessons[2].Slug] != 2 {
		t.Errorf("the account's stars: %v", st.stars)
	}
}

// typedLessonKeys plays lesson l as lessonKeys does, but types every
// fill-in's answers (text=…, Enter) except the first fill-in's, which takes
// its tiles.
func typedLessonKeys(c *Course, l int) []string {
	all := lessonKeys(c, l)
	les := c.Lessons[l]
	// lessonKeys' fill-in part, rebuilt: find where it starts and ends by
	// regenerating the pieces before and after.
	press := func(downs int) []string {
		var ks []string
		for i := 0; i < downs; i++ {
			ks = append(ks, "down:1", ":2")
		}
		return append(ks, "enter:1", ":2")
	}
	head := []string{":2", "enter:1", ":2"}
	head = append(head, press(1)...)
	for k, wi := range les.Words {
		if k == 5 {
			break
		}
		for o, x := range c.Words[wi].Options {
			if x == wi {
				head = append(head, press(o+1)...)
			}
		}
		head = append(head, "enter:1", ":2")
	}
	var fill []string
	for i, f := range les.Fill {
		for g, tiles := range f.Tiles {
			ans := strings.TrimSpace(f.Answers[g%len(f.Answers)])
			if i == 0 {
				for t, x := range tiles {
					if x == ans {
						fill = append(fill, press(t+1)...)
						break
					}
				}
				continue
			}
			fill = append(fill, "text="+ans+":1", ":1", "enter:1", ":2")
		}
		fill = append(fill, "enter:1", ":2")
	}
	// the tile version's fill-in part has the same position: replace it
	n := len(head)
	tileFill := 0
	for _, f := range les.Fill {
		for g, tiles := range f.Tiles {
			ans := strings.TrimSpace(f.Answers[g%len(f.Answers)])
			for t, x := range tiles {
				if x == ans {
					tileFill += len(press(t + 1))
					break
				}
			}
		}
		tileFill += 2
	}
	out := append([]string(nil), all[:n]...)
	out = append(out, fill...)
	return append(out, all[n+tileFill:]...)
}

// TestSyncOffline: played with the site unreachable, the typed answers wait
// in the progress file; the next time the game opens with the site there,
// they go, and the stars too.
func TestSyncOffline(t *testing.T) {
	gv := os.Getenv("GAMEVIBER")
	if gv == "" {
		var err error
		if gv, err = exec.LookPath("gameviber"); err != nil {
			t.Skip("no gameviber command (set GAMEVIBER)")
		}
	}
	root, _ := filepath.Abs("../..")
	items, _ := ReadSeed(filepath.Join(root, "seed", "ekzercoj-zagr-restructured.json"))
	ui, _ := ReadLocale(filepath.Join(root, "internal", "locale", "eo.json"))
	c, err := MakeCourse(items)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "game"), os.DirFS(filepath.Join(root, "game"))); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, "game", "esperanto-kurso-ludo.txt"))
	os.WriteFile(filepath.Join(dir, "game", "scripts", "course_data.gml"), []byte(GML(c, ui)), 0o644)
	play := func(base string, keys []string, extra int, expect string) {
		steps := extra
		for _, k := range keys {
			var n int
			fmt.Sscanf(k[strings.LastIndex(k, ":")+1:], "%d", &n)
			steps += n
		}
		cmd := exec.Command(gv, "play", "-base", base, "-steps", fmt.Sprint(steps), "-keys", strings.Join(keys, ","), "-expect", expect, "game")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "pass") || strings.Contains(string(out), "error") {
			t.Fatalf("%s: %v\n%s", base, err, out)
		}
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close() // nobody answers there: status 0
	typed := len(c.Lessons[0].Fill) - 1
	play(deadURL+"/ludo/", append([]string{":4"}, typedLessonKeys(c, 0)...), 100,
		fmt.Sprintf("global.stars[0] == 3 && global.ob_n == %d && !global.sync_ok", typed))

	st := &siteStub{refused: 1, items: map[string]*model.ContentItem{}} // up and answering
	for _, it := range items {
		st.items[it.Slug] = it
	}
	srv := httptest.NewServer(st.handler())
	defer srv.Close()
	play(srv.URL+"/ludo/", nil, 400, "global.ob_n == 0 && global.sync_ok && global.stars[0] == 3")
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.posts) != typed || st.stars[c.Lessons[0].Slug] != 3 {
		t.Errorf("after reopening: %d attempts (want %d), stars %v", len(st.posts), typed, st.stars)
	}
}
