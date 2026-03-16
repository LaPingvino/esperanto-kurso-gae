// Command export-seed writes the built-in seed groups to seed/*.json files.
// Run once to convert hardcoded Go seed data to portable JSON.
//
//	go run ./cmd/export-seed/ [-dir=./seed]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/handler"
)

func main() {
	dir := flag.String("dir", "./seed", "Output directory for seed JSON files")
	flag.Parse()

	if err := os.MkdirAll(*dir, 0755); err != nil {
		log.Fatalf("mkdir %s: %v", *dir, err)
	}

	for name, items := range handler.SeedGroups() {
		path := filepath.Join(*dir, name+".json")
		f, err := os.Create(path)
		if err != nil {
			log.Fatalf("create %s: %v", path, err)
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(items); err != nil {
			f.Close()
			log.Fatalf("encode %s: %v", path, err)
		}
		f.Close()
		fmt.Printf("Wrote %d items → %s\n", len(items), path)
	}
}
