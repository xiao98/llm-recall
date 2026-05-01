package adapter

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// CleanTitle normalises a session-list title for single-line table rendering.
// Newlines / tabs / carriage returns become spaces and consecutive whitespace
// collapses to one. W1 surfaced multi-line titles that broke tabwriter
// alignment; every adapter now funnels titles through here.
func CleanTitle(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

// ParseTime accepts the three timestamp shapes we have seen in the wild across
// the three vendors:
//   - RFC3339Nano (claude / gemini)
//   - RFC3339 (older codex)
//   - .NET 7-digit fractional seconds Z (codex on Windows: 2026-04-25T10:55:00.0000000Z)
//
// Returns zero time + non-nil error when none match. Callers decide whether to
// warn or silently fill zero — adapters generally pick silent so a single
// malformed timestamp doesn't spam stderr per session.
func ParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05.0000000Z07:00", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp: %q", s)
}

// IsCodexInjectedUserText returns true when a codex "user" message is
// actually a CLI-synthesised pseudo-user payload that we should never use as
// a session title (analogous to claude's <system-reminder> filter). The two
// observed prefixes are:
//   - "<environment_context>"  CLI-injected env / cwd dump
//   - "[Imported from Claude]" thread-import marker prepended to a real msg
//
// Whitespace at the head is tolerated. The check is on the FIRST visible
// non-empty line because some imports prepend the marker as line 1 with the
// real prompt below.
func IsCodexInjectedUserText(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	if strings.HasPrefix(t, "<environment_context>") {
		return true
	}
	if strings.HasPrefix(t, "[Imported from Claude]") {
		return true
	}
	return false
}

// SafeUTF8Truncate clamps `s` to at most `maxBytes` bytes without splitting a
// rune. Returns `s` unchanged when already within budget. Body fields written
// to the SQLite cache use this so we never store half-codepoints.
func SafeUTF8Truncate(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	// Walk to the highest rune boundary <= maxBytes.
	cut := 0
	for i := 0; i < len(s); {
		_, sz := utf8.DecodeRuneInString(s[i:])
		if i+sz > maxBytes {
			break
		}
		i += sz
		cut = i
	}
	return s[:cut]
}
