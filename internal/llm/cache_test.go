package llm

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SetCachePathForTest(filepath.Join(dir, "cache"))
	t.Cleanup(func() { SetCachePathForTest("") })

	key := CacheKey("model-x", "sys", "prompt")
	if _, ok := CacheGet(key); ok {
		t.Fatal("expected miss on empty cache")
	}
	want := Response{Text: "hello", InputToks: 11, OutputToks: 22}
	if err := CachePut(key, "model-x", want); err != nil {
		t.Fatalf("CachePut: %v", err)
	}
	got, ok := CacheGet(key)
	if !ok {
		t.Fatal("expected hit after Put")
	}
	if got.Text != want.Text || got.InputToks != want.InputToks || got.OutputToks != want.OutputToks {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, want)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	SetCachePathForTest(filepath.Join(dir, "cache"))
	t.Cleanup(func() { SetCachePathForTest("") })

	key := CacheKey("model-x", "sys", "prompt")
	stale := time.Now().Add(-(CacheTTL + time.Hour))
	if err := CachePutWithTime(key, "model-x", Response{Text: "stale"}, stale); err != nil {
		t.Fatalf("CachePutWithTime: %v", err)
	}
	if _, ok := CacheGet(key); ok {
		t.Fatal("expected stale entry to miss")
	}
}

func TestCacheKeyStability(t *testing.T) {
	a := CacheKey("m", "sys", "prompt")
	b := CacheKey("m", "sys", "prompt")
	c := CacheKey("m", "sys", "prompt2")
	if a != b {
		t.Errorf("same inputs should hash equal: %s vs %s", a, b)
	}
	if a == c {
		t.Errorf("different prompt should hash distinct: both %s", a)
	}
}
