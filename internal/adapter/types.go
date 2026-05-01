// Package adapter defines the SessionAdapter contract that every vendor
// (Claude / Codex / Gemini / ...) implements so that index/search/launcher
// can stay vendor-agnostic.
package adapter

import (
	"context"
	"time"
)

// ResumeMode reports how an adapter wants the launcher to drive its CLI when
// a session is selected. Each vendor's `--help` output has been audited (W3
// §0.4); the mode reflects what we observed, not what the docs claim.
type ResumeMode int

const (
	// ResumeDirect: the returned argv is a complete exec recipe. The launcher
	// chdirs into cwd, runs argv, and the CLI re-enters the picked session
	// with no further user input. Claude (`claude --resume <uuid>`) and codex
	// (`codex resume <uuid>`) match this.
	ResumeDirect ResumeMode = iota

	// ResumeInteractive: the returned argv launches the CLI but does NOT
	// auto-resume. The launcher prints a one-line hint telling the user the
	// in-app slash command to use. Gemini falls here: its `--resume` flag
	// accepts only "latest" / an integer index, never a session UUID, so we
	// can't construct a Direct recipe from a stored sessionId.
	ResumeInteractive

	// ResumeUnsupported: the vendor's CLI has no resume mechanism we can
	// drive. The launcher prints the sessionId and exits without exec.
	ResumeUnsupported
)

// SessionAdapter is the per-vendor parser interface.
//
// Discover walks the vendor's on-disk session store and returns Session
// metadata only (cheap). Read pulls the message stream for one Session
// (lazy / on-demand). ResumeCommand returns the exec recipe + mode that
// re-enters that session in the vendor's own CLI.
type SessionAdapter interface {
	Name() string
	Discover(ctx context.Context) ([]Session, error)
	Read(s Session) ([]Message, error)
	ResumeCommand(s Session) (argv []string, cwd string, mode ResumeMode, err error)
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
	Body      string    // concatenated user-message text (W3, may be "")
}

// Message is the cross-vendor message record. Text is already cleaned of
// tool-call envelopes — adapters are responsible for filtering.
type Message struct {
	Role string // "user" | "assistant" | "tool"
	Text string
	Time time.Time
}
