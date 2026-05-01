// W5-rev1 stats subcommand: terminal-native renderer (heatmap + 4×2 panel).
// Replaces the old PNG export to a Python backend; see CHANGELOG.
//
// Surface:
//
//	llm-recall stats           # interactive TUI when stdout is a tty
//	llm-recall stats --json    # JSON snapshot to stdout (pipe-friendly)
//
// On non-tty stdout the bare `stats` invocation auto-falls-back to JSON so
// `llm-recall stats > out.txt` and remote SSH pipes don't dump ANSI noise.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/index"
	"github.com/xiao98/llm-recall/internal/stats"
)

// tokenFallbackPerMsg: per-message token estimate used when a session's
// jsonl yielded zero tokens. TOKEN-AUDIT.md (now in internal/stats/) says
// all three vendors expose token fields; this is just a safety net.
// Conservative — biased low so we don't over-promise.
const tokenFallbackPerMsg = 3

func cmdStats(args []string, cfg *config.Config) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print JSON snapshot to stdout instead of TUI")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	// Refresh the cache so any newly-arrived sessions show up.
	if _, err := index.DiscoverAll(context.Background(), index.Options{
		UseCache: true,
		NeedBody: true,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warn: discover: %v\n", err)
	}

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

	model := stats.NewModel(sessions, time.Now(), tokenFallbackPerMsg).WithPromo(cfg)

	if *jsonOut {
		if err := model.WriteJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: json: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := model.RunOrFallback(os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: tui: %v\n", err)
		os.Exit(1)
	}
}
