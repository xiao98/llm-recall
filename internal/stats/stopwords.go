// Package stats — internal helpers for the `llm-recall stats` command:
// session aggregation, token parsing per vendor, top-topic extraction,
// platform-specific file open helper, and stopwords.
package stats

// stopwords blocks high-frequency tokens that would otherwise dominate
// the top-topics tally. Mix of English programmer-speak + Chinese 助词;
// trimmed to ~70 entries — enough to be useful, small enough to vet.
//
// W5 §5 calls for "50 词起步" so the list overshoots that on purpose.
// Adding a word: keep this list lowercased and don't reuse it for any
// other purpose; the topic extractor lowercases every candidate before
// the lookup.
var stopwords = map[string]struct{}{
	// English filler
	"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"and": {}, "or": {}, "but": {}, "if": {}, "then": {}, "in": {}, "on": {},
	"at": {}, "to": {}, "of": {}, "for": {}, "by": {}, "with": {}, "as": {},
	"i": {}, "you": {}, "he": {}, "she": {}, "we": {}, "they": {}, "it": {},
	"this": {}, "that": {}, "these": {}, "those": {}, "be": {}, "do": {},
	"have": {}, "has": {}, "had": {}, "not": {}, "so": {}, "no": {}, "yes": {},
	"can": {}, "will": {}, "just": {}, "now": {},
	// Chinese 助词 / 高频字
	"的": {}, "了": {}, "吗": {}, "呢": {}, "我": {}, "你": {}, "他": {},
	"这": {}, "那": {}, "有": {}, "在": {}, "是": {}, "和": {}, "也": {},
	"就": {}, "都": {}, "把": {}, "为": {}, "从": {}, "给": {}, "对": {},
	"会": {}, "要": {}, "去": {}, "来": {}, "好": {}, "啊": {}, "吧": {},
	"什": {}, "么": {}, "怎": {}, "样": {}, "一": {}, "个": {}, "下": {},
	"上": {}, "里": {}, "中": {}, "等": {},
}

// IsStopword reports whether the given token (lowercased by caller) is in
// the embedded blocklist.
func IsStopword(s string) bool {
	_, ok := stopwords[s]
	return ok
}
