// Package adapter defines the SessionAdapter contract that every vendor
// (Claude / Codex / Gemini / ...) implements so that index/search/launcher
// can stay vendor-agnostic.
package adapter

import (
	"context"
	"time"
)

// SessionAdapter is the per-vendor parser interface.
//
// Discover walks the vendor's on-disk session store and returns Session
// metadata only (cheap). Read pulls the message stream for one Session
// (lazy / on-demand). ResumeCommand returns the exec recipe that re-enters
// that session in the vendor's own CLI.
type SessionAdapter interface {
	Name() string
	Discover(ctx context.Context) ([]Session, error)
	Read(s Session) ([]Message, error)
	ResumeCommand(s Session) (argv []string, cwd string)
}

// Session is the cross-vendor metadata record. FilePath is required: the
// SQLite cache (W2) keys invalidation off the file's mtime, so losing the
// path means losing incremental scan.
type Session struct {
	Source    string    // adapter name, e.g. "claude"
	ID        string    // vendor session id (usually file stem)
	CWD       string    // working dir captured by the vendor CLI
	StartedAt time.Time // first record timestamp
	UpdatedAt time.Time // file mtime
	FilePath  string    // absolute path to the source jsonl
	Title     string    // first user-typed message, used as list title
}

// Message is the cross-vendor message record. Text is already cleaned of
// tool-call envelopes — adapters are responsible for filtering.
type Message struct {
	Role string // "user" | "assistant" | "tool"
	Text string
	Time time.Time
}
