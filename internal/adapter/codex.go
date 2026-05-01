package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Codex implements SessionAdapter against `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`.
type Codex struct {
	// Root overrides the default sessions dir when non-empty (tests).
	Root string
}

// NewCodex builds a Codex adapter rooted at $CODEX_HOME/sessions or ~/.codex/sessions.
func NewCodex() *Codex { return &Codex{} }

func (c *Codex) Name() string { return "codex" }

func (c *Codex) sessionsRoot() (string, error) {
	if c.Root != "" {
		return c.Root, nil
	}
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return filepath.Join(h, "sessions"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "sessions"), nil
}

// Discover walks YYYY/MM/DD subtrees and emits a Session per rollout-*.jsonl.
// Per-file failures don't abort the walk.
//
// NOTE: We deliberately do not scan ~/.codex/archived_sessions for W2 — task
// doc §2 leaves it for W3.
func (c *Codex) Discover(ctx context.Context) ([]Session, error) {
	root, err := c.sessionsRoot()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("codex sessions root is not a directory: %s", root)
	}

	var out []Session
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if werr != nil {
			fmt.Fprintf(os.Stderr, "warn: codex walk %s: %v\n", path, werr)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || filepath.Ext(name) != ".jsonl" {
			return nil
		}
		s, _, perr := scanCodexSessionFile(path, false)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "warn: codex parse %s: %v\n", path, perr)
			return nil
		}
		out = append(out, s)
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return out, err
	}
	return out, nil
}

func (c *Codex) Read(s Session) ([]Message, error) {
	return nil, errors.New("codex adapter: Read not implemented in W2")
}

// ResumeCommand returns the canonical `codex resume <id>` shape — the codex
// CLI exposes a `resume` subcommand that accepts a session UUID positional.
// W3 §0.4 audit confirmed (codex 0.128.0).
func (c *Codex) ResumeCommand(s Session) ([]string, string, ResumeMode, error) {
	if s.ID == "" {
		return nil, s.CWD, ResumeUnsupported, fmt.Errorf("codex: no session id")
	}
	return []string{"codex", "resume", s.ID}, s.CWD, ResumeDirect, nil
}

// ParseFile parses just enough of a rollout file to fill (id, cwd, started,
// title). The body field is left empty — callers that need it must use
// ParseFileFull, which streams the full file.
func (c *Codex) ParseFile(path string) (Session, error) {
	s, _, err := scanCodexSessionFile(path, false)
	return s, err
}

// ParseFileFull is the FileBodyParser path: it scans the entire rollout and
// returns the concatenated user message body in addition to the session
// metadata. Used by the TUI on cache miss.
func (c *Codex) ParseFileFull(path string) (Session, string, error) {
	return scanCodexSessionFile(path, true)
}

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexSessionMeta struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
}

type codexResponseItem struct {
	Type    string                 `json:"type"`
	Role    string                 `json:"role"`
	Content []codexResponseContent `json:"content"`
}

type codexResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// rolloutUUIDRe matches the trailing `<uuid>` of `rollout-...-<uuid>.jsonl`.
// Used as a fallback session id when session_meta is missing.
var rolloutUUIDRe = regexp.MustCompile(`-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl$`)

// scanCodexSessionFile is the shared scanner for both ParseFile (title-only,
// fast-exit) and ParseFileFull (run to EOF, accumulate body). Lines can be
// very large (reasoning payloads), so the scanner buffer is 8MB like Claude's.
//
// Body collection rules (only when collectBody=true):
//   - only `response_item` rows where payload.type=="message" && role=="user"
//   - skip codex pseudo-user messages (env_context / [Imported from Claude])
//   - join the user texts of all `input_text` parts of one message with " "
//   - separate messages with "\n---\n"
//   - clamp to 65536 bytes via SafeUTF8Truncate at the end
const codexBodyMaxBytes = 65536

func scanCodexSessionFile(path string, collectBody bool) (Session, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Session{}, "", err
	}

	f, err := os.Open(path)
	if err != nil {
		return Session{}, "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)

	var (
		id      string
		cwd     string
		started time.Time
		title   string
		body    strings.Builder
	)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var hdr codexLine
		if err := json.Unmarshal(line, &hdr); err != nil {
			continue
		}
		switch hdr.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(hdr.Payload, &meta); err == nil {
				if id == "" && meta.ID != "" {
					id = meta.ID
				}
				if cwd == "" && meta.CWD != "" {
					cwd = meta.CWD
				}
				if started.IsZero() {
					ts := meta.Timestamp
					if ts == "" {
						ts = hdr.Timestamp
					}
					if t, err := ParseTime(ts); err == nil {
						started = t
					}
				}
			}
		case "response_item":
			var item codexResponseItem
			if err := json.Unmarshal(hdr.Payload, &item); err != nil {
				continue
			}
			// We only ever care about real user messages here. function_call,
			// function_call_output, reasoning, local_shell_call, tool_search_call
			// all parse with item.Type != "message" and fall through.
			if item.Type != "message" || item.Role != "user" {
				continue
			}
			var b strings.Builder
			for _, c := range item.Content {
				if c.Type == "input_text" && c.Text != "" {
					if b.Len() > 0 {
						b.WriteByte(' ')
					}
					b.WriteString(c.Text)
				}
			}
			text := b.String()
			if text == "" {
				continue
			}
			// Skip CLI-synthesised pseudo-user payloads (env_context / import marker).
			if IsCodexInjectedUserText(text) {
				continue
			}
			cleaned := CleanTitle(text)
			if title == "" && cleaned != "" {
				title = cleaned
			}
			if collectBody {
				if body.Len() > 0 {
					body.WriteString("\n---\n")
				}
				body.WriteString(text)
			}
		}
		// Fast-exit only when we don't need a body.
		if !collectBody && id != "" && cwd != "" && !started.IsZero() && title != "" {
			break
		}
	}
	if err := sc.Err(); err != nil {
		// Tolerate read errors as long as we got an id.
		if id == "" {
			// Fall through to filename fallback below.
		}
	}

	if id == "" {
		if m := rolloutUUIDRe.FindStringSubmatch(filepath.Base(path)); len(m) == 2 {
			id = m[1]
			fmt.Fprintf(os.Stderr, "warn: codex %s: session_meta missing, using filename uuid\n", filepath.Base(path))
		} else {
			return Session{}, "", fmt.Errorf("no session id in %s", filepath.Base(path))
		}
	}

	finalBody := ""
	if collectBody {
		finalBody = SafeUTF8Truncate(body.String(), codexBodyMaxBytes)
	}

	return Session{
		Source:    "codex",
		ID:        id,
		CWD:       cwd,
		StartedAt: started,
		UpdatedAt: fi.ModTime(),
		FilePath:  path,
		Title:     title,
	}, finalBody, nil
}
