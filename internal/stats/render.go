// Terminal-native stats renderer.
//
// Composition (top → bottom, all left-aligned with 2-space inner padding):
//
//  1. Month axis row:   "May  Jun  Jul  Aug  …  Apr"
//  2. Heatmap rows ×7:  "Mon  ▒▓⋅⋅⋅…██"  (only Mon/Wed/Fri row labels are
//     shown — the screenshot leaves the other rows
//     unlabeled but still drawn as blank-cell rows)
//  3. Legend:           "Less ⋅ ▒ ▓ █ More"
//  4. Tab bar:          "All time · Last 7 days · Last 30 days"
//  5. 4×2 stats panel:  Two columns, four rows.
//
// Why one Model: the three views share the same heatmap (rebuilt once) and
// the same session list; the only thing that changes on tab-switch is the
// 4×2 panel content. Splitting into bubbles per pane would add ceremony
// for no clear gain — same call we made in W3 TUI.
//
// Glyphs / colours: orange-gradient block characters. The exact RGB values
// are tuned in colourFor(); we deliberately avoid 256-colour fallbacks
// because every modern terminal we ship to supports truecolor (lipgloss
// downgrades gracefully on the rare exception).
package stats

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xterm "github.com/charmbracelet/x/term"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/promo"
)

// TabKind picks a window for the 4×2 panel.
type TabKind int

const (
	TabAll TabKind = iota
	TabLast7
	TabLast30
)

func (t TabKind) Label() string {
	switch t {
	case TabAll:
		return "All time"
	case TabLast7:
		return "Last 7 days"
	case TabLast30:
		return "Last 30 days"
	}
	return "?"
}

func (t TabKind) Days() int {
	switch t {
	case TabLast7:
		return 7
	case TabLast30:
		return 30
	default:
		return WindowAll
	}
}

// Model is the bubbletea state for `llm-recall stats`.
type Model struct {
	now           time.Time
	sessions      []adapter.Session
	tokenFallback int64

	// pre-computed once (window-independent).
	heatmap Heatmap

	// computed per-tab, cached after first render.
	stats map[TabKind]Stats

	// W9: top-N topics across the FULL session set (window-independent).
	// We don't recompute on tab switch — topics are an "overall" lens.
	topics []TopicCount

	tab    TabKind
	width  int
	height int

	// promo is the W6 marketing config used to render the attribution
	// footer. nil ⇒ no footer (same as --no-promo).
	promo *config.Config

	// W9: noPet hides the pixel pet entirely (--no-pet flag).
	noPet bool

	// harness shim; nil in production. Mirrors the W3 TUI pattern.
	harness *renderHarness
}

// NewModel builds the stats Model. `sessions` should be the full set across
// all sources; `now` is parameterised for tests/harnesses.
func NewModel(sessions []adapter.Session, now time.Time, tokenFallback int64) *Model {
	m := &Model{
		now:           now,
		sessions:      sessions,
		tokenFallback: tokenFallback,
		heatmap:       BuildHeatmap(BuildDailyCounts(sessions, now)),
		stats:         map[TabKind]Stats{},
		topics:        TopTopics(sessions, 5),
		tab:           TabAll,
		harness:       loadRenderHarness(),
	}
	// Warm the all-time stats so the first paint is instant.
	m.stats[TabAll] = Compute(sessions, now, TabAll.Days(), tokenFallback)
	return m
}

// WithNoPet returns the same model with the pixel pet suppressed.
// Used by the --no-pet CLI flag (extra-minimal stats users) and by
// the JSON snapshot path.
func (m *Model) WithNoPet(no bool) *Model {
	m.noPet = no
	return m
}

// WithPromo attaches a promo config so View() renders the sponsored
// footer. Pass nil (or a Config with NoPromo=true) to suppress the
// footer. The setter pattern keeps the existing W5 NewModel signature
// stable while letting cmd_stats.go opt in.
func (m *Model) WithPromo(cfg *config.Config) *Model {
	m.promo = cfg
	return m
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "right", "l", "tab":
			m.tab = (m.tab + 1) % 3
			m.ensureTab(m.tab)
			return m, nil
		case "left", "h", "shift+tab":
			m.tab = (m.tab + 3 - 1) % 3
			m.ensureTab(m.tab)
			return m, nil
		case "1":
			m.tab = TabAll
			m.ensureTab(m.tab)
			return m, nil
		case "2":
			m.tab = TabLast7
			m.ensureTab(m.tab)
			return m, nil
		case "3":
			m.tab = TabLast30
			m.ensureTab(m.tab)
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) ensureTab(t TabKind) {
	if _, ok := m.stats[t]; ok {
		return
	}
	m.stats[t] = Compute(m.sessions, m.now, t.Days(), m.tokenFallback)
}

// Style palette. Lipgloss strips colours when the terminal can't render
// truecolor; we still ship sensible monochrome with bold/dim emphasis.
var (
	colOrange     = lipgloss.Color("#FF6B35") // primary accent
	colOrangeDim  = lipgloss.Color("#7A3A1F") // low-saturation orange
	colOrangeMid  = lipgloss.Color("#C04A20") // mid-saturation
	colMuted      = lipgloss.Color("#6B6B6B")
	colHeatEmpty  = lipgloss.Color("#2A2A2A")
	colTabActive  = lipgloss.NewStyle().Foreground(colOrange).Bold(true)
	colTabInactiv = lipgloss.NewStyle().Foreground(colMuted)
	colLabel      = lipgloss.NewStyle().Foreground(colMuted)
	colValue      = lipgloss.NewStyle().Foreground(colOrange).Bold(true)
)

// Glyphs (Unicode block characters; widths are 1 cell each).
const (
	glyphEmpty = "⋅" // ⋅ DOT OPERATOR
	glyphLow   = "▒" // ▒ MEDIUM SHADE
	glyphMid   = "▓" // ▓ DARK SHADE
	glyphHigh  = "█" // █ FULL BLOCK
)

func glyphFor(l HeatLevel) string {
	switch l {
	case HeatLow:
		return glyphLow
	case HeatMid:
		return glyphMid
	case HeatHigh:
		return glyphHigh
	default:
		return glyphEmpty
	}
}

func colourFor(l HeatLevel) lipgloss.Color {
	switch l {
	case HeatLow:
		return colOrangeDim
	case HeatMid:
		return colOrangeMid
	case HeatHigh:
		return colOrange
	default:
		return colHeatEmpty
	}
}

// effectiveWidth returns the column count to lay out against. Order of
// preference: LLM_RECALL_TEST_TERM_WIDTH env (test-only override) →
// WindowSizeMsg → real terminal via x/term → 80 fallback. The env wins
// over WindowSizeMsg so the harness can drive width without a real PTY
// even when stdout is attached to a real terminal.
func (m *Model) effectiveWidth() int {
	if v := os.Getenv("LLM_RECALL_TEST_TERM_WIDTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	if m.width > 0 {
		return m.width
	}
	if w, _, err := xterm.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	return 80
}

// View renders the full screen.
//
// Layout (W9):
//
//	┌─ heatmap ─────────────────────────┐  ┌─ pet ──┐
//	│ Mon ▒▓⋅⋅⋅…██                       │  │  ghost │
//	│ Wed ⋅⋅▓██…⋅⋅                       │  │   ASCII │
//	│ ...                                │  └────────┘
//	└────────────────────────────────────┘
//	📚 Top topics:  claude  历史  wiki  quant  feishu
//	[ All time · Last 7 days · Last 30 days ]
//	┌─ 4×2 panel ───────────────────────┐
//	│ ...                                │
//	└────────────────────────────────────┘
//	Per source bars …
//	footer: ←/→ q · attribution
//
// Pet renders only when terminal width ≥ 100 cols AND m.noPet is
// false; below that we collapse back to the W5 vertical layout.
func (m *Model) View() string {
	w := m.effectiveWidth()
	showPet := !m.noPet && w >= 100 && m.heatmap.Cols > 0

	var b strings.Builder
	b.WriteString("\n")
	if showPet {
		// Place the heatmap on the left and the pet sprite on the
		// right, vertically anchored to the top so the heatmap's
		// month axis stays at the visual top.
		left := m.viewHeatmap()
		right := RenderPet(ChooseState(m.stats[m.tab]))
		row := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
		b.WriteString(row)
	} else {
		b.WriteString(m.viewHeatmap())
	}
	b.WriteString("\n")
	b.WriteString(m.viewLegend())
	b.WriteString("\n\n")
	b.WriteString(m.viewTopics())
	b.WriteString("\n")
	b.WriteString(m.viewTabs())
	b.WriteString("\n\n")
	b.WriteString(m.viewPanel())
	b.WriteString("\n")
	b.WriteString(m.viewPerSource())
	b.WriteString("\n")
	b.WriteString(colLabel.Render("  ←/→ switch window · 1/2/3 jump · q quit"))
	b.WriteString("\n")
	// W6/W9: attribution footer. promo.StatsFooter returns "" when
	// --no-promo or when the user has set [promo] no_promo = true in
	// config.toml, so the line collapses to nothing in that case.
	if footer := promo.StatsFooter(m.promo); footer != "" {
		b.WriteString("\n")
		b.WriteString(colLabel.Render("  " + footer))
		b.WriteString("\n")
	}
	return b.String()
}

// viewTopics renders the W9 Top-N topics horizontal row. Empty when
// no topics were extracted (e.g. fresh install with empty bodies).
func (m *Model) viewTopics() string {
	if len(m.topics) == 0 {
		return ""
	}
	parts := []string{colLabel.Render("  📚 Top topics:")}
	for _, t := range m.topics {
		parts = append(parts, colValue.Render(t.Token))
	}
	return strings.Join(parts, "  ")
}

// viewHeatmap draws month axis + 7 weekday rows. When the terminal is too
// narrow to fit a meaningful heatmap (≤6 gutter + at least ~52 weeks worth
// of cells) we degrade to a one-line warning so the panel below still
// renders.
func (m *Model) viewHeatmap() string {
	if m.heatmap.Cols == 0 {
		return colLabel.Render("  (no sessions yet — run a few then come back)")
	}
	w := m.effectiveWidth()
	if w < 60 {
		return colLabel.Render(fmt.Sprintf("  ⚠ heatmap 需 ≥ 60 列宽，当前 %d；已隐藏", w))
	}

	// Month axis: align labels to their column positions. Each cell is 1
	// char wide; we leave a small left gutter for the row label.
	const gutter = "       " // matches "  Mon  " / blank rows
	var axis strings.Builder
	axis.WriteString(gutter)
	for c := 0; c < m.heatmap.Cols; c++ {
		lbl := m.heatmap.MonthLabels[c]
		switch {
		case lbl == "":
			axis.WriteString(" ")
		default:
			// Print the 3-char month label starting at this column,
			// then skip the next two columns (the label is 3 cells wide).
			// If there isn't room we just truncate.
			room := m.heatmap.Cols - c
			n := 3
			if room < n {
				n = room
			}
			axis.WriteString(lbl[:n])
			c += n - 1 // -1 because the loop will ++ once more
		}
	}
	axis.WriteString("\n")

	// 7 weekday rows. Only Mon/Wed/Fri labelled.
	rowLabel := func(r int) string {
		switch r {
		case 0:
			return "  Mon  "
		case 2:
			return "  Wed  "
		case 4:
			return "  Fri  "
		default:
			return "       "
		}
	}

	var rows strings.Builder
	rows.WriteString(axis.String())
	for r := 0; r < 7; r++ {
		rows.WriteString(colLabel.Render(rowLabel(r)))
		for c := 0; c < m.heatmap.Cols; c++ {
			cell := m.heatmap.Rows[r][c]
			if cell.Day.IsZero() {
				rows.WriteString(" ")
				continue
			}
			st := lipgloss.NewStyle().Foreground(colourFor(cell.Level))
			rows.WriteString(st.Render(glyphFor(cell.Level)))
		}
		rows.WriteString("\n")
	}
	return rows.String()
}

func (m *Model) viewLegend() string {
	parts := []string{
		colLabel.Render("  Less"),
		lipgloss.NewStyle().Foreground(colHeatEmpty).Render(glyphEmpty),
		lipgloss.NewStyle().Foreground(colOrangeDim).Render(glyphLow),
		lipgloss.NewStyle().Foreground(colOrangeMid).Render(glyphMid),
		lipgloss.NewStyle().Foreground(colOrange).Render(glyphHigh),
		colLabel.Render("More"),
	}
	return strings.Join(parts, " ")
}

func (m *Model) viewTabs() string {
	var parts []string
	for _, t := range []TabKind{TabAll, TabLast7, TabLast30} {
		st := colTabInactiv
		if t == m.tab {
			st = colTabActive
		}
		parts = append(parts, st.Render(t.Label()))
	}
	return "  " + strings.Join(parts, colLabel.Render("   ·   "))
}

// viewPanel renders the 4×2 stats grid.
func (m *Model) viewPanel() string {
	s := m.stats[m.tab]
	span := s.WindowDaysSpan
	if s.WindowDays > 0 && span > s.WindowDays {
		span = s.WindowDays
	}

	rows := [4][2][2]string{
		{
			{"Favorite source", orDash(s.FavoriteSource)},
			{"Total tokens", FormatTokens(s.TotalTokens)},
		},
		{
			{"Sessions", fmt.Sprintf("%d", s.Sessions)},
			{"Longest session", FormatDuration(s.LongestSession)},
		},
		{
			{"Active days", FormatActiveDays(s.ActiveDays, span)},
			{"Longest streak", FormatStreak(s.LongestStreak)},
		},
		{
			{"Most active day", FormatDate(s.MostActiveDay)},
			{"Current streak", FormatStreak(s.CurrentStreak)},
		},
	}

	// Compute label widths so the two columns align.
	labelW1, labelW2 := 0, 0
	valueW1 := 0
	for _, r := range rows {
		if l := len(r[0][0]); l > labelW1 {
			labelW1 = l
		}
		if l := len(r[1][0]); l > labelW2 {
			labelW2 = l
		}
		if l := len(r[0][1]); l > valueW1 {
			valueW1 = l
		}
	}

	// Width-aware degrade: <80 columns is too narrow for two label-value
	// columns side-by-side, so we stack vertically as 4×1 (8 lines tall
	// instead of 4) — same data, just reflowed.
	w := m.effectiveWidth()
	stack := w < 80

	var b strings.Builder
	for _, r := range rows {
		col1 := fmt.Sprintf("%s %s",
			colLabel.Render(padRight(r[0][0]+":", labelW1+1)),
			colValue.Render(r[0][1]),
		)
		if stack {
			col2 := fmt.Sprintf("%s %s",
				colLabel.Render(padRight(r[1][0]+":", labelW2+1)),
				colValue.Render(r[1][1]),
			)
			b.WriteString("  " + col1 + "\n")
			b.WriteString("  " + col2 + "\n")
			continue
		}
		// Right-pad column-1's rendered cell so col2 lines up. We can't
		// rely on len(col1) because lipgloss adds escape sequences; compute
		// the visible width manually.
		visW := labelW1 + 1 + 1 + valueW1
		col1 += strings.Repeat(" ", maxInt(0, visW-(labelW1+1+1+len(r[0][1]))))

		col2 := fmt.Sprintf("%s %s",
			colLabel.Render(padRight(r[1][0]+":", labelW2+1)),
			colValue.Render(r[1][1]),
		)
		b.WriteString("  " + col1 + "    " + col2 + "\n")
	}
	return b.String()
}

// viewPerSource renders a small horizontal bar chart with one row per source,
// sorted by session count desc. Bars scale relative to the largest source
// inside the current window, capped at perSourceBarMax cells. Sources with 0
// sessions in the window are omitted.
func (m *Model) viewPerSource() string {
	// Bar width adapts: max 24 cells, but never wider than (terminal-12)
	// so the count column and label fit on the same line in narrow terms.
	w := m.effectiveWidth()
	perSourceBarMax := 24
	if barCap := w - 12; barCap < perSourceBarMax {
		perSourceBarMax = barCap
	}
	if perSourceBarMax < 4 {
		perSourceBarMax = 4
	}
	s := m.stats[m.tab]
	if len(s.PerSource) == 0 {
		return ""
	}

	type kv struct {
		name string
		n    int
	}
	rows := make([]kv, 0, len(s.PerSource))
	max := 0
	for name, n := range s.PerSource {
		if n == 0 {
			continue
		}
		rows = append(rows, kv{name, n})
		if n > max {
			max = n
		}
	}
	if len(rows) == 0 {
		return ""
	}
	// Stable sort: descending count, ties broken by name asc.
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].n > rows[i].n || (rows[j].n == rows[i].n && rows[j].name < rows[i].name) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}

	nameW := 0
	for _, r := range rows {
		if l := len(r.name); l > nameW {
			nameW = l
		}
	}

	barStyle := lipgloss.NewStyle().Foreground(colOrange)
	dimBar := lipgloss.NewStyle().Foreground(colHeatEmpty)

	var b strings.Builder
	b.WriteString(colLabel.Render("  Per source") + "\n")
	for _, r := range rows {
		filled := perSourceBarMax * r.n / max
		if filled < 1 && r.n > 0 {
			filled = 1
		}
		empty := perSourceBarMax - filled
		bar := barStyle.Render(strings.Repeat("█", filled))
		if empty > 0 {
			bar += dimBar.Render(strings.Repeat("█", empty))
		}
		line := fmt.Sprintf("    %s  %s  %s",
			colLabel.Render(padRight(r.name, nameW)),
			bar,
			colValue.Render(fmt.Sprintf("%d", r.n)),
		)
		b.WriteString(line + "\n")
	}
	return b.String()
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// Run launches the bubbletea program. Mirrors W3 TUI's harness wiring so
// integration tests can drive the renderer without a real terminal.
func (m *Model) Run() error {
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if m.harness != nil {
		opts = []tea.ProgramOption{tea.WithoutSignalHandler(), tea.WithInput(nopReaderRender{})}
	}
	p := tea.NewProgram(m, opts...)
	if m.harness != nil {
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
			m.harness.drive(p, m)
		}()
	}
	_, err := p.Run()
	return err
}

// RunOrFallback enters the TUI when stdout is a tty, otherwise prints the
// JSON snapshot to `out` so pipe consumers don't get hosed by ANSI. The
// harness path always wins so integration tests can exercise the TUI even
// when stdout is captured.
func (m *Model) RunOrFallback(out io.Writer) error {
	if m.harness == nil && !isTTY(os.Stdout) {
		return m.WriteJSON(out)
	}
	return m.Run()
}

// nopReaderRender mirrors tui.nopReader; defined here so we don't import
// internal/tui (would create a cycle if tui ever wanted stats).
type nopReaderRender struct{}

func (nopReaderRender) Read(p []byte) (int, error) { return 0, io.EOF }

// isTTY reports whether the file refers to a terminal. We avoid pulling in
// mattn/go-isatty as a direct dep — the indirect copy is already in
// go.sum (via lipgloss) but we don't want to add it to require.
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
