package model

// Translation holds a community-contributed translation of content or a comment.
type Translation struct {
	ID         string `firestore:"-"`
	TargetType string `firestore:"target_type"` // "content"|"comment"
	TargetID   string `firestore:"target_id"`
	Language   string `firestore:"language"`
	Text       string `firestore:"text"`
	AuthorID   string `firestore:"author_id"`
	VoteScore  int    `firestore:"vote_score"`
}
