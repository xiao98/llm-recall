package llm

import (
	"context"
	"strings"
	"testing"

	"github.com/xiao98/llm-recall/internal/llm/prompts"
)

func TestMockClientCardFixture(t *testing.T) {
	t.Setenv("LLM_RECALL_LLM_MOCK", "1")
	c, err := NewClient(Anthropic, "", "", "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := c.Complete(context.Background(), Request{
		System: prompts.SystemCard,
		Prompt: "session content...",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.Contains(resp.Text, "调试") {
		t.Errorf("card fixture not returned: %s", resp.Text)
	}
}

func TestMockClientGoldFixture(t *testing.T) {
	t.Setenv("LLM_RECALL_LLM_MOCK", "1")
	c, err := NewClient(Anthropic, "", "", "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := c.Complete(context.Background(), Request{
		System: prompts.SystemGold,
		Prompt: "session bodies...",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(resp.Text), "[") {
		t.Errorf("gold fixture should start with JSON array: %s", resp.Text)
	}
}

func TestMockClient429(t *testing.T) {
	t.Setenv("LLM_RECALL_LLM_MOCK", "429")
	c, _ := NewClient(Anthropic, "", "", "")
	_, err := c.Complete(context.Background(), Request{System: "x", Prompt: "y"})
	if err == nil {
		t.Fatal("expected error from mock 429 mode")
	}
	ae, ok := IsAPIError(err)
	if !ok || ae.Status != 429 {
		t.Errorf("want 429 APIError, got %v", err)
	}
}

func TestMockCallCount(t *testing.T) {
	t.Setenv("LLM_RECALL_LLM_MOCK", "1")
	ResetMockCallCount()
	c, _ := NewClient(Anthropic, "", "", "")
	_, _ = c.Complete(context.Background(), Request{System: prompts.SystemCard, Prompt: "x"})
	_, _ = c.Complete(context.Background(), Request{System: prompts.SystemCard, Prompt: "y"})
	if got := MockCallCount(); got != 2 {
		t.Errorf("call count: got %d, want 2", got)
	}
}
