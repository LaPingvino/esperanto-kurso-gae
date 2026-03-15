package model

import (
	"strings"
	"time"
)

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
	VoteScore   int                    `firestore:"vote_score"`
	Version     int                    `firestore:"version"`
	ImageURL    string                 `firestore:"image_url"`
	SeriesSlug   string                 `firestore:"series_slug"`
	SeriesOrder  int                    `firestore:"series_order"`
	SeriesLabel  string                 `firestore:"series_label"`  // display name for series button
	SeriesParent string                 `firestore:"series_parent"` // slug of parent item (e.g. reading text)
	CreatedAt   time.Time              `firestore:"created_at"`
	UpdatedAt   time.Time              `firestore:"updated_at"`
}

func (c *ContentItem) strField(key string) string {
	if c.Content == nil {
		return ""
	}
	v, _ := c.Content[key].(string)
	return v
}

func (c *ContentItem) Question() string {
	q := c.strField("question")
	if q == "" {
		q = c.strField("sentence") // legacy key used by some fill-in seed items
	}
	// Strip Python "None" artifacts left by old seed generator.
	q = strings.ReplaceAll(q, "None", "")
	return q
}

// Answer returns the primary correct answer.
// For fill-in exercises with stored parts, the full reconstructed sentence is returned.
func (c *ContentItem) Answer() string {
	if c.Type == "fillin" {
		if full := c.fillinFullSentence(); full != "" {
			return full
		}
	}
	return c.strField("answer")
}

// fillinFullSentence reconstructs the full correct sentence from alternating
// videbla/solvo parts stored by the seed generator, returning "" if not present.
func (c *ContentItem) fillinFullSentence() string {
	if c.Content == nil {
		return ""
	}
	raw, ok := c.Content["parts"]
	if !ok {
		return ""
	}
	parts, ok := raw.([]interface{})
	if !ok || len(parts) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		m, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if v, _ := m["videbla"].(string); v != "" {
			sb.WriteString(v)
		}
		if s, _ := m["solvo"].(string); s != "" {
			sb.WriteString(s)
		}
	}
	result := sb.String()
	// Strip any Python None artifacts here too.
	result = strings.ReplaceAll(result, "None", "")
	return result
}

// GapAnswers returns the correct answer(s) for each gap in a fill-in exercise.
// If content["answers"] is an array, each element corresponds to one "___" gap.
// If only content["answer"] is set (legacy), it is returned as a single-element
// slice and will be applied to every gap (all gaps share the same answer).
func (c *ContentItem) GapAnswers() []string {
	if c.Content == nil {
		return nil
	}
	if raw, ok := c.Content["answers"]; ok {
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
	}
	if a := c.strField("answer"); a != "" {
		return []string{a}
	}
	return nil
}

// Alternatives returns additional accepted answers beyond the primary Answer().
func (c *ContentItem) Alternatives() []string {
	if c.Content == nil {
		return nil
	}
	raw, ok := c.Content["alternatives"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
func (c *ContentItem) Hint() string        { return c.strField("hint") }
func (c *ContentItem) AudioURL() string    { return c.strField("audio_url") }
func (c *ContentItem) VideoURL() string    { return c.strField("video_url") }
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
