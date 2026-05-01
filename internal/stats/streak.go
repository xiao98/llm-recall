// Streak math for the stats panel.
//
// "Streak" is consecutive calendar days on which at least one session ended
// (UpdatedAt). All inputs are pre-bucketed maps keyed by local-midnight, so
// timezone is the caller's responsibility.
package stats

import (
	"sort"
	"time"
)

// computeLongestStreak walks the active-day set in chronological order and
// finds the longest run of consecutive days. Empty set → 0.
func computeLongestStreak(perDay map[time.Time]int) int {
	if len(perDay) == 0 {
		return 0
	}
	days := make([]time.Time, 0, len(perDay))
	for d := range perDay {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	best, cur := 1, 1
	for i := 1; i < len(days); i++ {
		gap := days[i].Sub(days[i-1])
		// Two days are "consecutive" iff their midnight timestamps differ
		// by exactly 24h. We allow 23-25h to absorb DST half-day jumps;
		// any larger gap restarts the run.
		if gap >= 23*time.Hour && gap <= 25*time.Hour {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 1
		}
	}
	return best
}

// computeCurrentStreak counts consecutive active days ending today. If today
// has no session, the streak ends yesterday (still counted) — many users
// haven't run their first session of the day yet when they check stats.
// Returns 0 if neither today nor yesterday is active.
func computeCurrentStreak(perDay map[time.Time]int, now time.Time, loc *time.Location) int {
	if len(perDay) == 0 {
		return 0
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)

	cur := today
	if _, ok := perDay[today]; !ok {
		if _, ok := perDay[yesterday]; !ok {
			return 0
		}
		cur = yesterday
	}

	streak := 0
	for {
		if _, ok := perDay[cur]; !ok {
			break
		}
		streak++
		cur = cur.AddDate(0, 0, -1)
	}
	return streak
}
