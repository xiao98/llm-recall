// Package tui — banner placeholder.
//
// W3 reserves this slot for the W6 onboarding/promo banner. Until then,
// Banner() must return the empty string. Do NOT add YCAPI strings, group
// links, quotes, or CTAs here — that work belongs to W6 and is gated on the
// onboarding consent flow described in DEVDOC §4.
package tui

// Banner returns the optional top-of-TUI marketing banner. W3: always empty.
func Banner() string { return "" }
