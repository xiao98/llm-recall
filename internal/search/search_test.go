package search

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/index"
)

// fixtureCache builds a fresh cache populated with a small, predictable
// dataset that exercises source filtering, multi-word AND, and CJK matching.
func fixtureCache(t *testing.T) *index.Cache {
	t.Helper()
	dir := t.TempDir()
	c, err := index.OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	now := time.Now()
	rows := []struct {
		s    adapter.Session
		body string
	}{
		{
			adapter.Session{
				Source: "claude", ID: "c1", CWD: "/p1", UpdatedAt: now.Add(-1 * time.Hour),
				FilePath: "/p/c1.jsonl", Title: "claude 历史搜索能用吗",
			},
			"我之前在 claude 跑的项目历史现在怎么找回来",
		},
		{
			adapter.Session{
				Source: "claude", ID: "c2", CWD: "/p2", UpdatedAt: now.Add(-2 * time.Hour),
				FilePath: "/p/c2.jsonl", Title: "重构 sort 算法",
			},
			"用 python 写个快排",
		},
		{
			adapter.Session{
				Source: "codex", ID: "x1", CWD: "/p3", UpdatedAt: now.Add(-3 * time.Hour),
				FilePath: "/p/x1.jsonl", Title: "飞书 wiki 内嵌 bitable",
			},
			"飞书的 api 怎么调",
		},
		{
			adapter.Session{
				Source: "gemini", ID: "g1", CWD: "/p4", UpdatedAt: now.Add(-30 * time.Minute),
				FilePath: "/p/g1.jsonl", Title: "<gemini:hash1aaa> 你能链接 mcp 吗",
			},
			"飞书的 webhook 也行",
		},
	}
	for _, r := range rows {
		if err := c.Upsert(r.s, r.body, r.s.UpdatedAt.Unix(), 100); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

// TestSearch_EmptyQueryReturnsAllByRecency: empty input should return every
// row, ordered by updated_at desc — the cold-launch behaviour.
func TestSearch_EmptyQueryReturnsAllByRecency(t *testing.T) {
	c := fixtureCache(t)
	got, err := Search(c.DB(), "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rows, got %d", len(got))
	}
	if got[0].Session.ID != "g1" {
		t.Errorf("most-recent first row: want g1, got %q", got[0].Session.ID)
	}
}

// TestSearch_SingleWord_CJK: a single CJK word matches both title and body
// columns. This is the "飞书" demo case from W3 §验收.
func TestSearch_SingleWord_CJK(t *testing.T) {
	c := fixtureCache(t)
	got, err := Search(c.DB(), "飞书", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	// codex x1 (title), gemini g1 (body) should both hit; the others must not.
	ids := map[string]bool{}
	for _, r := range got {
		ids[r.Session.ID] = true
	}
	if !ids["x1"] {
		t.Errorf("飞书 should hit codex x1 (title): %v", ids)
	}
	if !ids["g1"] {
		t.Errorf("飞书 should hit gemini g1 (body): %v", ids)
	}
	if ids["c1"] || ids["c2"] {
		t.Errorf("飞书 should NOT hit c1/c2: %v", ids)
	}
}

// TestSearch_MultiWord_AND: two words must both appear (in either column).
// "claude 历史" matches the claude row but not the codex row that mentions
// only one of them.
func TestSearch_MultiWord_AND(t *testing.T) {
	c := fixtureCache(t)
	got, err := Search(c.DB(), "claude 历史", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one hit")
	}
	for _, r := range got {
		if r.Session.ID == "c2" {
			t.Errorf("c2 should not match (no 历史 anywhere): %v", r.Session)
		}
		if r.Session.ID == "x1" || r.Session.ID == "g1" {
			t.Errorf("non-claude row matched: %v", r.Session)
		}
	}
	hitC1 := false
	for _, r := range got {
		if r.Session.ID == "c1" {
			hitC1 = true
		}
	}
	if !hitC1 {
		t.Errorf("c1 (matches claude+历史) should be in results")
	}
}

// TestSearch_SourceFilter restricts the candidate set even when the query
// matches rows in other adapters.
func TestSearch_SourceFilter(t *testing.T) {
	c := fixtureCache(t)
	got, err := Search(c.DB(), "飞书", "codex", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.Session.Source != "codex" {
			t.Errorf("source filter leak: got %q", r.Session.Source)
		}
	}
	if len(got) != 1 || got[0].Session.ID != "x1" {
		t.Errorf("want only codex x1, got %+v", got)
	}
}

// TestSearch_LimitTruncates ensures the post-rank slice is clipped.
func TestSearch_LimitTruncates(t *testing.T) {
	c := fixtureCache(t)
	got, err := Search(c.DB(), "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("limit not enforced: got %d", len(got))
	}
}

// TestWords mirrors the helper used by the TUI for highlight tokenisation.
func TestWords(t *testing.T) {
	got := Words("Foo  bar 中文 ")
	want := []string{"foo", "bar", "中文"}
	if len(got) != len(want) {
		t.Fatalf("len: %v vs %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d]: %q vs %q", i, got[i], want[i])
		}
	}
}
