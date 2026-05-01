package llm

import (
	"strings"
	"testing"
)

// TestRedact covers the eight regex classes plus boundary cases. Order
// matters because the `sk-ant-...` prefix must beat the bare `sk-...`
// pattern; we assert that the longer match wins by counting only one
// redaction for an Anthropic-style key.
func TestRedact(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantN    int
		wantHave []string // substrings that MUST appear in cleaned output
		wantGone []string // substrings that MUST NOT appear in cleaned output
	}{
		{
			name:     "empty",
			in:       "",
			wantN:    0,
			wantHave: []string{},
			wantGone: []string{},
		},
		{
			name:     "no PII",
			in:       "Hello, world! 这是一段普通文本，没有秘密。",
			wantN:    0,
			wantHave: []string{"Hello, world!", "普通文本"},
			wantGone: []string{"<redacted>"},
		},
		{
			name:     "anthropic key (longer prefix wins)",
			in:       "key=" + "sk-ant-" + "api03-aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			wantN:    1,
			wantHave: []string{"<redacted>"},
			wantGone: []string{"sk-ant-", "aBcDeFgHi"},
		},
		{
			name:     "openai-style sk- key",
			in:       "OPENAI_API_KEY=" + "sk-" + "aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			wantN:    1,
			wantHave: []string{"<redacted>"},
			wantGone: []string{"sk-aBc"},
		},
		{
			name:     "github personal token",
			in:       "ghp" + "_abcdefghijklmnopqrstuvwxyz0123456789",
			wantN:    1,
			wantGone: []string{"ghp_abc"},
		},
		{
			name:     "github oauth token",
			in:       "gho" + "_abcdefghijklmnopqrstuvwxyz0123456789",
			wantN:    1,
			wantGone: []string{"gho_abc"},
		},
		{
			name:     "slack bot token",
			in:       "xoxb" + "-1234567890-1234567890-aBcDeFgHiJkLmNoPqRsTuVwXyZ123456",
			wantN:    1,
			wantGone: []string{"xoxb-"},
		},
		{
			name:     "email",
			in:       "contact me at user@example.com tomorrow",
			wantN:    1,
			wantHave: []string{"<redacted>"},
			wantGone: []string{"user@example.com"},
		},
		{
			name:     "CN mobile",
			in:       "phone: 13800138000 ok",
			wantN:    1,
			wantGone: []string{"13800138000"},
		},
		{
			name:     "ipv4",
			in:       "server at 192.168.1.42 listens",
			wantN:    1,
			wantGone: []string{"192.168.1.42"},
		},
		{
			name: "mixed multi-class",
			in: "key=" + "sk-ant-" + "api03-AAAAAAAAAAAAAAAAAAAAAA email=foo@bar.io " +
				"phone=13800138000 ip=10.0.0.1",
			wantN: 4,
			wantGone: []string{
				"sk-ant-", "foo@bar.io", "13800138000", "10.0.0.1",
			},
		},
		{
			name: "duplicate same-class",
			in: "k1=sk-aaaaaaaaaaaaaaaaaaaaa k2=sk-bbbbbbbbbbbbbbbbbbbbb " +
				"k3=sk-cccccccccccccccccccccc",
			wantN:    3,
			wantGone: []string{"sk-aaa", "sk-bbb", "sk-ccc"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, n := Redact(tc.in)
			if n != tc.wantN {
				t.Errorf("count: got %d, want %d (out=%q)", n, tc.wantN, got)
			}
			for _, s := range tc.wantHave {
				if !strings.Contains(got, s) {
					t.Errorf("missing required substring %q in %q", s, got)
				}
			}
			for _, s := range tc.wantGone {
				if strings.Contains(got, s) {
					t.Errorf("found leaked substring %q in %q", s, got)
				}
			}
		})
	}
}
