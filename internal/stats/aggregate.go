package stats

import (
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/xiao98/llm-recall/internal/adapter"
	"github.com/xiao98/llm-recall/internal/imggen"
)

// Aggregate walks a slice of cached sessions and produces the JSON payload
// the Python imggen backend expects.
//
// Filter: only sessions with UpdatedAt within the last `days` × 24h are
// counted. Token totals are computed by re-reading each session's source
// file (TokensFromFile) — this is unavoidable: the cache stores the user
// message body but not vendor-specific token metadata.
//
// `tokenFallbackPerMsg`: when no per-session token count is recoverable,
// estimate as message_count × this constant (TOKEN-AUDIT.md says all three
// vendors expose tokens; this is just a safety net for malformed files).
func Aggregate(sessions []adapter.Session, days int, tokenFallbackPerMsg int64, watermark bool) imggen.StatsRequest {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)

	perSource := map[string]int{}
	var total int
	var totalTokens int64
	var totalMessages int64
	var longest time.Duration
	topicCount := map[string]int{}

	for _, s := range sessions {
		if s.UpdatedAt.Before(cutoff) {
			continue
		}
		total++
		perSource[s.Source]++

		// Message-count estimate from body separator. Adapters concatenate
		// user messages with `\n---\n`; one separator means two messages.
		msgs := strings.Count(s.Body, "\n---\n") + 1
		if strings.TrimSpace(s.Body) == "" {
			msgs = 0
		}
		totalMessages += int64(msgs)

		// Token count via per-source field walk. Fall back to msg-count
		// heuristic only when the file gave us 0.
		t, _ := TokensFromFile(s.Source, s.FilePath)
		if t == 0 && tokenFallbackPerMsg > 0 {
			t = int64(msgs) * tokenFallbackPerMsg
		}
		totalTokens += t

		// Longest session = updated - started, but only when both are
		// non-zero and started <= updated (defensive against bad data).
		if !s.StartedAt.IsZero() && !s.UpdatedAt.IsZero() {
			d := s.UpdatedAt.Sub(s.StartedAt)
			if d > longest {
				longest = d
			}
		}

		// Topic word frequency.
		for _, w := range topicTokens(s.Body) {
			topicCount[w]++
		}
	}

	return imggen.StatsRequest{
		WindowDays:      days,
		TotalSessions:   total,
		TotalTokens:     totalTokens,
		TotalMessages:   totalMessages,
		TopTopics:       topNTopics(topicCount, 5),
		LongestSessionH: longest.Hours(),
		PerSource:       perSource,
		Watermark:       watermark,
		Format:          "square", // caller overrides to render two formats
	}
}

// topicTokens splits a body into "candidate topic" tokens. We walk runes:
//
//   - a run of alphanumerics (lowercased) becomes one token (English mode)
//   - a run of CJK characters becomes a sequence of 2-grams (Chinese mode)
//
// Anything shorter than 2 chars is dropped, as are stopwords. Returns the
// raw list — callers tally separately.
//
// Why 2-gram for CJK: single-character Chinese is too coarse to mean anything
// ("我", "了" survive even after stopword filtering). 2-grams catch most
// short topical noun phrases like "历史"、"项目"、"代码"; a real segmenter
// would do better but that pulls in jieba/cgo deps W5 §0 forbids.
func topicTokens(body string) []string {
	if body == "" {
		return nil
	}
	var out []string
	var enBuf []rune
	var cjkBuf []rune

	flushEN := func() {
		if len(enBuf) >= 2 {
			tok := strings.ToLower(string(enBuf))
			if !IsStopword(tok) {
				out = append(out, tok)
			}
		}
		enBuf = enBuf[:0]
	}
	flushCJK := func() {
		if len(cjkBuf) < 2 {
			cjkBuf = cjkBuf[:0]
			return
		}
		for i := 0; i < len(cjkBuf)-1; i++ {
			pair := string(cjkBuf[i : i+2])
			// drop bigrams whose first or second rune is a stopword
			a := string(cjkBuf[i : i+1])
			b := string(cjkBuf[i+1 : i+2])
			if IsStopword(a) || IsStopword(b) {
				continue
			}
			out = append(out, pair)
		}
		cjkBuf = cjkBuf[:0]
	}

	for _, r := range body {
		switch {
		case isCJK(r):
			flushEN()
			cjkBuf = append(cjkBuf, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			enBuf = append(enBuf, r)
		default:
			flushEN()
			flushCJK()
		}
	}
	flushEN()
	flushCJK()
	return out
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// topNTopics ranks (word -> count) and returns the top n keys. Ties are
// broken by lexicographic order so the output is deterministic across
// runs — important for tests and for reproducing a card.
func topNTopics(counts map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		// Discard topics that occurred only once: not statistically
		// interesting, dilutes the top-5 with single-shot noise.
		if v < 2 {
			continue
		}
		// Drop tokens that are pure ASCII numerals — they're rarely
		// "topical" and bubble up easily (file names, line numbers).
		if isAllDigits(k) {
			continue
		}
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.k
	}
	return out
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsDigit(r) {
			return false
		}
		i += size
	}
	return true
}
