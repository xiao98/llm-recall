// Display-side formatting for the stats panel.
//
// Kept separate from the aggregator so renderer + JSON exporter can share
// the same labels (e.g. "11d 2h 36m" Longest session, "Apr 27" Most active
// day) without dragging in any TUI deps.
package stats

import (
	"fmt"
	"time"
)

// FormatDuration renders a Go duration as the panel-style string:
//
//	d ≥ 1 day:    "11d 2h 36m"   (always 3 fields when days present)
//	1h ≤ d < 1d:  "4h 30m"
//	1m ≤ d < 1h:  "45m"
//	d < 1m:       "<1m"           (we don't render seconds)
//	d ≤ 0:        "—"
//
// Choices: floor on each unit (no rounding-up surprises); never emits 0d /
// 0h prefix once we're below that unit. Seconds dropped because the panel
// is at-a-glance — minutes is the right floor for "longest session".
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Minute {
		return "<1m"
	}
	totalMins := int64(d / time.Minute)
	days := totalMins / (24 * 60)
	hours := (totalMins % (24 * 60)) / 60
	mins := totalMins % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	default:
		return fmt.Sprintf("%dm", mins)
	}
}

// FormatDate renders a calendar day as "Apr 27" (English short-month + day,
// no leading zero on day). Zero time → "—".
func FormatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2")
}

// FormatTokens compresses big numbers into human-friendly k / m / b form.
// "14.5m" not "14523441". The exact count is in --json output for anyone
// who needs it.
func FormatTokens(n int64) string {
	switch {
	case n < 0:
		return "—"
	case n < 1_000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return trimZero(float64(n)/1_000.0) + "k"
	case n < 1_000_000_000:
		return trimZero(float64(n)/1_000_000.0) + "m"
	default:
		return trimZero(float64(n)/1_000_000_000.0) + "b"
	}
}

// trimZero formats `f` with at most one decimal digit, dropping ".0".
func trimZero(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}

// FormatStreak: "8 days", "1 day", "0 days" — accept the plural double for
// 0 because that matches how the screenshot reads ("Longest streak: 8 days").
func FormatStreak(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// FormatActiveDays: "23/79". Caller has already taken min so this is a
// thin wrapper, kept here to colocate display strings.
func FormatActiveDays(active, total int) string {
	if total <= 0 {
		total = 1
	}
	if active > total {
		active = total
	}
	return fmt.Sprintf("%d/%d", active, total)
}
