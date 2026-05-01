# llm-recall

跨厂商 LLM CLI 会话搜索 + 恢复终端工具。一个命令找回任意 Claude Code / Codex / Gemini 历史会话并直接进入。

Homepage: https://recall.youchun.tech

> Sponsored by [YCAPI](https://api.youchun.tech)（详见 [Privacy & Promo](#privacy--promo) 段；W6 起会有 onboarding 一次同意流）

## Install

### macOS / Linux (Homebrew)

```bash
brew install xiao98/tap/llm-recall
```

### Windows (Scoop)

```powershell
scoop bucket add xiao98 https://github.com/xiao98/scoop-bucket
scoop install llm-recall
```

### From source

```bash
go install github.com/xiao98/llm-recall/cmd/llm-recall@latest
```

## Usage

```bash
llm-recall                    # 进 TUI，输入即筛三家会话
llm-recall ls --all           # 列出所有会话（CLI dump）
llm-recall ls --source codex  # 只列 codex
llm-recall --no-dry-run       # TUI 选中后真启动子进程进入会话
```

## Supported sources (W4)

claude / codex / gemini —— 自动扫 `~/.claude/`、`~/.codex/sessions/`、`~/.gemini/tmp/*/chats/`，无需配置。

## Configuration

`llm-recall` reads an optional `config.toml` from `~/.config/llm-recall/` (macOS / Linux) or `%APPDATA%\llm-recall\` (Windows). Both sections are optional; missing keys fall back to documented defaults.

```toml
[promo]
no_promo        = false   # kill switch for banner / search footer / sponsored line
search_footer   = false   # opt-in TUI list-bottom "discussions" line
banner_freq     = 1.0     # 0.0–1.0; chance the banner renders on each launch
cta_probability = 0.05    # 0.0–1.0; chance the banner shows the CTA line

[llm]
vendor   = ""             # "anthropic" | "openai" | "" (auto-detect from env)
model    = ""             # "" = vendor default (claude-haiku-4-5-20251001 / gpt-4o-mini)
base_url = ""             # "" = official endpoint; e.g. "https://dash.youchun.tech/v1" for the YCAPI relay
```

### LLM (BYOK)

The W7 commands `card` and `gold` call your own LLM API. **Never** put `api_key` / `key` into `config.toml` — `llm-recall` reads keys only from environment variables and warns if it sees one in the TOML file.

| Env var                 | Purpose                                                |
| ----------------------- | ------------------------------------------------------ |
| `ANTHROPIC_API_KEY`     | Used when vendor resolves to anthropic (default first) |
| `OPENAI_API_KEY`        | Used when vendor resolves to openai                    |
| `LLM_RECALL_BASE_URL`   | Optional escape hatch; overrides `[llm] base_url`      |

**Vendor / model / base URL priority (high → low)**:

1. CLI flag (`--vendor`, `--model`, `--llm-base-url`)
2. Environment variable (`LLM_RECALL_BASE_URL` for base URL only; vendor is auto-detected from whichever `*_API_KEY` is set, anthropic wins ties)
3. `config.toml` `[llm]` section
4. Hardcoded defaults (`anthropic` → `claude-haiku-4-5-20251001`, `openai` → `gpt-4o-mini`)

**Routing through a relay**: set `base_url = "https://dash.youchun.tech/v1"` (or your own gateway). The vendor selection still controls request shape (Anthropic Messages vs OpenAI Chat Completions) — your gateway must speak whichever format matches the vendor you choose.

### Examples

```bash
# Card a single session (BYOK; defaults to anthropic if ANTHROPIC_API_KEY is set)
llm-recall card 26348a6c

# Top-10 gold quotes from the last 14 days, OpenAI mini, plain markdown to a file
llm-recall gold --days 14 --vendor openai --model gpt-4o-mini --md -y > gold.md

# Force a different relay endpoint
llm-recall gold --llm-base-url https://dash.youchun.tech/v1 -y

# Dry-run cost preview without -y; type 'y' to proceed
llm-recall card 26348a6c
```

## Privacy & Promo

W4 阶段：仅本地读 jsonl，不上传任何对话内容到任何后端。

W6 起会加：启动 banner / 生图水印 / `gold` 命令（BYOK 调用你自己的 LLM key）。届时首次启动会有一次性同意流，可用 `--no-promo` 关 banner / footer。详见 DEVDOC §4。

（占位 / W6 起加完整版）

## License

MIT

## Screenshot

(W7 出截屏 GIF；W4 占位)
