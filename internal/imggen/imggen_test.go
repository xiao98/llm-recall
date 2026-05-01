package imggen

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// minimalReq returns a StatsRequest the renderer would accept. We don't
// actually run Pillow here — the mock returns canned PNG bytes — so we only
// need fields the JSON contract requires.
func minimalReq() StatsRequest {
	return StatsRequest{
		WindowDays:    30,
		TotalSessions: 7,
		TotalTokens:   0,
		TotalMessages: 42,
		TopTopics:     []string{"go", "test"},
		PerSource:     map[string]int{"claude": 7},
		Watermark:     true,
		Format:        "square",
		Template:      "v1",
	}
}

// TestGenerateSuccess: happy path — mock backend returns PNG-magic bytes,
// imggen passes them through untouched.
func TestGenerateSuccess(t *testing.T) {
	want := []byte("\x89PNG\r\n\x1a\n-fake-bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/stats-card" {
			t.Errorf("path = %q, want /v1/stats-card", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
		_, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 2*time.Second)
	got, err := c.Generate(minimalReq())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("payload mismatch: got %q, want %q", got, want)
	}
}

// TestGenerateRetriesOn5xx: the backend 500s twice, then succeeds. Verify
// imggen made exactly 3 attempts and returned the third response.
func TestGenerateRetriesOn5xx(t *testing.T) {
	var calls int32
	want := []byte("ok-png")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			http.Error(w, "boom", 500)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	// Use a tiny client timeout — even the retries (1s + 2s sleeps) keep the
	// test under ~3.5s.
	c := NewClient(srv.URL, 1*time.Second)
	got, err := c.Generate(minimalReq())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got, want := atomic.LoadInt32(&calls), int32(3); got != want {
		t.Errorf("calls = %d, want %d", got, want)
	}
	if string(got) != "ok-png" {
		t.Errorf("payload mismatch")
	}
}

// TestGenerateExhaustsRetries: backend always 500s, expect ErrBackendUnavailable
// after exactly 3 attempts.
func TestGenerateExhaustsRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "still broken", 503)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 1*time.Second)
	_, err := c.Generate(minimalReq())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("expected ErrBackendUnavailable, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

// TestGenerate4xxNoRetry: 4xx is a programming bug — we should NOT retry,
// and the error should contain the response body.
func TestGenerate4xxNoRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "field required: total_sessions", 422)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, 1*time.Second)
	_, err := c.Generate(minimalReq())
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrBackendUnavailable) {
		t.Errorf("4xx should NOT be ErrBackendUnavailable, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("4xx must not retry, calls = %d", got)
	}
	if msg := err.Error(); !contains(msg, "field required") {
		t.Errorf("error should include backend body, got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
