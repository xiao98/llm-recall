// Mock LLM client. Activated when LLM_RECALL_LLM_MOCK is set.
//
// Modes:
//   - LLM_RECALL_LLM_MOCK=1   → return canned fixture text per command
//     (card or gold; chosen by inspecting req.System for the unique
//     system-prompt prefix).
//   - LLM_RECALL_LLM_MOCK=429 → simulate a rate-limit response so the
//     "friendly error" UX path can be exercised without burning a real
//     API quota.
//
// The mock additionally counts Complete() invocations so cache-hit
// tests can assert "second run did NOT hit the model".
package llm

import (
	"context"
	"os"
	"strings"
	"sync/atomic"

	"github.com/xiao98/llm-recall/internal/llm/prompts"
)

// MockCallCount is the package-level counter incremented every time a
// mockClient.Complete is invoked. Tests reset via ResetMockCallCount
// before exercising the cache.
var mockCallCount int64

// MockCallCount returns the current count. Atomic load.
func MockCallCount() int { return int(atomic.LoadInt64(&mockCallCount)) }

// ResetMockCallCount zeros the counter. Tests use this between phases.
func ResetMockCallCount() { atomic.StoreInt64(&mockCallCount, 0) }

type mockClient struct {
	vendor Vendor
	model  string
}

func newMockClient(v Vendor, model string) *mockClient {
	if v == "" {
		v = Anthropic
	}
	if model == "" {
		model = DefaultModel(v)
	}
	if model == "" {
		// vendor was "anthropic"-but-empty path, fall through.
		model = "mock-model"
	}
	return &mockClient{vendor: v, model: model}
}

func (m *mockClient) Vendor() Vendor { return m.vendor }
func (m *mockClient) Model() string  { return m.model }

func (m *mockClient) Complete(ctx context.Context, req Request) (Response, error) {
	atomic.AddInt64(&mockCallCount, 1)

	// 429 simulation: return an APIError as if the real API rate-
	// limited us. Lets card / gold exercise the friendly-error path.
	if mode := os.Getenv("LLM_RECALL_LLM_MOCK"); mode == "429" {
		return Response{}, &APIError{
			Vendor: m.vendor,
			Status: 429,
			Body:   `{"error":{"type":"rate_limit_error","message":"mock rate limit"}}`,
		}
	}

	// Pick the right fixture by sniffing the system prompt. The card
	// system prompt starts with a fixed sentinel string we control, so
	// a substring match is safe.
	if strings.Contains(req.System, prompts.SystemCardSentinel) {
		return Response{
			Text:       MockCardFixture,
			InputToks:  EstimateTokens(req.System) + EstimateTokens(req.Prompt),
			OutputToks: EstimateTokens(MockCardFixture),
		}, nil
	}
	// Default to gold. The gold system prompt also has a sentinel; we
	// don't strictly need to verify it.
	return Response{
		Text:       MockGoldFixture,
		InputToks:  EstimateTokens(req.System) + EstimateTokens(req.Prompt),
		OutputToks: EstimateTokens(MockGoldFixture),
	}, nil
}
