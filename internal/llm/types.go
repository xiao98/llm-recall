// Package llm — W7 BYOK LLM client abstraction for `card` / `gold`.
//
// Design philosophy: keep the surface small enough that a future agent can
// audit it in one sitting. Two vendors (Anthropic, OpenAI), one Complete()
// call, no streaming, no tools. The interesting code is in PII redaction
// (redact.go), the on-disk cache (cache.go), and the HTTP shape per vendor
// (anthropic.go / openai.go).
//
// We deliberately avoid:
//   - Streaming responses (card / gold are one-shot, ≤ 1 KB output)
//   - tiktoken or any real tokenizer (estimate as len(prompt)/4; off by
//     ±20% never changes a confirm decision at our cost magnitudes)
//   - SDK dependencies (HTTP calls are 50 LOC each; an SDK would dwarf
//     them and pin a TLS / JSON dep matrix)
package llm

import (
	"context"
	"errors"
	"fmt"
)

// Vendor identifies an LLM provider. Stored as a string so config.toml
// round-trips cleanly without a custom UnmarshalText.
type Vendor string

const (
	Anthropic Vendor = "anthropic"
	OpenAI    Vendor = "openai"
)

// DefaultModel returns the per-vendor model used when neither a flag nor
// config.toml specifies one. Hardcoded; users override with --model.
//
// Choices reflect 2026-05 pricing: cheap-tier models ($0.15–$1 / MTok in,
// $0.60–$5 out) keep `gold` under $0.05 for a typical week of sessions.
func DefaultModel(v Vendor) string {
	switch v {
	case Anthropic:
		return "claude-haiku-4-5-20251001"
	case OpenAI:
		return "gpt-4o-mini"
	default:
		return ""
	}
}

// DefaultBaseURL returns the official endpoint for vendor v. Empty for
// unknown vendors so callers can fail loudly.
func DefaultBaseURL(v Vendor) string {
	switch v {
	case Anthropic:
		return "https://api.anthropic.com"
	case OpenAI:
		return "https://api.openai.com"
	}
	return ""
}

// Request is the cross-vendor input. We pass a single concatenated System
// + Prompt rather than a chat-message array because card / gold are
// one-shot single-user prompts; the chat shape would be ceremony.
type Request struct {
	System    string
	Prompt    string
	MaxTokens int
}

// Response is the cross-vendor output. InputToks / OutputToks come from
// the API response when available; otherwise zero (the caller already
// has its own len/4 estimate to fall back on).
type Response struct {
	Text       string
	InputToks  int
	OutputToks int
}

// Client is the interface implemented by anthropicClient / openaiClient
// (real) and mockClient (test / LLM_RECALL_LLM_MOCK=1). The interface
// exists so the cache layer and the card / gold commands can talk to a
// fake under tests without HTTP ceremony.
type Client interface {
	Vendor() Vendor
	Model() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// APIError is the typed error returned by all real clients on non-2xx
// responses. card / gold inspect Status to print friendly hints
// (429 → "rate limit", 401 → "check your API key", etc).
type APIError struct {
	Vendor Vendor
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s api error: status=%d body=%s", e.Vendor, e.Status, truncForErr(e.Body))
}

// IsRetryable reports whether the API error class warrants a single
// retry. 429 (rate limit) and 5xx (transient) yes; 4xx other than 429
// are user errors and should fail fast.
func (e *APIError) IsRetryable() bool {
	return e.Status == 429 || (e.Status >= 500 && e.Status < 600)
}

// IsAPIError unwraps to *APIError if applicable. Returns (nil, false)
// otherwise.
func IsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// FriendlyAPIError converts an error into a one-line human-readable
// hint for stderr. card / gold use this to avoid leaking the raw HTTP
// body in 99% of failures.
func FriendlyAPIError(err error) string {
	ae, ok := IsAPIError(err)
	if !ok {
		return err.Error()
	}
	switch {
	case ae.Status == 401 || ae.Status == 403:
		return fmt.Sprintf("auth failed (%d) from %s; check your API key", ae.Status, ae.Vendor)
	case ae.Status == 429:
		return fmt.Sprintf("rate limit (429) from %s; retry in 30-60s or lower --max-tokens", ae.Vendor)
	case ae.Status >= 500:
		return fmt.Sprintf("server error (%d) from %s; try again shortly", ae.Status, ae.Vendor)
	case ae.Status == 400:
		return fmt.Sprintf("bad request (400) from %s: %s", ae.Vendor, truncForErr(ae.Body))
	default:
		return fmt.Sprintf("%s returned %d", ae.Vendor, ae.Status)
	}
}

// truncForErr keeps stderr lines short. 200 chars covers most one-line
// JSON error bodies; longer payloads get an ellipsis.
func truncForErr(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "…"
}

// EstimateTokens approximates the token count of a prompt. Hardcoded
// 4-char-per-token heuristic — accurate to within ±20% across English /
// Chinese / mixed inputs at the magnitudes we care about. We MUST NOT
// pull tiktoken (cgo / Python ABI) or anthropic-tokenizer (pulls in the
// SDK) for what is, ultimately, a UI nicety.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	n := len(s) / 4
	if n < 1 {
		return 1
	}
	return n
}

// NewClient picks the right concrete client for vendor + key + model +
// optional baseURL override. Honours LLM_RECALL_LLM_MOCK so test and
// e2e harnesses share one factory.
//
// Empty baseURL → DefaultBaseURL(vendor). Empty model → DefaultModel.
func NewClient(vendor Vendor, key, model, baseURL string) (Client, error) {
	if mockMode() {
		return newMockClient(vendor, model), nil
	}
	if key == "" {
		return nil, fmt.Errorf("no API key for %s; set ANTHROPIC_API_KEY or OPENAI_API_KEY", vendor)
	}
	if model == "" {
		model = DefaultModel(vendor)
	}
	if baseURL == "" {
		baseURL = DefaultBaseURL(vendor)
	}
	switch vendor {
	case Anthropic:
		return &anthropicClient{apiKey: key, model: model, baseURL: baseURL}, nil
	case OpenAI:
		return &openaiClient{apiKey: key, model: model, baseURL: baseURL}, nil
	default:
		return nil, fmt.Errorf("unknown vendor %q", vendor)
	}
}
