package model

// GameStars is the game's progress on an account (game/, "la ludo", the
// course played as a game at /ludo/): stars per lesson, by the lesson's
// reading slug, 1–3.
//
// MergeGameStars merges src into dst by taking the larger count per lesson,
// so neither side ever loses progress: a sync can be repeated, cross with
// another device's, or come late (offline) and the result is the same.
func MergeGameStars(dst, src map[string]int) map[string]int {
	if dst == nil {
		dst = map[string]int{}
	}
	for k, n := range src {
		if n > 3 {
			n = 3
		}
		if n > dst[k] {
			dst[k] = n
		}
	}
	return dst
}
