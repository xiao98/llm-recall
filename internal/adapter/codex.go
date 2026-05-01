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
		s, perr := parseCodexSessionFile(path)
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

// ResumeCommand returns the canonical `codex resume <id>` shape. Codex CLI's
// actual resume flag varies across versions, but the launcher (W3) will
// finalise; for now the recipe matches `codex --help`'s `resume` subcommand.
func (c *Codex) ResumeCommand(s Session) ([]string, string) {
	return []string{"codex", "resume", s.ID}, s.CWD
}

func (c *Codex) ParseFile(path string) (Session, error) {
	return parseCodexSessionFile(path)
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

// parseCodexSessionFile streams a codex rollout jsonl and returns once it has
// (sessionId, cwd, startedAt, title). Lines can be very large (reasoning
// payloads), so the scanner buffer is 8MB like Claude's.
func parseCodexSessionFile(path string) (Session, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Session{}, err
	}

	f, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)

	var (
		id      string
		cwd     string
		started time.Time
		title   string
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
					if t, err := parseCodexTime(ts); err == nil {
						started = t
					}
				}
			}
		case "response_item":
			if title != "" {
				continue
			}
			var item codexResponseItem
			if err := json.Unmarshal(hdr.Payload, &item); err != nil {
				continue
			}
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
			t := CleanTitle(b.String())
			if t != "" {
				title = t
			}
		}
		if id != "" && cwd != "" && !started.IsZero() && title != "" {
			break
		}
	}
	if err := sc.Err(); err != nil {
		// Scan errors are tolerable as long as we got an id.
		if id == "" {
			// Fall through to filename fallback below.
		}
	}

	if id == "" {
		if m := rolloutUUIDRe.FindStringSubmatch(filepath.Base(path)); len(m) == 2 {
			id = m[1]
			fmt.Fprintf(os.Stderr, "warn: codex %s: session_meta missing, using filename uuid\n", filepath.Base(path))
		} else {
			return Session{}, fmt.Errorf("no session id in %s", filepath.Base(path))
		}
	}

	return Session{
		Source:    "codex",
		ID:        id,
		CWD:       cwd,
		StartedAt: started,
		UpdatedAt: fi.ModTime(),
		FilePath:  path,
		Title:     title,
	}, nil
}

// parseCodexTime accepts both RFC3339Nano and the .NET-style 7-digit
// fractional seconds we see in rollouts on Windows ("2026-04-25T10:55:00.0000000Z").
func parseCodexTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}
