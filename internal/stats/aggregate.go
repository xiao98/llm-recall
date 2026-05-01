// Package stats: aggregate cached sessions into the metrics the terminal
// renderer needs.
//
// Two outputs:
//
//   - Stats:        4×2 panel metrics for one window (all-time / last 7d /
//     last 30d). Built by Compute().
//   - DailyCounts:  per-day session count over the past 12 months. Used by
//     the heatmap; window-independent (the heatmap always
//     shows a full year). Built by BuildDailyCounts().
//
// The TUI re-runs Compute() when the user toggles between windows but reuses
// the same DailyCounts. Token files are read at most once per session even
// when both functions need them.
package stats

import (
	"strings"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// WindowAll signals "no cutoff — count every session". Compute() treats any
// non-positive WindowDays as all-time.
const WindowAll = 0

// Stats is the 4×2 panel payload for one chosen window.
type Stats struct {
	// WindowDays is 0 for all-time, else the requested window in days.
	WindowDays int
	// WindowStart is the inclusive cutoff actually used. For all-time this
	// is the timestamp of the earliest session (or now if no sessions).
	WindowStart time.Time
	// Now snapshot used for the calculation, so render labels stay
	// consistent if the user lingers in the TUI past midnight.
	Now time.Time

	// 4×2 panel cells.
	FavoriteSource    string        // top source by session count; "" if no sessions
	TotalTokens       int64         // sum of per-session tokens (TOKEN-AUDIT.md rules)
	Sessions          int           // number of sessions in window
	LongestSession    time.Duration // max(updated - started)
	ActiveDays        int           // unique calendar days (local TZ) in window
	WindowDaysSpan    int           // denominator for ActiveDays (1 + days from window-start to today)
	LongestStreak     int           // max consecutive active days inside the window
	MostActiveDay     time.Time     // day with highest session count (zero if none)
	MostActiveDayHits int           // sessions on MostActiveDay
	CurrentStreak     int           // consecutive active days ending today (or yesterday if today blank)

	// TotalMessages is kept for fallback display and JSON consumers; not on
	// the rendered panel but cheap to compute alongside.
	TotalMessages int64
}

// DailyCount is one cell of the heatmap source data.
type DailyCount struct {
	Day      time.Time // local-midnight timestamp for the day
	Sessions int       // sessions whose UpdatedAt fell on this day
}

// Compute aggregates sessions for one window. windowDays<=0 means all-time.
//
// tokenFallbackPerMsg: when a session's jsonl yielded zero tokens, fall back
// to msg_count × this constant. 0 disables the fallback.
//
// `now` parameterised so tests are deterministic; production passes time.Now().
func Compute(sessions []adapter.Session, now time.Time, windowDays int, tokenFallbackPerMsg int64) Stats {
	loc := now.Location()
	var cutoff time.Time
	if windowDays > 0 {
		// Window is "last N days" inclusive of today. We anchor at
		// midnight of (today - (N-1)) so a 7-day window contains today
		// and the 6 previous calendar days, totalling 7 days.
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		cutoff = todayStart.AddDate(0, 0, -(windowDays - 1))
	}

	perSource := map[string]int{}
	perDay := map[time.Time]int{}
	var sessionsInWindow []adapter.Session
	var totalTokens int64
	var totalMessages int64
	var longest time.Duration

	for _, s := range sessions {
		if windowDays > 0 && s.UpdatedAt.Before(cutoff) {
			continue
		}
		sessionsInWindow = append(sessionsInWindow, s)
		perSource[s.Source]++

		// Bucket on local-midnight.
		t := s.UpdatedAt.In(loc)
		dayKey := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		perDay[dayKey]++

		// Message count from body separator. Adapters concatenate user
		// messages with "\n---\n"; one separator → two messages.
		msgs := strings.Count(s.Body, "\n---\n") + 1
		if strings.TrimSpace(s.Body) == "" {
			msgs = 0
		}
		totalMessages += int64(msgs)

		// Token count via per-source field walk. Fall back to msg-count
		// heuristic only when the file gave us 0.
		tk, _ := TokensFromFile(s.Source, s.FilePath)
		if tk == 0 && tokenFallbackPerMsg > 0 {
			tk = int64(msgs) * tokenFallbackPerMsg
		}
		totalTokens += tk

		// Longest = updated - started, defensive against bad data.
		if !s.StartedAt.IsZero() && !s.UpdatedAt.IsZero() {
			d := s.UpdatedAt.Sub(s.StartedAt)
			if d > longest {
				longest = d
			}
		}
	}

	// Favorite source = max session count; ties broken lex order so output
	// is deterministic across runs.
	favorite := pickFavorite(perSource)

	// Most-active-day = max bucket. Same tie-break as favorite.
	mostDay, mostHits := pickMostActiveDay(perDay)

	// Streaks computed from perDay set; constrained to the window.
	longestStreak := computeLongestStreak(perDay)
	currentStreak := computeCurrentStreak(perDay, now, loc)

	// WindowStart for "all-time" is the earliest session's day or `now` if
	// nothing happened. Used to compute ActiveDays denominator and label.
	var windowStart time.Time
	if windowDays > 0 {
		windowStart = cutoff
	} else {
		windowStart = earliestDay(sessionsInWindow, loc)
		if windowStart.IsZero() {
			windowStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		}
	}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	span := int(todayStart.Sub(windowStart).Hours()/24) + 1
	if span < 1 {
		span = 1
	}

	return Stats{
		WindowDays:        windowDays,
		WindowStart:       windowStart,
		Now:               now,
		FavoriteSource:    favorite,
		TotalTokens:       totalTokens,
		Sessions:          len(sessionsInWindow),
		LongestSession:    longest,
		ActiveDays:        len(perDay),
		WindowDaysSpan:    span,
		LongestStreak:     longestStreak,
		MostActiveDay:     mostDay,
		MostActiveDayHits: mostHits,
		CurrentStreak:     currentStreak,
		TotalMessages:     totalMessages,
	}
}

// BuildDailyCounts returns one DailyCount per calendar day from
// (now - 365 days) up to today inclusive, dense (zero-filled). Sessions
// older than the window are ignored.
//
// Heatmap consumers index this array directly; the renderer maps each cell
// to a (week-row, day-col) coordinate using Day.
func BuildDailyCounts(sessions []adapter.Session, now time.Time) []DailyCount {
	loc := now.Location()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	const totalDays = 366 // 365 days + today
	start := todayStart.AddDate(0, 0, -(totalDays - 1))

	perDay := make(map[time.Time]int, len(sessions))
	for _, s := range sessions {
		if s.UpdatedAt.Before(start) {
			continue
		}
		t := s.UpdatedAt.In(loc)
		dayKey := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		perDay[dayKey]++
	}

	out := make([]DailyCount, totalDays)
	for i := 0; i < totalDays; i++ {
		d := start.AddDate(0, 0, i)
		out[i] = DailyCount{Day: d, Sessions: perDay[d]}
	}
	return out
}

func earliestDay(sessions []adapter.Session, loc *time.Location) time.Time {
	var earliest time.Time
	for _, s := range sessions {
		t := s.UpdatedAt.In(loc)
		dayKey := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		if earliest.IsZero() || dayKey.Before(earliest) {
			earliest = dayKey
		}
	}
	return earliest
}

func pickFavorite(perSource map[string]int) string {
	best := ""
	bestN := -1
	for k, v := range perSource {
		switch {
		case v > bestN:
			best, bestN = k, v
		case v == bestN && k < best:
			best = k
		}
	}
	if bestN <= 0 {
		return ""
	}
	return best
}

func pickMostActiveDay(perDay map[time.Time]int) (time.Time, int) {
	var bestDay time.Time
	bestN := 0
	for d, v := range perDay {
		switch {
		case v > bestN:
			bestDay, bestN = d, v
		case v == bestN && !bestDay.IsZero() && d.Before(bestDay):
			// Tie-break: earlier date wins. Doesn't really matter,
			// but keeps test output stable.
			bestDay = d
		}
	}
	return bestDay, bestN
}
