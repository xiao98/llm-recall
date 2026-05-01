package adapter

import (
	"context"
	"strings"
	"testing"
)

// TestCodexDiscover_Sample exercises the four extractor cases on a fixture:
// session_meta gives id+cwd+started, the leading reasoning + function_call
// rows are skipped, and the title comes from the first user input_text after
// CleanTitle has folded the embedded newline.
func TestCodexDiscover_Sample(t *testing.T) {
	c := &Codex{Root: "testdata/codex"}
	sessions, err := c.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	s := sessions[0]

	if s.Source != "codex" {
		t.Errorf("Source: got %q", s.Source)
	}
	if s.ID != "0199e2c1-e4ba-73a3-bd39-24cc08a64fe6" {
		t.Errorf("ID: got %q", s.ID)
	}
	if s.CWD != `C:\Users\demo\proj` {
		t.Errorf("CWD: got %q", s.CWD)
	}
	if s.StartedAt.IsZero() {
		t.Errorf("StartedAt zero")
	}
	if s.Title != "帮我重构这段代码 并加注释" {
		t.Errorf("Title: got %q (want collapsed-newline user text, no reasoning/function_call leakage)", s.Title)
	}
	if strings.Contains(s.Title, "thinking") {
		t.Errorf("Title leaked reasoning content: %q", s.Title)
	}
}
