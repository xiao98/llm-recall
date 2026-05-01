// Test harness for the onboarding screen.
//
// Mirrors the W3 TUI harness pattern: when LLM_RECALL_ONBOARD_TEST_INPUT
// is set, drive the bubbletea program with synthetic key events and dump
// a snapshot to LLM_RECALL_ONBOARD_TEST_OUT.
//
// Script grammar (one char per action):
//
//	e   ⇒ press Enter (accept)
//	q   ⇒ press q (decline)
//	s   ⇒ snapshot (write current View() to OUT)
package promo

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type onboardHarness struct {
	script string
	out    string
}

func loadOnboardHarness() *onboardHarness {
	s := os.Getenv("LLM_RECALL_ONBOARD_TEST_INPUT")
	if s == "" {
		return nil
	}
	return &onboardHarness{
		script: s,
		out:    os.Getenv("LLM_RECALL_ONBOARD_TEST_OUT"),
	}
}

func (h *onboardHarness) drive(p *tea.Program, m *onboardingModel) {
	for _, c := range h.script {
		// Small delay so bubbletea's render loop catches up.
		time.Sleep(20 * time.Millisecond)
		switch c {
		case 'e':
			p.Send(tea.KeyMsg{Type: tea.KeyEnter})
		case 'q':
			p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		case 's':
			h.snapshot(m)
		}
	}
}

func (h *onboardHarness) snapshot(m *onboardingModel) {
	if h.out == "" {
		return
	}
	_ = os.WriteFile(h.out, []byte(m.View()), 0o600)
}
