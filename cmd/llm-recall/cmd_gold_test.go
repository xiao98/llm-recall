package main

import (
	"strings"
	"testing"

	"github.com/xiao98/llm-recall/internal/llm"
)

// TestParseGoldResponse confirms the parser handles three shapes
// (clean array, fenced array, wrapped object) and rejects garbage.
func TestParseGoldResponse(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantN   int
		wantErr bool
	}{
		{
			name:  "clean array",
			in:    `[{"quote":"a","comment":"b"},{"quote":"c","comment":"d"}]`,
			wantN: 2,
		},
		{
			name:  "fenced array",
			in:    "```json\n[{\"quote\":\"a\",\"comment\":\"b\"}]\n```",
			wantN: 1,
		},
		{
			name:  "wrapped object",
			in:    `{"entries":[{"quote":"a","comment":"b"}]}`,
			wantN: 1,
		},
		{
			name:    "garbage",
			in:      "not JSON at all",
			wantErr: true,
		},
		{
			name:    "empty array",
			in:      "[]",
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGoldResponse(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d entries", len(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantN {
				t.Errorf("got %d entries, want %d", len(got), tc.wantN)
			}
		})
	}
}

func TestParseGoldResponseFromFixture(t *testing.T) {
	got, err := parseGoldResponse(llm.MockGoldFixture)
	if err != nil {
		t.Fatalf("fixture parse: %v", err)
	}
	if len(got) != 10 {
		t.Errorf("fixture should have 10 entries, got %d", len(got))
	}
	if !strings.Contains(got[0].Quote, "做事都搞到一半") {
		t.Errorf("first quote unexpected: %s", got[0].Quote)
	}
}

func TestRenderGoldHasAllEntries(t *testing.T) {
	gd := llm.GoldData{
		WindowDays: 7,
		Entries: []llm.GoldEntry{
			{Quote: "alpha quote", Comment: "alpha comment"},
			{Quote: "beta quote", Comment: "beta comment"},
		},
		Footer: "── footer ──",
	}
	out := llm.RenderGold(gd)
	for _, want := range []string{
		"alpha quote", "alpha comment",
		"beta quote", "beta comment",
		"7 天金句 Top 2",
		"footer",
		" 1.", " 2.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderGoldMDIsPlainMarkdown(t *testing.T) {
	gd := llm.GoldData{
		WindowDays: 7,
		Entries: []llm.GoldEntry{
			{Quote: "alpha", Comment: "comment-a"},
			{Quote: "beta", Comment: "comment-b"},
		},
		Footer: "ignored",
	}
	out := llm.RenderGoldMD(gd)
	// Must NOT contain ANSI escape (CSI introducer).
	if strings.Contains(out, "\x1b[") {
		t.Errorf("MD output contains ANSI: %q", out)
	}
	// Must NOT contain box drawing characters.
	for _, bad := range []string{"╭", "╰", "│", "─"} {
		if strings.Contains(out, bad) {
			t.Errorf("MD output contains border char %q", bad)
		}
	}
	// Must contain markdown shape.
	if !strings.Contains(out, "1. **alpha**") || !strings.Contains(out, "   - comment-a") {
		t.Errorf("MD shape missing: %s", out)
	}
}

func TestTruncUTF8DoesNotSplitRune(t *testing.T) {
	// "你好世界" = 4 runes × 3 bytes each = 12 bytes.
	in := "你好世界"
	got := truncUTF8(in, 7) // cap below a rune boundary
	// Only complete runes should remain.
	for _, r := range got {
		if r == '�' {
			t.Errorf("got replacement char in %q", got)
		}
	}
	// "你好" is 6 bytes; we expect that, not "你好" + half of 世.
	if got != "你好" {
		t.Errorf("got %q, want 你好", got)
	}
}
