package adapter

import (
	"strings"
	"testing"
)

// TestClaudeParseFileFull verifies that ParseFileFull pulls every real user
// message into the body, joins with the separator, and skips system-reminder
// pseudo-messages — same filter the title path uses.
func TestClaudeParseFileFull(t *testing.T) {
	c := &Claude{Root: "testdata/projects"}
	files, err := c.ListFiles()
	if err != nil || len(files) == 0 {
		t.Fatalf("ListFiles: %v len=%d", err, len(files))
	}
	s, body, err := c.ParseFileFull(files[0].Path)
	if err != nil {
		t.Fatalf("ParseFileFull: %v", err)
	}
	if s.Title == "" {
		t.Errorf("Title empty")
	}
	// The fixture has one real user message ("帮我写一个排序函数") and one
	// injected <system-reminder>. Body must contain the real one and NOT
	// the injected one.
	if !strings.Contains(body, "帮我写一个排序函数") {
		t.Errorf("body missing real user msg: %q", body)
	}
	if strings.Contains(body, "system-reminder") {
		t.Errorf("body leaked system-reminder: %q", body)
	}
}

// TestCodexParseFileFull: codex fixture has reasoning + function_call rows
// before the real user message; body must contain only the user text.
func TestCodexParseFileFull(t *testing.T) {
	c := &Codex{Root: "testdata/codex"}
	files, err := c.ListFiles()
	if err != nil || len(files) == 0 {
		t.Fatalf("ListFiles: %v len=%d", err, len(files))
	}
	s, body, err := c.ParseFileFull(files[0].Path)
	if err != nil {
		t.Fatalf("ParseFileFull: %v", err)
	}
	if s.Title == "" {
		t.Errorf("Title empty")
	}
	// Body should contain the user prompt with newline preserved.
	if !strings.Contains(body, "帮我重构这段代码") {
		t.Errorf("body missing user msg: %q", body)
	}
	if strings.Contains(body, "thinking") || strings.Contains(body, "function_call") {
		t.Errorf("body leaked reasoning/function_call: %q", body)
	}
}

// TestGeminiParseFileFull_FormatA: the Format A fixture has one user message;
// body should equal that message's content (after CleanTitle in Title).
func TestGeminiParseFileFull_FormatA(t *testing.T) {
	g := &Gemini{Root: "testdata/gemini"}
	files, err := g.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	var aPath string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".json") {
			aPath = f.Path
		}
	}
	if aPath == "" {
		t.Fatal("Format A fixture not found")
	}
	_, body, err := g.ParseFileFull(aPath)
	if err != nil {
		t.Fatalf("ParseFileFull: %v", err)
	}
	if !strings.Contains(body, "分析这个文件夹的进度") {
		t.Errorf("body missing user content: %q", body)
	}
}

// TestGeminiParseFileFull_FormatB skips $set / $rewindTo lines and only
// keeps user messages — the gemini-only sentinel rows must not bleed into
// the body field.
func TestGeminiParseFileFull_FormatB(t *testing.T) {
	g := &Gemini{Root: "testdata/gemini"}
	files, err := g.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	var bPath string
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".jsonl") {
			bPath = f.Path
		}
	}
	if bPath == "" {
		t.Fatal("Format B fixture not found")
	}
	_, body, err := g.ParseFileFull(bPath)
	if err != nil {
		t.Fatalf("ParseFileFull: %v", err)
	}
	if !strings.Contains(body, "你在cli里有deepresearch的功能吗") {
		t.Errorf("body missing user content: %q", body)
	}
	for _, bad := range []string{"$set", "$rewindTo", "lastUpdated", "toolCalls"} {
		if strings.Contains(body, bad) {
			t.Errorf("body leaked %q: %q", bad, body)
		}
	}
}

// TestSafeUTF8Truncate: a payload exactly at the budget should pass through;
// anything over should be cut at a rune boundary, not mid-codepoint.
func TestSafeUTF8Truncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under", "abc", 10, "abc"},
		{"exact", "abc", 3, "abc"},
		{"over_ascii", "abcdef", 3, "abc"},
		{"over_cjk", "中文hi", 4, "中"},      // 中=3 bytes, 文=3 bytes; budget 4 fits 中 only
		{"over_cjk_2", "中文hi", 6, "中文"},   // both fit
		{"over_cjk_split", "中文a", 5, "中"}, // 5 bytes: 中 (3) fits, 文 (3) overflows
	}
	for _, tc := range cases {
		got := SafeUTF8Truncate(tc.in, tc.max)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestIsCodexInjectedUserText: only the two known prefixes count, anything
// else is a real user message.
func TestIsCodexInjectedUserText(t *testing.T) {
	cases := map[string]bool{
		"<environment_context>cwd=/x":       true,
		"  <environment_context>cwd=/x":     true,
		"[Imported from Claude] cleaned up": true,
		"hello world":                       false,
		"<environment but not the tag>":     false,
		"":                                  false,
	}
	for in, want := range cases {
		if got := IsCodexInjectedUserText(in); got != want {
			t.Errorf("IsCodexInjectedUserText(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestParseTime: each of the three accepted formats must round-trip.
func TestParseTime(t *testing.T) {
	cases := []string{
		"2026-04-25T10:55:00Z",
		"2026-04-25T10:55:00.589Z",
		"2026-04-25T10:55:00.0000000Z", // .NET 7-digit
	}
	for _, in := range cases {
		if _, err := ParseTime(in); err != nil {
			t.Errorf("ParseTime(%q) failed: %v", in, err)
		}
	}
	if _, err := ParseTime("not a date"); err == nil {
		t.Error("ParseTime should fail on garbage")
	}
}
