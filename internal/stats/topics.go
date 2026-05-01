// Top-N topics extracted from session bodies.
//
// Goal: a one-line summary of "what did the user actually work on
// recently" surfaced above the 4×2 panel. Restored at W9 after
// W5-rev2 dropped an earlier version with a different scoring model.
//
// Algorithm (deliberately simple — full TF-IDF / TextRank would dwarf
// the rest of stats):
//
//  1. For each session body, lowercase + extract:
//     - English tokens via Unicode letter runs (length ≥ 2)
//     - Chinese tokens via 2-char sliding bigrams over the CJK
//     Unified Ideograph range
//  2. Drop stopwords (see stopwords.go).
//  3. Count by token; rank by count desc, name asc.
//  4. Return the top N.
//
// The bigram approach for Chinese is intentional: a real segmenter
// (jieba etc) would pull in cgo or an MB-sized dict. Bigrams are
// noisier but adequate for terminal-display "topics" and add zero
// dependency. The stopword list filters out the most common 2-char
// noise ("我们" / "可以" / etc).
package stats

import (
	"sort"
	"strings"
	"unicode"

	"github.com/xiao98/llm-recall/internal/adapter"
)

// TopicCount is one (token, count) pair returned by TopTopics.
type TopicCount struct {
	Token string
	Count int
}

// TopTopics returns the top-N tokens by frequency across all session
// bodies. Empty input yields an empty slice (never nil-panics into
// the renderer).
func TopTopics(sessions []adapter.Session, n int) []TopicCount {
	if n <= 0 || len(sessions) == 0 {
		return nil
	}
	counts := map[string]int{}
	for _, s := range sessions {
		if s.Body == "" {
			continue
		}
		extractTokens(s.Body, counts)
	}
	if len(counts) == 0 {
		return nil
	}
	out := make([]TopicCount, 0, len(counts))
	for tok, c := range counts {
		out = append(out, TopicCount{Token: tok, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Token < out[j].Token
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// extractTokens scans `body` and increments `counts` for each non-stop
// token discovered. We make two passes per character: when we see an
// ASCII letter run, harvest a single English token; when we see a CJK
// run, slide a 2-char window. Other characters (punctuation, digits,
// whitespace) act as separators.
func extractTokens(body string, counts map[string]int) {
	rs := []rune(body)
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch {
		case isASCIILetter(r):
			j := i
			for j < len(rs) && isASCIILetter(rs[j]) {
				j++
			}
			tok := strings.ToLower(string(rs[i:j]))
			if !isEnglishStopword(tok) {
				counts[tok]++
			}
			i = j
		case isCJK(r):
			// Bigram window over the CJK run.
			j := i
			for j < len(rs) && isCJK(rs[j]) {
				j++
			}
			run := rs[i:j]
			for k := 0; k+1 < len(run); k++ {
				tok := string(run[k : k+2])
				if !isChineseStopword(tok) {
					counts[tok]++
				}
			}
			i = j
		default:
			i++
		}
	}
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isCJK matches the most common Chinese / Japanese / Korean ideograph
// blocks. Doesn't try to be exhaustive — anything outside this range
// is treated as a separator and won't generate bigrams.
func isCJK(r rune) bool {
	switch {
	case unicode.Is(unicode.Han, r):
		return true
	case r >= 0x3040 && r <= 0x30FF: // hiragana + katakana
		return true
	}
	return false
}
