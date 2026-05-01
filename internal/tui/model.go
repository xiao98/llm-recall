// Package tui implements the live-search TUI.
//
// The user types in a search box; every keystroke triggers a debounced
// SQL+fuzzy search against the SQLite cache; results land in the left list;
// the highlighted row's full body lands in the right preview with query
// matches reverse-rendered. Enter exits the TUI and asks the launcher to
// resume the chosen session; Esc / Ctrl-C exits with code 0.
//
// The whole TUI is one bubbletea Model. We deliberately do not split it
// into per-pane bubbles — the panes share state (selection drives preview
// content; query drives both list filter and preview highlight) and folding
// them into separate models would mean threading messages through
// components for no clear gain.
package tui

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/search"
)

// Selection is the model's terminal output: nil ⇒ user quit without picking,
// non-nil ⇒ launcher should resume *Selection.Session.
type Selection struct {
	Session adapter.Session
}

// Config is everything the caller needs to build a Model.
type Config struct {
	// DB is the open SQLite handle from index.Cache. Required.
	DB *sql.DB
	// InitialQuery is the search box's starting content. Useful for the
	// `--source` shortcut or a future "remember last query" feature; W3
	// always passes "".
	InitialQuery string
	// Source filters the search to one adapter. "" = all adapters.
	Source string
	// DryRun is shown in the status bar so users know whether Enter will
	// actually exec or just print the recipe.
	DryRun bool
	// Promo is the W6 marketing config. May be nil — the renderer treats
	// nil as "no banner / no footer", same as NoPromo=true.
	Promo *config.Config
}

// Model is the bubbletea state.
type Model struct {
	cfg Config

	// Layout: width/height come from a tea.WindowSizeMsg on first paint.
	width, height int
	// tooSmall is set when width<60 or height<16. View() returns a plain
	// fallback string in that case so lipgloss never has to render a 4-cell
	// pane. Recomputed on every WindowSizeMsg.
	tooSmall bool
	// Layout slots, populated by recomputeLayout(). View() reads these
	// directly instead of recomputing — avoids the bug where View() and
	// formatList() drifted apart in W3.
	listW, listH, previewW int

	// Search box (top), list pane (left), preview pane (right). textinput +
	// viewport from bubbles handle low-level concerns (cursor, scrolling).
	input    textinput.Model
	preview  viewport.Model
	selected int // index into results
	// scrollOffset is the index of the first list row currently shown.
	// We keep it in Model rather than recomputing in View() so the scroll
	// state is stable across resizes.
	scrollOffset int

	results []search.Result

	// stamp is incremented on every keystroke and stamped onto each
	// outstanding searchMsg so we can drop stale ones.
	stamp uint64

	// chosen is set when Update wants to exit with a Selection.
	chosen *Selection

	err error

	// External controls used by the test harness; nil in production.
	harness *harness
}

// New builds a Model in its initial state. Call Run() once configured.
func New(cfg Config) *Model {
	in := textinput.New()
	in.Placeholder = "search…"
	in.CharLimit = 256
	in.SetValue(cfg.InitialQuery)
	in.Focus()

	pv := viewport.New(80, 10)

	return &Model{
		cfg:     cfg,
		input:   in,
		preview: pv,
		harness: loadHarness(),
	}
}

// Init kicks off the very first search (empty query → most recent rows).
// The search is async; the empty list briefly shown before searchMsg arrives
// is acceptable.
func (m *Model) Init() tea.Cmd {
	m.stamp++
	return tea.Batch(textinput.Blink, runSearchCmd(m.cfg.DB, m.stamp, m.input.Value(), m.cfg.Source))
}

// Update is the dispatcher. We special-case the half-dozen control keys
// before letting textinput consume the rest. Window resize fans out to
// preview's viewport. Search results are accepted only if they're still the
// most recent.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.tooSmall = msg.Width < 60 || msg.Height < 16
		m.relayout()
		m.clampScroll()
		return m, nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case searchMsg:
		// Stale guard: a faster keystroke has bumped m.stamp past this.
		if msg.Stamp != m.stamp {
			return m, nil
		}
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.results = msg.Results
		if m.selected >= len(m.results) {
			m.selected = 0
		}
		m.clampScroll()
		m.refreshPreview()
		return m, nil

	}
	return m, nil
}

func (m *Model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch classify(k) {
	case keyQuit:
		return m, tea.Quit
	case keyEnter:
		if len(m.results) > 0 {
			sel := m.results[m.selected]
			m.chosen = &Selection{Session: sel.Session}
		}
		return m, tea.Quit
	case keyDown:
		if m.selected+1 < len(m.results) {
			m.selected++
			m.clampScroll()
			m.refreshPreview()
		}
		return m, nil
	case keyUp:
		if m.selected > 0 {
			m.selected--
			m.clampScroll()
			m.refreshPreview()
		}
		return m, nil
	case keyPageDown:
		m.preview.HalfPageDown()
		return m, nil
	case keyPageUp:
		m.preview.HalfPageUp()
		return m, nil
	}
	// Default: feed to text input + kick off a debounced search.
	prev := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(k)
	cur := m.input.Value()
	if cur != prev {
		m.stamp++
		m.selected = 0
		searchCmd := runSearchCmd(m.cfg.DB, m.stamp, cur, m.cfg.Source)
		// We deliberately drop textinput's cursor-blink cmd here because
		// it loops via tea.Tick and was observed to compete with the
		// search cmd for scheduling. Cursor blink is purely cosmetic.
		_ = cmd
		return m, searchCmd
	}
	return m, cmd
}

// View, relayout, refreshPreview, formatList, formatPreview are all in
// view.go. Splitting keeps this file focused on state transitions.

// Chosen reports the user's selection, or nil if they quit without picking.
func (m *Model) Chosen() *Selection { return m.chosen }

// Run launches the bubbletea program. Returns the chosen Selection (may be
// nil) plus the final error (also possibly nil). On error nil + the error
// is returned so the caller can decide whether to print or fall through.
//
// When LLM_RECALL_TEST_INPUT is set, the harness drives the program by
// posting synthetic key messages via Program.Send, then dumps a snapshot
// and sends Quit / Enter / Esc as the script demands.
func (m *Model) Run() (*Selection, error) {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if m.harness != nil {
		// Headless: don't open the alt screen (would smash test stdout),
		// silence signal handling (so timeouts work), and feed an EOF
		// stdin so the input reader doesn't block on a real terminal.
		opts = []tea.ProgramOption{tea.WithoutSignalHandler(), tea.WithInput(nopReader{})}
	}
	p := tea.NewProgram(m, opts...)
	if m.harness != nil {
		// Seed an initial WindowSizeMsg so View() / relayout() pick a sane
		// width even without a real terminal. Override default 120x30 with
		// LLM_RECALL_TEST_TERM_WIDTH/HEIGHT when set so the resize harness
		// can exercise multiple sizes against the same binary.
		w, h := envTermSize(120, 30)
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Send(tea.WindowSizeMsg{Width: w, Height: h})
			m.harness.drive(p, m)
		}()
	}
	if _, err := p.Run(); err != nil {
		return nil, err
	}
	return m.chosen, nil
}

// nopReader is a stdin substitute used in harness mode so bubbletea's input
// reader sees a closed stream and never blocks waiting for keystrokes from
// the real terminal.
type nopReader struct{}

func (nopReader) Read(p []byte) (int, error) { return 0, io.EOF }

// envTermSize lets the harness override the seed WindowSizeMsg via env vars.
// We don't use a real PTY because the resize bug we're fixing is layout
// math, not terminal I/O — feeding a fake WindowSizeMsg exercises the same
// code path Update() runs on a SIGWINCH.
func envTermSize(defW, defH int) (int, int) {
	w := envInt("LLM_RECALL_TEST_TERM_WIDTH", defW)
	h := envInt("LLM_RECALL_TEST_TERM_HEIGHT", defH)
	return w, h
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// refreshPreview redraws the right pane to reflect m.selected. We re-render
// from scratch every time rather than diffing — it's cheap (one body, max
// 64KB) and avoids stale-state bugs.
func (m *Model) refreshPreview() {
	if len(m.results) == 0 {
		m.preview.SetContent("")
		return
	}
	if m.selected >= len(m.results) {
		m.selected = 0
	}
	body := m.results[m.selected].Session.Body
	if body == "" {
		body = m.results[m.selected].Session.Title
	}
	words := search.Words(m.input.Value())
	m.preview.SetContent(highlightBody(body, words))
}

// debugSnapshot returns a multi-line string capturing the visible state.
// Used by the test harness so the verifier can grep for expected substrings
// without screen-scraping ANSI. This is intentionally NOT used by View().
func (m *Model) debugSnapshot() string {
	var b strings.Builder
	fmt.Fprintf(&b, "QUERY=%s\n", m.input.Value())
	fmt.Fprintf(&b, "SOURCE=%s\n", m.cfg.Source)
	fmt.Fprintf(&b, "DRYRUN=%t\n", m.cfg.DryRun)
	fmt.Fprintf(&b, "TERM=%dx%d\n", m.width, m.height)
	fmt.Fprintf(&b, "TOOSMALL=%t\n", m.tooSmall)
	fmt.Fprintf(&b, "LISTW=%d LISTH=%d PREVIEWW=%d\n", m.listW, m.listH, m.previewW)
	fmt.Fprintf(&b, "SELECTED=%d SCROLL=%d RANGE=[%d,%d)\n",
		m.selected, m.scrollOffset, m.scrollOffset, m.scrollOffset+m.listH)
	fmt.Fprintf(&b, "PREVIEW_OFFSET=%d\n", m.preview.YOffset)
	fmt.Fprintf(&b, "RESULTS=%d\n", len(m.results))
	maxRows := 5
	if len(m.results) < maxRows {
		maxRows = len(m.results)
	}
	for i := 0; i < maxRows; i++ {
		r := m.results[i]
		fmt.Fprintf(&b, "  [%d] %s | %s | %s\n", i, r.Session.Source, r.Session.ID, r.Session.Title)
	}
	if len(m.results) > 0 {
		body := m.results[m.selected].Session.Body
		if len([]rune(body)) > 200 {
			body = string([]rune(body)[:200])
		}
		fmt.Fprintf(&b, "PREVIEW=%s\n", body)
	}
	return b.String()
}
