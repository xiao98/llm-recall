// Heatmap layout for the terminal stats card.
//
// We render a 7-row (Sun…Sat or Mon…Sun depending on locale; we hard-code
// Mon-first to match the claude /stats screenshot) × N-week grid covering
// the past ~12 months. Each cell holds:
//
//   - a "level" 0/1/2/3 chosen from the day's session count
//   - the underlying date so the renderer can label months on the x-axis
//
// The level → glyph + colour mapping lives in render.go; this module is
// pure data so it's easy to test.
package stats

import "time"

// HeatLevel is the 4-bucket scale: 0 empty, 1 low, 2 mid, 3 high.
type HeatLevel int

const (
	HeatEmpty HeatLevel = 0
	HeatLow   HeatLevel = 1
	HeatMid   HeatLevel = 2
	HeatHigh  HeatLevel = 3
)

// HeatCell is one square on the calendar.
type HeatCell struct {
	Day      time.Time // local-midnight; zero ⇒ leading/trailing pad
	Level    HeatLevel
	Sessions int
}

// Heatmap is the grid plus the column-level month axis.
type Heatmap struct {
	// Rows is 7 weekday rows, Mon at index 0 … Sun at index 6.
	// Each row holds Cols cells. Padding cells at the leading edge of
	// week-0 (days before the window starts) and trailing edge of the
	// last week have HeatCell{Day: zero}.
	Rows [7][]HeatCell
	Cols int

	// MonthLabels[i] is the month name to print above column i, or "" if
	// no label for that column. We label a column iff its Mon-cell is
	// the first Monday of a new month relative to the previous column.
	MonthLabels []string
}

// BuildHeatmap turns a daily-count array (oldest→newest, dense) into a
// 7×N grid. The array's first cell becomes the leftmost column; the
// algorithm prefixes blank cells until the first cell sits on its own
// weekday row.
//
// Weekday convention: Mon=0, Tue=1, …, Sun=6. Matches the screenshot
// (Mon / Wed / Fri labels on rows 0/2/4).
func BuildHeatmap(days []DailyCount) Heatmap {
	if len(days) == 0 {
		return Heatmap{Cols: 0}
	}

	// Bucket daily counts into HeatLevel.
	leveled := make([]HeatCell, len(days))
	for i, d := range days {
		leveled[i] = HeatCell{
			Day:      d.Day,
			Level:    bucket(d.Sessions),
			Sessions: d.Sessions,
		}
	}

	// Find the leading pad: how many blank cells go before days[0] so
	// days[0]'s weekday lands on the right row.
	leadPad := mondayWeekday(days[0].Day)

	totalCells := leadPad + len(leveled)
	cols := (totalCells + 6) / 7
	totalCells = cols * 7 // pad to full last column

	var grid [7][]HeatCell
	for r := 0; r < 7; r++ {
		grid[r] = make([]HeatCell, cols)
	}

	for i := 0; i < totalCells; i++ {
		col := i / 7
		row := i % 7
		switch {
		case i < leadPad:
			grid[row][col] = HeatCell{} // blank
		case i-leadPad < len(leveled):
			grid[row][col] = leveled[i-leadPad]
		default:
			grid[row][col] = HeatCell{} // trailing blank
		}
	}

	// Month labels: first column whose Mon cell lands in a new month.
	labels := make([]string, cols)
	prevMonth := time.Month(0)
	for c := 0; c < cols; c++ {
		// Use the first non-empty cell in the column; usually the Mon row,
		// but in column 0 with leadPad>0 it might be later in the week.
		var anchor time.Time
		for r := 0; r < 7; r++ {
			if !grid[r][c].Day.IsZero() {
				anchor = grid[r][c].Day
				break
			}
		}
		if anchor.IsZero() {
			continue
		}
		if anchor.Month() != prevMonth {
			labels[c] = anchor.Format("Jan")
			prevMonth = anchor.Month()
		}
	}

	return Heatmap{Rows: grid, Cols: cols, MonthLabels: labels}
}

// bucket maps a session count to a HeatLevel.
//
//	0     → empty
//	1     → low
//	2     → mid
//	≥3    → high
//
// Thresholds picked to match a typical user: most active days have 1-2
// sessions, "high" should be reserved for the truly busy days. If a future
// power user wants finer granularity we'll quantile-bucket; for v1, fixed
// thresholds are predictable and readable.
func bucket(n int) HeatLevel {
	switch {
	case n <= 0:
		return HeatEmpty
	case n == 1:
		return HeatLow
	case n == 2:
		return HeatMid
	default:
		return HeatHigh
	}
}

// mondayWeekday returns 0 for Mon, 1 for Tue, … 6 for Sun. time.Weekday is
// Sun=0 by default so we shift by 1 mod 7.
func mondayWeekday(t time.Time) int {
	w := int(t.Weekday()) // Sun=0..Sat=6
	return (w + 6) % 7    // Mon=0..Sun=6
}
