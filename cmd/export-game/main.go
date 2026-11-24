// Command export-game writes the course's content as the game's GML data
// script (internal/gamedata; the game is game/, a GameViber project).
//
//	go run ./cmd/export-game [-seed seed/ekzercoj-zagr-restructured.json] [-locale internal/locale/eo.json] [-out game/scripts/course_data.gml]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
)

func main() {
	seed := flag.String("seed", "seed/ekzercoj-zagr-restructured.json", "the seed file with the Zagreb lessons")
	locale := flag.String("locale", "internal/locale/eo.json", "the site's Esperanto interface strings")
	out := flag.String("out", "game/scripts/course_data.gml", "the GML data script to write")
	flag.Parse()
	items, err := gamedata.ReadSeed(*seed)
	if err != nil {
		log.Fatal(err)
	}
	ui, err := gamedata.ReadLocale(*locale)
	if err != nil {
		log.Fatal(err)
	}
	ents, err := os.ReadDir(filepath.Dir(*locale))
	if err != nil {
		log.Fatal(err)
	}
	for _, e := range ents {
		if code, ok := strings.CutSuffix(e.Name(), ".json"); ok {
			gamedata.SiteLangs[code] = true
		}
	}
	c, err := gamedata.MakeCourse(items)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(gamedata.GML(c, ui)), 0o644); err != nil {
		log.Fatal(err)
	}
	nf, nb, nv := c.Counts()
	fmt.Printf("%d lessons: %d fill-ins, %d sentences to build, %d word cards (%d words, %d languages) → %s\n",
		len(c.Lessons), nf, nb, nv, len(c.Words), len(c.Langs), *out)
}
