// Test harness for the stats Model. Same pattern as internal/tui/harness.go:
//
//   - LLM_RECALL_STATS_TEST_INPUT="123\eq" → script of keys to send. Each
//     non-special character is sent as-is. Escape forms:
//     \n  Enter
//     \e  Esc
//     \r  Right arrow
//     \l  Left arrow
//     \q  the literal 'q' (not really needed; here for symmetry)
//   - LLM_RECALL_STATS_TEST_OUTPUT=<path> → write a snapshot of the rendered
//     view + the cached Stats per tab to that path, JSON-encoded so the
//     verifier can grep without ANSI parsing.
//
// Production users never see this — env vars are off by default.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const renderKeystrokeDelay = 80 * time.Millisecond
const renderTerminalDelay = 200 * time.Millisecond

type renderHarness struct {
	script  []renderStep
	outPath string
}

type renderStep struct {
	kind renderKind
	r    rune
}

type renderKind int

const (
	rkRune renderKind = iota
	rkEnter
	rkEsc
	rkRight
	rkLeft
	rkDone
)

func loadRenderHarness() *renderHarness {
	raw := os.Getenv("LLM_RECALL_STATS_TEST_INPUT")
	if raw == "" {
		return nil
	}
	return &renderHarness{
		script:  parseRenderScript(raw),
		outPath: os.Getenv("LLM_RECALL_STATS_TEST_OUTPUT"),
	}
}

func parseRenderScript(s string) []renderStep {
	var out []renderStep
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' && i+1 < len(rs) {
			switch rs[i+1] {
			case 'n':
				out = append(out, renderStep{kind: rkEnter})
				i++
				continue
			case 'e':
				out = append(out, renderStep{kind: rkEsc})
				i++
				continue
			case 'r':
				out = append(out, renderStep{kind: rkRight})
				i++
				continue
			case 'l':
				out = append(out, renderStep{kind: rkLeft})
				i++
				continue
			case '\\':
				out = append(out, renderStep{kind: rkRune, r: '\\'})
				i++
				continue
			}
		}
		out = append(out, renderStep{kind: rkRune, r: r})
	}
	out = append(out, renderStep{kind: rkDone})
	return out
}

func (h *renderHarness) drive(p *tea.Program, m *Model) {
	for _, step := range h.script {
		switch step.kind {
		case rkRune:
			if step.r == ' ' {
				p.Send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{step.r}})
			} else {
				p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{step.r}})
			}
			time.Sleep(renderKeystrokeDelay)
		case rkRight:
			p.Send(tea.KeyMsg{Type: tea.KeyRight})
			time.Sleep(renderKeystrokeDelay)
		case rkLeft:
			p.Send(tea.KeyMsg{Type: tea.KeyLeft})
			time.Sleep(renderKeystrokeDelay)
		case rkEnter:
			p.Send(tea.KeyMsg{Type: tea.KeyEnter})
			time.Sleep(renderKeystrokeDelay)
		case rkEsc:
			time.Sleep(renderTerminalDelay)
			h.dumpSnapshot(m)
			p.Send(tea.KeyMsg{Type: tea.KeyEsc})
			return
		case rkDone:
			time.Sleep(renderTerminalDelay)
			h.dumpSnapshot(m)
			p.Quit()
			return
		}
	}
}

func (h *renderHarness) dumpSnapshot(m *Model) {
	snap := struct {
		Tab         string           `json:"tab"`
		HeatmapCols int              `json:"heatmap_cols"`
		MonthLabels []string         `json:"month_labels"`
		StatsPerTab map[string]Stats `json:"stats_per_tab"`
		ViewSample  string           `json:"view_sample"` // first 300 chars of View()
	}{
		Tab:         m.tab.Label(),
		HeatmapCols: m.heatmap.Cols,
		MonthLabels: m.heatmap.MonthLabels,
		StatsPerTab: map[string]Stats{},
	}
	for k, v := range m.stats {
		snap.StatsPerTab[k.Label()] = v
	}
	v := m.View()
	if len([]rune(v)) > 300 {
		snap.ViewSample = string([]rune(v)[:300])
	} else {
		snap.ViewSample = v
	}

	data, _ := json.MarshalIndent(snap, "", "  ")
	if h.outPath != "" {
		if err := os.WriteFile(h.outPath, data, 0o644); err == nil {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "---STATS-SNAPSHOT---\n%s\n---END---\n", data)
}
