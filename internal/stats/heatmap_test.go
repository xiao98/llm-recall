package stats

import (
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TestBucket covers the 4-level mapping (0/1/2/≥3 sessions per day).
func TestBucket(t *testing.T) {
	cases := []struct {
		n    int
		want HeatLevel
	}{
		{0, HeatEmpty},
		{1, HeatLow},
		{2, HeatMid},
		{3, HeatHigh},
		{99, HeatHigh},
		{-5, HeatEmpty},
	}
	for _, c := range cases {
		got := bucket(c.n)
		if got != c.want {
			t.Errorf("bucket(%d) = %v, want %v", c.n, got, c.want)
		}
	}
}

// TestBuildHeatmapBuckets feeds 5 days into BuildHeatmap and asserts that
// each session count maps to the right level glyph.
func TestBuildHeatmapBuckets(t *testing.T) {
	// Five days, oldest first: 0, 1, 2, 3, 7 sessions.
	base := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC) // Mon
	days := []DailyCount{
		{Day: base, Sessions: 0},
		{Day: base.AddDate(0, 0, 1), Sessions: 1},
		{Day: base.AddDate(0, 0, 2), Sessions: 2},
		{Day: base.AddDate(0, 0, 3), Sessions: 3},
		{Day: base.AddDate(0, 0, 4), Sessions: 7},
	}
	h := BuildHeatmap(days)
	if h.Cols != 1 {
		t.Fatalf("cols = %d, want 1 (5 days fit in week 0 starting Monday)", h.Cols)
	}
	wantLevels := []HeatLevel{HeatEmpty, HeatLow, HeatMid, HeatHigh, HeatHigh}
	for i, want := range wantLevels {
		got := h.Rows[i][0].Level
		if got != want {
			t.Errorf("row %d: level=%v want %v (%d sessions)", i, got, want, days[i].Sessions)
		}
	}
	// Trailing rows (Sat, Sun) inside the column should be blank-day cells.
	for r := 5; r < 7; r++ {
		if !h.Rows[r][0].Day.IsZero() {
			t.Errorf("row %d col 0 should be blank, got %v", r, h.Rows[r][0])
		}
	}
}

// TestBuildHeatmapLeadPad: a Wednesday-anchored series should leave the
// Mon and Tue rows of column 0 blank.
func TestBuildHeatmapLeadPad(t *testing.T) {
	base := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC) // Wed
	days := []DailyCount{
		{Day: base, Sessions: 5},
		{Day: base.AddDate(0, 0, 1), Sessions: 5}, // Thu
	}
	h := BuildHeatmap(days)
	if h.Cols != 1 {
		t.Fatalf("cols = %d, want 1", h.Cols)
	}
	if !h.Rows[0][0].Day.IsZero() || !h.Rows[1][0].Day.IsZero() {
		t.Errorf("Mon/Tue should be blank pad, got %v / %v", h.Rows[0][0], h.Rows[1][0])
	}
	if h.Rows[2][0].Sessions != 5 {
		t.Errorf("Wed cell should hold 5 sessions, got %v", h.Rows[2][0])
	}
}

// TestBuildHeatmapMultiCol: 14 contiguous days starting Monday → 2 columns.
func TestBuildHeatmapMultiCol(t *testing.T) {
	base := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC) // Mon
	days := make([]DailyCount, 14)
	for i := range days {
		days[i] = DailyCount{Day: base.AddDate(0, 0, i), Sessions: 1}
	}
	h := BuildHeatmap(days)
	if h.Cols != 2 {
		t.Fatalf("cols = %d, want 2", h.Cols)
	}
	for r := 0; r < 7; r++ {
		for c := 0; c < 2; c++ {
			if h.Rows[r][c].Day.IsZero() {
				t.Errorf("row %d col %d unexpectedly blank", r, c)
			}
			if h.Rows[r][c].Level != HeatLow {
				t.Errorf("row %d col %d level=%v, want low", r, c, h.Rows[r][c].Level)
			}
		}
	}
}

// TestBuildDailyCounts spans 366 days with one in-range session.
func TestBuildDailyCounts(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	in := []adapter.Session{
		{Source: "claude", UpdatedAt: now.Add(-2 * 24 * time.Hour)},
		{Source: "claude", UpdatedAt: now.Add(-2 * 24 * time.Hour)},
		{Source: "claude", UpdatedAt: now.Add(-400 * 24 * time.Hour)}, // out of window
	}
	dc := BuildDailyCounts(in, now)
	if len(dc) != 366 {
		t.Fatalf("len = %d, want 366", len(dc))
	}
	// Today is dc[365]; -2d is dc[363].
	if dc[363].Sessions != 2 {
		t.Errorf("day -2 sessions = %d, want 2", dc[363].Sessions)
	}
	if dc[365].Sessions != 0 {
		t.Errorf("today sessions = %d, want 0", dc[365].Sessions)
	}
}
