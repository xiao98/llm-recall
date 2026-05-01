// W5 stats subcommand: aggregate cached sessions over a window, hand the
// payload to the Python imggen backend, drop two PNGs (square + vertical)
// onto disk under ~/Pictures/llm-recall/.
//
// We reuse the W2/W3 cache (no re-scan of jsonl files at command start) so
// the round-trip stays fast even with thousands of sessions: jsonl files
// are only re-read for token counts, and only once per session in window.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/imggen"
	"github.com/xiao98/llm-recall/internal/index"
	"github.com/xiao98/llm-recall/internal/stats"
)

// defaultBackendURL is the post-W5-deploy default. Local dev passes
// `--backend http://localhost:8001`. Once the user owns the deploy box this
// constant moves into a config file (W6 territory) — for now it's hard-
// coded so the CLI works out of the box once they ship it.
const defaultBackendURL = "http://216.144.229.139:8001"

// tokenFallbackPerMsg is the multiplier for sessions whose jsonl produced
// zero tokens (malformed file, missing fields, very old session). The
// TOKEN-AUDIT.md proves all three vendors expose tokens, so this is just
// a safety net. Conservative — biased low so we don't over-promise.
const tokenFallbackPerMsg = 3

func cmdStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	days := fs.Int("days", 30, "window size in days")
	backend := fs.String("backend", defaultBackendURL, "imggen backend base URL")
	noWatermark := fs.Bool("no-watermark", false, "disable the corner watermark")
	template := fs.String("template", "v1", "template to render: v1|v2|v3")
	noOpen := fs.Bool("no-open", false, "skip the 'open in explorer' prompt")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *days <= 0 {
		fmt.Fprintln(os.Stderr, "error: --days must be > 0")
		os.Exit(1)
	}
	switch *template {
	case "v1", "v2", "v3":
	default:
		fmt.Fprintf(os.Stderr, "error: --template must be v1|v2|v3, got %q\n", *template)
		os.Exit(1)
	}

	// 1. Refresh the cache so any newly-arrived sessions show up.
	if _, err := index.DiscoverAll(context.Background(), index.Options{
		UseCache: true,
		NeedBody: true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warn: discover: %v\n", err)
	}

	// 2. Read every cached row across all sources.
	dbPath, err := index.CacheDBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cache path: %v\n", err)
		os.Exit(1)
	}
	cache, err := index.OpenCache(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open cache: %v\n", err)
		os.Exit(1)
	}
	defer cache.Close()

	var sessions []adapter.Session
	for _, src := range []string{"claude", "codex", "gemini"} {
		rows, err := cache.ListBySource(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: list %s: %v\n", src, err)
			continue
		}
		sessions = append(sessions, rows...)
	}
	if len(sessions) == 0 {
		fmt.Fprintln(os.Stderr, "no sessions found in cache; run `llm-recall ls` once first.")
		os.Exit(1)
	}

	// 3. Aggregate into the JSON payload. Format will be overwritten per
	// render below; everything else is window-derived.
	req := stats.Aggregate(sessions, *days, tokenFallbackPerMsg, !*noWatermark)
	req.Template = *template

	if req.TotalSessions == 0 {
		fmt.Fprintf(os.Stderr, "no sessions in the last %d days; nothing to render.\n", *days)
		os.Exit(1)
	}

	// 4. Render two formats.
	outDir, err := picturesDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: pictures dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: mkdir %s: %v\n", outDir, err)
		os.Exit(1)
	}

	dateTag := time.Now().Format("2006-01-02")
	jobs := []struct {
		format, suffix string
	}{
		{"square", "1080x1080"},
		{"vertical", "1080x1920"},
	}
	written := make([]string, 0, len(jobs))
	for _, j := range jobs {
		req.Format = j.format
		png, err := imggen.Generate(req, *backend)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: render %s: %v\n", j.format, err)
			os.Exit(1)
		}
		path := filepath.Join(outDir, fmt.Sprintf("stats-%s-%s.png", dateTag, j.suffix))
		if err := os.WriteFile(path, png, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Printf("→ %s\n", path)
		written = append(written, path)
	}

	if *noOpen || len(written) == 0 {
		return
	}

	// 5. Best-effort interactive prompt. Skip on non-tty stdin (CI, tests).
	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		return
	}
	fmt.Print("Open in Explorer? [y/n] ")
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	if strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "y") {
		_ = stats.OpenInExplorer(filepath.Dir(written[0]))
	}
}

// picturesDir returns the platform-conventional Pictures dir under the
// llm-recall subfolder. Windows → %USERPROFILE%\Pictures\llm-recall;
// macOS / Linux → ~/Pictures/llm-recall. Caller must MkdirAll.
func picturesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Pictures", "llm-recall"), nil
}
