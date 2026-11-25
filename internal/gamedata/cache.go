package gamedata

import (
	"sync"

	"github.com/LaPingvino/esperanto-kurso-gae/internal/model"
)

// Cache keeps the course built from a content set and builds it again only
// when the set is another one (the content store's cache reloaded: a new
// slice), so serving the game costs no Datastore reads of its own.
type Cache struct {
	mu     sync.Mutex
	first  *model.ContentItem
	n      int
	course *Course
}

// Get is the course for items.
func (c *Cache) Get(items []*model.ContentItem) *Course {
	c.mu.Lock()
	defer c.mu.Unlock()
	var first *model.ContentItem
	if len(items) > 0 {
		first = items[0]
	}
	if c.course == nil || first != c.first || len(items) != c.n {
		c.course, c.first, c.n = Build(items), first, len(items)
	}
	return c.course
}
