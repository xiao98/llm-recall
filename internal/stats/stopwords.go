// Stopword lists used by topics.go. Two pools because we tokenise EN +
// ZH with different rules — Chinese words are sliced on character
// boundaries, English tokens come out lowercased.
//
// Lists are intentionally short (≤ 80 entries each). The goal is to
// kick out the most common filler words (functional verbs, pronouns,
// articles, the names of the tools we observe in basically every
// session). Anything more aggressive risks suppressing the actual
// "topic" of the user's work.
package stats

// englishStopwords are matched case-insensitively after lowercasing.
// We omit very domain-specific words ("function", "class") because
// those ARE topics for a code-history tool. The brand of the LLM CLI
// the user is talking to (claude / codex / gemini) is included here
// since otherwise it dominates every Top 5 and tells the user nothing.
var englishStopwords = map[string]struct{}{
	// articles / pronouns
	"a": {}, "an": {}, "the": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"i": {}, "me": {}, "my": {}, "we": {}, "us": {}, "our": {},
	"you": {}, "your": {}, "yours": {},
	"he": {}, "she": {}, "it": {}, "its": {}, "they": {}, "them": {}, "their": {},
	// auxiliaries / common verbs
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {},
	"do": {}, "does": {}, "did": {}, "done": {}, "doing": {},
	"have": {}, "has": {}, "had": {}, "having": {},
	"can": {}, "could": {}, "should": {}, "would": {}, "will": {}, "shall": {}, "may": {}, "might": {}, "must": {},
	"get": {}, "got": {}, "make": {}, "made": {}, "go": {}, "goes": {}, "went": {},
	// prepositions / conjunctions
	"and": {}, "or": {}, "but": {}, "if": {}, "then": {}, "else": {}, "so": {}, "as": {},
	"of": {}, "in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "with": {}, "to": {}, "from": {},
	"into": {}, "out": {}, "up": {}, "down": {}, "over": {}, "about": {},
	// fillers / answers
	"yes": {}, "no": {}, "ok": {}, "okay": {}, "yeah": {}, "yep": {}, "nope": {},
	"please": {}, "thanks": {}, "thank": {},
	// LLM CLI brand names (always present, never a "topic")
	"claude": {}, "codex": {}, "gemini": {}, "gpt": {}, "openai": {}, "anthropic": {},
	"llm": {}, "ai": {},
	// generic dev verbs we see in every prompt
	"help": {}, "use": {}, "using": {}, "run": {}, "running": {}, "fix": {}, "add": {}, "see": {}, "check": {},
	"want": {}, "need": {}, "try": {}, "let": {}, "show": {}, "tell": {}, "know": {},
	// the word "code" / "file" — too generic
	"code": {}, "file": {}, "files": {}, "function": {},
}

// chineseStopwords are matched against runes/bigrams from the
// character-segmented stream. CJK has no whitespace so we compare
// tokens that came out of the segmenter (most are 1–2 char fillers).
var chineseStopwords = map[string]struct{}{
	// 单字 — 极常见
	"的": {}, "了": {}, "和": {}, "是": {}, "在": {}, "也": {}, "都": {}, "就": {}, "还": {}, "或": {},
	"我": {}, "你": {}, "他": {}, "她": {}, "它": {}, "们": {}, "这": {}, "那": {}, "有": {}, "没": {},
	"很": {}, "要": {}, "把": {}, "让": {}, "给": {}, "对": {}, "从": {}, "到": {}, "为": {}, "被": {},
	"上": {}, "下": {}, "里": {}, "外": {}, "中": {}, "内": {}, "前": {}, "后": {}, "再": {}, "又": {},
	"会": {}, "能": {}, "可": {}, "得": {}, "着": {}, "过": {}, "去": {}, "来": {}, "做": {}, "用": {},
	// 双字 — 高频虚词
	"我们": {}, "你们": {}, "他们": {}, "她们": {}, "它们": {},
	"什么": {}, "怎么": {}, "为什么": {}, "如何": {}, "哪里": {}, "哪个": {},
	"现在": {}, "已经": {}, "可以": {}, "应该": {}, "需要": {}, "可能": {}, "或者": {}, "因为": {}, "所以": {},
	"这个": {}, "那个": {}, "这样": {}, "那样": {}, "一个": {}, "一下": {}, "一些": {},
	"问题": {}, "时候": {}, "地方": {}, "东西": {},
	"谢谢": {}, "请问": {},
	// 高频动词 — 太通用，不是 topic
	"运行": {}, "看看": {}, "帮我": {}, "执行": {}, "解释": {},
}

// isEnglishStopword reports whether the lowercased token is in the EN
// pool. Tokens 1 char long are also dropped (no information).
func isEnglishStopword(tok string) bool {
	if len(tok) <= 1 {
		return true
	}
	_, ok := englishStopwords[tok]
	return ok
}

// isChineseStopword for CJK tokens. We additionally drop pure-digit
// tokens like "2026" / "100" — they're noisy in topic lists.
func isChineseStopword(tok string) bool {
	if _, ok := chineseStopwords[tok]; ok {
		return true
	}
	allDigit := true
	for _, r := range tok {
		if r < '0' || r > '9' {
			allDigit = false
			break
		}
	}
	return allDigit
}
