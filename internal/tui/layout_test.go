package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/mattn/go-runewidth"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/search"
)

// newTestModel mirrors the New() factory but skips the DB. Layout math is
// pure — it doesn't touch SQLite — so we can assert on it without a real
// cache.
func newTestModel() *Model {
	in := textinput.New()
	in.SetValue("")
	pv := viewport.New(80, 10)
	cfg := &config.Config{}
	cfg.Promo.NoPromo = true // shut off promo so banner=0, footer=0
	return &Model{
		cfg:     Config{Promo: cfg},
		input:   in,
		preview: pv,
	}
}

// TestRecomputeLayout walks the four reference sizes from the HOTFIX spec
// and asserts the slot widths/heights match the documented budget.
//
// Budget (with NoPromo so banner=0, searchFooter=0):
//
//	listH = height - inputBarH(1) - inputBorderH(2) - listBorders(2) - footerH(1)
//	      = height - 6
//	listW = width / 2
//	previewW = width - listW
func TestRecomputeLayout(t *testing.T) {
	cases := []struct {
		name                               string
		w, h                               int
		wantListW, wantListH, wantPreviewW int
		wantTooSmall                       bool
	}{
		// 100×30 dev-default: roomy.
		{"100x30", 100, 30, 50, 24, 50, false},
		// 80×24 HN screenshot baseline.
		{"80x24", 80, 24, 40, 18, 40, false},
		// 60×16 minimum supported size.
		{"60x16", 60, 16, 30, 10, 30, false},
		// 50×12 below floor → tooSmall fallback, all slots zero.
		{"50x12", 50, 12, 0, 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestModel()
			m.width = c.w
			m.height = c.h
			m.tooSmall = c.w < minTermWidth || c.h < minTermHeight
			m.relayout()
			if m.tooSmall != c.wantTooSmall {
				t.Errorf("tooSmall: got %v want %v", m.tooSmall, c.wantTooSmall)
			}
			if m.listW != c.wantListW {
				t.Errorf("listW: got %d want %d", m.listW, c.wantListW)
			}
			if m.listH != c.wantListH {
				t.Errorf("listH: got %d want %d", m.listH, c.wantListH)
			}
			if m.previewW != c.wantPreviewW {
				t.Errorf("previewW: got %d want %d", m.previewW, c.wantPreviewW)
			}
		})
	}
}

// TestTooSmallView asserts the fallback string contains the floor and the
// actual size so the user knows exactly how much to enlarge.
func TestTooSmallView(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 50, 12
	m.tooSmall = true
	m.relayout()
	out := m.View()
	if !strings.Contains(out, "terminal too small") {
		t.Errorf("missing too-small banner; got %q", out)
	}
	if !strings.Contains(out, "60") || !strings.Contains(out, "16") {
		t.Errorf("missing minimum size in fallback; got %q", out)
	}
	if !strings.Contains(out, "50") || !strings.Contains(out, "12") {
		t.Errorf("missing actual size in fallback; got %q", out)
	}
}

// TestFormatRowSingleLine asserts every rendered row is exactly one line
// and exactly `width` display columns wide. This guards the bug where W3
// rendered metadata + title on two separate rows.
func TestFormatRowSingleLine(t *testing.T) {
	s := adapter.Session{
		Source:    "claude",
		ID:        "26348a6c-154a-4efc-958b-bee80e8a4bdc",
		CWD:       `C:\Users\肖浩\llm-recall`,
		UpdatedAt: time.Date(2026, 5, 1, 12, 17, 0, 0, time.Local),
		Title:     "claude code的历史会话管理太垃圾了，开搞 cli 工具",
	}
	for _, w := range []int{40, 60, 80, 120} {
		row := formatRow(s, w)
		if strings.Contains(row, "\n") {
			t.Errorf("width=%d: row contains newline (double-line bug): %q", w, row)
		}
		if got := runewidth.StringWidth(row); got != w {
			t.Errorf("width=%d: got display width %d", w, got)
		}
	}
}

// TestClampScrollKeepsSelectedVisible exercises the scroll window math
// across boundary cases: scrolling past the bottom, jumping to the top,
// and shrinking the viewport via resize.
func TestClampScrollKeepsSelectedVisible(t *testing.T) {
	m := newTestModel()
	m.results = make([]search.Result, 200)
	m.width, m.height = 100, 30
	m.relayout()
	// Walk down past the listH boundary; scrollOffset should track.
	for i := 0; i < 50; i++ {
		m.selected = i
		m.clampScroll()
		if m.selected < m.scrollOffset || m.selected >= m.scrollOffset+m.listH {
			t.Fatalf("selected=%d out of [%d,%d)", m.selected, m.scrollOffset, m.scrollOffset+m.listH)
		}
	}
	// Resize to a shorter window — the selected row must still be in view.
	m.height = 18
	m.relayout()
	m.clampScroll()
	if m.selected < m.scrollOffset || m.selected >= m.scrollOffset+m.listH {
		t.Fatalf("after resize: selected=%d out of [%d,%d)", m.selected, m.scrollOffset, m.scrollOffset+m.listH)
	}
	// Jump back to the top: scrollOffset should reset to 0.
	m.selected = 0
	m.clampScroll()
	if m.scrollOffset != 0 {
		t.Fatalf("scrollOffset after selecting [0]: %d", m.scrollOffset)
	}
}
