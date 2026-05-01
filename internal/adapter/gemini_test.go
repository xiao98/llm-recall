package adapter

import (
	"context"
	"os"
	"strings"
	"testing"
)

func mkdirAll(p string) error     { return os.MkdirAll(p, 0o755) }
func writeFile(p, c string) error { return os.WriteFile(p, []byte(c), 0o644) }

// TestGeminiDiscover_BothFormats covers the dual-format reality on disk:
// Format A `.json` (legacy, content is a string) and Format B `.jsonl`
// (current, content is `[{text}]` plus interleaved $set/$rewindTo sentinels).
//
// Neither testdata project dir has a metadata.json, so cwd must be empty
// and the title must carry the `<gemini:xxxxxxxx>` prefix.
func TestGeminiDiscover_BothFormats(t *testing.T) {
	g := &Gemini{Root: "testdata/gemini"}
	sessions, err := g.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(sessions))
	}

	byID := map[string]Session{}
	for _, s := range sessions {
		byID[s.ID] = s
	}

	a, ok := byID["1e38cc56-8ae4-4e5d-95ff-359f570ab40c"]
	if !ok {
		t.Fatalf("Format A session missing; got %+v", byID)
	}
	if a.Source != "gemini" {
		t.Errorf("A.Source: got %q", a.Source)
	}
	if a.CWD != "" {
		t.Errorf("A.CWD: want empty (no metadata.json), got %q", a.CWD)
	}
	if a.StartedAt.IsZero() {
		t.Errorf("A.StartedAt zero")
	}
	wantPrefixA := "<gemini:hash1aaa>" // first 8 chars of "hash1aaa00000000"
	if !strings.HasPrefix(a.Title, wantPrefixA+" ") {
		t.Errorf("A.Title prefix: want %q-prefixed, got %q", wantPrefixA, a.Title)
	}
	if !strings.Contains(a.Title, "分析这个文件夹的进度 并总结") {
		t.Errorf("A.Title content: got %q", a.Title)
	}

	b, ok := byID["81e6964e-a363-4d2d-a1ac-e7d06bf51334"]
	if !ok {
		t.Fatalf("Format B session missing; got %+v", byID)
	}
	if b.CWD != "" {
		t.Errorf("B.CWD: want empty, got %q", b.CWD)
	}
	if b.StartedAt.IsZero() {
		t.Errorf("B.StartedAt zero")
	}
	wantPrefixB := "<gemini:hash2bbb>" // first 8 chars of "hash2bbb11111111"
	if !strings.HasPrefix(b.Title, wantPrefixB+" ") {
		t.Errorf("B.Title prefix: want %q-prefixed, got %q", wantPrefixB, b.Title)
	}
	if !strings.Contains(b.Title, "你在cli里有deepresearch的功能吗") {
		t.Errorf("B.Title content: got %q", b.Title)
	}
	// $set / $rewindTo / non-user gemini messages must not leak into the title.
	for _, bad := range []string{"$set", "$rewindTo", "lastUpdated", "toolCalls"} {
		if strings.Contains(b.Title, bad) {
			t.Errorf("B.Title leaked %q: %q", bad, b.Title)
		}
	}
}

// TestGeminiCWDFromProjectRoot verifies the .project_root fallback (this
// machine writes one when the project dir name is a friendly slug).
func TestGeminiCWDFromProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	projDir := tmp + "/myproject"
	chatsDir := projDir + "/chats"
	if err := mkdirAll(chatsDir); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(projDir+"/.project_root", "C:\\real\\cwd"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(chatsDir+"/session-X.jsonl",
		`{"sessionId":"sid","projectHash":"ph","startTime":"2026-04-25T12:08:09.589Z"}`+"\n"+
			`{"id":"u","timestamp":"2026-04-25T12:08:18.829Z","type":"user","content":[{"text":"hello"}]}`+"\n",
	); err != nil {
		t.Fatal(err)
	}

	g := &Gemini{Root: tmp}
	sessions, err := g.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1, got %d", len(sessions))
	}
	if sessions[0].CWD != `C:\real\cwd` {
		t.Errorf("cwd from .project_root: got %q", sessions[0].CWD)
	}
	if strings.HasPrefix(sessions[0].Title, "<gemini:") {
		t.Errorf("title should not be prefixed when cwd resolved: %q", sessions[0].Title)
	}
}
