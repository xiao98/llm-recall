// llm-recall entry point.
//
// W9 surface (changes vs W6/W7):
//   - `llm-recall onboarding` removed (popup deleted; first launch goes
//     directly into the TUI). README now carries the attribution line
//     instead of an in-app consent box.
//   - `llm-recall login` added (W9): interactive LLM provider setup
//     (vendor / api key / base url / model / storage). Key never goes
//     through CLI flags — only stdin pipe in non-interactive mode.
//   - `--no-promo` retained (still kills banner / footer / attribution).
//
// We hand-route subcommands rather than pull in cobra/viper — the surface
// is small enough that an explicit switch is clearer than the indirection.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/index"
	"github.com/xiao98/llm-recall/internal/launcher"
	"github.com/xiao98/llm-recall/internal/tui"
)

// version is overwritten at release time via -ldflags '-X main.version=...'.
var version = "0.0.1-dev"

// knownSubcommands gates the routing decision in main(). Anything else (or
// nothing) goes to the TUI command, which is the W3 default.
var knownSubcommands = map[string]struct{}{
	"ls":      {},
	"stats":   {},
	"version": {},
	"help":    {},
	"card":    {}, // W7
	"gold":    {}, // W7
	"login":   {}, // W9
}

func main() {
	// Strip --no-promo before subcommand routing. Each subcommand also
	// declares its own flag.FlagSet, so to avoid duplicating the flag in
	// three places we extract it up front. This keeps `--no-promo` valid
	// in any position relative to the subcommand name.
	rawArgs := os.Args[1:]
	flagNoPromo, rawArgs := stripNoPromo(rawArgs)

	cfg, _ := config.Load(flagNoPromo) // never fatal; Load logs warns

	// Subcommand selection happens on the cleaned args.
	if len(rawArgs) == 0 {
		cmdTUI(nil, cfg)
		return
	}
	first := rawArgs[0]
	switch first {
	case "ls":
		cmdLs(rawArgs[1:])
	case "stats":
		cmdStats(rawArgs[1:], cfg)
	case "card":
		cmdCard(rawArgs[1:], cfg)
	case "gold":
		cmdGold(rawArgs[1:], cfg)
	case "login":
		cmdLogin(rawArgs[1:])
	case "version", "-v", "--version":
		fmt.Println("llm-recall", version)
	case "help", "-h", "--help":
		usage()
	default:
		if _, ok := knownSubcommands[first]; ok {
			usage()
			os.Exit(1)
		}
		// Unknown first arg → assume the user is invoking the TUI with
		// flags. Anything that turns out to be malformed will surface in
		// flag.Parse below.
		cmdTUI(rawArgs, cfg)
	}
}

// stripNoPromo removes the first occurrence of --no-promo (or -no-promo)
// from args and returns the boolean + the cleaned slice. We do this
// manually rather than via a global flag.FlagSet because subcommand
// FlagSets refuse unknown flags; promoting --no-promo to a top-level
// flag would force every subcommand to register it too.
func stripNoPromo(args []string) (bool, []string) {
	out := make([]string, 0, len(args))
	found := false
	for _, a := range args {
		if a == "--no-promo" || a == "-no-promo" {
			found = true
			continue
		}
		out = append(out, a)
	}
	return found, out
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: llm-recall [tui-flags] | ls [...] | stats [...] | login [...] | card [...] | gold [...] | version")
	fmt.Fprintln(os.Stderr, "  (no args)             open TUI search; Enter on a row execs the resume recipe")
	fmt.Fprintln(os.Stderr, "  --dry-run             print the exec line without spawning the child (for debugging)")
	fmt.Fprintln(os.Stderr, "  --no-promo            disable banner / search footer / attribution line")
	fmt.Fprintln(os.Stderr, "  --source <name>       limit to one adapter")
	fmt.Fprintln(os.Stderr, "  ls [-n N] [--all] [--no-cache] [--source claude|codex|gemini]")
	fmt.Fprintln(os.Stderr, "                        list LLM CLI sessions on this machine")
	fmt.Fprintln(os.Stderr, "  stats [--json]")
	fmt.Fprintln(os.Stderr, "                        terminal-native stats (heatmap + Top topics + 4×2 panel)")
	fmt.Fprintln(os.Stderr, "  login [--vendor X] [--base-url X] [--model X] [--use-keyring] [--pipe-key]")
	fmt.Fprintln(os.Stderr, "                        configure LLM provider (interactive; key never via flag)")
	fmt.Fprintln(os.Stderr, "  card <session-id> [-y] [--no-cache] [--vendor X] [--model X] [--llm-base-url X]")
	fmt.Fprintln(os.Stderr, "                        BYOK LLM card render of a single session")
	fmt.Fprintln(os.Stderr, "  gold [--days N] [-y] [--md] [--no-cache] [--vendor X] [--model X] [--llm-base-url X]")
	fmt.Fprintln(os.Stderr, "                        BYOK LLM Top-10 quote miner over the last N days")
	fmt.Fprintln(os.Stderr, "  version               print version")
}

// validSources gates --source values. New adapters must be added here.
var validSources = map[string]struct{}{
	"claude": {},
	"codex":  {},
	"gemini": {},
}

func cmdLs(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	limit := fs.Int("n", 50, "max rows to show")
	all := fs.Bool("all", false, "show all rows (overrides -n)")
	noCache := fs.Bool("no-cache", false, "force re-parse, ignore SQLite cache (still updates it)")
	source := fs.String("source", "", "limit to one adapter: claude|codex|gemini")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *source != "" {
		if _, ok := validSources[*source]; !ok {
			fmt.Fprintf(os.Stderr, "error: --source must be one of claude|codex|gemini, got %q\n", *source)
			os.Exit(1)
		}
	}

	sessions, err := index.DiscoverAll(context.Background(), index.Options{
		UseCache: !*noCache,
		Source:   *source,
		// ls intentionally does NOT request bodies — keeps it as fast as W2.
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !*all && *limit > 0 && len(sessions) > *limit {
		sessions = sessions[:*limit]
	}

	home, _ := os.UserHomeDir()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SOURCE\tUPDATED\tCWD\tSESSION\tTITLE")
	for _, s := range sessions {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			s.Source,
			s.UpdatedAt.Local().Format("2006-01-02 15:04"),
			shortCWD(s.CWD, home),
			shortID(s.ID),
			truncate(s.Title, 80),
		)
	}
	_ = w.Flush()
}

// cmdTUI parses TUI flags and runs the bubbletea model. Real exec is the
// default; --dry-run flips to print-only (debugging / scripting). The legacy
// --no-dry-run flag is accepted as a no-op alias with a deprecation warning
// so users on the old habit don't get an "unknown flag" error mid-demo.
func cmdTUI(args []string, cfg *config.Config) {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "print the exec line without spawning the child (for debugging)")
	legacyNoDryRun := fs.Bool("no-dry-run", false, "deprecated alias (real exec is now the default; this flag is a no-op)")
	source := fs.String("source", "", "limit to one adapter: claude|codex|gemini")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if *legacyNoDryRun {
		fmt.Fprintln(os.Stderr, "warn: --no-dry-run is now the default; flag is a no-op and will be removed in v0.3")
	}
	if *source != "" {
		if _, ok := validSources[*source]; !ok {
			fmt.Fprintf(os.Stderr, "error: --source must be one of claude|codex|gemini, got %q\n", *source)
			os.Exit(1)
		}
	}

	// Open the cache directly so the TUI's search SQL can run against it
	// without re-doing path resolution. DiscoverAll runs first to populate
	// any newly-arrived sessions and to backfill body fields after the v2
	// schema upgrade.
	if _, err := index.DiscoverAll(context.Background(), index.Options{
		UseCache: true,
		Source:   *source,
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

	model := tui.New(tui.Config{
		DB:     cache.DB(),
		Source: *source,
		DryRun: *dryRun,
		Promo:  cfg,
	})
	sel, err := model.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: tui: %v\n", err)
		os.Exit(1)
	}
	if sel == nil {
		// User quit (Esc/Ctrl-C) without picking. Exit 0.
		return
	}

	l := launcher.New(*dryRun)
	code, err := l.Run(sel.Session)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: launcher: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	if code != 0 {
		os.Exit(code)
	}
}

// shortCWD replaces the user home prefix with `~` and clamps overall length
// to 25 chars by elliding the leading bytes.
func shortCWD(cwd, home string) string {
	if cwd == "" {
		return ""
	}
	c := cwd
	if home != "" {
		// Compare case-insensitively on Windows where C:\Users\X and
		// c:\users\x are the same path.
		if pathHasPrefix(c, home) {
			rest := c[len(home):]
			rest = strings.TrimLeft(rest, `\/`)
			if rest == "" {
				c = "~/"
			} else {
				c = "~/" + filepath.ToSlash(rest)
			}
		} else {
			c = filepath.ToSlash(c)
		}
	} else {
		c = filepath.ToSlash(c)
	}
	const max = 25
	if len([]rune(c)) <= max {
		return c
	}
	r := []rune(c)
	return "…" + string(r[len(r)-(max-1):])
}

func pathHasPrefix(path, prefix string) bool {
	if len(prefix) == 0 || len(path) < len(prefix) {
		return false
	}
	return strings.EqualFold(path[:len(prefix)], prefix)
}

// shortID renders the first 8 chars of a UUID-style id; preserves the full
// string when shorter.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// truncate ellipsizes by rune count to keep CJK widths sane.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// silence unused-import warning.
var _ = adapter.Session{}
