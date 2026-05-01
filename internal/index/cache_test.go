package index

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

func mkSession(src, id string, cwd string, t time.Time, fp string, title string) adapter.Session {
	return adapter.Session{
		Source:    src,
		ID:        id,
		CWD:       cwd,
		StartedAt: t,
		UpdatedAt: t,
		FilePath:  fp,
		Title:     title,
	}
}

// TestCache_UpsertGetDelete walks the four operations the discover layer
// relies on: Upsert (with body), GetByPath hit, GetByPath miss, DeleteByPaths.
func TestCache_UpsertGetDelete(t *testing.T) {
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	defer c.Close()

	now := time.Unix(1700000000, 0)
	a := mkSession("claude", "id-a", "/cwd/a", now, "/p/a.jsonl", "ttl-a")
	b := mkSession("codex", "id-b", "/cwd/b", now, "/p/b.jsonl", "ttl-b")
	cc := mkSession("gemini", "id-c", "/cwd/c", now, "/p/c.jsonl", "ttl-c")

	for _, s := range []adapter.Session{a, b, cc} {
		if err := c.Upsert(s, "body-"+s.ID, now.Unix(), 100); err != nil {
			t.Fatalf("Upsert %s: %v", s.ID, err)
		}
	}

	got, fmtime, fsize, hit, err := c.GetByPath("claude", "/p/a.jsonl")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !hit || got.Title != "ttl-a" || fmtime != now.Unix() || fsize != 100 {
		t.Errorf("Get hit: %+v %d %d %v", got, fmtime, fsize, hit)
	}
	if got.Body != "body-id-a" {
		t.Errorf("body not roundtripped: got %q", got.Body)
	}
	_, _, _, hit, err = c.GetByPath("claude", "/no/such")
	if err != nil {
		t.Fatalf("Get miss: %v", err)
	}
	if hit {
		t.Errorf("expected miss")
	}
}

// TestCache_UpsertOverwritesOnMtimeChange simulates the increment path:
// initial parse, file mtime bumps, re-parse with new title — the cache row
// must reflect the latest values (including body).
func TestCache_UpsertOverwritesOnMtimeChange(t *testing.T) {
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	defer c.Close()

	t1 := time.Unix(1700000000, 0)
	s := mkSession("claude", "sid", "/cwd", t1, "/p/x.jsonl", "v1")
	if err := c.Upsert(s, "body-v1", t1.Unix(), 10); err != nil {
		t.Fatal(err)
	}

	t2 := time.Unix(1700001000, 0)
	s.UpdatedAt = t2
	s.Title = "v2"
	if err := c.Upsert(s, "body-v2", t2.Unix(), 20); err != nil {
		t.Fatal(err)
	}

	got, fmtime, fsize, hit, err := c.GetByPath("claude", "/p/x.jsonl")
	if err != nil || !hit {
		t.Fatalf("Get: %v %v", err, hit)
	}
	if got.Title != "v2" {
		t.Errorf("title not overwritten: %q", got.Title)
	}
	if got.Body != "body-v2" {
		t.Errorf("body not overwritten: %q", got.Body)
	}
	if fmtime != t2.Unix() || fsize != 20 {
		t.Errorf("mtime/size not overwritten: %d %d", fmtime, fsize)
	}
}

// TestCache_StaleSweep mirrors what discoverOne does after a full pass:
// PathsBySource minus what's on disk == dead, DeleteByPaths drops them.
func TestCache_StaleSweep(t *testing.T) {
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	now := time.Unix(1700000000, 0)
	for _, p := range []string{"/p/1", "/p/2", "/p/3"} {
		if err := c.Upsert(mkSession("claude", p, "", now, p, "t"), "", now.Unix(), 1); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := c.PathsBySource("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("want 3 paths, got %d", len(paths))
	}

	if err := c.DeleteByPaths("claude", []string{"/p/2"}); err != nil {
		t.Fatal(err)
	}
	paths, _ = c.PathsBySource("claude")
	if _, gone := paths["/p/2"]; gone {
		t.Errorf("/p/2 should be deleted")
	}
	if len(paths) != 2 {
		t.Errorf("want 2 paths after delete, got %d", len(paths))
	}
}

// TestCache_BatchUpsert exercises the transactional path used by discoverOne.
func TestCache_BatchUpsert(t *testing.T) {
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	batch, err := c.BeginUpsert()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	for _, p := range []string{"/p/a", "/p/b", "/p/c"} {
		s := mkSession("codex", p, "", now, p, "t")
		if err := batch.Upsert(s, "body-"+p, now.Unix(), int64(len(p))); err != nil {
			t.Fatalf("batch upsert: %v", err)
		}
	}
	if err := batch.Commit(); err != nil {
		t.Fatal(err)
	}
	all, err := c.ListBySource("codex")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("want 3, got %d", len(all))
	}
}

// TestCache_SchemaVersion checks that OpenCache leaves the schema_version
// table at CurrentSchemaVersion. Verifies the W3 v2 migration ran.
func TestCache_SchemaVersion(t *testing.T) {
	dir := t.TempDir()
	c, err := OpenCache(filepath.Join(dir, "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	v, err := c.Version()
	if err != nil {
		t.Fatal(err)
	}
	if v != CurrentSchemaVersion {
		t.Errorf("schema_version: want %d, got %d", CurrentSchemaVersion, v)
	}
}
