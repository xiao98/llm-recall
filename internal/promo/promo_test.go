package promo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiao98/llm-recall/internal/config"
)

// TestOnboardingStateMachine: not-accepted → write → accepted (+ JSON
// content has accepted_at + version).
func TestOnboardingStateMachine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "onboarding-accepted")
	SetOnboardingPathForTest(path)
	t.Cleanup(func() { SetOnboardingPathForTest("") })

	if OnboardingAccepted() {
		t.Fatalf("fresh tempdir reports accepted=true")
	}
	if err := WriteOnboardingAccepted("0.2.0"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !OnboardingAccepted() {
		t.Fatalf("after write, accepted=false")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var rec AcceptedRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, data)
	}
	if rec.AcceptedAt == "" {
		t.Errorf("accepted_at is empty: %s", data)
	}
	if rec.Version != "0.2.0" {
		t.Errorf("version=%q want %q", rec.Version, "0.2.0")
	}
}

// TestCTAProbability_Deterministic: monkey-patched rand100Fn drives a
// known sequence so we can assert exact CTA presence per draw.
func TestCTAProbability_Deterministic(t *testing.T) {
	cfg := config.Defaults()

	// First call inside Banner is the BannerFreq gate (skipped at
	// freq=1.0 — we never enter the threshold branch). Then comes the
	// CTA gate: < 5 ⇒ CTA shown.
	//
	// Replace rand100Fn with a deterministic counter; pin pickQuoteFn so
	// the quote text is stable.
	origR, origQ := rand100Fn, pickQuoteFn
	t.Cleanup(func() { rand100Fn, pickQuoteFn = origR, origQ })

	pickQuoteFn = func() int { return 0 }

	cases := []struct {
		draw   int
		hasCTA bool
	}{
		{0, true},
		{4, true},
		{5, false},
		{50, false},
		{99, false},
	}
	for _, c := range cases {
		rand100Fn = func() int { return c.draw }
		got := Banner(cfg)
		if got == "" {
			t.Fatalf("draw=%d: banner empty", c.draw)
		}
		hasCTA := contains(got, CTAURL)
		if hasCTA != c.hasCTA {
			t.Errorf("draw=%d: hasCTA=%v want %v\n%s", c.draw, hasCTA, c.hasCTA, got)
		}
	}
}

// TestCTAProbability_Random: 1000 real-random draws; ratio in [0.03, 0.07].
// Uses the production crypto/rand path. Flaky risk is bounded — the 99%
// confidence interval for p=0.05, n=1000 is [0.034, 0.066]; we use a
// slightly wider [0.025, 0.085] window to keep CI green.
func TestCTAProbability_Random(t *testing.T) {
	cfg := config.Defaults()
	hits := 0
	const n = 1000
	for i := 0; i < n; i++ {
		b := Banner(cfg)
		if contains(b, CTAURL) {
			hits++
		}
	}
	ratio := float64(hits) / float64(n)
	if ratio < 0.025 || ratio > 0.085 {
		t.Errorf("CTA ratio=%.3f out of [0.025, 0.085]; n=%d hits=%d", ratio, n, hits)
	}
	t.Logf("CTA ratio=%.3f (%d/%d)", ratio, hits, n)
}

// TestNoPromoKillsAll: NoPromo=true ⇒ Banner / SearchFooter / StatsFooter
// all return "".
func TestNoPromoKillsAll(t *testing.T) {
	cfg := &config.Config{Promo: config.PromoConfig{
		NoPromo:        true,
		SearchFooter:   true, // even when search-footer is opted in
		BannerFreq:     1.0,
		CTAProbability: 1.0, // 100% CTA, doesn't matter — banner is suppressed
	}}
	if got := Banner(cfg); got != "" {
		t.Errorf("Banner with NoPromo=true: %q", got)
	}
	if got := SearchFooter(cfg, "claude history wiki"); got != "" {
		t.Errorf("SearchFooter with NoPromo=true: %q", got)
	}
	if got := StatsFooter(cfg); got != "" {
		t.Errorf("StatsFooter with NoPromo=true: %q", got)
	}
}

// TestSearchFooter_Gating: default cfg (SearchFooter=false) ⇒ empty.
// SearchFooter=true + non-empty query ⇒ contains the first word.
// SearchFooter=true + empty query ⇒ empty (no false positive).
func TestSearchFooter_Gating(t *testing.T) {
	def := config.Defaults()
	if got := SearchFooter(def, "claude history"); got != "" {
		t.Errorf("default SearchFooter not empty: %q", got)
	}

	on := &config.Config{Promo: config.PromoConfig{SearchFooter: true}}
	if got := SearchFooter(on, ""); got != "" {
		t.Errorf("SearchFooter on empty query: %q", got)
	}
	got := SearchFooter(on, "claude history wiki")
	if got == "" {
		t.Fatalf("SearchFooter on=%v with query: empty", on)
	}
	if !contains(got, "claude") {
		t.Errorf("SearchFooter missing first word: %q", got)
	}
	if contains(got, "history") {
		t.Errorf("SearchFooter contains second word (should be first only): %q", got)
	}
}

// TestQuotesLoaded: pool ≥ 30, no empty entries. We avoid asserting on
// any specific quote string — the file is meant to be re-fetchable, so
// hard-coding a string would couple the test to fetch output.
func TestQuotesLoaded(t *testing.T) {
	if len(Quotes) < 30 {
		t.Errorf("len(Quotes)=%d, want >= 30", len(Quotes))
	}
	for i, q := range Quotes {
		if q == "" {
			t.Errorf("Quotes[%d] empty", i)
		}
	}
}

// TestBannerFreqZero: BannerFreq=0 ⇒ never renders, regardless of CTA.
func TestBannerFreqZero(t *testing.T) {
	cfg := &config.Config{Promo: config.PromoConfig{
		BannerFreq:     0,
		CTAProbability: 1.0,
	}}
	for i := 0; i < 50; i++ {
		if got := Banner(cfg); got != "" {
			t.Fatalf("BannerFreq=0 still rendered: %q", got)
		}
	}
}

// contains is strings.Contains under a different name to keep this file
// stdlib-only (it is, but the rename keeps the import surface obvious).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	if substr == "" {
		return 0
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
