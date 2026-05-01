// BYOK key detection.
//
// W7 used a 2-tier chain (env → fail). W9 promotes credentials.toml /
// system keyring to the front of the chain so users can run
// `llm-recall login` once and forget about exporting env vars.
//
// Priority for a *given* vendor:
//
//  1. credentials.toml has a section for that vendor with api_key set
//     → use it. (When use_keyring=true, the file is just a marker; we
//     fetch the real key from the keyring instead.)
//  2. credentials.toml says use_keyring=true OR has no entry but the
//     keyring has one → use keyring entry.
//  3. ANTHROPIC_API_KEY / OPENAI_API_KEY env var (legacy / CI use)
//     → use it.
//  4. None of the above → ErrNoCredentials with a friendly hint
//     pointing at `llm-recall login`.
//
// When NO vendor is pinned, we ask the same chain to pick *any*
// configured vendor, in stable order (anthropic → openai). This
// matches W7's behaviour where an OPENAI_API_KEY-only environment
// auto-resolves to OpenAI.
package llm

import (
	"errors"
	"fmt"
	"os"

	"github.com/xiao98/llm-recall/internal/credentials"
)

// ErrNoCredentials is the W9 sentinel returned when no credential
// source has a key. main.go's Friendly* helpers translate this into
// the "Run: llm-recall login" hint.
var ErrNoCredentials = errors.New("no LLM API key configured")

// ErrNoKey is kept as an alias for ErrNoCredentials so existing W7
// callers (cmd_card / cmd_gold's errors.Is) keep working unchanged.
// Tests in detect_test.go also reference ErrNoKey.
var ErrNoKey = ErrNoCredentials

// FriendlyNoCredsHint is what main / card / gold print to stderr when
// they hit ErrNoCredentials. Pinned in one place so all three commands
// stay in lock-step on the wording.
const FriendlyNoCredsHint = "no LLM API key configured.\n  Run:  llm-recall login\n  Or:   export OPENAI_API_KEY=sk-...    (env var fallback)"

// resolveVendorCred runs the W9 priority chain for one vendor. Errors
// other than ErrNotFound from intermediate stages bubble up; ErrNotFound
// just means "try the next tier".
func resolveVendorCred(vendor string) (credentials.Cred, error) {
	// Tier 1: credentials.toml.
	c, err := credentials.Load(vendor)
	switch {
	case err == nil && c.APIKey != "":
		return c, nil
	case err == nil && c.UseKeyring:
		// File flagged keyring; fetch real key from keyring.
		kc, kerr := credentials.LoadFromKeyring(vendor)
		if kerr == nil {
			// Inherit BaseURL / Model from the file when the keyring
			// entry omitted them — keyring stores the JSON we wrote on
			// SaveToKeyring, so they should be there, but be defensive.
			if kc.BaseURL == "" {
				kc.BaseURL = c.BaseURL
			}
			if kc.Model == "" {
				kc.Model = c.Model
			}
			return kc, nil
		}
		// Keyring failed despite the marker — log a warn and fall
		// through to env so the user isn't blocked by a broken
		// Secret Service / Credential Manager.
		if errors.Is(kerr, credentials.ErrKeyringUnavailable) {
			fmt.Fprintf(os.Stderr,
				"warn: credentials.toml says use_keyring for %s but keyring is unavailable; trying env var fallback (%v)\n",
				vendor, kerr)
		} else if !errors.Is(kerr, credentials.ErrNotFound) {
			fmt.Fprintf(os.Stderr,
				"warn: keyring fetch for %s: %v; trying env var fallback\n", vendor, kerr)
		}
	case errors.Is(err, credentials.ErrNotFound):
		// Tier 2: keyring direct (no toml entry but maybe a stale entry
		// from a previous install). Cheap to probe.
		kc, kerr := credentials.LoadFromKeyring(vendor)
		if kerr == nil && kc.APIKey != "" {
			return kc, nil
		}
		// Otherwise, fall through.
	default:
		// Hard error reading the file (bad permissions / parse). Surface.
		return credentials.Cred{}, err
	}

	// Tier 3: env var.
	envName := envForVendor(stringToVendor(vendor))
	if envName != "" {
		if k := os.Getenv(envName); k != "" {
			return credentials.Cred{Vendor: vendor, APIKey: k}, nil
		}
	}
	return credentials.Cred{}, ErrNoCredentials
}

// DetectCred returns whichever vendor has a configured credential,
// preferring anthropic when both are present (matches W7 semantics so
// existing user habits survive the upgrade). Returns ErrNoCredentials
// when nothing is configured.
func DetectCred() (credentials.Cred, error) {
	for _, v := range []string{"anthropic", "openai"} {
		c, err := resolveVendorCred(v)
		if err == nil {
			return c, nil
		}
		if !errors.Is(err, ErrNoCredentials) {
			return credentials.Cred{}, err
		}
	}
	return credentials.Cred{}, ErrNoCredentials
}

// CredForVendor is the W9 replacement for KeyForVendor. Returns a full
// Cred (vendor / key / base_url / model) so settings.Resolve can take
// model + base_url from the file when CLI / env didn't override.
func CredForVendor(v Vendor) (credentials.Cred, error) {
	return resolveVendorCred(string(v))
}

// DetectKey is the legacy W7 entrypoint. Kept for source compatibility
// with existing test suites and any external callers that already
// depend on the (Vendor, key, error) shape. Prefer DetectCred for new
// code.
func DetectKey() (Vendor, string, error) {
	c, err := DetectCred()
	if err != nil {
		return "", "", err
	}
	return stringToVendor(c.Vendor), c.APIKey, nil
}

// KeyForVendor mirrors W7's narrow API: "give me the key for this
// vendor or fail". Routes through the W9 chain under the hood so
// credentials.toml / keyring-stored keys work too.
func KeyForVendor(v Vendor) (string, error) {
	c, err := resolveVendorCred(string(v))
	if err != nil {
		return "", err
	}
	return c.APIKey, nil
}

// EnvBaseURL returns the optional LLM_RECALL_BASE_URL escape hatch.
// Empty string when unset (callers fall back to DefaultBaseURL).
func EnvBaseURL() string { return os.Getenv("LLM_RECALL_BASE_URL") }

// mockMode reports whether LLM_RECALL_LLM_MOCK is set to a non-empty
// value. Used by NewClient to short-circuit to mockClient.
func mockMode() bool { return os.Getenv("LLM_RECALL_LLM_MOCK") != "" }

// MockMode is the exported wrapper used by tests/cmd code.
func MockMode() bool { return mockMode() }

// stringToVendor converts a credentials package vendor string to the
// llm package Vendor type. Returns "" for unknown values; resolveVendorCred
// has already validated the input by this point so production never
// hits the empty branch.
func stringToVendor(s string) Vendor {
	switch s {
	case "anthropic":
		return Anthropic
	case "openai":
		return OpenAI
	}
	return ""
}
