// Card prompt templates. Hardcoded per TASKS-W7.md §3 — these are NOT
// user-configurable. A V2 might support i18n / personality presets but
// W7 ships one well-tuned pair to keep behaviour predictable.
package prompts

// SystemCardSentinel is a stable substring the mock client uses to
// recognise card-mode requests (so it can return the right fixture).
// It also appears in the system prompt below, so changing the wording
// of SystemCard requires updating this sentinel too.
const SystemCardSentinel = "concise summarizer"

// SystemCard is the system message for the `card` command.
const SystemCard = `You are a concise summarizer. Given a developer's chat with an LLM, produce ONE short sentence (≤ 50 chars, prefer action-oriented Chinese if input is Chinese, else English) describing what the user is doing in this session. Do NOT start with "用户在..." or "The user is...". Just say the action.`

// PromptCardTpl is the user prompt template. {body} is replaced via
// strings.ReplaceAll at render time — we do not use text/template to
// keep escape semantics dead simple (no `{{ }}` collisions with code
// snippets in the body).
const PromptCardTpl = `Session content (脱敏后):
<<<
{body}
>>>

Output: just the sentence. No quotes, no markdown.`
