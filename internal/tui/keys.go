package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// keyAction is the parsed intent of a single tea.KeyMsg. Centralising this
// here keeps Update small and lets the test harness drive the same code path
// without depending on terminal-specific key codes.
type keyAction int

const (
	keyNone keyAction = iota
	keyQuit
	keyEnter
	keyDown
	keyUp
	keyPageDown
	keyPageUp
	keyTextInput // not literally a key; means "let the input field consume this"
)

// classify maps a tea.KeyMsg to a keyAction. Arrow keys and Ctrl-N/P move
// the list; PageUp/PageDown scroll the preview pane; Enter selects; Esc and
// Ctrl-C quit. Everything else is delegated to the search box.
func classify(k tea.KeyMsg) keyAction {
	switch k.String() {
	case "esc", "ctrl+c":
		return keyQuit
	case "enter":
		return keyEnter
	case "down", "ctrl+n":
		return keyDown
	case "up", "ctrl+p":
		return keyUp
	case "pgdown", "ctrl+f":
		return keyPageDown
	case "pgup", "ctrl+b":
		return keyPageUp
	}
	return keyTextInput
}
