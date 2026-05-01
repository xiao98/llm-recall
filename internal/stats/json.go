// JSON snapshot output for `llm-recall stats --json`.
//
// Goal: pipe-friendly machine-readable form. We dump:
//
//   - per-tab stats (all-time / 7d / 30d) — same shape as the panel
//   - a compressed daily-counts array for the past 366 days
//
// The TUI fall-back on non-tty stdout calls WriteJSON too, so a user who
// runs `llm-recall stats > out.txt` doesn't get gibberish.
package stats

import (
	"encoding/json"
	"io"
	"time"
)

type jsonSnapshot struct {
	GeneratedAt   string         `json:"generated_at"`
	StatsPerTab   map[string]any `json:"stats_per_tab"`
	HeatmapCols   int            `json:"heatmap_cols"`
	HeatmapMonths []string       `json:"heatmap_months"`
	DailyCounts   []dailyJSON    `json:"daily_counts"`
	// W9: Top-N topic tokens across the full session set, descending
	// by frequency. Pipeline consumers can render this however they
	// like; the TUI hardcodes 5.
	TopTopics []topicJSON `json:"top_topics"`
}

type topicJSON struct {
	Token string `json:"token"`
	Count int    `json:"count"`
}

type dailyJSON struct {
	Day      string `json:"day"` // YYYY-MM-DD
	Sessions int    `json:"sessions"`
}

// WriteJSON emits the snapshot. Pretty-printed because the typical reader
// is a human with a pipe to less.
func (m *Model) WriteJSON(w io.Writer) error {
	// Make sure all three windows are populated.
	for _, t := range []TabKind{TabAll, TabLast7, TabLast30} {
		m.ensureTab(t)
	}

	statsByTab := map[string]any{}
	for _, t := range []TabKind{TabAll, TabLast7, TabLast30} {
		s := m.stats[t]
		statsByTab[t.Label()] = map[string]any{
			"window_days":         s.WindowDays,
			"window_days_span":    s.WindowDaysSpan,
			"favorite_source":     s.FavoriteSource,
			"per_source":          s.PerSource,
			"total_tokens":        s.TotalTokens,
			"total_tokens_human":  FormatTokens(s.TotalTokens),
			"sessions":            s.Sessions,
			"longest_session_h":   s.LongestSession.Hours(),
			"longest_session_str": FormatDuration(s.LongestSession),
			"active_days":         s.ActiveDays,
			"longest_streak":      s.LongestStreak,
			"current_streak":      s.CurrentStreak,
			"most_active_day":     formatDayOrEmpty(s.MostActiveDay),
			"most_active_day_n":   s.MostActiveDayHits,
			"total_messages":      s.TotalMessages,
		}
	}

	// Daily counts: only days in the past year, oldest → newest. We rebuild
	// from sessions to avoid re-formatting cells embedded in the heatmap
	// grid (which has padding cells).
	daily := BuildDailyCounts(m.sessions, m.now)
	rows := make([]dailyJSON, 0, len(daily))
	for _, d := range daily {
		rows = append(rows, dailyJSON{
			Day:      d.Day.Format("2006-01-02"),
			Sessions: d.Sessions,
		})
	}

	topics := make([]topicJSON, 0, len(m.topics))
	for _, t := range m.topics {
		topics = append(topics, topicJSON{Token: t.Token, Count: t.Count})
	}

	snap := jsonSnapshot{
		GeneratedAt:   m.now.Format("2006-01-02T15:04:05Z07:00"),
		StatsPerTab:   statsByTab,
		HeatmapCols:   m.heatmap.Cols,
		HeatmapMonths: m.heatmap.MonthLabels,
		DailyCounts:   rows,
		TopTopics:     topics,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

func formatDayOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}
