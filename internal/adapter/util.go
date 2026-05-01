package adapter

import "strings"

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
