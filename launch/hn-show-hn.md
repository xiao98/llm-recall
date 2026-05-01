# Hacker News — Show HN draft

> Submit on HN once. The self-comment goes as the **first** reply, posted by you, immediately after the submission appears. This is the Show HN convention — extra technical detail and disclosures.

---

**Title** (en-dash `–`, NOT hyphen `-`):

```
Show HN: llm-recall – fzf-style search across Claude/Codex/Gemini CLI sessions
```

**URL**:

```
https://recall.youchun.tech
```

(Landing page. README + GitHub repo are linked from there. Show HN guidelines prefer a demoable URL over a raw GitHub repo.)

---

**Body** (260 words, four paragraphs):

I use Claude Code, Codex, and Gemini CLI side by side, and finding past sessions is per-tool: each stores jsonl in its own directory layout (`~/.claude/projects/`, `~/.codex/sessions/YYYY/MM/DD/`, `~/.gemini/tmp/<shortid>/chats/`) with a different metadata header, and each `--resume` flag has different semantics. I wanted one command that searches all of them and resumes whichever you pick.

`llm-recall` is a small Go TUI that pulls all three into one local SQLite index and gives fzf-style fuzzy search across them. Hit Enter and it `chdir`s to the original cwd and runs that vendor's resume command. It also has three terminal-native sub-commands: `stats` (GitHub-contribution-style heatmap + 4×2 panel, pure local), `gold` (single LLM call that picks your top 10 quotes from the last N days, BYOK), and `card <id>` (lipgloss rounded card for one session). All three render directly in the terminal — no PNG backend, screenshot is the share format.

Tech: Go 1.22, Charm bubbletea / lipgloss, `modernc.org/sqlite` (pure Go, no cgo), single static binary across mac/linux/windows × amd64/arm64, ~6MB. No daemon, no telemetry. The only outbound traffic ever is `gold`/`card` calling **your own** LLM endpoint (auto-detected from `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`), with regex PII redaction (5 classes) and a token+cost confirm before each call.

Known limitations: Gemini `--resume <UUID>` is rejected upstream (gemini-cli #20480 / #23489) so `llm-recall` falls back to `cd <cwd> && gemini` plus a `/chat resume <id>` hint. Adapters for aider / opencode / cline are V2 — the adapter interface is documented in DEVDOC.md if you want to PR one.

Repo: <https://github.com/xiao98/llm-recall>. Install: `brew install xiao98/tap/llm-recall`.

---

**Self-comment** (post immediately after submission, as the **first** reply on your own thread):

```
Some technical details that didn't fit in the post:

— **Adapter architecture**: each vendor has a parser implementing
`SessionAdapter` (Discover / Read / ResumeCommand). For incremental
scanning Codex and Gemini also implement an optional `FileLister` +
`FileParser` pair so we only re-parse files whose mtime moved. Claude
is a flat directory, full scan is fine. Adding a new vendor is one file
+ a header schema sample — see `internal/adapter/claude.go` as the
cleanest reference.

— **Storage**: `modernc.org/sqlite`, pure Go, no cgo. DB at
`~/.cache/llm-recall/index.db` (mac: `~/Library/Caches/llm-recall/`,
windows: `%LOCALAPPDATA%\llm-recall\Cache\`). FTS5 + sahilm/fuzzy on top
for multi-keyword AND with CJK rune-level matching.

— **stats / gold / card rendering**: all lipgloss, all terminal-native.
The stats heatmap is the GitHub contribution graph in `⋅ ▒ ▓ █` four
buckets with 24-bit color. I started this project planning a Pillow +
Scaleway PNG backend for shareable images, but a quick prototype showed
that a screenshot of the terminal output already looks great and ships
zero infra dependency, so I yanked the backend in W5. Single binary, no
deploy.

— **BYOK privacy posture**: `gold` and `card` are the only network
calls in the entire tool. Vendor is auto-detected from `*_API_KEY`
(ANTHROPIC wins ties), and you can override with `--vendor / --model /
--llm-base-url` flags or `[llm]` section in `~/.config/llm-recall/
config.toml`. Five classes of PII are regex-redacted client-side before
the call (API keys, OAuth tokens, email, phone, IPv4). Token+cost
estimate is shown and confirmed (or `-y` to skip). Results cache locally
for 7 days.

— **vs Dicklesworthstone/coding_agent_session_search**: theirs is Rust
with ~11 vendor adapters, optimized for breadth. Mine is Go with 3 and
optimized for the screenshot-share loop (stats heatmap + gold/card with
lipgloss). Different shapes; not mutually exclusive.

— **Sponsorship transparency**: project is sponsored by YCAPI (an LLM
API relay). I'm calling this out here because the codebase has banner /
stats footer / search footer with a sponsored line. All of it is gated
behind a one-time onboarding consent screen and a `--no-promo` kill
switch (`[promo] no_promo = true` in config also works). gold/card do
**not** default-route through the relay — they hit whatever endpoint
your `*_API_KEY` resolves to. https://api.youchun.tech if curious.

Happy to answer questions.
```
