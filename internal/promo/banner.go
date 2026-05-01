// W6 banner / footer renderers.
//
// Three surfaces: TUI top banner, search list-bottom footer, stats
// bottom sponsored line. All three respect the same kill switches:
//
//   - cfg.Promo.NoPromo == true            → empty string everywhere.
//   - cfg.Promo.BannerFreq < 1.0           → banner skips probabilistically.
//   - cfg.Promo.SearchFooter == false      → search footer empty.
//
// Randomness uses crypto/rand, not math/rand. Reasons: (a) test
// determinism is achieved by a monkey-patch hook (rand100Fn) rather than
// by a seeded PRNG, so we don't have to pick a seed strategy; (b)
// math/rand's default source has changed semantics across Go versions;
// (c) the cost is irrelevant — this fires once per process launch.
package promo

import (
	cryptorand "crypto/rand"

	"github.com/xiao98/llm-recall/internal/config"
)

// CTAURL is where the 5%-probability call-to-action line points. Per
// task §0.1 it lands on the project homepage, NOT directly on the
// sponsor's API console — the homepage acts as a soft funnel (and is
// cheaper to update without recompiling the binary).
const CTAURL = "https://recall.youchun.tech"

// rand100Fn returns an int in [0, 99]. Indirected through a var so tests
// can swap in a deterministic sequence without touching crypto/rand.
// Production path uses a single byte; modulo bias is negligible at our
// thresholds (5% / 100% etc).
var rand100Fn = cryptoRand100

func cryptoRand100() int {
	var b [1]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		// crypto/rand.Read on every supported OS effectively never fails;
		// if it does, fall back to "no CTA". Do not panic — banner is
		// pure cosmetic.
		return 99
	}
	return int(b[0]) % 100
}

// pickQuoteFn is the index picker. Same monkey-patch trick as rand100Fn.
var pickQuoteFn = cryptoPickQuote

func cryptoPickQuote() int {
	if len(Quotes) == 0 {
		return 0
	}
	var b [2]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 0
	}
	n := int(b[0])<<8 | int(b[1])
	return n % len(Quotes)
}

// Banner returns the top-of-TUI banner string, or "" when promo is off.
//
// Format:
//
//	💡  <quote>
//	→ 加入 YCAPI 群: https://recall.youchun.tech     (only on 5% draws)
//
// We deliberately do NOT prepend ANSI styling here — the caller
// (internal/tui/banner.go) wraps the result in lipgloss so the TUI's
// dim/colour palette stays consistent. Plain string in, plain string
// out; styling is the caller's job.
func Banner(cfg *config.Config) string {
	if cfg == nil || cfg.Promo.NoPromo {
		return ""
	}
	// Frequency gate: cfg.Promo.BannerFreq lets advanced users dial the
	// banner display rate down. Default 1.0 means always show.
	if cfg.Promo.BannerFreq <= 0 {
		return ""
	}
	if cfg.Promo.BannerFreq < 1 {
		// Multiply by 100 and compare to a 0..99 draw — same precision as
		// the CTA gate. < instead of <=. BannerFreq == 0 already short-
		// circuited above.
		threshold := int(cfg.Promo.BannerFreq * 100)
		if rand100Fn() >= threshold {
			return ""
		}
	}

	MaybeWarnFallback()

	if len(Quotes) == 0 {
		return ""
	}
	q := Quotes[pickQuoteFn()]
	line := "  💡  " + q

	// CTA gate. CTAProbability is in [0,1]; we threshold an int draw in
	// [0,99]. Probability of 0.05 ⇒ draw < 5 hits ⇒ ~5% rate.
	threshold := int(cfg.Promo.CTAProbability * 100)
	if threshold > 0 && rand100Fn() < threshold {
		line += "\n  → 加入 YCAPI 群: " + CTAURL
	}
	return line
}

// SearchFooter returns the optional list-bottom "discussions" line for
// the TUI search view. Default config returns "" (off); only fires when
// the user opts in via `[promo] search_footer = true` in config.toml AND
// has typed at least one query word.
//
// We split the query on whitespace and pick the first non-empty word —
// using the whole query (which can be many words) would make the line
// too long for narrow terminals. The first word is also the most
// "topic-like" by user habit.
func SearchFooter(cfg *config.Config, query string) string {
	if cfg == nil || cfg.Promo.NoPromo || !cfg.Promo.SearchFooter {
		return ""
	}
	first := firstWord(query)
	if first == "" {
		return ""
	}
	return "🔍 YCAPI 群里有人在讨论「" + first + "」 →"
}

// StatsFooter is the W5 "sponsored by" line for `llm-recall stats`. Pure
// string return; the stats renderer wraps it in lipgloss the same way it
// wraps the existing key-hint line.
func StatsFooter(cfg *config.Config) string {
	if cfg == nil || cfg.Promo.NoPromo {
		return ""
	}
	return "── llm-recall · sponsored by YCAPI ──"
}

// firstWord trims leading whitespace and returns up to the first
// whitespace boundary. Returns "" for empty / whitespace-only input.
func firstWord(s string) string {
	// Manual scan to avoid pulling strings.Fields' allocator for a
	// hot-path call on every keystroke.
	start := 0
	for start < len(s) {
		c := s[start]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			break
		}
		start++
	}
	end := start
	for end < len(s) {
		c := s[end]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		end++
	}
	return s[start:end]
}
