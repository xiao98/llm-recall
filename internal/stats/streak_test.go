package stats

import (
	"testing"
	"time"
)

// helper: build perDay map from a list of day-offsets relative to base.
func buildPerDay(base time.Time, offsets []int) map[time.Time]int {
	m := map[time.Time]int{}
	for _, off := range offsets {
		d := base.AddDate(0, 0, off)
		m[d]++
	}
	return m
}

func TestComputeLongestStreakEmpty(t *testing.T) {
	if got := computeLongestStreak(nil); got != 0 {
		t.Errorf("empty streak = %d, want 0", got)
	}
}

func TestComputeLongestStreakSingleton(t *testing.T) {
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	pd := buildPerDay(base, []int{0})
	if got := computeLongestStreak(pd); got != 1 {
		t.Errorf("singleton streak = %d, want 1", got)
	}
}

// 5-day run with a 2-day gap then a 3-day run → longest = 5.
func TestComputeLongestStreakRuns(t *testing.T) {
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	pd := buildPerDay(base, []int{0, 1, 2, 3, 4 /* gap */, 7, 8, 9})
	if got := computeLongestStreak(pd); got != 5 {
		t.Errorf("longest streak = %d, want 5", got)
	}
}

func TestComputeCurrentStreakActiveToday(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 1, 14, 0, 0, 0, loc)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, loc)
	pd := buildPerDay(today, []int{0, -1, -2, -4})
	got := computeCurrentStreak(pd, now, loc)
	if got != 3 {
		t.Errorf("current streak = %d, want 3 (today + yesterday + day-2)", got)
	}
}

func TestComputeCurrentStreakActiveYesterdayOnly(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, loc) // morning, hasn't run today
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, loc)
	pd := buildPerDay(today, []int{-1, -2, -3})
	got := computeCurrentStreak(pd, now, loc)
	if got != 3 {
		t.Errorf("current streak from yesterday = %d, want 3", got)
	}
}

func TestComputeCurrentStreakStale(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 1, 8, 0, 0, 0, loc)
	today := time.Date(2026, 5, 1, 0, 0, 0, 0, loc)
	pd := buildPerDay(today, []int{-7, -8})
	got := computeCurrentStreak(pd, now, loc)
	if got != 0 {
		t.Errorf("stale streak = %d, want 0", got)
	}
}
