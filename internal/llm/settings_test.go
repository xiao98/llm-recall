package llm

import (
	"strings"
	"testing"

	"github.com/xiao98/llm-recall/internal/config"
)

func TestResolveFlagPrecedence(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-fromenv")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_RECALL_LLM_MOCK", "")

	cfg := &config.Config{}
	cfg.LLM.Vendor = "openai"
	cfg.LLM.Model = "gpt-4o"
	cfg.LLM.BaseURL = "https://from.config"

	// Flags should win over both config and env.
	rs, err := Resolve("anthropic", "claude-sonnet-4-6", "https://from.flag", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rs.Vendor != Anthropic {
		t.Errorf("vendor: got %s, want anthropic", rs.Vendor)
	}
	if rs.Model != "claude-sonnet-4-6" {
		t.Errorf("model: got %s", rs.Model)
	}
	if rs.BaseURL != "https://from.flag" {
		t.Errorf("baseURL: got %s", rs.BaseURL)
	}
	if rs.Key != "sk-ant-fromenv" {
		t.Errorf("key: got %s", rs.Key)
	}
}

func TestResolveConfigBeatsDefaults(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-x")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_RECALL_BASE_URL", "")
	t.Setenv("LLM_RECALL_LLM_MOCK", "")

	cfg := &config.Config{}
	cfg.LLM.Model = "claude-haiku-from-config"

	rs, err := Resolve("", "", "", cfg)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rs.Vendor != Anthropic {
		t.Errorf("vendor auto-detect: got %s", rs.Vendor)
	}
	if rs.Model != "claude-haiku-from-config" {
		t.Errorf("model: got %s", rs.Model)
	}
	if rs.BaseURL != DefaultBaseURL(Anthropic) {
		t.Errorf("default baseURL: got %s", rs.BaseURL)
	}
}

func TestResolveNoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_RECALL_LLM_MOCK", "")
	_, err := Resolve("", "", "", nil)
	if err == nil {
		t.Fatal("expected error when no key set")
	}
	if !strings.Contains(err.Error(), "API key") && !strings.Contains(err.Error(), "in environment") {
		t.Errorf("err message: %v", err)
	}
}

func TestResolveMockModeNoKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("LLM_RECALL_LLM_MOCK", "1")
	rs, err := Resolve("", "", "", nil)
	if err != nil {
		t.Fatalf("Resolve in mock mode: %v", err)
	}
	if rs.Vendor != Anthropic {
		t.Errorf("mock vendor: got %s", rs.Vendor)
	}
}
