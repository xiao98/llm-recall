// Test harness for the TUI.
//
// When the env var LLM_RECALL_TEST_INPUT is non-empty, the harness:
//
//  1. Treats the variable's value as a script of literal characters to type
//     into the search box. Two escape sequences are recognised:
//     \n  → press Enter (selects the current result and quits)
//     \e  → press Esc   (quits without selecting)
//     Any other backslash is rendered literally.
//  2. Replays the script one rune at a time via tea.Program.Send so the
//     bubbletea loop processes each keystroke through the same path as a
//     real terminal event. Inter-keystroke delay > debounceWindow so the
//     search command for keystroke N lands before keystroke N+1 fires.
//  3. After the script finishes (or hits an explicit \n / \e), writes a
//     debugSnapshot() to the file path in LLM_RECALL_TEST_OUTPUT (or stderr
//     when unset) and tells the program to quit. The snapshot is plain
//     text so the W3 acceptance suite can assert with simple grep.
//
// This is the cleanest way to validate the live-search flow non-interactively
// without rebuilding bubbletea's input handling. It runs only when the env
// var is set, so production users never see it.
package tui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// keystrokeDelay is the wait between scripted keystrokes. Must comfortably
// exceed debounceWindow (50 ms) so the search command for keystroke N lands
// before keystroke N+1 fires. 120 ms is a safe middle ground on a busy CI.
const keystrokeDelay = 120 * time.Millisecond

// terminalDelay is the wait after the script finishes before snapshot+quit.
// Gives the final searchMsg time to arrive. fuzzy.FindFrom on a few hundred
// large bodies can take ~1s on a hot laptop; we wait long enough that the
// last keystroke's search has definitely landed.
const terminalDelay = 1500 * time.Millisecond

type harness struct {
	script  []scriptStep
	outPath string
}

type scriptStep struct {
	kind  scriptKind
	runes []rune
}

type scriptKind int

const (
	scriptInput scriptKind = iota
	scriptEnter
	scriptEsc
	scriptDown     // \d → down arrow
	scriptUp       // \u → up arrow
	scriptPageDown // \D → page down
	scriptPageUp   // \U → page up
	scriptDone     // sentinel: dump snapshot + quit
)

// loadHarness inspects the env and returns a configured harness, or nil if
// the env var is unset.
func loadHarness() *harness {
	raw := os.Getenv("LLM_RECALL_TEST_INPUT")
	if raw == "" {
		return nil
	}
	return &harness{
		script:  parseScript(raw),
		outPath: os.Getenv("LLM_RECALL_TEST_OUTPUT"),
	}
}

// parseScript turns the env var into a step list. Walking rune-by-rune so
// CJK input ("飞书") is preserved exactly as typed.
func parseScript(s string) []scriptStep {
	var steps []scriptStep
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			steps = append(steps, scriptStep{kind: scriptInput, runes: append([]rune(nil), buf...)})
			buf = buf[:0]
		}
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' && i+1 < len(rs) {
			next := rs[i+1]
			switch next {
			case 'n':
				flush()
				steps = append(steps, scriptStep{kind: scriptEnter})
				i++
				continue
			case 'e':
				flush()
				steps = append(steps, scriptStep{kind: scriptEsc})
				i++
				continue
			case 'd':
				flush()
				steps = append(steps, scriptStep{kind: scriptDown})
				i++
				continue
			case 'u':
				flush()
				steps = append(steps, scriptStep{kind: scriptUp})
				i++
				continue
			case 'D':
				flush()
				steps = append(steps, scriptStep{kind: scriptPageDown})
				i++
				continue
			case 'U':
				flush()
				steps = append(steps, scriptStep{kind: scriptPageUp})
				i++
				continue
			case '\\':
				buf = append(buf, '\\')
				i++
				continue
			}
		}
		buf = append(buf, r)
	}
	flush()
	steps = append(steps, scriptStep{kind: scriptDone})
	return steps
}

// drive runs the script in a goroutine, posting tea.KeyMsgs and tea.QuitMsgs
// to the program. Called from Run() right after tea.NewProgram. The
// goroutine exits when scriptDone / scriptEsc / scriptEnter has fired.
func (h *harness) drive(p *tea.Program, m *Model) {
	go func() {
		for _, step := range h.script {
			switch step.kind {
			case scriptInput:
				for _, r := range step.runes {
					// textinput recognises KeyRunes for printable chars and
					// KeySpace for the single ' ' code point. Sending a
					// space as KeyRunes is silently dropped.
					if r == ' ' {
						p.Send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{r}})
					} else {
						p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
					}
					time.Sleep(keystrokeDelay)
				}
			case scriptDown:
				p.Send(tea.KeyMsg{Type: tea.KeyDown})
				time.Sleep(keystrokeDelay)
			case scriptUp:
				p.Send(tea.KeyMsg{Type: tea.KeyUp})
				time.Sleep(keystrokeDelay)
			case scriptPageDown:
				p.Send(tea.KeyMsg{Type: tea.KeyPgDown})
				time.Sleep(keystrokeDelay)
			case scriptPageUp:
				p.Send(tea.KeyMsg{Type: tea.KeyPgUp})
				time.Sleep(keystrokeDelay)
			case scriptEnter:
				time.Sleep(terminalDelay) // let last search settle
				h.dumpSnapshot(m)
				p.Send(tea.KeyMsg{Type: tea.KeyEnter})
				return
			case scriptEsc:
				time.Sleep(terminalDelay)
				h.dumpSnapshot(m)
				p.Send(tea.KeyMsg{Type: tea.KeyEsc})
				return
			case scriptDone:
				time.Sleep(terminalDelay)
				h.dumpSnapshot(m)
				p.Quit()
				return
			}
		}
	}()
}

// dumpSnapshot writes the model's debug state to outPath / stderr. Errors are
// best-effort: a missing directory or permission denial just falls back to
// stderr so the verifier can still see something.
//
// When LLM_RECALL_TEST_VIEW=1 we also append the live View() output (with
// ANSI stripped to keep grep happy). Useful for asserting "list item is
// single-line" / "tooSmall fallback rendered".
func (h *harness) dumpSnapshot(m *Model) {
	out := m.debugSnapshot()
	if os.Getenv("LLM_RECALL_TEST_VIEW") != "" {
		out += "---VIEW---\n" + stripANSI(m.View()) + "\n"
	}
	if h.outPath != "" {
		if err := os.WriteFile(h.outPath, []byte(out), 0o644); err == nil {
			return
		}
	}
	fmt.Fprint(os.Stderr, "---SNAPSHOT---\n"+out+"---END---\n")
}

// stripANSI removes all ANSI CSI/SGR escape sequences from s. We only strip
// the "ESC [ … letter" form; that's everything lipgloss emits today. A
// dedicated dependency is overkill for this single test-only path.
func stripANSI(s string) string {
	var b []byte
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				if c >= 0x40 && c <= 0x7e {
					j++
					break
				}
				j++
			}
			i = j
			continue
		}
		b = append(b, s[i])
		i++
	}
	return string(b)
}
