package llm

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/xiao98/llm-recall/internal/llm/prompts"
)

// TestCacheSkipsLLMOnHit demonstrates the contract: write once via mock
// → second call with same key reads from disk → mock invocation count
// stays at 1.
func TestCacheSkipsLLMOnHit(t *testing.T) {
	t.Setenv("LLM_RECALL_LLM_MOCK", "1")
	dir := t.TempDir()
	SetCachePathForTest(filepath.Join(dir, "cache"))
	t.Cleanup(func() { SetCachePathForTest("") })

	system := prompts.SystemCard
	prompt := "session content..."
	model := "claude-haiku-4-5-20251001"
	key := CacheKey(model, system, prompt)

	c, _ := NewClient(Anthropic, "", model, "")

	ResetMockCallCount()
	// First call: miss → invoke mock → write cache
	if _, ok := CacheGet(key); ok {
		t.Fatal("expected miss on empty cache")
	}
	resp, err := c.Complete(context.Background(), Request{System: system, Prompt: prompt, MaxTokens: 256})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := CachePut(key, model, resp); err != nil {
		t.Fatalf("CachePut: %v", err)
	}
	if MockCallCount() != 1 {
		t.Errorf("after first call: count=%d, want 1", MockCallCount())
	}

	// Second call: hit → no mock invocation
	if _, ok := CacheGet(key); !ok {
		t.Fatal("expected hit on second")
	}
	if MockCallCount() != 1 {
		t.Errorf("after cache hit (should NOT call mock): count=%d, want 1", MockCallCount())
	}
}
