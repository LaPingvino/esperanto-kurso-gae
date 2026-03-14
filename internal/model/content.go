package model

import "time"

// ContentItem is a single exercise / learning item.
type ContentItem struct {
	Slug       string                 `firestore:"-"`
	Type       string                 `firestore:"type"`
	Content    map[string]interface{} `firestore:"content"`
	Tags       []string               `firestore:"tags"`
	Source     string                 `firestore:"source"`
	AuthorID   string                 `firestore:"author_id"`
	Status     string                 `firestore:"status"` // draft|pending|approved|rejected
	Rating     float64                `firestore:"rating"`
	RD         float64                `firestore:"rd"`
	Volatility float64                `firestore:"volatility"`
	VoteScore  int                    `firestore:"vote_score"`
	Version    int                    `firestore:"version"`
	ImageURL   string                 `firestore:"image_url"`
	CreatedAt  time.Time              `firestore:"created_at"`
	UpdatedAt  time.Time              `firestore:"updated_at"`
}

func (c *ContentItem) strField(key string) string {
	if c.Content == nil {
		return ""
	}
	v, _ := c.Content[key].(string)
	return v
}

func (c *ContentItem) Question() string    { return c.strField("question") }
func (c *ContentItem) Answer() string      { return c.strField("answer") }
func (c *ContentItem) Hint() string        { return c.strField("hint") }
func (c *ContentItem) AudioURL() string    { return c.strField("audio_url") }
func (c *ContentItem) Word() string        { return c.strField("word") }
func (c *ContentItem) Definition() string  { return c.strField("definition") }
func (c *ContentItem) Title() string       { return c.strField("title") }
func (c *ContentItem) Text() string        { return c.strField("text") }

// Options returns the list of multiple-choice options.
func (c *ContentItem) Options() []string {
	if c.Content == nil {
		return nil
	}
	raw, ok := c.Content["options"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// CorrectIndex returns the 0-based index of the correct option.
func (c *ContentItem) CorrectIndex() int {
	if c.Content == nil {
		return 0
	}
	switch v := c.Content["correct_index"].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
