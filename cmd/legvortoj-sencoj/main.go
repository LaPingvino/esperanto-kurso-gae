// Command legvortoj-sencoj lists the words of the readings whose sense is
// open to doubt (a word of several senses in ReVo where the proposal from
// its sentence is not the main sense, or another sense fits as well), for
// review, into internal/legvortoj/sencoj.json. Reviewed picks
// ("kontrolita": true) already in the file are kept as they are.
//
//	scripts/builder.sh read 'type=reading' | jq -r '.items[].Slug' |
//	  while read s; do scripts/builder.sh get "$s"; done | jq -s . > readings.json
//	go run ./cmd/legvortoj-sencoj readings.json
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/legvortoj"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

func main() {
	out := "internal/legvortoj/sencoj.json"
	if len(os.Args) < 2 {
		log.Fatal("usage: legvortoj-sencoj readings.json (builder-api items)")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var items []struct {
		Slug    string         `json:"slug"`
		Type    string         `json:"type"`
		Content map[string]any `json:"content"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		log.Fatal(err)
	}
	picks := legvortoj.Picks() // the embedded file: reviewed picks stay
	res := map[string]map[string]legvortoj.Pick{}
	pairs, multi, doubt := 0, 0, 0
	for _, it := range items {
		if it.Type != "reading" {
			continue
		}
		r := &model.ContentItem{Slug: it.Slug, Type: it.Type, Content: it.Content}
		for _, u := range legvortoj.Words(r.Slug, r.Text(), nil) {
			pairs++
			if !u.Ambiguous() {
				continue
			}
			multi++
			if p, ok := picks[r.Slug][u.Lemma]; ok && p.Checked {
				put(res, r.Slug, u.Lemma, p)
				continue
			}
			if !u.Doubt {
				continue
			}
			doubt++
			p := legvortoj.Pick{Sense: u.Sense, Sentence: u.Sentence}
			for _, s := range u.Hit.Senses {
				p.Choices = append(p.Choices, s.Mrk+": "+s.Def)
			}
			put(res, r.Slug, u.Lemma, p)
		}
	}
	b, _ := json.MarshalIndent(sorted(res), "", " ")
	if err := os.WriteFile(out, append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d (reading, word) pairs; %d of words of several senses; %d listed for review → %s\n", pairs, multi, doubt, out)
}

func put(m map[string]map[string]legvortoj.Pick, r, w string, p legvortoj.Pick) {
	if m[r] == nil {
		m[r] = map[string]legvortoj.Pick{}
	}
	m[r][w] = p
}

// sorted keeps the file's order stable (encoding/json sorts map keys).
func sorted(m map[string]map[string]legvortoj.Pick) map[string]map[string]legvortoj.Pick {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return m
}
