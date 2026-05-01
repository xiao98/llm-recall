// Disk-backed LLM response cache. 7-day TTL. SHA-256 of (model+system
// +prompt) → JSON file. Skips network on hit.
//
// Why a file cache and not SQLite: the response payload is short (≤ 1
// KB JSON) and read-mostly; a flat directory of small JSON files is
// dead simple to inspect (`cat`), zip up for support, and rm -rf to
// reset. SQLite would force schema migration ceremony for V2.
//
// Path: $XDG_CACHE_HOME/llm-recall/llm-cache/<sha>.json on Linux,
// ~/Library/Caches/llm-recall/llm-cache/ on macOS,
// %LocalAppData%\llm-recall\llm-cache\ on Windows. Resolved via
// os.UserCacheDir() so we don't reinvent per-platform conventions.
package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CacheTTL is the on-disk record validity window. Bodies older than
// this are treated as a miss; the file is left in place (the writer
// will clobber it on the new write).
const CacheTTL = 7 * 24 * time.Hour

// cacheRecord is the on-disk shape. Keep it stable: changing field
// names would invalidate every cached entry on upgrade. Add fields
// only with `omitempty` so missing-field reads still decode.
type cacheRecord struct {
	CachedAt     time.Time `json:"cached_at"`
	Model        string    `json:"model"`
	ResponseText string    `json:"response_text"`
	InputToks    int       `json:"input_toks"`
	OutputToks   int       `json:"output_toks"`
}

// cachePathOverride is a test hook. Set via SetCachePathForTest.
var cachePathOverride string

// SetCachePathForTest redirects cacheRoot() to the supplied directory.
// Pass "" to clear. Tests pair with t.Cleanup(func(){ …("") }).
func SetCachePathForTest(p string) { cachePathOverride = p }

// cacheRoot returns the absolute directory that holds <sha>.json files.
// MkdirAll'd lazily on Put; never returned as an error to the caller
// in prod paths (cache failures are non-fatal — we just bypass the
// cache).
func cacheRoot() string {
	if cachePathOverride != "" {
		return cachePathOverride
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		// Fallback: project-relative. Better than aborting. Users on
		// exotic platforms (no $HOME, container, etc.) still get a
		// working binary, just with cache that lives next to the cwd.
		base = filepath.Join(".", "llm-recall-cache")
	}
	return filepath.Join(base, "llm-recall", "llm-cache")
}

// CacheRoot is the exported wrapper for tests / debug commands.
func CacheRoot() string { return cacheRoot() }

// CacheKey computes the SHA-256 hex of (model | NUL | system | NUL |
// prompt). The NUL separators prevent collisions where an empty
// system + nonempty prompt would hash the same as a prefix-shifted
// pair.
func CacheKey(model, system, prompt string) string {
	h := sha256.New()
	h.Write([]byte(model))
	h.Write([]byte{0})
	h.Write([]byte(system))
	h.Write([]byte{0})
	h.Write([]byte(prompt))
	return hex.EncodeToString(h.Sum(nil))
}

// CacheGet returns (response, true) on a fresh hit, (zero, false) on
// miss / stale / read-error. Read errors are intentionally collapsed to
// "miss" — the worst-case is one extra LLM round trip, never a crash.
func CacheGet(key string) (Response, bool) {
	path := filepath.Join(cacheRoot(), key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Response{}, false
	}
	var rec cacheRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return Response{}, false
	}
	if time.Since(rec.CachedAt) > CacheTTL {
		return Response{}, false
	}
	return Response{
		Text:       rec.ResponseText,
		InputToks:  rec.InputToks,
		OutputToks: rec.OutputToks,
	}, true
}

// CachePut writes a record at the given key. Returns an error so tests
// can assert on disk semantics; production callers log-and-ignore.
func CachePut(key, model string, resp Response) error {
	root := cacheRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	rec := cacheRecord{
		CachedAt:     time.Now().UTC(),
		Model:        model,
		ResponseText: resp.Text,
		InputToks:    resp.InputToks,
		OutputToks:   resp.OutputToks,
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(root, key+".json")
	// 0600 — cached LLM responses are derived from your prompts; assume
	// they may carry residual sensitive context the user didn't redact.
	return os.WriteFile(path, data, 0o600)
}

// CachePutWithTime is the test-only writer that lets us forge a stale
// record (CachedAt in the past) so the TTL branch in CacheGet can be
// exercised without sleeping for a week. Production code calls CachePut.
func CachePutWithTime(key, model string, resp Response, when time.Time) error {
	root := cacheRoot()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	rec := cacheRecord{
		CachedAt:     when,
		Model:        model,
		ResponseText: resp.Text,
		InputToks:    resp.InputToks,
		OutputToks:   resp.OutputToks,
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, key+".json"), data, 0o600)
}
