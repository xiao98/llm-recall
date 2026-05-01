// Package credentials manages BYOK LLM provider credentials on disk +
// in the OS keyring (W9).
//
// Storage paths follow the same per-OS convention as config.toml:
//   - macOS / Linux: $XDG_CONFIG_HOME (or ~/.config) / llm-recall / credentials.toml
//   - Windows:       %APPDATA%\llm-recall\credentials.toml
//
// File mode 0600. Schema (TOML, one section per vendor):
//
//	[anthropic]
//	api_key     = "sk-ant-..."
//	base_url    = "https://api.anthropic.com"     # optional
//	model       = "claude-haiku-4-5-20251001"     # optional
//	use_keyring = false                           # opt-in keyring
//
//	[openai]
//	api_key     = "sk-..."
//	base_url    = "https://api.openai.com/v1"
//	model       = "gpt-4o-mini"
//	use_keyring = false
//
// When `use_keyring` is true, `api_key` in the file is left empty and
// the actual secret lives in the OS keyring under service "llm-recall",
// key = vendor (e.g. "anthropic"), value = JSON-encoded Cred. We pin
// service / key explicitly so a future second binary on the same host
// never collides.
//
// Path override hook (CredPathOverride) exists so cmd_login_test.go and
// detect_test.go can run hermetically against a tempdir without
// touching the user's real ~/.config.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/zalando/go-keyring"
)

// Cred is one provider's full settings bundle.
//
// Only Vendor + APIKey are required by the LLM client; BaseURL / Model
// are optional and fall back to vendor defaults when empty. UseKeyring
// is a per-vendor opt-in flag persisted to the file so detect.go can
// decide whether to also probe the keyring.
type Cred struct {
	Vendor     string `json:"vendor" toml:"-"`
	APIKey     string `json:"api_key" toml:"api_key"`
	BaseURL    string `json:"base_url,omitempty" toml:"base_url,omitempty"`
	Model      string `json:"model,omitempty" toml:"model,omitempty"`
	UseKeyring bool   `json:"-" toml:"use_keyring,omitempty"`
}

// Validate checks the bare minimum (vendor recognised; key non-empty
// unless keyring is in play). Callers should run this before sending a
// Cred to the network — a malformed cred is far cheaper to surface here
// than via a 401 from the LLM provider.
func (c Cred) Validate() error {
	switch c.Vendor {
	case "anthropic", "openai":
	case "":
		return errors.New("credentials: vendor empty")
	default:
		return fmt.Errorf("credentials: unknown vendor %q", c.Vendor)
	}
	if !c.UseKeyring && strings.TrimSpace(c.APIKey) == "" {
		return errors.New("credentials: api_key empty")
	}
	return nil
}

// fileSchema is the on-disk TOML shape: one section per vendor.
type fileSchema struct {
	Anthropic *Cred `toml:"anthropic,omitempty"`
	OpenAI    *Cred `toml:"openai,omitempty"`
}

// keyringService is the bucket name we register against the OS keyring.
// Pinning it explicitly (rather than reading from version) means a
// 0.3 → 0.4 upgrade still finds the same secret without a migration step.
const keyringService = "llm-recall"

// ErrKeyringUnavailable is returned by LoadFromKeyring / SaveToKeyring
// when the OS keyring backend cannot be reached. Callers should fall
// back to credentials.toml + a stderr warn. We treat any error from
// zalando/go-keyring as keyring-unavailable: the library itself does
// not differentiate "no backend" from "denied" in a portable way.
var ErrKeyringUnavailable = errors.New("system keyring unavailable")

// ErrNotFound is returned by Load / LoadFromKeyring when the requested
// vendor has no stored credential. detect.go treats this as
// "fall through to the next priority tier", not as a hard error.
var ErrNotFound = errors.New("credentials: vendor not configured")

// CredPathOverride lets tests redirect the credentials.toml path
// without mutating $HOME / $APPDATA. Production keeps it "" so Path()
// returns the canonical OS-specific location.
var CredPathOverride string

// Path returns the absolute path to credentials.toml on this OS.
//
// macOS / Linux: $XDG_CONFIG_HOME or ~/.config / llm-recall /
// Windows: %APPDATA% / llm-recall /
//
// Falls back to the cwd on the rare error from os.UserConfigDir; that
// matches config.ConfigPath()'s behaviour so the two files always
// co-locate.
func Path() string {
	if CredPathOverride != "" {
		return CredPathOverride
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return filepath.Join(".", "llm-recall", "credentials.toml")
	}
	return filepath.Join(dir, "llm-recall", "credentials.toml")
}

// Dir returns the parent directory of Path(). Exposed because cmd_login
// also wants to chmod the directory before writing the file.
func Dir() string { return filepath.Dir(Path()) }

// Load reads credentials.toml and returns the credential for the given
// vendor. Missing file or missing section ⇒ ErrNotFound (not an error
// type detect.go treats as fatal). Unparseable file ⇒ surfacing the
// parse error as-is so the user can fix the typo.
//
// We deliberately do not auto-pull from the keyring here. detect.go is
// the one place that orchestrates the priority chain (file → keyring →
// env), so Load() is intentionally narrow: "what does the file say?"
func Load(vendor string) (Cred, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Cred{}, ErrNotFound
		}
		return Cred{}, fmt.Errorf("credentials: read %s: %w", path, err)
	}
	var f fileSchema
	if _, derr := toml.Decode(string(data), &f); derr != nil {
		return Cred{}, fmt.Errorf("credentials: parse %s: %w", path, derr)
	}
	c := pickVendor(&f, vendor)
	if c == nil {
		return Cred{}, ErrNotFound
	}
	out := *c
	out.Vendor = vendor
	return out, nil
}

// LoadAny returns the first vendor section found in the file, in
// stable order (anthropic → openai). Used when the caller has not
// pinned a vendor and just wants "whatever the user configured".
// Returns ErrNotFound when no section is present.
func LoadAny() (Cred, error) {
	for _, v := range []string{"anthropic", "openai"} {
		c, err := Load(v)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Cred{}, err
		}
	}
	return Cred{}, ErrNotFound
}

// Save writes the supplied credential to credentials.toml. The file is
// created with mode 0600 (and the parent directory with 0700) so a
// multi-user host doesn't leak the secret. Existing sections for OTHER
// vendors are preserved — we only rewrite the section matching c.Vendor.
//
// When c.UseKeyring is true, the api_key in the file is intentionally
// blanked. Callers that want the secret in the keyring must call
// SaveToKeyring(c) separately; this function does NOT chain to keyring
// automatically because cmd_login wants to surface the keyring-failure
// error to the user (and offer file fallback) rather than silently
// half-write state.
func Save(c Cred) error {
	if err := c.Validate(); err != nil {
		return err
	}
	path := Path()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("credentials: mkdir %s: %w", dir, err)
	}

	// Read existing file (if any) so we preserve the other vendor section.
	var f fileSchema
	if data, err := os.ReadFile(path); err == nil {
		if _, derr := toml.Decode(string(data), &f); derr != nil {
			return fmt.Errorf("credentials: re-read %s: %w", path, derr)
		}
	}
	toWrite := c
	if toWrite.UseKeyring {
		// Don't echo the secret to disk when keyring is in play; the
		// keyring is the source of truth.
		toWrite.APIKey = ""
	}
	switch c.Vendor {
	case "anthropic":
		f.Anthropic = &toWrite
	case "openai":
		f.OpenAI = &toWrite
	default:
		return fmt.Errorf("credentials: unknown vendor %q", c.Vendor)
	}

	tmp, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("credentials: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	enc := toml.NewEncoder(tmp)
	if eerr := enc.Encode(f); eerr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("credentials: encode: %w", eerr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("credentials: close temp: %w", cerr)
	}
	// Mode 0600 — user-only readable. On Windows, os.Chmod's effect on
	// ACLs is best-effort; the moral signal is more important than the
	// strict enforcement, since on Windows the file's parent dir is
	// already user-scoped under %APPDATA%.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("credentials: chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("credentials: rename: %w", err)
	}
	return nil
}

// LoadFromKeyring fetches the JSON-encoded Cred from the OS keyring
// under service "llm-recall", key = vendor. Returns ErrNotFound when
// the entry is missing, ErrKeyringUnavailable when the backend itself
// is unreachable.
func LoadFromKeyring(vendor string) (Cred, error) {
	if vendor == "" {
		return Cred{}, ErrNotFound
	}
	raw, err := keyring.Get(keyringService, vendor)
	if err != nil {
		// zalando/go-keyring returns keyring.ErrNotFound for the
		// "entry doesn't exist" case; everything else is treated as
		// "backend down" so the caller can fall back gracefully.
		if errors.Is(err, keyring.ErrNotFound) {
			return Cred{}, ErrNotFound
		}
		return Cred{}, fmt.Errorf("%w: %v", ErrKeyringUnavailable, err)
	}
	var c Cred
	if jerr := json.Unmarshal([]byte(raw), &c); jerr != nil {
		return Cred{}, fmt.Errorf("credentials: decode keyring entry: %w", jerr)
	}
	c.Vendor = vendor
	return c, nil
}

// SaveToKeyring stores the JSON-encoded Cred under service
// "llm-recall", key = vendor. Returns ErrKeyringUnavailable when the
// backend is unreachable.
func SaveToKeyring(c Cred) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Vendor == "" {
		return errors.New("credentials: SaveToKeyring with empty vendor")
	}
	// Force UseKeyring=true on the wire so a later Load won't think the
	// file is the source of truth.
	c.UseKeyring = true
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("credentials: encode keyring entry: %w", err)
	}
	if kerr := keyring.Set(keyringService, c.Vendor, string(data)); kerr != nil {
		return fmt.Errorf("%w: %v", ErrKeyringUnavailable, kerr)
	}
	return nil
}

// pickVendor returns the *Cred for the given vendor name from the
// parsed file, or nil when missing. We resolve the right pointer at
// the schema layer rather than a map[string]*Cred because fileSchema's
// field names give us nicer toml field defaults.
func pickVendor(f *fileSchema, vendor string) *Cred {
	switch vendor {
	case "anthropic":
		return f.Anthropic
	case "openai":
		return f.OpenAI
	default:
		return nil
	}
}
