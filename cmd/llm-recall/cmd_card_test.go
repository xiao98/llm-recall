package main

import (
	"strings"
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/llm"
)

// TestRenderCardSmoke walks the data path: build CardData → RenderCard
// → assert that the title, action, and footer all surface in the output.
// Functional coverage of the renderer is also implicit in mock-mode
// integration runs from the harness (see TASKS-W7 self-check item 3).
func TestRenderCardSmoke(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 17, 0, 0, time.UTC)
	d := llm.CardData{
		SessionID8: "26348a6c",
		Source:     "claude",
		When:       now.Format("2006-01-02 15:04"),
		FirstUser:  "claude code的历史会话管理太垃圾了",
		Action:     "调试 sqlite cache 的 mtime 失效逻辑",
		CWD:        "~/llm-recall",
		Footer:     "── llm-recall · Created within the YC TECH community ──",
	}
	out := llm.RenderCard(d)
	for _, want := range []string{
		"26348a6c",
		"claude",
		"2026-05-01 12:17",
		"claude code的历史会话管理太垃圾了",
		"在做：",
		"调试 sqlite cache",
		"~/llm-recall",
		"YC TECH community",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in card render:\n%s", want, out)
		}
	}
}

func TestRenderCardNoPromo(t *testing.T) {
	d := llm.CardData{
		SessionID8: "abcd1234",
		Source:     "claude",
		When:       "2026-05-01 10:00",
		FirstUser:  "x",
		Action:     "y",
		CWD:        "~/",
		// No footer ⇒ --no-promo path.
	}
	out := llm.RenderCard(d)
	if strings.Contains(out, "Created within") || strings.Contains(out, "YC TECH") {
		t.Errorf("no-promo card should not include attribution line:\n%s", out)
	}
}

func TestFirstUserSnippet(t *testing.T) {
	body := "这是第一段用户消息，应该被截取出来。\n\n---\n\n下面是第二段，不应该出现。"
	got := firstUserSnippet(body, 200)
	if !strings.Contains(got, "第一段") {
		t.Errorf("missing first chunk: %s", got)
	}
	if strings.Contains(got, "第二段") {
		t.Errorf("leaked second chunk: %s", got)
	}
}
