// Settings resolution: CLI flag > env > config.toml > hardcoded default.
//
// Why a dedicated helper: card and gold both need to translate the same
// (flag, env, config) tuple into a (vendor, model, baseURL, key) tuple
// before constructing a Client. Duplicating that across two files is
// asking for the day they drift. One helper, one test.
package llm

import (
	"fmt"
	"strings"

	"github.com/xiao98/llm-recall/internal/config"
)

// ResolvedSettings is the bundle every command actually uses.
type ResolvedSettings struct {
	Vendor  Vendor
	Model   string
	BaseURL string
	Key     string // "" in mock mode (mock client doesn't need one)
}

// Resolve reduces the priority chain to a single struct. Returns an
// error only when the user has been ambiguous in a way we can't fix
// silently (unknown vendor; --vendor X but env XXX_API_KEY unset).
//
// Mock mode (LLM_RECALL_LLM_MOCK=1) shortcuts the key check: we still
// need a vendor for default-model resolution, but no API call will
// actually happen so a missing env var is fine.
func Resolve(flagVendor, flagModel, flagBaseURL string, cfg *config.Config) (ResolvedSettings, error) {
	var out ResolvedSettings

	// Step 1: pick vendor.
	switch {
	case flagVendor != "":
		v, err := parseVendor(flagVendor)
		if err != nil {
			return out, err
		}
		out.Vendor = v
	case cfg != nil && cfg.LLM.Vendor != "":
		v, err := parseVendor(cfg.LLM.Vendor)
		if err != nil {
			return out, err
		}
		out.Vendor = v
	default:
		// Auto-detect from environment. In mock mode, if no env var is
		// set, default to anthropic so deterministic tests still work
		// without exporting a key. With an env var set, mock mode still
		// honours the user's choice (so the OpenAI wire shape can be
		// exercised under mock).
		v, _, derr := DetectKey()
		switch {
		case derr == nil:
			out.Vendor = v
		case mockMode():
			out.Vendor = Anthropic
		default:
			return out, derr
		}
	}

	// Step 2: model.
	switch {
	case flagModel != "":
		out.Model = flagModel
	case cfg != nil && cfg.LLM.Model != "":
		out.Model = cfg.LLM.Model
	default:
		out.Model = DefaultModel(out.Vendor)
	}

	// Step 3: base URL.
	switch {
	case flagBaseURL != "":
		out.BaseURL = flagBaseURL
	case EnvBaseURL() != "":
		out.BaseURL = EnvBaseURL()
	case cfg != nil && cfg.LLM.BaseURL != "":
		out.BaseURL = cfg.LLM.BaseURL
	default:
		out.BaseURL = DefaultBaseURL(out.Vendor)
	}

	// Step 4: credential bundle (key + optional base_url / model).
	// Mock mode skips (no real call). Otherwise we run the W9 chain:
	// credentials.toml > keyring > env var > error. When the file
	// supplied base_url / model and the user didn't override on flag
	// or env, propagate them onto the resolved settings — that's why
	// CredForVendor returns a Cred and not just a key.
	if !mockMode() {
		c, err := CredForVendor(out.Vendor)
		if err != nil {
			return out, ErrNoCredentials
		}
		out.Key = c.APIKey
		// Promote BaseURL / Model from credentials.toml only when the
		// earlier resolution tier (flag / env / config.toml) didn't
		// already set something. We compare against the hardcoded
		// vendor default so a user-set "" in config.toml still loses
		// to a non-empty credentials.toml entry.
		if c.BaseURL != "" && (out.BaseURL == "" || out.BaseURL == DefaultBaseURL(out.Vendor)) {
			out.BaseURL = c.BaseURL
		}
		if c.Model != "" && (out.Model == "" || out.Model == DefaultModel(out.Vendor)) {
			out.Model = c.Model
		}
	}
	return out, nil
}

func parseVendor(s string) (Vendor, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "anthropic":
		return Anthropic, nil
	case "openai":
		return OpenAI, nil
	default:
		return "", fmt.Errorf("unknown vendor %q (expected anthropic|openai)", s)
	}
}

func envForVendor(v Vendor) string {
	switch v {
	case Anthropic:
		return "ANTHROPIC_API_KEY"
	case OpenAI:
		return "OPENAI_API_KEY"
	}
	return "<unknown>"
}
