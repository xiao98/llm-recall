// Anthropic Messages API client.
//
// Endpoint: POST {baseURL}/v1/messages
// Headers:  x-api-key: <key>
//
//	anthropic-version: 2023-06-01
//	content-type: application/json
//
// We deliberately speak the JSON wire format directly rather than pulling
// in github.com/anthropics/anthropic-sdk-go. Reasons:
//
//  1. The SDK pulls a transitive HTTP-helper dep tree we don't need.
//  2. Anthropic has changed the SDK's request shape twice in two years;
//     the wire format `messages.create` is older and more stable.
//  3. The full call (build → POST → decode) is ~80 LOC; an SDK wrapper
//     is comparable, and reviewers can audit ours in one screen.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicVersionHeader = "2023-06-01"

type anthropicClient struct {
	apiKey  string
	model   string
	baseURL string
}

func (c *anthropicClient) Vendor() Vendor { return Anthropic }
func (c *anthropicClient) Model() string  { return c.model }

// anthropicRequestBody mirrors the Messages API JSON shape. Only the
// fields card / gold actually need are present.
type anthropicRequestBody struct {
	Model     string                `json:"model"`
	System    string                `json:"system,omitempty"`
	MaxTokens int                   `json:"max_tokens"`
	Messages  []anthropicRequestMsg `json:"messages"`
}

type anthropicRequestMsg struct {
	Role    string `json:"role"` // always "user" for our one-shot prompts
	Content string `json:"content"`
}

type anthropicResponseBody struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	// Error envelope (4xx / 5xx).
	Type string `json:"type,omitempty"`
	Err  struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *anthropicClient) Complete(ctx context.Context, req Request) (Response, error) {
	if req.MaxTokens <= 0 {
		req.MaxTokens = 1024
	}
	body := anthropicRequestBody{
		Model:     c.model,
		System:    req.System,
		MaxTokens: req.MaxTokens,
		Messages: []anthropicRequestMsg{
			{Role: "user", Content: req.Prompt},
		},
	}
	return doRetried(ctx, func(ctx context.Context) (Response, error) {
		return c.callOnce(ctx, body)
	})
}

func (c *anthropicClient) callOnce(ctx context.Context, body anthropicRequestBody) (Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	url := c.baseURL + "/v1/messages"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersionHeader)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := httpClient().Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, &APIError{Vendor: Anthropic, Status: resp.StatusCode, Body: string(respBody)}
	}
	var out anthropicResponseBody
	if err := json.Unmarshal(respBody, &out); err != nil {
		return Response{}, fmt.Errorf("anthropic: decode: %w", err)
	}
	if len(out.Content) == 0 {
		return Response{}, errors.New("anthropic: empty content")
	}
	// Concatenate every text-typed block. Non-text blocks (e.g. tool_use)
	// are ignored — card / gold do not request tools.
	var text string
	for _, b := range out.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return Response{
		Text:       text,
		InputToks:  out.Usage.InputTokens,
		OutputToks: out.Usage.OutputTokens,
	}, nil
}

// doRetried executes fn once, and on a retryable APIError waits 2s and
// retries exactly once. Non-API errors and 4xx errors fail fast.
//
// Single retry is the right cadence: most 429s are minute-scale; a
// second retry would compound the wait without improving success
// probability meaningfully on consumer-tier keys. 5xx are rare and we
// prefer to return control to the user (who can re-run).
func doRetried(ctx context.Context, fn func(context.Context) (Response, error)) (Response, error) {
	resp, err := fn(ctx)
	if err == nil {
		return resp, nil
	}
	ae, ok := IsAPIError(err)
	if !ok || !ae.IsRetryable() {
		return Response{}, err
	}
	// 2s backoff. Exposed here (rather than a const at top) because it
	// is the only place the timing matters and inlining keeps the read
	// linear.
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-timer.C:
	}
	return fn(ctx)
}

// httpClient returns the package-shared HTTP client. 5s connect /
// header timeout, 60s total — long enough for the longest gold prompt
// at the 2026 model latencies (~20s p99 for haiku-class).
func httpClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 5 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}
}
