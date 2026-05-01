// Package tui — banner pass-through.
//
// W3 reserved this slot as `func Banner() string { return "" }`. W6
// wires it into internal/promo, which owns the actual quote pool and
// the --no-promo gate. The TUI calls Banner() during View() and renders
// the result above the search bar.
//
// Keeping this file as a thin pass-through (rather than inlining the
// promo call at the call site) preserves the W3 boundary: TUI only
// imports promo at one well-defined seam, so reverting marketing is a
// one-line change here.
package tui

import (
	"github.com/xiao98/llm-recall/internal/config"
	"github.com/xiao98/llm-recall/internal/promo"
)

// Banner returns the optional top-of-TUI marketing banner. The result is
// the empty string when promo is disabled (cfg.Promo.NoPromo == true)
// or when the random gate skips this launch.
func Banner(cfg *config.Config) string {
	return promo.Banner(cfg)
}
