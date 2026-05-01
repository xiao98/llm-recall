// BYOK key detection.
//
// Order: ANTHROPIC_API_KEY → OPENAI_API_KEY → error. Anthropic wins ties
// because the default models we ship are tuned for haiku-class prompts.
// `--vendor` flag overrides this; when --vendor is set we still need the
// matching env var, otherwise we fail fast with a vendor-specific hint.
package llm

import (
	"errors"
	"os"
)

// ErrNoKey is the sentinel returned when neither ANTHROPIC_API_KEY nor
// OPENAI_API_KEY is set in the environment. main.go inspects this with
// errors.Is and prints the BYOK hint + exits 2.
var ErrNoKey = errors.New("no API key in environment")

// DetectKey returns the auto-detected vendor and its key, or ErrNoKey if
// none. Anthropic is preferred when both are set.
//
// Returning vendor + key together (rather than two separate funcs) lets
// the caller print exactly which env var was used in --verbose flows
// without re-reading os.Getenv.
func DetectKey() (Vendor, string, error) {
	if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" {
		return Anthropic, k, nil
	}
	if k := os.Getenv("OPENAI_API_KEY"); k != "" {
		return OpenAI, k, nil
	}
	return "", "", ErrNoKey
}

// KeyForVendor reads the env var matching `v` (used when --vendor is
// explicit). Returns ErrNoKey if the matching var is unset.
func KeyForVendor(v Vendor) (string, error) {
	var name string
	switch v {
	case Anthropic:
		name = "ANTHROPIC_API_KEY"
	case OpenAI:
		name = "OPENAI_API_KEY"
	default:
		return "", errors.New("unknown vendor")
	}
	if k := os.Getenv(name); k != "" {
		return k, nil
	}
	return "", ErrNoKey
}

// EnvBaseURL returns the optional LLM_RECALL_BASE_URL escape hatch.
// Empty string when unset (callers fall back to DefaultBaseURL).
func EnvBaseURL() string { return os.Getenv("LLM_RECALL_BASE_URL") }

// mockMode reports whether LLM_RECALL_LLM_MOCK is set to a non-empty
// value. Used by NewClient to short-circuit to mockClient.
func mockMode() bool { return os.Getenv("LLM_RECALL_LLM_MOCK") != "" }

// MockMode is the exported wrapper used by tests/cmd code.
func MockMode() bool { return mockMode() }
