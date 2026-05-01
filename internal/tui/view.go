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

const (
	minWidth          = 80
	listMinWidth      = 40
	previewMinWidth   = 30
	statusBarLines    = 1
	searchBarLines    = 3 // border top + content + border bottom
	previewBorderRows = 2
)

// relayout recomputes pane sizes from the current window dimensions. Called
// on the first WindowSizeMsg and any subsequent resize. Defensive minimums
// stop the layout from collapsing on tiny terminals.
func (m *Model) relayout() {
	w := m.width
	if w < minWidth {
		w = minWidth
	}
	listW := w / 2
	if listW < listMinWidth {
		listW = listMinWidth
	}
	previewW := w - listW - 2 // -2 for border between
	if previewW < previewMinWidth {
		previewW = previewMinWidth
	}
	bodyH := m.height - searchBarLines - statusBarLines - previewBorderRows
	if bodyH < 4 {
		bodyH = 4
	}
	m.input.Width = w - 4
	m.preview.Width = previewW - 2
	m.preview.Height = bodyH - 1
	m.refreshPreview()
}

// View renders the three-pane layout. Top: search box. Middle: list (left)
// and preview (right). Bottom: status bar with key hints + result counter.
//
// W6 inserts an optional banner above the search bar and an optional
// search footer between the list/preview row and the status bar.
// Both come from internal/promo and respect the --no-promo kill switch;
// when promo is off they are empty strings and JoinVertical drops them.
func (m *Model) View() string {
	if m.height == 0 || m.width == 0 {
		return "loading…"
	}
	listW := m.width / 2
	if listW < listMinWidth {
		listW = listMinWidth
	}
	previewW := m.width - listW - 2
	if previewW < previewMinWidth {
		previewW = previewMinWidth
	}
	bodyH := m.height - searchBarLines - statusBarLines - previewBorderRows
	if bodyH < 4 {
		bodyH = 4
	}

	searchBar := styleBorder.
		Width(m.width - 2).
		Render("search: " + m.input.View())

	list := m.formatList(listW-2, bodyH)
	listBox := styleBorder.Width(listW).Height(bodyH + 2).Render(list)

	previewBox := styleBorder.Width(previewW).Height(bodyH + 2).Render(m.preview.View())

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
// reverse-rendered. Each row is `<source>  <date>  <cwd-short>  <id8>  <title>`,
// width-clamped to `width` runewidth columns so CJK doesn't break alignment.
func (m *Model) formatList(width, height int) string {
	if len(m.results) == 0 {
		if m.err != nil {
			return styleErr.Render("error: " + m.err.Error())
		}
		return styleStatus.Render("(no results)")
	}
	visible := height
	if visible <= 0 {
		visible = 1
	}
	// Scroll window: keep selected in view.
	start := 0
	if m.selected >= visible {
		start = m.selected - visible + 1
	}
	end := start + visible
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
		b.WriteByte('\n')
	}
	return b.String()
}

// formatRow renders one Session row clipped to `width` display columns.
func formatRow(s adapter.Session, width int) string {
	source := s.Source
	if len(source) > 6 {
		source = source[:6]
	}
	date := s.UpdatedAt.Local().Format("01-02 15:04")
	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	cwd := shortPath(s.CWD, 18)
	title := s.Title
	prefix := fmt.Sprintf("%-6s %s %-18s %s ", source, date, cwd, id)
	prefixW := runewidth.StringWidth(prefix)
	titleBudget := width - prefixW
	if titleBudget < 4 {
		titleBudget = 4
	}
	title = clipDisplay(title, titleBudget)
	out := prefix + title
	// Pad to exactly `width` so reverse-render fills the row evenly.
	pad := width - runewidth.StringWidth(out)
	if pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// shortPath replicates main.go's short-CWD logic but in a context that
// doesn't have access to $HOME (the TUI uses absolute or `~/…` paths
// already in the Session). We only need to clamp width; trim home left to
// the caller, which is fine because cache rows store as-is.
func shortPath(p string, max int) string {
	if p == "" {
		return ""
	}
	w := runewidth.StringWidth(p)
	if w <= max {
		// Pad right.
		return p + strings.Repeat(" ", max-w)
	}
	// Right-truncate with leading ellipsis (path tail is more identifying).
	rs := []rune(p)
	for len(rs) > 0 {
		if runewidth.StringWidth("…"+string(rs[1:])) <= max {
			return "…" + string(rs[1:])
		}
		rs = rs[1:]
	}
	return "…"
}

// clipDisplay truncates `s` to fit in `n` display columns, appending an
// ellipsis when it had to cut. runewidth-aware so CJK halves don't sneak past.
func clipDisplay(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= n {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if used+w+1 > n { // +1 reserves room for "…"
			break
		}
		b.WriteRune(r)
		used += w
	}
	b.WriteRune('…')
	return b.String()
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
