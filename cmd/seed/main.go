// Command seed loads exercise data from JSON files into Datastore and can
// also promote a user to admin role.
//
// Usage:
//
//	go run ./cmd/seed/ -project=esperanto-kurso [-dir=./seed]
//	go run ./cmd/seed/ -project=esperanto-kurso -promote=<token>
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/store"
)

func main() {
	projectID := flag.String("project", "esperanto-kurso", "GCP project ID")
	seedDir := flag.String("dir", "./seed", "Directory containing JSON seed files")
	promote := flag.String("promote", "", "Token of user to promote to admin role")
	flag.Parse()

	ctx := context.Background()

	db, err := store.NewDatastoreClient(ctx, *projectID)
	if err != nil {
		log.Fatalf("datastore.NewClient: %v", err)
	}
	defer db.Close()

	if *promote != "" {
		if err := promoteUser(ctx, db, *promote); err != nil {
			log.Fatalf("promote: %v", err)
		}
		return
	}

	pattern := filepath.Join(*seedDir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("glob %s: %v", pattern, err)
	}
	if len(files) == 0 {
		log.Fatalf("no JSON files found in %s", *seedDir)
	}

	cs := store.NewContentStore(db)
	for _, path := range files {
		if err := seedFile(ctx, cs, path); err != nil {
			log.Printf("ERROR seeding %s: %v", path, err)
		}
	}
}

// promoteUser finds a user by token and sets their role to "admin".
func promoteUser(ctx context.Context, db *datastore.Client, token string) error {
	us := store.NewUserStore(db)
	u, err := us.GetByToken(ctx, token)
	if err != nil {
		return fmt.Errorf("GetByToken: %w", err)
	}
	if u == nil {
		return fmt.Errorf("no user with that token — have they visited the site yet?")
	}
	u.Role = "admin"
	if err := us.Update(ctx, u); err != nil {
		return fmt.Errorf("Update: %w", err)
	}
	log.Printf("User %s promoted to admin", u.ID)
	return nil
}

type seedItem struct {
	Slug       string                 `json:"slug"`
	Type       string                 `json:"type"`
	Content    map[string]interface{} `json:"content"`
	Tags       []string               `json:"tags"`
	Source     string                 `json:"source"`
	Status     string                 `json:"status"`
	Rating     float64                `json:"rating"`
	RD         float64                `json:"rd"`
	Volatility float64                `json:"volatility"`
	ImageURL   string                 `json:"image_url"`
}

func seedFile(ctx context.Context, cs *store.ContentStore, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var items []seedItem
	if err := json.NewDecoder(f).Decode(&items); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	log.Printf("Seeding %d items from %s", len(items), path)

	for _, si := range items {
		if si.Slug == "" {
			log.Printf("  SKIP: item without slug")
			continue
		}
		if si.Rating == 0 {
			si.Rating = 1500
		}
		if si.RD == 0 {
			si.RD = 350
		}
		if si.Volatility == 0 {
			si.Volatility = 0.06
		}
		if si.Status == "" {
			si.Status = "draft"
		}

		item := &model.ContentItem{
			Slug:       si.Slug,
			Type:       si.Type,
			Content:    si.Content,
			Tags:       si.Tags,
			Source:     si.Source,
			AuthorID:   "seed",
			Status:     si.Status,
			Rating:     si.Rating,
			RD:         si.RD,
			Volatility: si.Volatility,
			VoteScore:  0,
			Version:    1,
			ImageURL:   si.ImageURL,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		if err := cs.Create(ctx, item); err != nil {
			log.Printf("  ERROR %s: %v", item.Slug, err)
			continue
		}
		log.Printf("  OK    %s (%s, rating=%.0f)", item.Slug, item.Type, item.Rating)
	}
	return nil
}
