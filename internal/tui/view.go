package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/promo"
)

// Lipgloss styles. Single source of truth so the colour palette is easy to
// swap if the user has a light terminal.
var (
	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder())

	styleHeader = lipgloss.NewStyle().
			Bold(true)

	styleSelected = lipgloss.NewStyle().
			Reverse(true)

	styleHit = lipgloss.NewStyle().
			Reverse(true).
			Bold(true)

	styleStatus = lipgloss.NewStyle().
			Faint(true)

	styleSource = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4"))

	styleErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
)

// Layout constants. Hard floor on minTermWidth/Height triggers the
// too-small fallback; everything below is the inner-pane minimum once the
// terminal is large enough to render at all.
const (
	minTermWidth  = 60
	minTermHeight = 16

	inputBarH = 1 // single line of textinput, no border
	footerH   = 1 // status line
	bordersV  = 4 // 2 borders for list, 2 for preview (top + bottom each)

	// Search-input border row count. The input is wrapped in a rounded
	// border which adds 2 vertical lines (top/bottom).
	inputBorderH = 2
)

// relayout recomputes pane sizes from the current window dimensions. Called
// on every WindowSizeMsg. Defensive minimums stop the layout from
// collapsing on tiny-but-still-renderable terminals.
//
// Vertical budget:
//
//	height = banner + (input + 2 borders) + (list + 2 borders) + footer + (footer-promo opt 1)
//	       = banner + 3 + listH+2 + 1
//	listH  = height - banner - 3 - 2 - 1 - searchFooterH
//
// Horizontal budget: list and preview share the row, each takes half plus
// its own borders. listW reported here is the outer width including the
// border; the inner content width is listW-2.
func (m *Model) relayout() {
	if m.tooSmall {
		m.listW, m.listH, m.previewW = 0, 0, 0
		return
	}

	bannerH := bannerLines(promo.Banner(m.cfg.Promo))
	searchFooterH := bannerLines(promo.SearchFooter(m.cfg.Promo, m.input.Value()))

	// Vertical: total height = banner + input(1+2 borders) + listBox(listH+2 borders) + searchFooter + footer
	listH := m.height - bannerH - inputBarH - inputBorderH - 2 - searchFooterH - footerH
	if listH < 3 {
		listH = 3
	}

	// Horizontal: two equal panes. -2 for the gap absorbed by the borders'
	// shared seam.
	listW := m.width / 2
	previewW := m.width - listW

	m.listW = listW
	m.listH = listH
	m.previewW = previewW

	// textinput is rendered inside the bordered search bar. Its inner width
	// is the bar width (-2 for borders) minus the "search: " prefix.
	const searchPrefix = "search: "
	innerW := m.width - 2 - runewidth.StringWidth(searchPrefix)
	if innerW < 4 {
		innerW = 4
	}
	m.input.Width = innerW

	// Preview viewport: inner content fits inside the bordered box.
	previewInnerW := previewW - 2
	if previewInnerW < 4 {
		previewInnerW = 4
	}
	m.preview.Width = previewInnerW
	m.preview.Height = listH
	m.refreshPreview()
}

// bannerLines counts how many terminal rows a banner string occupies.
// Empty string -> 0 rows. We don't wrap on width because banner content is
// short by construction (one quote ≤ ~70 cells); the only multi-line case
// is the 5%-CTA draw which inserts a second \n line by hand.
func bannerLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// clampScroll keeps m.scrollOffset such that the selected row is visible
// inside [scrollOffset, scrollOffset+listH). Called on key navigation,
// resize, and search-result delivery.
func (m *Model) clampScroll() {
	if m.listH <= 0 {
		m.scrollOffset = 0
		return
	}
	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
		return
	}
	if m.selected >= m.scrollOffset+m.listH {
		m.scrollOffset = m.selected - m.listH + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxStart := len(m.results) - m.listH
	if maxStart < 0 {
		maxStart = 0
	}
	if m.scrollOffset > maxStart {
		m.scrollOffset = maxStart
	}
}

// View renders the full TUI: banner (opt) + search box + list/preview row +
// search-footer (opt) + status bar. When the terminal is below the minimum
// supported size we bail out to a plain three-line message — lipgloss
// rendering on 50-column terminals breaks the borders and is worse UX
// than telling the user to enlarge.
func (m *Model) View() string {
	if m.height == 0 || m.width == 0 {
		return "loading…"
	}
	if m.tooSmall {
		return fmt.Sprintf(
			"\n  terminal too small (need ≥ %d×%d, got %d×%d)\n  please enlarge the window\n",
			minTermWidth, minTermHeight, m.width, m.height,
		)
	}

	searchBar := styleBorder.
		Width(m.width - 2).
		Render("search: " + m.input.View())

	listInnerW := m.listW - 2
	if listInnerW < 4 {
		listInnerW = 4
	}
	listBox := styleBorder.
		Width(m.listW).
		Height(m.listH + 2).
		Render(m.formatList(listInnerW, m.listH))

	previewBox := styleBorder.
		Width(m.previewW).
		Height(m.listH + 2).
		Render(m.preview.View())

	mid := lipgloss.JoinHorizontal(lipgloss.Top, listBox, previewBox)
	status := m.formatStatus()

	// W6 promo seams: top banner + search-footer line. Banner() / footer()
	// return "" when --no-promo or when the random gate skips this launch,
	// so the JoinVertical call below collapses cleanly to the W3 layout.
	var parts []string
	if banner := promo.Banner(m.cfg.Promo); banner != "" {
		parts = append(parts, styleStatus.Render(banner))
	}
	parts = append(parts, searchBar, mid)
	if footer := promo.SearchFooter(m.cfg.Promo, m.input.Value()); footer != "" {
		parts = append(parts, styleStatus.Render(footer))
	}
	parts = append(parts, status)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// formatList renders the left pane: one row per result, the selected row
// reverse-rendered. Each row is single-line: source, date+time, id8, cwd,
// title — clamped to runewidth columns so CJK text aligns correctly. The
// scrollOffset window is applied here; clampScroll() owns the math.
func (m *Model) formatList(width, height int) string {
	if len(m.results) == 0 {
		if m.err != nil {
			return styleErr.Render("error: " + m.err.Error())
		}
		return styleStatus.Render("(no results)")
	}
	if height <= 0 {
		height = 1
	}
	start := m.scrollOffset
	end := start + height
	if end > len(m.results) {
		end = len(m.results)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		row := formatRow(m.results[i].Session, width)
		if i == m.selected {
			row = styleSelected.Render(row)
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatRow renders one Session row clipped to `width` display columns,
// always single-line.
//
// Field budget (display columns, runewidth-aware):
//
//	source(≤7) " " date(5) " " time(5) " " id(8) " " cwd(varW) " " title(restW)
//
// Fixed prefix = 7+1+5+1+5+1+8+1 = 29 cols (cwd onwards are flexible).
// cwd takes ~30% of the remaining width, title the rest. We never split
// inside a multi-cell rune (runewidth.Truncate handles CJK correctly).
func formatRow(s adapter.Session, width int) string {
	const fixedPrefix = 29
	source := runewidth.Truncate(s.Source, 7, "")
	source = padRightDisplay(source, 7)

	t := s.UpdatedAt.Local()
	date := t.Format("01-02")
	clock := t.Format("15:04")

	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	id = padRightDisplay(id, 8)

	remaining := width - fixedPrefix
	if remaining < 6 {
		remaining = 6
	}
	cwdW := remaining * 30 / 100
	if cwdW < 4 {
		cwdW = 4
	}
	if cwdW > remaining-2 {
		cwdW = remaining - 2
	}
	titleW := remaining - cwdW - 1 // -1 for the space between cwd and title
	if titleW < 1 {
		titleW = 1
	}

	cwd := truncateLeftDisplay(s.CWD, cwdW)
	cwd = padRightDisplay(cwd, cwdW)
	title := runewidth.Truncate(s.Title, titleW, "…")
	title = padRightDisplay(title, titleW)

	out := source + " " + date + " " + clock + " " + id + " " + cwd + " " + title
	// Ensure exactly `width` cells so the reverse-render selector fills the row.
	pad := width - runewidth.StringWidth(out)
	if pad > 0 {
		out += strings.Repeat(" ", pad)
	} else if pad < 0 {
		out = runewidth.Truncate(out, width, "")
	}
	return out
}

// padRightDisplay pads s on the right with spaces until it occupies n
// display cells. If s is already wider, it is left untouched (caller is
// responsible for truncating first).
func padRightDisplay(s string, n int) string {
	w := runewidth.StringWidth(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// truncateLeftDisplay clamps s to n display cells, keeping the right side
// (path tail is more identifying than the home prefix). Adds a leading "…"
// when truncation occurred. CJK-safe via runewidth.
func truncateLeftDisplay(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= n {
		return s
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		cand := "…" + string(rs[i+1:])
		if runewidth.StringWidth(cand) <= n {
			return cand
		}
	}
	return "…"
}

// formatStatus is the bottom bar. Keys + result counter + dry-run badge so
// users always know what Enter will do.
func (m *Model) formatStatus() string {
	mode := "DRY-RUN"
	if !m.cfg.DryRun {
		mode = "EXEC"
	}
	src := m.cfg.Source
	if src == "" {
		src = "all"
	}
	parts := []string{
		"↑↓ select",
		"⏎ resume",
		"esc quit",
		fmt.Sprintf("source: %s", src),
		fmt.Sprintf("%d hits", len(m.results)),
		mode,
	}
	return styleStatus.Render(strings.Join(parts, "  "))
}

// hitRange is one [start, end) byte range to highlight in the preview.
type hitRange struct{ start, end int }

// highlightBody reverse-renders every occurrence of any query word inside
// `body`. Case-insensitive byte match — simple and fast on the ≤64 KB body
// budget. Multiple words highlight independently; overlap is rare given
// bodies are user-typed prose.
func highlightBody(body string, words []string) string {
	if len(words) == 0 || body == "" {
		return body
	}
	lower := strings.ToLower(body)
	var hits []hitRange
	for _, w := range words {
		if w == "" {
			continue
		}
		start := 0
		for {
			idx := strings.Index(lower[start:], w)
			if idx < 0 {
				break
			}
			a := start + idx
			b := a + len(w)
			hits = append(hits, hitRange{a, b})
			start = b
		}
	}
	if len(hits) == 0 {
		return body
	}
	sortHits(hits)
	merged := hits[:0:cap(hits)]
	for _, h := range hits {
		if len(merged) > 0 && h.start <= merged[len(merged)-1].end {
			if h.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = h.end
			}
			continue
		}
		merged = append(merged, h)
	}
	var b strings.Builder
	cursor := 0
	for _, h := range merged {
		b.WriteString(body[cursor:h.start])
		b.WriteString(styleHit.Render(body[h.start:h.end]))
		cursor = h.end
	}
	b.WriteString(body[cursor:])
	return b.String()
}

// sortHits is a tiny insertion sort — `hits` is bounded by O(words *
// occurrences) which is always small for typed queries on a 64 KB body.
func sortHits(h []hitRange) {
	for i := 1; i < len(h); i++ {
		for j := i; j > 0 && h[j].start < h[j-1].start; j-- {
			h[j], h[j-1] = h[j-1], h[j]
		}
	}
}
