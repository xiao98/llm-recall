// Onboarding consent state.
//
// State machine is dead simple:
//
//	Not accepted → user runs llm-recall → onboarding screen → Enter ⇒
//	write a sentinel JSON file → second launch sees the file and skips
//	the screen.
//
// The sentinel lives at OnboardingPath() (ConfigDir/llm-recall/
// onboarding-accepted, NOT in the project directory — that's a hard
// invariant from the W6 §0 permission list). JSON content is the
// timestamp the user accepted at and the binary's version, so a future
// privacy-policy bump can detect "user accepted v0.2.0 but we're now
// v0.5.0 with a different banner" and re-prompt. We don't act on that
// today; the field is forward-looking insurance.
package promo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/xiao98/llm-recall/internal/config"
)

// AcceptedRecord is the on-disk shape. Public so tests in other packages
// can decode the file without copy-pasting field names.
type AcceptedRecord struct {
	AcceptedAt string `json:"accepted_at"`
	Version    string `json:"version"`
}

// onboardingPathOverride lets tests redirect the path without mutating
// $HOME / $APPDATA. When set, OnboardingPath returns this value verbatim.
var onboardingPathOverride string

// SetOnboardingPathForTest is the only sanctioned way to override the
// path. Tests should pair it with t.Cleanup(func(){ ...("") }).
func SetOnboardingPathForTest(p string) { onboardingPathOverride = p }

// OnboardingPath returns the absolute path to the onboarding-accepted
// sentinel. Co-located with config.toml under ConfigDir/llm-recall/ so a
// privacy-conscious user can wipe both with one rm -rf.
func OnboardingPath() string {
	if onboardingPathOverride != "" {
		return onboardingPathOverride
	}
	dir := filepath.Dir(config.ConfigPath())
	return filepath.Join(dir, "onboarding-accepted")
}

// OnboardingAccepted reports whether the sentinel exists. We do not try
// to validate the JSON inside — even a corrupt or empty file counts as
// "user has been through onboarding once", because re-prompting on file
// corruption is more annoying than the alternative.
func OnboardingAccepted() bool {
	_, err := os.Stat(OnboardingPath())
	return err == nil
}

// WriteOnboardingAccepted creates the sentinel with the current UTC
// timestamp and the supplied version string. The directory is created
// with 0700 (user-only) so a multi-user system doesn't leak the fact
// that this user has consented to anything.
func WriteOnboardingAccepted(version string) error {
	path := OnboardingPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	rec := AcceptedRecord{
		AcceptedAt: time.Now().UTC().Format(time.RFC3339),
		Version:    version,
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	// 0600: user-readable only. The file is just a marker but it
	// implicitly reveals that llm-recall has been run on this machine,
	// which is mildly fingerprintable. No need to expose to other users.
	return os.WriteFile(path, data, 0o600)
}
