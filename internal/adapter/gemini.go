package adapter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Gemini implements SessionAdapter against `~/.gemini/tmp/<project>/chats/session-*.{json,jsonl}`.
// Two formats coexist on disk after Gemini CLI's 2026-04 archive change:
// legacy single-object `.json` and current line-delimited `.jsonl`. We dispatch
// strictly by extension — both files start with `{`, so heuristics are unsafe.
type Gemini struct {
	// Root overrides the default tmp dir when non-empty (tests).
	Root string
}

func NewGemini() *Gemini { return &Gemini{} }

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) tmpRoot() (string, error) {
	if g.Root != "" {
		return g.Root, nil
	}
	if h := os.Getenv("GEMINI_CLI_HOME"); h != "" {
		return filepath.Join(h, "tmp"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "tmp"), nil
}

// Discover walks <root>/<projectShortid>/chats/ and emits a Session per
// session-*.{json,jsonl} file. Checkpoint files (`checkpoint-*.{json,jsonl}`)
// and rewind-snapshot files inside `<sessionId>/<short>.jsonl` subdirs are
// deliberately skipped — they're not first-class sessions.
func (g *Gemini) Discover(ctx context.Context) ([]Session, error) {
	root, err := g.tmpRoot()
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
		return nil, fmt.Errorf("gemini tmp root is not a directory: %s", root)
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
		projectDir := filepath.Join(root, pd.Name())
		chatsDir := filepath.Join(projectDir, "chats")
		entries, err := os.ReadDir(chatsDir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				fmt.Fprintf(os.Stderr, "warn: gemini read %s: %v\n", chatsDir, err)
			}
			continue
		}
		// CWD fallback: try sibling metadata files inside the project dir.
		cwdHint := geminiProjectCWD(projectDir)

		for _, e := range entries {
			if e.IsDir() {
				// Rewind-checkpoint subdirs (`<sessionId>/<short>.jsonl`) live here.
				continue
			}
			name := e.Name()
			if !strings.HasPrefix(name, "session-") {
				continue
			}
			ext := filepath.Ext(name)
			if ext != ".json" && ext != ".jsonl" {
				continue
			}
			path := filepath.Join(chatsDir, name)
			s, _, perr := scanGeminiSessionFile(path, ext, pd.Name(), cwdHint, false)
			if perr != nil {
				fmt.Fprintf(os.Stderr, "warn: gemini parse %s: %v\n", path, perr)
				continue
			}
			out = append(out, s)
		}
	}
	return out, nil
}

func (g *Gemini) Read(s Session) ([]Message, error) {
	return nil, errors.New("gemini adapter: Read not implemented in W2")
}

// ResumeCommand: gemini's `--resume` flag accepts only "latest" or an integer
// session index (W3 §0.4 audit of `gemini --help`). It does NOT accept the
// session UUID we have stored, so we cannot construct a Direct recipe.
// Falling back to ResumeInteractive: launch `gemini` in the original cwd and
// hint the user to use `gemini --list-sessions` + index, or the in-app
// `/chat resume <tag>` command, to re-enter the picked session.
func (g *Gemini) ResumeCommand(s Session) ([]string, string, ResumeMode, error) {
	return []string{"gemini"}, s.CWD, ResumeInteractive, nil
}

// ParseFile derives projectShortid + cwdHint from the path layout
// (<root>/<project>/chats/<file>) and parses the file. Used by the cache
// miss path that needs only metadata.
func (g *Gemini) ParseFile(path string) (Session, error) {
	chatsDir := filepath.Dir(path)       // <root>/<project>/chats
	projectDir := filepath.Dir(chatsDir) // <root>/<project>
	projectShortid := filepath.Base(projectDir)
	cwdHint := geminiProjectCWD(projectDir)
	ext := filepath.Ext(path)
	s, _, err := scanGeminiSessionFile(path, ext, projectShortid, cwdHint, false)
	return s, err
}

// ParseFileFull is the FileBodyParser path: scan-all and extract the
// concatenated user-message body alongside the metadata.
func (g *Gemini) ParseFileFull(path string) (Session, string, error) {
	chatsDir := filepath.Dir(path)
	projectDir := filepath.Dir(chatsDir)
	projectShortid := filepath.Base(projectDir)
	cwdHint := geminiProjectCWD(projectDir)
	ext := filepath.Ext(path)
	return scanGeminiSessionFile(path, ext, projectShortid, cwdHint, true)
}

// geminiProjectCWDFile is one of the JSON shapes we have observed gemini
// writing into the project shortid dir. Either `rootDir` is the absolute path
// of the workspace, or `directories` lists one or more workspace roots; the
// first non-empty value wins.
type geminiProjectCWDFile struct {
	RootDir     string   `json:"rootDir"`
	Directories []string `json:"directories"`
	// Older builds may use a different key name; try a few before giving up.
	Cwd         string `json:"cwd"`
	Workspace   string `json:"workspace"`
	ProjectRoot string `json:"projectRoot"`
	Root        string `json:"root"`
}

// geminiProjectCWD looks for a CWD hint in the project shortid dir, returning
// the first abs path it can resolve. Order (per DEVDOC §3 P0-1):
//  1. metadata.json   { rootDir | directories | ... }
//  2. workspace.json  same schema
//  3. .project_root   single-line plain-text absolute path
//  4. ""              (caller prepends `<gemini:xxxxxxxx>` title prefix)
//
// Read failures and parse failures are silent: each fallback is tried in
// turn, and a missing/invalid file is just "not this one, next".
func geminiProjectCWD(projectDir string) string {
	for _, name := range []string{"metadata.json", "workspace.json"} {
		path := filepath.Join(projectDir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var f geminiProjectCWDFile
		if err := json.Unmarshal(b, &f); err != nil {
			continue
		}
		if c := strings.TrimSpace(f.RootDir); c != "" {
			return c
		}
		for _, d := range f.Directories {
			if c := strings.TrimSpace(d); c != "" {
				return c
			}
		}
		// Older / alternate keys — accept whichever shows up first.
		for _, c := range []string{f.Cwd, f.Workspace, f.ProjectRoot, f.Root} {
			if c = strings.TrimSpace(c); c != "" {
				return c
			}
		}
	}
	// Plain-text fallback used by recent gemini builds on this machine.
	if b, err := os.ReadFile(filepath.Join(projectDir, ".project_root")); err == nil {
		s := strings.TrimSpace(string(b))
		if s != "" {
			return s
		}
	}
	return ""
}

// geminiTitlePrefix tags a title with the project shortid prefix when CWD is
// unknown, so users can still tell sessions from different projects apart in
// `ls` output.
func geminiTitlePrefix(projectShortid string) string {
	short := projectShortid
	if len(short) > 8 {
		short = short[:8]
	}
	return "<gemini:" + short + "> "
}

const geminiBodyMaxBytes = 65536

// scanGeminiSessionFile is the unified entry that dispatches by extension
// and threads `collectBody` into both the .json and .jsonl parsers. Returns
// (session, body, err). When collectBody is false, body is "".
func scanGeminiSessionFile(path, ext, projectShortid, cwdHint string, collectBody bool) (Session, string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Session{}, "", err
	}

	var (
		sessionID string
		startedAt time.Time
		title     string
		body      string
	)
	switch ext {
	case ".json":
		sessionID, startedAt, title, body, err = scanGeminiFormatA(path, collectBody)
	case ".jsonl":
		sessionID, startedAt, title, body, err = scanGeminiFormatB(path, collectBody)
	default:
		return Session{}, "", fmt.Errorf("unexpected gemini ext %q", ext)
	}
	if err != nil {
		return Session{}, "", err
	}
	if sessionID == "" {
		return Session{}, "", fmt.Errorf("no sessionId in %s", filepath.Base(path))
	}

	cwd := cwdHint
	cleanTitle := CleanTitle(title)
	if cwd == "" {
		cleanTitle = geminiTitlePrefix(projectShortid) + cleanTitle
	}

	if collectBody {
		body = SafeUTF8Truncate(body, geminiBodyMaxBytes)
	}

	return Session{
		Source:    "gemini",
		ID:        sessionID,
		CWD:       cwd,
		StartedAt: startedAt,
		UpdatedAt: fi.ModTime(),
		FilePath:  path,
		Title:     cleanTitle,
	}, body, nil
}

// geminiFormatAEnvelope holds the top-level keys we care about. `messages`
// stays raw so we only unmarshal the entries we need — full decoding of
// every assistant turn (with multi-MB `thoughts` arrays) is wasteful.
type geminiFormatAEnvelope struct {
	SessionID   string            `json:"sessionId"`
	StartTime   string            `json:"startTime"`
	LastUpdated string            `json:"lastUpdated"`
	Messages    []json.RawMessage `json:"messages"`
}

type geminiFormatAMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"` // Format A: content is a plain string.
}

func scanGeminiFormatA(path string, collectBody bool) (string, time.Time, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", time.Time{}, "", "", err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.UseNumber()

	var env geminiFormatAEnvelope
	if err := dec.Decode(&env); err != nil && err != io.EOF {
		return "", time.Time{}, "", "", err
	}
	var started time.Time
	if env.StartTime != "" {
		if t, err := ParseTime(env.StartTime); err == nil {
			started = t
		}
	}
	var (
		title string
		body  strings.Builder
	)
	for _, raw := range env.Messages {
		var m geminiFormatAMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if m.Type != "user" {
			continue
		}
		if m.Content == "" {
			continue
		}
		if title == "" {
			title = m.Content
		}
		if collectBody {
			if body.Len() > 0 {
				body.WriteString("\n---\n")
			}
			body.WriteString(m.Content)
		} else if title != "" {
			break
		}
	}
	return env.SessionID, started, title, body.String(), nil
}

// geminiFormatBLine matches both the metadata header line and message lines.
// `Content` stays raw to handle string-vs-array variance defensively.
type geminiFormatBLine struct {
	// Metadata fields (only on the first line).
	SessionID   string `json:"sessionId"`
	StartTime   string `json:"startTime"`
	LastUpdated string `json:"lastUpdated"`
	// Message fields.
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
}

type geminiContentPart struct {
	Text string `json:"text"`
}

func scanGeminiFormatB(path string, collectBody bool) (string, time.Time, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", time.Time{}, "", "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)

	var (
		sessionID string
		started   time.Time
		title     string
		body      strings.Builder
	)
	for sc.Scan() {
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		// Skip the partial-update / rewind sentinels without unmarshalling them.
		if len(raw) >= 2 && raw[0] == '{' && raw[1] == '"' {
			if hasJSONKey(raw, "$set") || hasJSONKey(raw, "$rewindTo") {
				continue
			}
		}
		var line geminiFormatBLine
		if err := json.Unmarshal(raw, &line); err != nil {
			continue
		}
		// Metadata line: sessionId is set.
		if sessionID == "" && line.SessionID != "" {
			sessionID = line.SessionID
			if line.StartTime != "" {
				if t, err := ParseTime(line.StartTime); err == nil {
					started = t
				}
			}
		}
		// User messages contribute to title + body.
		if line.Type == "user" {
			if t := extractGeminiContent(line.Content); t != "" {
				if title == "" {
					title = t
				}
				if collectBody {
					if body.Len() > 0 {
						body.WriteString("\n---\n")
					}
					body.WriteString(t)
				}
			}
		}
		if !collectBody && sessionID != "" && title != "" {
			break
		}
	}
	// scanner.Err() is non-fatal — we surface what we have.
	return sessionID, started, title, body.String(), nil
}

// extractGeminiContent handles both the legacy string and the current
// `[{text:...}]` array shapes that appear in `.jsonl`.
func extractGeminiContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	case '[':
		var parts []geminiContentPart
		if err := json.Unmarshal(raw, &parts); err == nil {
			var b strings.Builder
			for _, p := range parts {
				if p.Text == "" {
					continue
				}
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(p.Text)
			}
			return b.String()
		}
	}
	return ""
}

// hasJSONKey is a cheap byte-level probe: does `raw` start with `{"<key>":`?
// Used to short-circuit `$set` / `$rewindTo` lines before we try to unmarshal.
func hasJSONKey(raw []byte, key string) bool {
	prefix := []byte(`{"` + key + `":`)
	return len(raw) >= len(prefix) && string(raw[:len(prefix)]) == string(prefix)
}
