package stats

import (
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TestComputeWindow: only sessions inside the day-window should count;
// all-time bypasses the cutoff.
func TestComputeWindow(t *testing.T) {
	// Anchor at a fixed instant so day-bucketing is deterministic.
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	in := []adapter.Session{
		{Source: "claude", UpdatedAt: now.Add(-2 * 24 * time.Hour), StartedAt: now.Add(-2*24*time.Hour - time.Hour), Body: "hello world\n---\nsecond"},
		{Source: "codex", UpdatedAt: now.Add(-10 * 24 * time.Hour), StartedAt: now.Add(-11 * 24 * time.Hour), Body: "go test code review\n---\nbug fix"},
		{Source: "gemini", UpdatedAt: now.Add(-40 * 24 * time.Hour), StartedAt: now.Add(-41 * 24 * time.Hour), Body: "out of window"},
	}

	got30 := Compute(in, now, 30, 5)
	if got30.Sessions != 2 {
		t.Errorf("30d sessions = %d, want 2", got30.Sessions)
	}
	if got30.WindowDays != 30 {
		t.Errorf("WindowDays = %d, want 30", got30.WindowDays)
	}

	gotAll := Compute(in, now, 0, 5)
	if gotAll.Sessions != 3 {
		t.Errorf("all-time sessions = %d, want 3", gotAll.Sessions)
	}
	if gotAll.WindowDays != 0 {
		t.Errorf("WindowDays = %d, want 0", gotAll.WindowDays)
	}
}

// TestComputeMessageFallback: when no token data is available the per-msg
// estimate kicks in.
func TestComputeMessageFallback(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	in := []adapter.Session{
		// 3 messages worth of body via 2 separators.
		{Source: "claude", UpdatedAt: now, StartedAt: now.Add(-time.Hour), Body: "one\n---\ntwo\n---\nthree"},
	}
	got := Compute(in, now, 30, 7)
	if got.TotalMessages != 3 {
		t.Errorf("total_messages = %d, want 3", got.TotalMessages)
	}
	// FilePath empty → tokensFromFile returns 0 → fallback kicks in: 3×7 = 21.
	if got.TotalTokens != 21 {
		t.Errorf("total_tokens = %d, want 21 (fallback 3×7)", got.TotalTokens)
	}
}

// TestComputeFavoriteSource: the source with the most sessions wins.
func TestComputeFavoriteSource(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	in := []adapter.Session{
		{Source: "claude", UpdatedAt: now.Add(-time.Hour)},
		{Source: "claude", UpdatedAt: now.Add(-2 * time.Hour)},
		{Source: "codex", UpdatedAt: now.Add(-3 * time.Hour)},
	}
	got := Compute(in, now, 30, 0)
	if got.FavoriteSource != "claude" {
		t.Errorf("FavoriteSource = %q, want claude", got.FavoriteSource)
	}
}

// TestComputeLongestSession: max(updated - started).
func TestComputeLongestSession(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	in := []adapter.Session{
		{Source: "claude", StartedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-time.Hour)},  // 1h
		{Source: "claude", StartedAt: now.Add(-25 * time.Hour), UpdatedAt: now.Add(-time.Hour)}, // 24h
		{Source: "claude", StartedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-30 * time.Second)},
	}
	got := Compute(in, now, 30, 0)
	want := 24 * time.Hour
	if got.LongestSession != want {
		t.Errorf("LongestSession = %v, want %v", got.LongestSession, want)
	}
}

// TestEmpty: zero sessions doesn't panic.
func TestEmpty(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	got := Compute(nil, now, 30, 0)
	if got.Sessions != 0 {
		t.Errorf("empty sessions, got %d", got.Sessions)
	}
	if got.FavoriteSource != "" {
		t.Errorf("empty favorite = %q", got.FavoriteSource)
	}
}
