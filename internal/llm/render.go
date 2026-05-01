// Lipgloss renderers for card / gold output.
//
// Two surfaces:
//
//   - RenderCard: rounded-border 80-col card with title bar, first user
//     message snippet, LLM-generated "在做：…" line, cwd footer, and
//     optional sponsored footer.
//   - RenderGold: rounded-border 80-col list of (idx, quote, comment)
//     triples, optional sponsored footer.
//   - RenderGoldMD: plain markdown list with NO ANSI / borders / footer
//     — for `gold --md > out.md` automation.
//
// We keep the colour palette in sync with internal/stats (orange +
// muted-grey) so the visual brand stays consistent across commands.
// Importing internal/stats would create a cycle (stats imports config,
// llm will eventually want config too); inlining the three constants
// is cheaper than a refactor.
package llm

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette: kept in sync with internal/stats/render.go. If you change one,
// change the other. (DEVDOC P0-5 will be authoritative once W7 lands.)
var (
	cardColAccent = lipgloss.Color("#FF6B35") // primary orange
	cardColMuted  = lipgloss.Color("#6B6B6B") // dim grey
	cardColText   = lipgloss.Color("#E0E0E0") // body text
)

// CardWidth is the on-card content width, exclusive of the border.
// 80 columns total (border + content); subtract 4 for L/R padding +
// border to get 76 internal cells.
const CardWidth = 80

// CardData is what cmd_card.go hands the renderer. The raw types are
// intentionally `string` (not adapter.Session) so this package stays
// free of cross-package coupling for testing.
type CardData struct {
	SessionID8 string // first 8 chars of session id
	Source     string // "claude" | "codex" | "gemini"
	When       string // formatted "yyyy-mm-dd hh:mm"
	FirstUser  string // first user message snippet (≤ ~200 chars)
	Action     string // LLM one-liner, "在做：…" prepended by caller
	CWD        string // shortened cwd
	Footer     string // sponsored line; empty disables
}

// RenderCard returns the full ANSI-styled card. Callers print to stdout
// directly; we do not write to os.Stdout here so testing stays trivial.
func RenderCard(d CardData) string {
	title := fmt.Sprintf("session %s · %s · %s", d.SessionID8, d.Source, d.When)

	var inner strings.Builder
	if d.FirstUser != "" {
		quoteStyle := lipgloss.NewStyle().Foreground(cardColText).Italic(true)
		inner.WriteString(quoteStyle.Render(`"` + d.FirstUser + `"`))
		inner.WriteString("\n\n")
	}
	if d.Action != "" {
		actionStyle := lipgloss.NewStyle().Foreground(cardColAccent).Bold(true)
		inner.WriteString(actionStyle.Render("在做："))
		inner.WriteString(lipgloss.NewStyle().Foreground(cardColText).Render(d.Action))
		inner.WriteString("\n\n")
	}
	if d.CWD != "" {
		inner.WriteString(lipgloss.NewStyle().Foreground(cardColMuted).Render("cwd: " + d.CWD))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cardColMuted).
		Padding(0, 2).
		Width(CardWidth - 2)

	header := lipgloss.NewStyle().Foreground(cardColAccent).Bold(true).Render("┄ " + title + " ┄")
	body := box.Render(inner.String())

	out := header + "\n" + body
	if d.Footer != "" {
		out += "\n" + lipgloss.NewStyle().Foreground(cardColMuted).Render("  "+d.Footer)
	}
	return out
}

// GoldEntry is one item in the Top-N list.
type GoldEntry struct {
	Quote   string
	Comment string
}

// GoldData is the renderable bundle for `gold`.
type GoldData struct {
	WindowDays int
	Entries    []GoldEntry
	Footer     string
}

// RenderGold returns the lipgloss-styled rounded-border listing.
func RenderGold(d GoldData) string {
	title := fmt.Sprintf("你的 %d 天金句 Top %d", d.WindowDays, len(d.Entries))

	var inner strings.Builder
	quoteStyle := lipgloss.NewStyle().Foreground(cardColText).Bold(true)
	commentStyle := lipgloss.NewStyle().Foreground(cardColMuted)
	for i, e := range d.Entries {
		if i > 0 {
			inner.WriteString("\n")
		}
		inner.WriteString(fmt.Sprintf(" %2d.  ", i+1))
		inner.WriteString(quoteStyle.Render(e.Quote))
		inner.WriteString("\n")
		inner.WriteString("      → ")
		inner.WriteString(commentStyle.Render(e.Comment))
		inner.WriteString("\n")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cardColMuted).
		Padding(0, 1).
		Width(CardWidth - 2)

	header := lipgloss.NewStyle().Foreground(cardColAccent).Bold(true).Render("┄ " + title + " ┄")
	body := box.Render(strings.TrimRight(inner.String(), "\n"))

	out := header + "\n" + body
	if d.Footer != "" {
		out += "\n" + lipgloss.NewStyle().Foreground(cardColMuted).Render("  "+d.Footer)
	}
	return out
}

// RenderGoldMD returns plain markdown for `gold --md`. No ANSI, no
// border, no footer. Each entry is `N. **quote**\n   - comment\n`.
func RenderGoldMD(d GoldData) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 你的 %d 天金句 Top %d\n\n", d.WindowDays, len(d.Entries)))
	for i, e := range d.Entries {
		b.WriteString(fmt.Sprintf("%d. **%s**\n   - %s\n", i+1, e.Quote, e.Comment))
	}
	return b.String()
}

// SafeTrunc returns s truncated to ≤ maxRunes runes (not bytes), with
// an ellipsis suffix when truncation occurred. Preserves CJK widths
// because we measure in runes.
func SafeTrunc(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	if maxRunes < 1 {
		return "…"
	}
	return string(r[:maxRunes-1]) + "…"
}
