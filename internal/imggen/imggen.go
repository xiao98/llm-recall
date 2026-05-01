// Package imggen is the Go-side client for the Python imggen backend.
//
// It does exactly one thing: POST a StatsRequest JSON to the backend and
// hand back the PNG bytes. We isolate it as a package so cmd/llm-recall
// stays free of HTTP plumbing and so the unit test can mock the backend
// with httptest.NewServer.
//
// Failure modes:
//
//   - backend down / 5xx / timeout → ErrBackendUnavailable, retried up to
//     two extra times with 1s, 2s backoff. The CLI surfaces this as a
//     friendly "couldn't reach renderer" message rather than a stack trace.
//   - 4xx → wrapped error containing the response body. The body is the
//     bug: Pydantic is loud about field validation, so dumping the response
//     gives the user enough to file an issue.
package imggen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StatsRequest is the JSON contract with backend/schema.py:StatsRequest.
// Field names + tags must stay in sync; mismatches surface as a 422 from
// Pydantic.
type StatsRequest struct {
	WindowDays      int            `json:"window_days"`
	TotalSessions   int            `json:"total_sessions"`
	TotalTokens     int64          `json:"total_tokens"`
	TotalMessages   int64          `json:"total_messages"`
	TopTopics       []string       `json:"top_topics"`
	LongestSessionH float64        `json:"longest_session_hours"`
	PerSource       map[string]int `json:"per_source"`
	Watermark       bool           `json:"watermark"`
	Format          string         `json:"format"`   // "square" | "vertical"
	Template        string         `json:"template"` // "v1" | "v2" | "v3"
}

// ErrBackendUnavailable signals that the renderer is currently unreachable
// or returning 5xx. Treated as recoverable by callers (retry later, drop a
// note, exit with a friendly message). Distinct from a 4xx programming bug.
var ErrBackendUnavailable = errors.New("imggen: backend unavailable")

// retryAttempts is the total number of POSTs (initial + retries). 3 means
// we send the request once, then retry twice on 5xx / network error. The
// backoff between attempts is 1s and 2s.
const retryAttempts = 3

// Generate POSTs req as JSON to <backendURL>/v1/stats-card and returns the
// PNG bytes from the response body. The HTTP timeout is per-attempt 5s.
//
// If the renderer is unreachable on every attempt we wrap ErrBackendUnavailable
// so callers can errors.Is() against it. Use NewClient for tests.
func Generate(req StatsRequest, backendURL string) ([]byte, error) {
	c := NewClient(backendURL, 5*time.Second)
	return c.Generate(req)
}

// Client is the configurable form. cmd/llm-recall calls Generate; tests
// build a Client directly so they can dial httptest.NewServer.
type Client struct {
	BackendURL string
	HTTP       *http.Client
}

func NewClient(backendURL string, timeout time.Duration) *Client {
	return &Client{
		BackendURL: backendURL,
		HTTP:       &http.Client{Timeout: timeout},
	}
}

func (c *Client) Generate(req StatsRequest) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("imggen: marshal request: %w", err)
	}
	url := strings.TrimRight(c.BackendURL, "/") + "/v1/stats-card"

	var lastErr error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s after first failure, 2s after
			// second. Avoids hammering an OOM'd backend.
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		png, err := c.postOnce(url, body)
		if err == nil {
			return png, nil
		}
		lastErr = err
		if !errors.Is(err, ErrBackendUnavailable) {
			// 4xx or marshal-level — retrying won't help.
			return nil, err
		}
	}
	return nil, fmt.Errorf("imggen: backend unreachable after %d attempts: %w",
		retryAttempts, lastErr)
}

// postOnce sends a single attempt. Returns ErrBackendUnavailable for
// network errors / 5xx / timeouts, a body-bearing error for 4xx, or the
// PNG bytes on success.
func (c *Client) postOnce(url string, body []byte) ([]byte, error) {
	resp, err := c.HTTP.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackendUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == 200:
		png, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("imggen: read body: %w", err)
		}
		return png, nil
	case resp.StatusCode >= 500:
		// Read up to a small budget so a misbehaving backend can't
		// hold us indefinitely; the body is debug info, not load.
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%w: status %d: %s", ErrBackendUnavailable,
			resp.StatusCode, strings.TrimSpace(string(preview)))
	default:
		// 4xx — surface body so the user sees the validation error.
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("imggen: status %d: %s", resp.StatusCode,
			strings.TrimSpace(string(preview)))
	}
}
