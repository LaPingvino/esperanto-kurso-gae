package store

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestAdvanceStreak(t *testing.T) {
	now := day("2026-07-16").Add(14 * time.Hour) // practice mid-day UTC

	tests := []struct {
		name         string
		startAt      time.Time
		lastPractice time.Time
		wantStart    time.Time
		wantDays     int
	}{
		{
			name:         "first practice ever (zero last practice)",
			startAt:      time.Time{},
			lastPractice: time.Time{},
			wantStart:    day("2026-07-16"),
			wantDays:     1,
		},
		{
			name:         "second practice same day keeps anchor",
			startAt:      day("2026-07-14"),
			lastPractice: day("2026-07-16").Add(9 * time.Hour),
			wantStart:    day("2026-07-14"),
			wantDays:     3,
		},
		{
			name:         "practiced yesterday extends streak",
			startAt:      day("2026-07-14"),
			lastPractice: day("2026-07-15").Add(23 * time.Hour),
			wantStart:    day("2026-07-14"),
			wantDays:     3,
		},
		{
			name:         "two-day gap resets streak",
			startAt:      day("2026-07-10"),
			lastPractice: day("2026-07-13").Add(12 * time.Hour),
			wantStart:    day("2026-07-16"),
			wantDays:     1,
		},
		{
			name:         "migration: no anchor, practiced today",
			startAt:      time.Time{},
			lastPractice: day("2026-07-16").Add(8 * time.Hour),
			wantStart:    day("2026-07-16"),
			wantDays:     1,
		},
		{
			name:         "migration: no anchor, practiced yesterday",
			startAt:      time.Time{},
			lastPractice: day("2026-07-15").Add(20 * time.Hour),
			wantStart:    day("2026-07-15"),
			wantDays:     2,
		},
		{
			name:         "corrupt future anchor clamps to today",
			startAt:      day("2026-07-20"),
			lastPractice: day("2026-07-16"),
			wantStart:    day("2026-07-16"),
			wantDays:     1,
		},
		{
			name:         "anchor with time-of-day component is truncated",
			startAt:      day("2026-07-14").Add(15 * time.Hour),
			lastPractice: day("2026-07-15").Add(18 * time.Hour),
			wantStart:    day("2026-07-14"),
			wantDays:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotDays := advanceStreak(tt.startAt, tt.lastPractice, now)
			if !gotStart.Equal(tt.wantStart) {
				t.Errorf("start = %v, want %v", gotStart, tt.wantStart)
			}
			if gotDays != tt.wantDays {
				t.Errorf("days = %d, want %d", gotDays, tt.wantDays)
			}
		})
	}
}

// A user who only browses (LastSeenAt fresh, LastPracticeAt old) must lose
// the streak on their next practice — browsing must not keep it alive.
func TestAdvanceStreakBrowsingDoesNotExtend(t *testing.T) {
	now := day("2026-07-16").Add(10 * time.Hour)
	// Practiced a week ago; anchor even earlier. LastSeenAt (fresh, from
	// browsing) is deliberately NOT what gets passed in here.
	gotStart, gotDays := advanceStreak(day("2026-07-01"), day("2026-07-09"), now)
	if !gotStart.Equal(day("2026-07-16")) || gotDays != 1 {
		t.Errorf("got start %v days %d, want reset to 2026-07-16 / 1", gotStart, gotDays)
	}
}
