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
	"strings"
	"time"
)

// Claude implements SessionAdapter against `~/.claude/projects/*/*.jsonl`.
type Claude struct {
	// Root overrides the default `~/.claude/projects` when non-empty (tests).
	Root string
}

// NewClaude builds a Claude adapter rooted at the user's home `.claude/projects`.
func NewClaude() *Claude { return &Claude{} }

func (c *Claude) Name() string { return "claude" }

func (c *Claude) projectsRoot() (string, error) {
	if c.Root != "" {
		return c.Root, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Discover lists every <project-dir>/<sessionId>.jsonl directly under the
// projects root and emits a Session per file. We do NOT recurse below
// <project-dir>: deeper folders (e.g. `<sid>/subagents/agent-*.jsonl`)
// are sub-agent transcripts that pollute the top-level history view.
// Per-file failures do not abort the walk.
func (c *Claude) Discover(ctx context.Context) ([]Session, error) {
	root, err := c.projectsRoot()
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
		return nil, fmt.Errorf("claude projects root is not a directory: %s", root)
	}

	projectDirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var out []Session
	for _, pd := range projectDirs {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		if !pd.IsDir() {
			continue
		}
		projectPath := filepath.Join(root, pd.Name())
		entries, err := os.ReadDir(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: claude read %s: %v\n", projectPath, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if filepath.Ext(e.Name()) != ".jsonl" {
				continue
			}
			path := filepath.Join(projectPath, e.Name())
			s, _, err := scanClaudeSessionFile(path, false)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: claude parse %s: %v\n", path, err)
				continue
			}
			out = append(out, s)
		}
	}
	return out, nil
}

// Read is intentionally minimal for W1; full message extraction lands in W3
// when the TUI preview pane needs it. Returning an error keeps callers honest
// about not relying on it yet.
func (c *Claude) Read(s Session) ([]Message, error) {
	return nil, errors.New("claude adapter: Read not implemented in W1")
}

// ResumeCommand returns the canonical `claude --resume <id>` invocation. The
// claude CLI accepts a session UUID directly via -r/--resume (audited W3
// §0.4 against claude --help); ResumeDirect mode means the launcher just
// chdirs to cwd and exec's.
func (c *Claude) ResumeCommand(s Session) ([]string, string, ResumeMode, error) {
	if s.ID == "" {
		return nil, s.CWD, ResumeUnsupported, fmt.Errorf("claude: no session id")
	}
	return []string{"claude", "--resume", s.ID}, s.CWD, ResumeDirect, nil
}

// ParseFile is the FileParser capability — single-file parse for the cache
// miss path that only needs metadata (id/cwd/title/started). Discover()'s
// loop body uses the same helper.
func (c *Claude) ParseFile(path string) (Session, error) {
	s, _, err := scanClaudeSessionFile(path, false)
	return s, err
}

// ParseFileFull is the FileBodyParser path: scan the full file and return
// (session, body). The TUI uses this on cache miss / forced rescan.
func (c *Claude) ParseFileFull(path string) (Session, string, error) {
	return scanClaudeSessionFile(path, true)
}

// claudeRecord is a permissive view of one jsonl row. Only the fields we
// actually inspect are decoded; the rest is swallowed.
type claudeRecord struct {
	Type      string         `json:"type"`
	SessionID string         `json:"sessionId"`
	CWD       string         `json:"cwd"`
	Timestamp string         `json:"timestamp"`
	Message   *claudeMessage `json:"message"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

const claudeBodyMaxBytes = 65536

// scanClaudeSessionFile is the unified scanner. When collectBody=false it
// fast-exits as soon as (id, cwd, title, started) are known, matching W1's
// O(first-few-rows) cost. When collectBody=true it runs to EOF, joining every
// real (non-injected) user message with "\n---\n" and clamping at 64KB.
//
// Single helper guards against the Title-extraction logic and Body-extraction
// logic drifting apart.
func scanClaudeSessionFile(path string, collectBody bool) (Session, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Session{}, "", err
	}

	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if id == "" {
		return Session{}, "", fmt.Errorf("empty session id from filename")
	}

	f, err := os.Open(path)
	if err != nil {
		return Session{}, "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Some Claude jsonl rows embed base64 images and blow past the 64KB default.
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)

	var (
		cwd     string
		title   string
		started time.Time
		body    strings.Builder
	)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec claudeRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			// Tolerate single-line corruption.
			continue
		}
		if cwd == "" && rec.CWD != "" {
			cwd = rec.CWD
		}
		if started.IsZero() && rec.Timestamp != "" {
			if t, err := ParseTime(rec.Timestamp); err == nil {
				started = t
			}
		}
		if rec.Type == "user" && rec.Message != nil {
			text, ok := extractUserText(rec.Message.Content)
			if ok {
				if title == "" {
					title = CleanTitle(text)
				}
				if collectBody {
					if body.Len() > 0 {
						body.WriteString("\n---\n")
					}
					body.WriteString(text)
				}
			}
		}
		// Fast-exit only when body is not needed.
		if !collectBody && cwd != "" && title != "" && !started.IsZero() {
			break
		}
	}
	if err := sc.Err(); err != nil {
		// Even on read error, surface what we have if it's enough.
		if cwd == "" {
			return Session{}, "", err
		}
	}

	if cwd == "" {
		return Session{}, "", fmt.Errorf("no cwd field found in %s", filepath.Base(path))
	}

	finalBody := ""
	if collectBody {
		finalBody = SafeUTF8Truncate(body.String(), claudeBodyMaxBytes)
	}

	return Session{
		Source:    "claude",
		ID:        id,
		CWD:       cwd,
		StartedAt: started,
		UpdatedAt: fi.ModTime(),
		FilePath:  path,
		Title:     title,
	}, finalBody, nil
}

// extractUserText pulls the displayable user text from a Claude `message.content`
// payload, which may be either a JSON string or an array of typed parts.
// Returns ok=false when the content is a CLI-injected pseudo-user message
// (system reminders, slash-command echoes, command stdout).
func extractUserText(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	// Case 1: plain string.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", false
		}
		s = strings.TrimSpace(s)
		if isInjectedUserText(s) {
			return "", false
		}
		if s == "" {
			return "", false
		}
		return s, true
	}
	// Case 2: array of parts. Take the first {"type":"text","text":...}.
	if raw[0] == '[' {
		var parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &parts); err != nil {
			return "", false
		}
		for _, p := range parts {
			if p.Type != "text" {
				continue
			}
			t := strings.TrimSpace(p.Text)
			if t == "" || isInjectedUserText(t) {
				continue
			}
			return t, true
		}
	}
	return "", false
}

// injectedTags marks payloads that the Claude CLI synthesizes as if they
// came from the user (system reminders, /commands, command output). They
// are useless as a session title.
var injectedTags = []string{
	"system-reminder",
	"local-command-",
	"command-name",
	"command-message",
	"command-stdout",
}

func isInjectedUserText(s string) bool {
	if !strings.HasPrefix(s, "<") {
		return false
	}
	for _, tag := range injectedTags {
		if strings.Contains(s, tag) {
			return true
		}
	}
	return false
}
