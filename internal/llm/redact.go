// PII redaction. Runs *before* every LLM call (card / gold).
//
// Why redact at the wire boundary rather than at parse time: we want the
// SQLite cache (W2) to keep verbatim user content so the TUI / `ls`
// preview stay faithful to what the user typed. Only the network egress
// — which is the leakage surface — gets scrubbed.
//
// Patterns are listed in *most-specific-first* order. The longer-prefix
// API-key forms (`sk-ant-…`) must precede the bare `sk-…` form,
// otherwise the broader pattern will eat the longer match and our
// reported count will be wrong. Same for `gho_` / `ghp_`.
//
// The regex set is intentionally finite — we are not building a
// general-purpose DLP. Coverage is the eight classes called out in
// TASKS-W7.md §2; missing classes are documented in DEVDOC §4.
package llm

import (
	"regexp"
)

// redactToken is what we substitute for every match. Single token to
// keep the model's downstream summarisation behaviour stable
// regardless of how many items got redacted.
const redactToken = "<redacted>"

// redactPatterns is the ordered match set. Order matters — see file
// header. We compile once at package init (via MustCompile) so the hot
// path is allocation-free.
var redactPatterns = []*regexp.Regexp{
	// Anthropic API key: `sk-ant-…`. Must precede the generic `sk-…`.
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{20,}`),
	// OpenAI / generic `sk-…` API key.
	regexp.MustCompile(`sk-[a-zA-Z0-9\-_]{20,}`),
	// GitHub OAuth token (`gho_` + 36 chars).
	regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),
	// GitHub Personal Access Token (`ghp_` + 36 chars).
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),
	// Slack bot token (`xoxb-` + ≥ 50 chars including dashes).
	regexp.MustCompile(`xoxb-[a-zA-Z0-9\-]{50,}`),
	// Email address. Generic enough to catch most personal addresses;
	// will also nuke `user@example.com` test data, which is fine — the
	// LLM doesn't need it to summarise.
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	// CN mobile (11 digits, leading 1[3-9]). \b boundaries avoid
	// stripping inside longer numeric sequences (timestamps).
	regexp.MustCompile(`\b1[3-9]\d{9}\b`),
	// IPv4. Loose; matches some non-IPs like 999.999.999.999 too.
	// Cost of overmatching here is "summary loses an IP-like token" —
	// acceptable.
	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
}

// Redact runs every pattern against s, replaces every match with
// redactToken, and returns (cleaned-string, total-match-count). The
// count is reported to stderr by the caller as
// "redacted N item(s) before LLM call" — but only when N > 0.
//
// Implementation detail: we count by re-finding before replacing
// because ReplaceAllString does not return the number of replacements.
// At our prompt sizes (≤ 100 KB) the double-pass cost is irrelevant
// (microseconds).
func Redact(s string) (string, int) {
	if s == "" {
		return s, 0
	}
	out := s
	count := 0
	for _, re := range redactPatterns {
		matches := re.FindAllString(out, -1)
		if len(matches) == 0 {
			continue
		}
		count += len(matches)
		out = re.ReplaceAllString(out, redactToken)
	}
	return out, count
}
