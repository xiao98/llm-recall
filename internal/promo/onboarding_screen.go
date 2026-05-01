// Onboarding consent screen — single bubbletea program.
//
// This is the literal text from TASKS-W6.md §4, copied verbatim. Per the
// task spec, do NOT "improve" the wording — the策划方 vetted it for
// transparency / compliance, and changing words here makes the
// onboarding contract diverge from what the user agreed to.
//
// The widget is intentionally a *separate* bubbletea program, not a
// state in the W3 search Model. Reasons:
//
//  1. Easier to gate at main(): "if not accepted, run this; on success,
//     proceed". Folding it into the search Model would require a
//     special "consent screen" state and complicate the test harness.
//  2. The screen has a single unique exit code — Enter ⇒ accept, q ⇒
//     decline + exit. Bubbletea's Cmd model is overkill but harmless.
//  3. Re-running it later (`llm-recall onboarding`) is a one-liner.
package promo

import (
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// onboardingTextLines is the content area, line by line. Borders /
// padding are added by the renderer so the visible width auto-adapts to
// CJK runewidth.
//
// IMPORTANT: lines are checked-in verbatim from TASKS-W6.md §4. Do not
// reflow / translate / abbreviate.
var onboardingTextLines = []string{
	" 跨厂商 LLM CLI 会话搜索 + 恢复终端工具",
	"",
	" Sponsored by YCAPI (https://api.youchun.tech)",
	" Homepage: https://recall.youchun.tech",
	"",
	" 营销注入说明（你看到的所有 YCAPI 痕迹）：",
	"   • 启动时顶栏一条金句 banner，5% 概率含加群链接",
	"   • stats 命令底部一行 sponsored 字符串",
	"   • （可选）搜索结果底部讨论关联条",
	"   • gold 功能用你自己的 LLM API key，不走 YCAPI 网关",
	"",
	" 关闭方式：",
	"   --no-promo               关 banner / footer / sponsored",
	"   config.toml              细粒度调（详见 README）",
	"",
	" Enter 接受继续， q 退出",
}

// onboardingTitle is the boxed top line. Kept separate so the border
// renderer can slot it into the corner.
const onboardingTitle = " Welcome to llm-recall "

// onboardingModel is the bubbletea state for the consent screen. We keep
// it small — width is captured for centering, accepted is the result.
type onboardingModel struct {
	width    int
	height   int
	accepted bool
	done     bool

	// harness drives the program in tests; nil in production.
	harness *onboardHarness
}

func (m *onboardingModel) Init() tea.Cmd { return nil }

func (m *onboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.accepted = true
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.accepted = false
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *onboardingModel) View() string {
	if m.width == 0 {
		// Pre-resize: render at a sensible default so the harness
		// snapshot still includes the box.
		m.width = 80
	}
	return renderOnboardingBox(m.width)
}

// renderOnboardingBox draws the bordered consent box. We compute an
// inner width from the longest line (runewidth-aware) so CJK widths
// don't desync the border. The screen-width parameter is only used for
// optional horizontal centering.
func renderOnboardingBox(screenW int) string {
	innerW := 0
	for _, l := range onboardingTextLines {
		if w := runewidth.StringWidth(l); w > innerW {
			innerW = w
		}
	}
	// Title sits inside the top border line, so factor that into
	// innerW too.
	if w := runewidth.StringWidth(onboardingTitle); w > innerW {
		innerW = w
	}
	// Visual breathing room.
	innerW += 2

	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(innerW)

	var b strings.Builder
	b.WriteString(onboardingTitle)
	b.WriteString("\n")
	for i, l := range onboardingTextLines {
		// runewidth-aware padding so trailing border (when style adds
		// it) lines up across CJK / ASCII rows. lipgloss handles this
		// when Width() is set on the style, but the manual pad is a
		// belt-and-suspenders against terminal differences.
		pad := innerW - runewidth.StringWidth(l) - 2 // -2 for left/right padding
		if pad < 0 {
			pad = 0
		}
		b.WriteString(l + strings.Repeat(" ", pad))
		if i < len(onboardingTextLines)-1 {
			b.WriteString("\n")
		}
	}
	box := style.Render(b.String())

	// Optional centering: only if the screen is meaningfully wider than
	// the box. Saves an awkward sliver of left-aligned text on tiny
	// terminals.
	if screenW > innerW+8 {
		box = lipgloss.PlaceHorizontal(screenW, lipgloss.Center, box)
	}
	return box
}

// RunOnboarding launches the bubbletea program and returns whether the
// user accepted. Errors (e.g. bubbletea init failure) print to stderr
// and return false — refusing to consent on error keeps the privacy
// posture conservative.
func RunOnboarding() bool {
	m := &onboardingModel{harness: loadOnboardHarness()}
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if m.harness != nil {
		opts = []tea.ProgramOption{tea.WithoutSignalHandler(), tea.WithInput(eofReader{})}
	}
	p := tea.NewProgram(m, opts...)
	if m.harness != nil {
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Send(tea.WindowSizeMsg{Width: 100, Height: 30})
			m.harness.drive(p, m)
		}()
	}
	if _, err := p.Run(); err != nil {
		// Print and decline. The caller (main) then exits 0.
		_, _ = io.WriteString(os.Stderr, "warn: onboarding: "+err.Error()+"\n")
		return false
	}
	return m.accepted
}

// eofReader is a stdin shim for harness mode (mirrors the W3/W5 pattern).
type eofReader struct{}

func (eofReader) Read(p []byte) (int, error) { return 0, io.EOF }
