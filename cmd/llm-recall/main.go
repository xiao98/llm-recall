// llm-recall entry point. W1 only ships `ls` and `version`; we deliberately
// route subcommands with a hand-written switch so the binary stays
// dependency-free until the surface area justifies a CLI framework.
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
	"github.com/xiao98/llm-recall/internal/index"
)

const version = "0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "ls":
		cmdLs(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("llm-recall", version)
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: llm-recall <ls|version>")
	fmt.Fprintln(os.Stderr, "  ls [-n N] [--all]    list LLM CLI sessions on this machine")
	fmt.Fprintln(os.Stderr, "  version              print version")
}

func cmdLs(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	limit := fs.Int("n", 50, "max rows to show")
	all := fs.Bool("all", false, "show all rows (overrides -n)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	sessions, err := index.DiscoverAll(context.Background())
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

// silence unused-import warning if a future refactor drops adapter access here.
var _ = adapter.Session{}
