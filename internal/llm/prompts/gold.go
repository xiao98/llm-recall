// Gold prompt templates. Hardcoded per TASKS-W7.md §3.
package prompts

// SystemGoldSentinel is a stable substring used by the mock client to
// detect gold-mode requests.
const SystemGoldSentinel = "quote curator"

// SystemGold is the system message for the `gold` command.
const SystemGold = `You are a quote curator for a developer's LLM chat history. Pick the Top 10 most quotable lines from the user's own messages — opinions, sharp expressions, principle statements, or witty observations. Each ≤ 60 chars. Skip generic questions like "how do I..." or "what does X mean".`

// SystemGoldStrict is used on the retry pass when the first response
// fails JSON parsing. It nails the JSON-only requirement to the front of
// the prompt so the retry has a higher chance of producing parsable output.
const SystemGoldStrict = `You are a quote curator. ONLY output valid JSON (a JSON array of 10 {quote, comment} objects). NO markdown, NO commentary, NO code fences. Pick the Top 10 most quotable lines from the user's own messages — opinions, sharp expressions, principle statements, or witty observations. Each quote ≤ 60 chars.`

// PromptGoldTpl: the user prompt template. {bodies} is replaced.
const PromptGoldTpl = `Session bodies (脱敏后, 时间排序):
<<<
{bodies}
>>>

Output JSON array (no markdown wrapper):
[
  {"quote": "用户原话", "comment": "≤ 30 char 入选理由"},
  ...
]

Strictly 10 items. quote 必须来自上面的内容，不要编造。`
