package adapter

import (
	"context"
	"testing"
)

// TestClaudeDiscover_Sample validates the four hot extractor cases on a
// fixture jsonl: id from filename stem, cwd from first cwd-bearing record,
// title skips the system-reminder pseudo-user message and lands on the real
// one, and StartedAt parses RFC3339Nano.
func TestClaudeDiscover_Sample(t *testing.T) {
	c := &Claude{Root: "testdata/projects"}
	sessions, err := c.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	s := sessions[0]

	if s.Source != "claude" {
		t.Errorf("Source: want claude, got %q", s.Source)
	}
	if s.ID != "abc12345-aaaa-bbbb-cccc-deadbeef0001" {
		t.Errorf("ID: got %q", s.ID)
	}
	if s.CWD != `C:\Users\demo\proj` {
		t.Errorf("CWD: got %q", s.CWD)
	}
	if s.Title != "帮我写一个排序函数" {
		t.Errorf("Title: got %q (system-reminder must be skipped)", s.Title)
	}
	if s.FilePath == "" {
		t.Errorf("FilePath empty")
	}
	if s.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt zero")
	}
	if s.StartedAt.IsZero() {
		t.Errorf("StartedAt zero")
	}
}

func TestIsInjectedUserText(t *testing.T) {
	cases := map[string]bool{
		"<system-reminder>foo</system-reminder>":         true,
		"<local-command-stdout>x</local-command-stdout>": true,
		"<command-name>/help</command-name>":             true,
		"hello world":                                    false,
		"<just a tag>":                                   false,
	}
	for in, want := range cases {
		if got := isInjectedUserText(in); got != want {
			t.Errorf("isInjectedUserText(%q) = %v, want %v", in, got, want)
		}
	}
}
