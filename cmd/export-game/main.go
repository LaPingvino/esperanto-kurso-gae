// Command export-game writes the game's data script: the interface strings
// and the built-in fallback part of the course (internal/gamedata), made
// from the site's seed data. The game itself loads the course from the site
// at run time (/ludo-api/); this is only what it plays when the site can't
// be reached and nothing is cached yet. It also prints a summary of the
// whole course as the seed has it.
//
//	go run ./cmd/export-game [-seed seed] [-locale internal/locale/eo.json] [-out game/scripts/course_data.gml]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/gamedata"
)

func main() {
	seed := flag.String("seed", "seed", "the seed folder")
	locale := flag.String("locale", "internal/locale/eo.json", "the site's Esperanto interface strings")
	out := flag.String("out", "game/scripts/course_data.gml", "the GML data script to write")
	flag.Parse()
	items, err := gamedata.ReadSeedDir(*seed)
	if err != nil {
		log.Fatal(err)
	}
	ui, err := gamedata.ReadLocale(*locale)
	if err != nil {
		log.Fatal(err)
	}
	c := gamedata.Build(items)
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(gamedata.GML(c, ui)), 0o644); err != nil {
		log.Fatal(err)
	}
	kinds := map[byte]int{}
	for _, p := range c.Places {
		for _, e := range p.Ex {
			kinds[e.Kind]++
		}
	}
	fmt.Printf("%d series, %d places; exercises: %d fill-in, %d choice, %d vocabulary, %d typed answer, %d word card, %d sentence; %d languages → %s\n",
		len(c.Series), len(c.Places), kinds['F'], kinds['M'], kinds['Q'], kinds['H'], kinds['C'], kinds['B'], len(c.Langs), *out)
}
