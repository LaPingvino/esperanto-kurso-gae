package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	seedpkg "github.com/LaPingvino/esperanto-kurso-gae/seed"
)

// The content prepared for the builder API (content/builder/*.json) passes
// the API's own checks, collides with no seed slug, and its series parents are
// in the same files: so `scripts/builder.sh load content/builder` goes through.
func TestBuilderContentFiles(t *testing.T) {
	files, _ := filepath.Glob("../../content/builder/*.json")
	if len(files) == 0 {
		t.Fatal("no content/builder/*.json")
	}
	seedSlugs := map[string]bool{}
	ents, _ := seedpkg.FS.ReadDir(".")
	for _, e := range ents {
		b, _ := seedpkg.FS.ReadFile(e.Name())
		var xs []map[string]any
		json.Unmarshal(b, &xs)
		for _, x := range xs {
			for _, k := range []string{"Slug", "slug"} {
				if s, ok := x[k].(string); ok {
					seedSlugs[s] = true
				}
			}
		}
	}
	all := map[string]*BuilderItem{}
	var order []*BuilderItem
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var batch []BuilderItem
		if err := json.Unmarshal(b, &batch); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(batch) > 500 {
			t.Errorf("%s: %d items, the API takes 500 at a time", f, len(batch))
		}
		for i := range batch {
			it := &batch[i]
			if p := ValidateBuilderItem(it); p != "" {
				t.Errorf("%s: %s needs %s", f, it.Slug, p)
			}
			if all[it.Slug] != nil || seedSlugs[it.Slug] {
				t.Errorf("%s: slug %s is taken", f, it.Slug)
			}
			if it.Trusted {
				t.Errorf("%s: %s is trusted; prepared content goes in pending", f, it.Slug)
			}
			all[it.Slug] = it
			order = append(order, it)
		}
	}
	idx := map[int]int{}
	for _, it := range order {
		if it.SeriesParent != "" && all[it.SeriesParent] == nil {
			t.Errorf("%s: parent %s not in the files", it.Slug, it.SeriesParent)
		}
		if it.Rating == 0 && it.Level == "" {
			t.Errorf("%s: no level or rating", it.Slug)
		}
		if it.Type == "multiplechoice" {
			idx[int(it.Content["correct_index"].(float64))]++
		}
		if it.Type == "fillin" && strings.Contains(builderStr(it.Content, "answer"), " ") {
			t.Errorf("%s: a fill-in answer of more than one word (%q)", it.Slug, builderStr(it.Content, "answer"))
		}
	}
	if len(idx) < 3 {
		t.Errorf("the right options sit at too few places: %v", idx)
	}
}
