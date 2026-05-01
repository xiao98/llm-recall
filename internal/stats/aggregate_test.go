package stats

import (
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TestAggregateWindow: only sessions inside the day-window should count.
func TestAggregateWindow(t *testing.T) {
	now := time.Now()
	in := []adapter.Session{
		{Source: "claude", UpdatedAt: now.Add(-2 * 24 * time.Hour), StartedAt: now.Add(-2*24*time.Hour - time.Hour), Body: "hello world\n---\nsecond"},
		{Source: "codex", UpdatedAt: now.Add(-10 * 24 * time.Hour), StartedAt: now.Add(-11 * 24 * time.Hour), Body: "go test code review\n---\nbug fix"},
		{Source: "gemini", UpdatedAt: now.Add(-40 * 24 * time.Hour), StartedAt: now.Add(-41 * 24 * time.Hour), Body: "out of window"},
	}
	req := Aggregate(in, 30, 5, true)
	if req.TotalSessions != 2 {
		t.Errorf("total = %d, want 2", req.TotalSessions)
	}
	if req.PerSource["claude"] != 1 || req.PerSource["codex"] != 1 {
		t.Errorf("per_source = %v, want claude=1 codex=1", req.PerSource)
	}
	if req.PerSource["gemini"] != 0 {
		t.Errorf("gemini should be filtered out, got %d", req.PerSource["gemini"])
	}
	if req.WindowDays != 30 {
		t.Errorf("window_days = %d, want 30", req.WindowDays)
	}
	if req.Watermark != true {
		t.Errorf("watermark passthrough broken")
	}
}

// TestAggregateMessageFallback: when no token data is available the
// renderer can still show message counts.
func TestAggregateMessageFallback(t *testing.T) {
	now := time.Now()
	in := []adapter.Session{
		// 3 messages worth of body via 2 separators.
		{Source: "claude", UpdatedAt: now, StartedAt: now.Add(-time.Hour), Body: "one\n---\ntwo\n---\nthree"},
	}
	req := Aggregate(in, 30, 7, true)
	if req.TotalMessages != 3 {
		t.Errorf("total_messages = %d, want 3", req.TotalMessages)
	}
	// File doesn't exist (no FilePath set) so token fallback applies:
	// 3 messages × 7 = 21.
	if req.TotalTokens != 21 {
		t.Errorf("total_tokens = %d, want 21 (fallback 3×7)", req.TotalTokens)
	}
}

// TestTopicTokensCJK: Chinese 2-grams should appear; stopword 我/的 must be
// dropped from the bigram if either rune is a stopword.
func TestTopicTokensCJK(t *testing.T) {
	// "项目代码" → 项目, 目代, 代码 (none stopwords).
	// "我的项目" → 我的 (skip: 我 is stopword), 的项 (skip: 的), 项目 (keep)
	got := topicTokens("项目代码 我的项目")
	want := map[string]bool{"项目": true, "目代": true, "代码": true}
	for _, g := range got {
		if !want[g] {
			t.Logf("unexpected bigram in topicTokens(): %q", g)
		}
	}
	hasItem := func(s string) bool {
		for _, g := range got {
			if g == s {
				return true
			}
		}
		return false
	}
	if !hasItem("项目") {
		t.Errorf("expected 项目 in topics, got %v", got)
	}
	// 我的, 的项 should both be filtered out.
	if hasItem("我的") || hasItem("的项") {
		t.Errorf("stopword bigrams leaked: %v", got)
	}
}

// TestTopNTopics: ties broken lexicographically; singletons dropped.
func TestTopNTopics(t *testing.T) {
	counts := map[string]int{
		"alpha": 5,
		"beta":  5, // tie with alpha
		"gamma": 3,
		"delta": 1, // singleton — dropped
		"123":   9, // pure digits — dropped
	}
	got := topNTopics(counts, 5)
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("topic[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
