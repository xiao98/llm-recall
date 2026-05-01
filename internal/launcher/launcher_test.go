package launcher

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TestBuild_Direct_Claude verifies the claude path produces a Direct mode
// recipe with `claude --resume <id>` and the session's CWD.
func TestBuild_Direct_Claude(t *testing.T) {
	s := adapter.Session{
		Source: "claude",
		ID:     "abc123",
		CWD:    `C:\Users\demo\proj`,
	}
	plan, err := Build(s)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if plan.Mode != adapter.ResumeDirect {
		t.Errorf("Mode: want Direct, got %v", plan.Mode)
	}
	want := []string{"claude", "--resume", "abc123"}
	if !equalStrings(plan.Argv, want) {
		t.Errorf("Argv: want %v, got %v", want, plan.Argv)
	}
	if plan.CWD != s.CWD {
		t.Errorf("CWD: got %q", plan.CWD)
	}
}

// TestBuild_Direct_Codex verifies the codex subcommand recipe.
func TestBuild_Direct_Codex(t *testing.T) {
	s := adapter.Session{
		Source: "codex",
		ID:     "uuid-codex",
		CWD:    "/proj",
	}
	plan, err := Build(s)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if plan.Mode != adapter.ResumeDirect {
		t.Errorf("Mode: want Direct, got %v", plan.Mode)
	}
	want := []string{"codex", "resume", "uuid-codex"}
	if !equalStrings(plan.Argv, want) {
		t.Errorf("Argv: want %v, got %v", want, plan.Argv)
	}
}

// TestBuild_Interactive_Gemini verifies gemini drops to Interactive mode and
// that a Hint string is populated.
func TestBuild_Interactive_Gemini(t *testing.T) {
	s := adapter.Session{
		Source: "gemini",
		ID:     "gem-uuid",
		CWD:    "/g",
	}
	plan, err := Build(s)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if plan.Mode != adapter.ResumeInteractive {
		t.Errorf("Mode: want Interactive, got %v", plan.Mode)
	}
	if !equalStrings(plan.Argv, []string{"gemini"}) {
		t.Errorf("Argv: want [gemini], got %v", plan.Argv)
	}
	if plan.Hint == "" {
		t.Errorf("Hint should be populated for Interactive mode")
	}
	if !strings.Contains(plan.Hint, s.ID) {
		t.Errorf("Hint should mention sessionId %q: %q", s.ID, plan.Hint)
	}
}

// TestRunPlan_DryRun_PrintsExecLine asserts the canonical `→ exec:` stdout
// line for every mode and confirms no child process is launched.
func TestRunPlan_DryRun_PrintsExecLine(t *testing.T) {
	cases := []struct {
		name string
		plan Plan
		want string
	}{
		{
			"direct",
			Plan{
				Argv:   []string{"claude", "--resume", "abc"},
				CWD:    `C:\proj`,
				Mode:   adapter.ResumeDirect,
				Source: "claude",
			},
			"→ exec: claude --resume abc in C:\\proj",
		},
		{
			"interactive",
			Plan{
				Argv:      []string{"gemini"},
				CWD:       "/g",
				Mode:      adapter.ResumeInteractive,
				Hint:      "→ 进入后请运行：/chat resume <tag> sid:xyz",
				Source:    "gemini",
				SessionID: "xyz",
			},
			"→ exec: gemini in /g",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			l := &Launcher{DryRun: true, Stdout: &out, Stderr: &out}
			code, err := l.RunPlan(&tc.plan)
			if err != nil {
				t.Fatalf("RunPlan: %v", err)
			}
			if code != 0 {
				t.Errorf("exit code: want 0, got %d", code)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("stdout missing %q:\n%s", tc.want, out.String())
			}
			if tc.plan.Mode == adapter.ResumeInteractive && !strings.Contains(out.String(), tc.plan.Hint) {
				t.Errorf("stdout should also contain hint %q:\n%s", tc.plan.Hint, out.String())
			}
		})
	}
}

// TestRunPlan_Unsupported asserts the no-exec, sessionId-only output.
func TestRunPlan_Unsupported(t *testing.T) {
	var out bytes.Buffer
	l := &Launcher{DryRun: true, Stdout: &out, Stderr: &out}
	plan := &Plan{
		Mode:      adapter.ResumeUnsupported,
		Source:    "fake",
		SessionID: "sid-1",
	}
	code, err := l.RunPlan(plan)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if !strings.Contains(out.String(), "fake 不支持 CLI resume") {
		t.Errorf("stdout missing unsupported line:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "sid-1") {
		t.Errorf("stdout missing sessionId:\n%s", out.String())
	}
}

// TestRunPlan_NotInPath asserts that an exec mode pointing at a missing
// binary returns 127 (and prints to stderr) without panicking.
func TestRunPlan_NotInPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	l := &Launcher{
		DryRun: false,
		Stdout: &stdout,
		Stderr: &stderr,
	}
	plan := &Plan{
		Argv:   []string{"this-binary-definitely-does-not-exist-xyz", "--foo"},
		CWD:    "",
		Mode:   adapter.ResumeDirect,
		Source: "fake",
	}
	code, err := l.RunPlan(plan)
	if err != nil {
		t.Fatalf("RunPlan: %v", err)
	}
	if code != 127 {
		t.Errorf("exit code: want 127, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not found in PATH") {
		t.Errorf("stderr missing not-in-PATH line:\n%s", stderr.String())
	}
}

// TestBuild_UnknownSource fails with an error rather than returning a bogus
// plan — callers must not silently dispatch to a missing adapter.
func TestBuild_UnknownSource(t *testing.T) {
	_, err := Build(adapter.Session{Source: "no-such-vendor", ID: "x", UpdatedAt: time.Now()})
	if err == nil {
		t.Errorf("expected error for unknown source")
	}
}

// TestRunRealExec_FakeHook verifies non-dry-run path under the FAKE_EXEC test
// hook: argv / cwd / source land in stderr exactly as constructed, and the
// process returns 0 instead of really spawning the child.
func TestRunRealExec_FakeHook(t *testing.T) {
	t.Setenv("LLM_RECALL_LAUNCHER_FAKE_EXEC", "1")
	cwd := t.TempDir() // must exist so chdir doesn't warn
	cases := []struct {
		name   string
		plan   Plan
		expect string
	}{
		{
			"claude_direct",
			Plan{Argv: []string{"claude", "--resume", "abc-123"}, CWD: cwd, Mode: adapter.ResumeDirect, Source: "claude"},
			"FAKE_EXEC argv=[claude --resume abc-123]",
		},
		{
			"codex_direct",
			Plan{Argv: []string{"codex", "resume", "x9"}, CWD: cwd, Mode: adapter.ResumeDirect, Source: "codex"},
			"FAKE_EXEC argv=[codex resume x9]",
		},
		{
			"gemini_interactive",
			Plan{Argv: []string{"gemini"}, CWD: cwd, Mode: adapter.ResumeInteractive, Hint: "→ 进入后请运行：/chat resume sid42", Source: "gemini", SessionID: "sid42"},
			"FAKE_EXEC argv=[gemini]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			l := &Launcher{DryRun: false, Stdout: &stdout, Stderr: &stderr}
			start := time.Now()
			code, err := l.RunPlan(&tc.plan)
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("RunPlan: %v", err)
			}
			if code != 0 {
				t.Errorf("exit code: want 0, got %d", code)
			}
			if !strings.Contains(stderr.String(), tc.expect) {
				t.Errorf("stderr missing %q:\n%s", tc.expect, stderr.String())
			}
			// Interactive mode pauses 1.5s before exec so the user can read
			// the hint; the other modes should not.
			if tc.plan.Mode == adapter.ResumeInteractive {
				if elapsed < 1400*time.Millisecond {
					t.Errorf("interactive: expected ≥ 1.4s pause before exec, got %v", elapsed)
				}
				if !strings.Contains(stderr.String(), tc.plan.Hint) {
					t.Errorf("interactive: hint should land on stderr:\n%s", stderr.String())
				}
			} else {
				if elapsed > 500*time.Millisecond {
					t.Errorf("non-interactive: unexpected delay %v", elapsed)
				}
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
