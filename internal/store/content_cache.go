package store

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// Approved-content cache.
//
// Public pages list content constantly (vortaro loads every vocab item, reading
// pages load hundreds of tagged items, every page runs recommendations). Doing
// that as Datastore queries bills one entity read per item per request, which
// crawlers turn into millions of reads a day. The approved set is small (a few
// MB), so each instance keeps it in memory and filters locally.
//
// Invalidation: every content write bumps a single ContentMeta/version entity.
// Instances check it at most once per versionCheckInterval (one Lookup) and
// reload when it changed. Rating and vote updates happen on every attempt, so
// they do not bump the version — the local copy is patched instead and other
// instances pick them up at the next maxCacheAge refresh.

const (
	contentMetaKind      = "ContentMeta"
	versionCheckInterval = time.Minute
	maxCacheAge          = 6 * time.Hour
)

type contentMeta struct {
	Version int64 `datastore:"version,noindex"`
}

type contentCache struct {
	mu        sync.RWMutex
	loadMu    sync.Mutex // serialises reloads so concurrent requests don't all query
	items     []*model.ContentItem
	version   int64
	loadedAt  time.Time
	checkedAt time.Time
	valid     bool
}

func (s *ContentStore) metaKey() *datastore.Key {
	return datastore.NameKey(contentMetaKind, "version", nil)
}

// bumpVersion marks the approved-content cache stale on this instance and,
// via the shared version entity, on all other instances.
func (s *ContentStore) bumpVersion(ctx context.Context) {
	s.cache.mu.Lock()
	s.cache.valid = false
	s.cache.mu.Unlock()
	_, err := s.db.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		var m contentMeta
		if err := tx.Get(s.metaKey(), &m); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
		m.Version++
		_, err := tx.Put(s.metaKey(), &m)
		return err
	})
	if err != nil {
		log.Printf("content_cache: bump version: %v", err)
	}
}

// approved returns the cached approved items, sorted by slug (Datastore key
// order). The returned slice and items are shared: callers must not modify them.
func (s *ContentStore) approved(ctx context.Context) ([]*model.ContentItem, error) {
	c := &s.cache
	c.mu.RLock()
	fresh := c.valid && time.Since(c.checkedAt) < versionCheckInterval && time.Since(c.loadedAt) < maxCacheAge
	items := c.items
	c.mu.RUnlock()
	if fresh {
		return items, nil
	}

	c.loadMu.Lock()
	defer c.loadMu.Unlock()

	// Another goroutine may have refreshed while we waited.
	c.mu.RLock()
	fresh = c.valid && time.Since(c.checkedAt) < versionCheckInterval && time.Since(c.loadedAt) < maxCacheAge
	items, haveVersion, valid, loadedAt := c.items, c.version, c.valid, c.loadedAt
	c.mu.RUnlock()
	if fresh {
		return items, nil
	}

	var m contentMeta
	if err := s.db.Get(ctx, s.metaKey(), &m); err != nil && err != datastore.ErrNoSuchEntity {
		if valid {
			return items, nil // serve stale rather than fail
		}
		return nil, err
	}
	if valid && m.Version == haveVersion && time.Since(loadedAt) < maxCacheAge {
		c.mu.Lock()
		c.checkedAt = time.Now()
		c.mu.Unlock()
		return items, nil
	}

	q := datastore.NewQuery(contentKind).FilterField("status", "=", "approved")
	loaded, err := s.runContentQuery(ctx, q)
	if err != nil {
		if valid {
			return items, nil
		}
		return nil, err
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].Slug < loaded[j].Slug })

	now := time.Now()
	c.mu.Lock()
	c.items, c.version, c.loadedAt, c.checkedAt, c.valid = loaded, m.Version, now, now, true
	c.mu.Unlock()
	return loaded, nil
}

// filterApproved returns up to limit cached approved items matching keep
// (limit <= 0 means no limit), as shallow copies so per-request field tweaks
// don't leak into the cache.
func (s *ContentStore) filterApproved(ctx context.Context, limit int, keep func(*model.ContentItem) bool) ([]*model.ContentItem, error) {
	all, err := s.approved(ctx)
	if err != nil {
		return nil, err
	}
	var out []*model.ContentItem
	for _, it := range all {
		if keep != nil && !keep(it) {
			continue
		}
		cp := *it
		out = append(out, &cp)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// patchCachedRating updates the local cached copy after a rating change.
func (s *ContentStore) patchCachedRating(slug string, rating, rd, volatility float64) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	i := sort.Search(len(s.cache.items), func(i int) bool { return s.cache.items[i].Slug >= slug })
	if i < len(s.cache.items) && s.cache.items[i].Slug == slug {
		cp := *s.cache.items[i]
		cp.Rating, cp.RD, cp.Volatility = rating, rd, volatility
		// Copy-on-write: readers may hold the old slice.
		items := make([]*model.ContentItem, len(s.cache.items))
		copy(items, s.cache.items)
		items[i] = &cp
		s.cache.items = items
	}
}

func hasTag(it *model.ContentItem, tag string) bool {
	for _, t := range it.Tags {
		if t == tag {
			return true
		}
	}
	return false
}
