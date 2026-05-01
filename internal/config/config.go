// Package config holds llm-recall's user-tunable runtime config.
//
// The only thing W6 reads from disk is the [promo] section. We could have
// kept this inline in internal/promo, but a top-level config package leaves
// room for W7+ knobs (BYOK keys, network endpoints) without a second
// migration.
//
// Loading order: zero-value defaults → TOML file at ConfigPath() if it
// exists → flag overrides applied by the caller. Missing file is not an
// error; unreadable file logs a warn to stderr but still returns defaults.
//
// File location:
//   - macOS / Linux: ~/.config/llm-recall/config.toml
//   - Windows:       %APPDATA%\llm-recall\config.toml
//
// We deliberately do NOT pull in viper. The file is one section, four
// fields; BurntSushi/toml is a single tiny pure-Go dep already vetted by
// many Go tools.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the in-memory runtime config. All fields are nested by section
// to match the TOML layout (so e.g. `[promo] no_promo = true`).
type Config struct {
	Promo PromoConfig `toml:"promo"`
}

// PromoConfig governs the W6 marketing-injection surface.
//
//   - NoPromo is the kill switch. True ⇒ banner / search footer / stats
//     footer all return empty strings. Set by --no-promo flag at runtime
//     or `[promo] no_promo = true` in config.toml.
//   - SearchFooter gates the optional TUI list-bottom "discussions" line.
//     Default false (off) so out-of-the-box TUI stays clean.
//   - BannerFreq lets advanced users dial banner display down (0.0–1.0).
//     Default 1.0 (always show in TUI). 0.0 ⇒ off; 0.5 ⇒ 50% of launches.
//   - CTAProbability is the per-banner odds of the "join group" call-to-
//     action line appearing under the quote. Default 0.05 (5%); the spec
//     fixes this for transparency. Users can drop it to 0; raising it
//     above 0.05 is allowed but discouraged.
type PromoConfig struct {
	NoPromo        bool    `toml:"no_promo"`
	SearchFooter   bool    `toml:"search_footer"`
	BannerFreq     float64 `toml:"banner_freq"`
	CTAProbability float64 `toml:"cta_probability"`
}

// Defaults returns a Config populated with the documented defaults. Used
// both as the initial state for Load() and as the canonical source of
// truth for tests that want a known baseline.
func Defaults() *Config {
	return &Config{
		Promo: PromoConfig{
			NoPromo:        false,
			SearchFooter:   false,
			BannerFreq:     1.0,
			CTAProbability: 0.05,
		},
	}
}

// ConfigPath returns the absolute path to config.toml on this OS. We use
// os.UserConfigDir which already encodes the per-OS conventions:
// ~/Library/Application Support on macOS, ~/.config on Linux,
// %AppData% on Windows. The W6 spec calls for ~/.config on macOS for
// symmetry with Linux, but UserConfigDir's default is more canonical and
// the user can always symlink. Falls back to the cwd on the rare error.
func ConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return filepath.Join(".", "llm-recall", "config.toml")
	}
	return filepath.Join(dir, "llm-recall", "config.toml")
}

// Load returns a Config built from defaults overlaid with config.toml (if
// present) and finally with `flagNoPromo` (if true). Errors reading the
// file are logged to stderr but never fatal — the runtime always gets a
// usable Config. Returning the error too lets the caller distinguish
// "default" from "had to recover".
func Load(flagNoPromo bool) (*Config, error) {
	cfg := Defaults()

	path := ConfigPath()
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if _, derr := toml.Decode(string(data), cfg); derr != nil {
			fmt.Fprintf(os.Stderr, "warn: parse %s: %v (using defaults)\n", path, derr)
			cfg = Defaults()
		}
	case os.IsNotExist(err):
		// First-run case: silent. config.toml is optional.
	default:
		fmt.Fprintf(os.Stderr, "warn: read %s: %v (using defaults)\n", path, err)
	}

	if flagNoPromo {
		cfg.Promo.NoPromo = true
	}

	// Bound the floats. Bad config shouldn't crash the renderer; it
	// should just behave conservatively (no negative probabilities, no
	// >1.0 frequencies that would produce nonsense randomness).
	if cfg.Promo.BannerFreq < 0 {
		cfg.Promo.BannerFreq = 0
	}
	if cfg.Promo.BannerFreq > 1 {
		cfg.Promo.BannerFreq = 1
	}
	if cfg.Promo.CTAProbability < 0 {
		cfg.Promo.CTAProbability = 0
	}
	if cfg.Promo.CTAProbability > 1 {
		cfg.Promo.CTAProbability = 1
	}

	return cfg, nil
}
