package stats

import (
	"strings"
	"testing"
	"time"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/config"
)

// TestStatsFooter_RenderedWhenPromoOn: a Model built with a default
// Config (NoPromo=false) renders the sponsored line in View().
func TestStatsFooter_RenderedWhenPromoOn(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	sessions := []adapter.Session{{
		ID:        "abc",
		Source:    "claude",
		UpdatedAt: now.Add(-24 * time.Hour),
	}}
	m := NewModel(sessions, now, 3).WithPromo(config.Defaults())
	v := m.View()
	if !strings.Contains(v, "sponsored by YCAPI") {
		t.Errorf("default promo: missing sponsored line\n--- view ---\n%s", v)
	}
}

// TestStatsFooter_SuppressedWhenNoPromo: NoPromo=true ⇒ no sponsored line.
func TestStatsFooter_SuppressedWhenNoPromo(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	sessions := []adapter.Session{{
		ID:        "abc",
		Source:    "claude",
		UpdatedAt: now.Add(-24 * time.Hour),
	}}
	cfg := &config.Config{Promo: config.PromoConfig{NoPromo: true}}
	m := NewModel(sessions, now, 3).WithPromo(cfg)
	v := m.View()
	if strings.Contains(v, "sponsored by YCAPI") {
		t.Errorf("--no-promo: sponsored line still rendered\n--- view ---\n%s", v)
	}
}

// TestStatsFooter_SuppressedWhenNilPromo: nil cfg ⇒ no sponsored line.
func TestStatsFooter_SuppressedWhenNilPromo(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	sessions := []adapter.Session{{
		ID:        "abc",
		Source:    "claude",
		UpdatedAt: now.Add(-24 * time.Hour),
	}}
	m := NewModel(sessions, now, 3) // no WithPromo call
	v := m.View()
	if strings.Contains(v, "sponsored by YCAPI") {
		t.Errorf("nil promo: sponsored line still rendered\n--- view ---\n%s", v)
	}
}
