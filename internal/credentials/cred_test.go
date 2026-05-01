package credentials

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSaveLoadRoundTrip writes a Cred, reads it back, asserts fields
// survived. Also verifies the file ends up at the override path so we
// don't accidentally write to the user's real ~/.config during CI.
func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.toml")
	CredPathOverride = path
	t.Cleanup(func() { CredPathOverride = "" })

	c := Cred{
		Vendor:  "openai",
		APIKey:  "sk-test-1234",
		BaseURL: "https://dash.youchun.tech/v1",
		Model:   "gpt-4o-mini",
	}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("openai")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "sk-test-1234" || got.BaseURL != "https://dash.youchun.tech/v1" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

// TestLoadMissing returns ErrNotFound, not a generic IO error. detect.go
// relies on errors.Is to distinguish "no file" from "broken file".
func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	CredPathOverride = filepath.Join(dir, "nope.toml")
	t.Cleanup(func() { CredPathOverride = "" })

	_, err := Load("openai")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestSavePreservesOtherVendor: writing openai must NOT clobber an
// existing anthropic section. This is the bug the integration test
// would otherwise catch only after the user lost their key.
func TestSavePreservesOtherVendor(t *testing.T) {
	dir := t.TempDir()
	CredPathOverride = filepath.Join(dir, "credentials.toml")
	t.Cleanup(func() { CredPathOverride = "" })

	a := Cred{Vendor: "anthropic", APIKey: "sk-ant-A"}
	if err := Save(a); err != nil {
		t.Fatalf("save anthropic: %v", err)
	}
	o := Cred{Vendor: "openai", APIKey: "sk-O"}
	if err := Save(o); err != nil {
		t.Fatalf("save openai: %v", err)
	}
	got, err := Load("anthropic")
	if err != nil {
		t.Fatalf("re-load anthropic: %v", err)
	}
	if got.APIKey != "sk-ant-A" {
		t.Errorf("anthropic clobbered: %+v", got)
	}
}

// TestUseKeyringBlanksFileKey: when c.UseKeyring is true, Save must
// NOT write the secret into the file. The file is just a marker that
// says "real key lives in keyring".
func TestUseKeyringBlanksFileKey(t *testing.T) {
	dir := t.TempDir()
	CredPathOverride = filepath.Join(dir, "credentials.toml")
	t.Cleanup(func() { CredPathOverride = "" })

	c := Cred{Vendor: "openai", APIKey: "sk-secret", UseKeyring: true}
	if err := Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load("openai")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.APIKey != "" {
		t.Errorf("UseKeyring=true left api_key in file: %q", got.APIKey)
	}
	if !got.UseKeyring {
		t.Errorf("UseKeyring flag did not persist: %+v", got)
	}
}

// TestValidate exercises the obvious failure modes.
func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		c    Cred
		want bool
	}{
		{"empty vendor", Cred{APIKey: "x"}, true},
		{"unknown vendor", Cred{Vendor: "claude", APIKey: "x"}, true},
		{"missing key", Cred{Vendor: "openai"}, true},
		{"key on file path", Cred{Vendor: "openai", APIKey: "x"}, false},
		{"keyring sentinel ok with empty key", Cred{Vendor: "openai", UseKeyring: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if (err != nil) != tc.want {
				t.Errorf("Validate err=%v want-err=%v", err, tc.want)
			}
		})
	}
}

// TestPathRespectsOverride: the override is the only sanctioned way to
// redirect the path; covers the trivial getter so future refactors
// don't accidentally drop the hook.
func TestPathRespectsOverride(t *testing.T) {
	CredPathOverride = "/tmp/foo.toml"
	t.Cleanup(func() { CredPathOverride = "" })
	if got := Path(); !strings.HasSuffix(got, "foo.toml") {
		t.Errorf("Path()=%q want suffix foo.toml", got)
	}
	_ = runtime.GOOS // keep import live for future per-OS branches
}
