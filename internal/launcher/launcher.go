// Package launcher executes the resume recipe an adapter produces.
//
// One Launcher implements three modes:
//
//   - ResumeDirect: chdir to session.CWD, exec argv. The vendor CLI re-enters
//     the picked session automatically. Claude / Codex.
//   - ResumeInteractive: chdir + exec argv, but argv only opens the CLI; the
//     launcher prints a one-line hint telling the user the slash command to
//     run inside. Gemini.
//   - ResumeUnsupported: don't exec anything; print the sessionId so the user
//     can copy/paste it into the vendor's own picker.
//
// DryRun=true short-circuits all three to a single stdout line and returns,
// for the TUI's default "show me what would happen" UX.
package launcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// Launcher carries the user-visible toggles. Stdout/Stderr are exported so
// tests can capture them; production code leaves them at os.Stdout/os.Stderr.
type Launcher struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	// Stdin is rarely overridden, but tests need to be able to feed nothing
	// without inheriting the real terminal stdin.
	Stdin io.Reader
}

// New builds a Launcher wired to os.Stdin/Stdout/Stderr by default.
func New(dryRun bool) *Launcher {
	return &Launcher{
		DryRun: dryRun,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Plan is a parsed resume recipe — ready for either Run or DryRun output.
// Returned by Build so tests can assert on it without spawning processes.
type Plan struct {
	Argv []string
	CWD  string
	Mode adapter.ResumeMode
	// Hint is the additional one-liner the user sees in Interactive mode
	// (printed after the `→ exec:` line). Empty for the other modes.
	Hint string
	// SessionID is carried for the Unsupported path (used in the printed line).
	SessionID string
	// Source for human-readable error / hint messages.
	Source string
}

// Build asks the matching adapter for the resume recipe, normalises the
// resulting Plan, and is the only place that knows about ResumeMode-specific
// hint text. Building is pure (no fs / process side-effects) — useful for
// tests and for the TUI's "preview the action" line.
func Build(s adapter.Session) (*Plan, error) {
	a, err := findAdapter(s.Source)
	if err != nil {
		return nil, err
	}
	argv, cwd, mode, err := a.ResumeCommand(s)
	if err != nil {
		return &Plan{Argv: argv, CWD: cwd, Mode: ResumeUnsupportedFallback(mode), SessionID: s.ID, Source: s.Source}, err
	}
	p := &Plan{
		Argv:      argv,
		CWD:       cwd,
		Mode:      mode,
		SessionID: s.ID,
		Source:    s.Source,
	}
	if mode == adapter.ResumeInteractive {
		p.Hint = interactiveHint(s)
	}
	return p, nil
}

// ResumeUnsupportedFallback reduces an unknown / error mode to Unsupported so
// callers don't have to special-case nil-vs-zero.
func ResumeUnsupportedFallback(m adapter.ResumeMode) adapter.ResumeMode {
	if m == adapter.ResumeDirect || m == adapter.ResumeInteractive || m == adapter.ResumeUnsupported {
		return m
	}
	return adapter.ResumeUnsupported
}

// findAdapter resolves source name → SessionAdapter. We index by Name() rather
// than by type so this stays correct when launcher_test swaps in stubs.
func findAdapter(name string) (adapter.SessionAdapter, error) {
	for _, a := range registeredAdapters() {
		if a.Name() == name {
			return a, nil
		}
	}
	return nil, fmt.Errorf("launcher: unknown source %q", name)
}

// registeredAdapters is var-shaped so tests can override it with stubs.
var registeredAdapters = func() []adapter.SessionAdapter {
	return []adapter.SessionAdapter{
		adapter.NewClaude(),
		adapter.NewCodex(),
		adapter.NewGemini(),
	}
}

// interactiveHint phrases the post-launch instruction. Today only gemini
// hits this path: list-sessions then `--resume <index>`, plus the in-app
// `/chat resume <tag>` slash-command as a manual fallback. We keep the line
// short enough to read at a glance.
func interactiveHint(s adapter.Session) string {
	if s.Source == "gemini" {
		return fmt.Sprintf("→ 进入后请运行：/chat resume <tag>  或先 `gemini --list-sessions` 找到 sessionId %s 的索引", s.ID)
	}
	return fmt.Sprintf("→ 进入后请运行：/chat resume <tag> (sessionId: %s)", s.ID)
}

// Run executes the plan associated with `s`. Returns the exit status the
// caller should propagate to os.Exit (0 on dry-run / hint-only paths, the
// child's exit code on real exec, 127 on "cli not in PATH").
func (l *Launcher) Run(s adapter.Session) (int, error) {
	plan, err := Build(s)
	if err != nil {
		return 1, err
	}
	return l.RunPlan(plan)
}

// RunPlan is the same as Run but takes a pre-built Plan. Used by tests so they
// can hand-craft the recipe without going through Build → adapter registry.
func (l *Launcher) RunPlan(plan *Plan) (int, error) {
	out := l.Stdout
	if out == nil {
		out = os.Stdout
	}
	errw := l.Stderr
	if errw == nil {
		errw = os.Stderr
	}

	switch plan.Mode {
	case adapter.ResumeUnsupported:
		fmt.Fprintf(out, "%s 不支持 CLI resume，sessionId: %s\n", plan.Source, plan.SessionID)
		return 0, nil
	case adapter.ResumeInteractive, adapter.ResumeDirect:
		// Common path: print the exec line, then either return (dry-run) or exec.
		fmt.Fprintf(out, "→ exec: %s in %s\n", joinArgv(plan.Argv), plan.CWD)
		if plan.Mode == adapter.ResumeInteractive && plan.Hint != "" {
			// Print the hint to stderr so it survives stdout redirection,
			// and pause briefly so the child REPL doesn't paint over it
			// before the user has read the sessionId.
			fmt.Fprintln(errw, "")
			fmt.Fprintln(errw, plan.Hint)
			fmt.Fprintln(errw, "")
		}
		if l.DryRun {
			return 0, nil
		}
		if plan.Mode == adapter.ResumeInteractive {
			time.Sleep(1500 * time.Millisecond)
		}
		return l.exec(plan, errw)
	}
	return 1, fmt.Errorf("launcher: unknown mode %d", plan.Mode)
}

// exec performs the actual chdir + child-process spawn. We always go through
// exec.Command rather than syscall.Exec so the same code path works on
// Windows (no syscall.Exec) and so tests can intercept via Stdout/Stderr.
//
// chdir is best-effort: a missing/inaccessible cwd warns and proceeds in the
// current directory rather than aborting. The vendor CLI may still pick up
// the original cwd via its own session metadata.
func (l *Launcher) exec(plan *Plan, errw io.Writer) (int, error) {
	if len(plan.Argv) == 0 {
		return 1, fmt.Errorf("launcher: empty argv")
	}
	// Test hook. When LLM_RECALL_LAUNCHER_FAKE_EXEC is set, write a one-line
	// trace to errw and return 0 instead of really spawning the child. Lets
	// integration tests assert argv/cwd construction without owning a real
	// claude/codex/gemini binary.
	if os.Getenv("LLM_RECALL_LAUNCHER_FAKE_EXEC") != "" {
		fmt.Fprintf(errw, "FAKE_EXEC argv=%v cwd=%s source=%s\n", plan.Argv, plan.CWD, plan.Source)
		return 0, nil
	}
	// Resolve the binary via PATH (and PATHEXT on Windows, so `claude` finds
	// `claude.cmd`). If it's not there, exit 127 to match the shell convention.
	binPath, err := exec.LookPath(plan.Argv[0])
	if err != nil {
		fmt.Fprintf(errw, "%s not found in PATH; install it to resume %s sessions\n", plan.Argv[0], plan.Source)
		return 127, nil
	}

	if plan.CWD != "" {
		if fi, statErr := os.Stat(plan.CWD); statErr != nil || !fi.IsDir() {
			fmt.Fprintf(errw, "warn: cwd %q missing/inaccessible, launching in current directory\n", plan.CWD)
		} else {
			if err := os.Chdir(plan.CWD); err != nil {
				fmt.Fprintf(errw, "warn: chdir %q: %v\n", plan.CWD, err)
			}
		}
	}

	cmd := exec.Command(binPath, plan.Argv[1:]...)
	cmd.Stdin = l.Stdin
	cmd.Stdout = l.Stdout
	cmd.Stderr = l.Stderr
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		// Pull out the child's exit code if we have one; otherwise propagate
		// 1 plus the error so the caller can log.
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return 1, err
	}
	return cmd.ProcessState.ExitCode(), nil
}

// joinArgv stringifies argv for the human-readable `→ exec:` line. We don't
// shell-escape — the line is purely informational, not meant to be re-run
// in a shell. (Quoting would obscure paths with spaces more than help.)
func joinArgv(argv []string) string {
	if len(argv) == 0 {
		return ""
	}
	out := argv[0]
	for _, a := range argv[1:] {
		out += " " + a
	}
	return out
}

// onWindows is a tiny helper kept for documentation purposes — exec.LookPath
// already does the right thing on both OSes, but tests assert behaviour
// that's only meaningful on Windows (PATHEXT lookup).
func onWindows() bool { return runtime.GOOS == "windows" }
