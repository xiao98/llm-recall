package llm

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/xiao98/llm-recall/internal/credentials"
)

// redirectCreds isolates the test from any real credentials.toml on
// the dev machine. Returns the path so tests can write fixtures into
// it. Tests that just want "no file" can ignore the path.
func redirectCreds(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.toml")
	credentials.CredPathOverride = path
	t.Cleanup(func() { credentials.CredPathOverride = "" })
	return path
}

func TestDetectKey(t *testing.T) {
	cases := []struct {
		name       string
		ant, oai   string
		wantVendor Vendor
		wantErr    bool
	}{
		{"only anthropic", "sk-ant-xxx", "", Anthropic, false},
		{"only openai", "", "sk-xxx", OpenAI, false},
		{"both → anthropic wins", "sk-ant-xxx", "sk-xxx", Anthropic, false},
		{"neither", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			redirectCreds(t)
			t.Setenv("ANTHROPIC_API_KEY", tc.ant)
			t.Setenv("OPENAI_API_KEY", tc.oai)
			v, _, err := DetectKey()
			if tc.wantErr {
				if !errors.Is(err, ErrNoKey) {
					t.Fatalf("want ErrNoKey, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v != tc.wantVendor {
				t.Errorf("vendor: got %s, want %s", v, tc.wantVendor)
			}
		})
	}
}

func TestKeyForVendor(t *testing.T) {
	redirectCreds(t)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-foo")
	if _, err := KeyForVendor(Anthropic); !errors.Is(err, ErrNoKey) {
		t.Errorf("expected ErrNoKey for empty anthropic env, got %v", err)
	}
	k, err := KeyForVendor(OpenAI)
	if err != nil || k != "sk-foo" {
		t.Errorf("openai: got (%q,%v), want (sk-foo,nil)", k, err)
	}
}

// TestCredentialsFileBeatsEnv: W9 priority chain — credentials.toml
// wins over an env var even when both are set.
func TestCredentialsFileBeatsEnv(t *testing.T) {
	redirectCreds(t)
	if err := credentials.Save(credentials.Cred{
		Vendor: "openai",
		APIKey: "sk-from-file",
	}); err != nil {
		t.Fatalf("seed creds: %v", err)
	}
	t.Setenv("OPENAI_API_KEY", "sk-from-env")
	t.Setenv("ANTHROPIC_API_KEY", "")
	c, err := DetectCred()
	if err != nil {
		t.Fatalf("DetectCred: %v", err)
	}
	if c.APIKey != "sk-from-file" {
		t.Errorf("expected file key to win, got %q", c.APIKey)
	}
}
