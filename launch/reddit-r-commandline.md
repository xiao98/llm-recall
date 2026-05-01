# Reddit r/commandline submission draft

> 发到 r/commandline。Title ≤ 100 chars per Reddit limit. Functional / descriptive — no hype words. Body 200–400 words.
>
> Pre-flight: 确认账号有 r/commandline 历史发言，否则 mod queue 容易压；检查子版规（无重复内容、无 self-promo only）。

---

**Title** (88 chars):

```
llm-recall – fuzzy search & resume across Claude / Codex / Gemini CLI sessions in one TUI
```

**Link** (Reddit "url" field): `https://recall.youchun.tech` (landing page; README also linked from there)

**Body** (320 words):

If you use more than one LLM CLI side by side — Claude Code for refactors, Codex for scripts, Gemini for cross-model checks — you've probably noticed that searching past sessions is per-tool and per-format. Each one stores jsonl in its own layout (`~/.claude/projects/`, `~/.codex/sessions/YYYY/MM/DD/`, `~/.gemini/tmp/<shortid>/chats/`) with a different metadata header, and each `--resume` flag has different semantics.

`llm-recall` is a small Go TUI that pulls all three into one local SQLite index and gives you fzf-style fuzzy search across them. Hit Enter on a result and it `chdir`s to the original cwd and runs that vendor's resume command (Gemini falls back to `gemini` + a `/chat resume <id>` hint because `gemini --resume <UUID>` isn't accepted by upstream — issues #20480 / #23489).

It also ships three terminal-native sub-commands:

- `llm-recall stats` — GitHub-contribution-style heatmap + 4×2 panel (sessions, tokens, streaks, per-source breakdown). Pure local aggregation, no network.
- `llm-recall gold` — single LLM call that picks your top 10 quotes from the last N days (BYOK, prompt is in `internal/llm/prompts/gold.go`).
- `llm-recall card <id>` — lipgloss rounded card for a single session.

Tech: Go 1.22 + Charm bubbletea / lipgloss, `modernc.org/sqlite` (pure Go, no cgo), single static binary, ~6MB. No daemon, no telemetry, no cloud — the only outbound traffic ever is `gold`/`card` calling **your own** LLM endpoint (Anthropic or OpenAI auto-detected from `*_API_KEY`, with PII redaction + cost confirm before each call).

**Install**: `brew install xiao98/tap/llm-recall` / `scoop install llm-recall` / `go install github.com/xiao98/llm-recall/cmd/llm-recall@latest`.

**Sponsorship transparency**: project is sponsored by YCAPI (an LLM API relay). All sponsored UI is gated behind a one-time onboarding consent screen and a `--no-promo` kill switch (also `[promo] no_promo = true` in config.toml). gold/card never route through YCAPI — they call whatever endpoint your `*_API_KEY` resolves to.

Repo: <https://github.com/xiao98/llm-recall>

---

**TL;DR**: Single Go binary that fuzzy-searches Claude/Codex/Gemini CLI session history in one TUI and resumes any of them with one keypress. Stats heatmap + gold quote miner + session card included. BYOK, no telemetry, MIT.

---

## Comments I expect & how I'll respond

> Use these as rough scripts. Don't paste verbatim — match the commenter's tone. Never escalate.

1. **"Isn't this just `Dicklesworthstone/coding_agent_session_search` (725★ Rust)?"**

   > Different scope. They cover more vendors at the parser layer (11 adapters last I checked) and it's a great Rust impl. `llm-recall` is narrower (3 vendors) but bundles a TUI + stats heatmap + gold/card commands so screenshots are shareable out of the box. If you want widest vendor coverage today, use theirs; if you want the bubbletea TUI + stats, try this. Not mutually exclusive.

2. **"Why no fzf integration / pipe-friendly mode?"**

   > fzf is great for files but resume needs the vendor-specific cmd + cwd, which fzf can't carry. That said, `gold --md` outputs plain markdown for piping, and `ls --no-cache --source codex` is dump-friendly. A `--print-resume-cmd` flag for piping into a shell is reasonable — file an issue and I'll prioritize.

3. **"Privacy — what does it upload?"**

   > Index is local SQLite. stats is pure local aggregation, zero network. gold/card call **your own** LLM endpoint (auto-detected from `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`, never through the sponsor's relay unless you explicitly set `[llm] base_url`). PII is regex-redacted before the API call (5 classes: API keys, OAuth tokens, email, phone, IPv4) and you get a token+cost confirm prompt unless `-y`. No telemetry, no crash report.

4. **"Gemini `--resume <id>` doesn't work for me."**

   > Known upstream limitation (gemini-cli #20480 / #23489): the flag accepts `latest` or an integer index, not UUIDs. `llm-recall` falls back to `cd <cwd> && gemini` and prints the `/chat resume <id>` hint so you paste it into the interactive prompt. Once upstream lands real UUID support I'll switch to the flag path.

5. **"Token cost concerns."**

   > BYOK + before-call cost preview (length/4 estimation) + 7-day result cache + `-y` to skip confirm in scripts. For gold the worst case is one call over up to 100KB of text (auto-sampled to 50 sessions if larger). With Haiku-class models that's < $0.01.

6. **"Will this support aider / opencode / cline?"**

   > V2. The adapter interface (`SessionAdapter` + optional `FileLister`/`FileParser` for incremental scanning) is documented in DEVDOC.md §2.1. Easiest path is a PR — `internal/adapter/claude.go` is the cleanest reference impl.

7. **"Why Go and not Rust?"**

   > Single static binary across mac/linux/windows × amd64/arm64 with goreleaser, plus the bubbletea/lipgloss TUI ecosystem is mature. If you prefer Rust, Dicklesworthstone's project covers more vendors. Both are valid.

8. **"How does the sponsorship work — is it pay-to-route?"**

   > No. Sponsorship pays for development time. The relay is a separate product and `llm-recall` doesn't default-route through it — it always calls whatever endpoint your `*_API_KEY` resolves to. The only sponsored UI is the banner / stats footer / search footer, all gated behind onboarding consent + `--no-promo`. Source is open, audit yourself.
