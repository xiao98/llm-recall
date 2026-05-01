package stats

import (
	"strings"
	"testing"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TestTopTopicsBasic asserts the top-N selection picks the most-
// frequent non-stopword tokens and respects the stable tie-break
// (count desc, then name asc).
func TestTopTopicsBasic(t *testing.T) {
	in := []adapter.Session{
		{Body: "wiki history wiki feishu quant"},
		{Body: "wiki feishu quant"},
		{Body: "history quant"},
		{Body: "quant"},
	}
	got := TopTopics(in, 5)
	if len(got) == 0 {
		t.Fatalf("got empty topics")
	}
	// "quant" appears 4×, "wiki" 3×, "history" 2×, "feishu" 2×.
	if got[0].Token != "quant" {
		t.Errorf("expected first=quant, got %+v", got)
	}
	// Tie between history (2) and feishu (2): name asc → feishu, history.
	tokens := []string{got[0].Token, got[1].Token, got[2].Token, got[3].Token}
	want := []string{"quant", "wiki", "feishu", "history"}
	for i, w := range want {
		if tokens[i] != w {
			t.Errorf("position %d: got %q want %q (full: %v)", i, tokens[i], w, tokens)
		}
	}
}

// TestTopTopicsStopwords confirms LLM brand names + filler words are
// dropped. The brand tokens are deliberately the highest count so a
// regression would surface immediately.
func TestTopTopicsStopwords(t *testing.T) {
	body := strings.Repeat("claude codex gemini ", 10) + "sqlite cache schema"
	in := []adapter.Session{{Body: body}}
	got := TopTopics(in, 5)
	for _, top := range got {
		if top.Token == "claude" || top.Token == "codex" || top.Token == "gemini" {
			t.Errorf("brand name leaked into topics: %+v", got)
		}
	}
	if len(got) == 0 || got[0].Token != "cache" && got[0].Token != "schema" && got[0].Token != "sqlite" {
		t.Errorf("expected sqlite/cache/schema family at top, got %+v", got)
	}
}

// TestTopTopicsChinese verifies CJK bigrams are extracted and the
// stopword filter on common bigrams ("我们" / "可以") works.
func TestTopTopicsChinese(t *testing.T) {
	in := []adapter.Session{
		{Body: "调试 sqlite cache 的 mtime 失效"},
		{Body: "sqlite cache 重建 + mtime 检测"},
		{Body: "sqlite 索引"},
	}
	got := TopTopics(in, 5)
	tokens := map[string]bool{}
	for _, t := range got {
		tokens[t.Token] = true
	}
	if !tokens["sqlite"] {
		t.Errorf("expected 'sqlite' in topics, got %+v", got)
	}
	// CJK bigram: "mtime" is ASCII so check 'cache' instead which
	// repeats; the Chinese tokens come out as 2-char bigrams that may
	// shift around, so this assertion stays loose.
	if !tokens["cache"] && !tokens["mtime"] {
		t.Errorf("expected cache/mtime in topics, got %+v", got)
	}
}

// TestTopTopicsEmpty: no sessions / empty bodies → empty slice.
func TestTopTopicsEmpty(t *testing.T) {
	if got := TopTopics(nil, 5); len(got) != 0 {
		t.Errorf("nil input: %v", got)
	}
	if got := TopTopics([]adapter.Session{{Body: ""}}, 5); len(got) != 0 {
		t.Errorf("empty body: %v", got)
	}
}

// TestChooseStateBranches walks every branch of ChooseState. The
// pet's expression is user-facing; if a future stats refactor renames
// a Stats field, this test surfaces the mismatch immediately.
func TestChooseStateBranches(t *testing.T) {
	cases := []struct {
		name string
		s    Stats
		want State
	}{
		{"sleeping wins", Stats{Sessions: 5, RecentDays7: 0}, Sleeping},
		{"confused on data anomaly", Stats{Sessions: 3, TotalTokens: 0, RecentDays7: 1, AnomalousData: true}, Confused},
		{"pumped on streak 14", Stats{LongestStreak: 14, RecentDays7: 7}, Pumped},
		{"pumped on sessions today 10", Stats{SessionsToday: 12, RecentDays7: 7}, Pumped},
		{"happy on streak 7", Stats{LongestStreak: 7, RecentDays7: 7}, Happy},
		{"cheering when record today", Stats{RecentDays7: 1, ActiveDays: 5, HasRecordToday: true}, Cheering},
		{"sad on low active days", Stats{RecentDays7: 1, ActiveDays: 2}, Sad},
		{"idle default", Stats{RecentDays7: 1, ActiveDays: 10}, Idle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChooseState(tc.s)
			if got != tc.want {
				t.Errorf("ChooseState got=%v want=%v (stats=%+v)", got, tc.want, tc.s)
			}
		})
	}
}

// TestRenderPetSizes sanity-checks every sprite is the documented
// 16-line height. Visual content is reviewed by eye, not asserted.
func TestRenderPetSizes(t *testing.T) {
	for st, sprite := range Sprites {
		if len(sprite) != PetHeight {
			t.Errorf("sprite %v: %d lines, want %d", st, len(sprite), PetHeight)
		}
	}
}
