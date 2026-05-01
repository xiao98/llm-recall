# llm-recall

> 跨厂商 LLM CLI 会话搜索 + 恢复终端工具。fzf 风格，支持 Claude Code / Codex / Gemini CLI；单 Go 二进制、零依赖、无 telemetry。

[![Release](https://img.shields.io/github/v/release/xiao98/llm-recall)](https://github.com/xiao98/llm-recall/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/xiao98/llm-recall.svg)](https://pkg.go.dev/github.com/xiao98/llm-recall)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Homepage: <https://recall.youchun.tech> · Sponsored by [YCAPI](https://api.youchun.tech)（详情见 [Privacy & Promo](#privacy--promo) 段，含 `--no-promo` 一键关闭）

<!-- screenshot: docs/screenshots/tui.gif | 录制脚本见 launch/storyboard.md §2 TUI search -->

## What it does

同时在用 Claude Code / Codex / Gemini 的开发者，三家会话散落在三处目录、三套 CLI、三个 `--resume` 语义，搜索历史只能各家 CLI 各搜一遍。`llm-recall` 把三家 jsonl 会话拉到一个本地 SQLite 索引里：

- **TUI 实时模糊搜索**：输入即筛，多关键词 AND，中文按 unicode 字处理
- **回车直接 resume**：自动 `cd` 到原 cwd，调对应 CLI 的 `--resume`（Gemini 退化交互式）
- **终端原生 stats / gold / card**：lipgloss 渲染，截屏即传播，无 PNG 后端
- **BYOK**：`gold` / `card` 调用你自己的 LLM key，对话内容**不**经任何中转

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

### Go install (任何平台)

```bash
go install github.com/xiao98/llm-recall/cmd/llm-recall@latest
```

### From source

```bash
git clone https://github.com/xiao98/llm-recall && cd llm-recall
go build -o llm-recall ./cmd/llm-recall
```

首次启动会进 onboarding 同意流（一次性），按 `Enter` 接受，按 `q` 退出。同意标记落 `~/.config/llm-recall/onboarding-accepted`。

## Usage

### TUI 模糊搜索（默认）

```bash
llm-recall                    # dry-run（选中只打印 resume 命令）
llm-recall --no-dry-run       # 选中后真起子进程进入会话
llm-recall --source codex     # 只搜 codex 会话
```

<!-- screenshot: docs/screenshots/tui.gif | 录制脚本见 launch/storyboard.md §2 TUI search -->

输入框 → 列表实时筛选 → ↑↓ 选中 → 右侧预览原文 + 命中片段高亮 → Enter resume。

### Stats heatmap

```bash
llm-recall stats              # GitHub-style 7-row 贡献日历 + 4×2 stats 面板
llm-recall stats --json       # 给 pipe 用
```

<!-- screenshot: docs/screenshots/stats.gif | 录制脚本见 launch/storyboard.md §1 stats -->

`1/2/3` 切 All time / Last 7 days / Last 30 days，`q` 退出。终端原生渲染（`⋅ ▒ ▓ █` 四档），lipgloss 24-bit 色，截屏直接发朋友圈。

### Gold（LLM 抽 Top 10 金句）

```bash
llm-recall gold                       # 默认扫 7 天，BYOK
llm-recall gold --days 30 -y          # 30 天，跳 cost 确认
llm-recall gold --md > gold.md        # 输出纯 markdown，pipe 友好
llm-recall gold --vendor openai --model gpt-4o-mini
```

<!-- screenshot: docs/screenshots/gold.gif | 录制脚本见 launch/storyboard.md §3 gold -->

单次 LLM 调用挑出你说过的 Top 10 金句 + 一句话点评。total > 100KB 自动 sample 50 会话。结果落 `~/.cache/llm-recall/llm-cache/`，7 天 TTL，`--no-cache` 强刷。Prompt 模板：[`internal/llm/prompts/gold.go`](internal/llm/prompts/gold.go)。

### Card（单会话名片）

```bash
llm-recall card 26348a6c              # 短 id 前缀模糊匹配
llm-recall card 26348a6c -y           # 跳 cost 确认
llm-recall card 26348a6c --no-cache
```

<!-- screenshot: docs/screenshots/card.gif | 录制脚本见 launch/storyboard.md §4 card -->

lipgloss 圆角卡片：会话头 + 首条用户消息（截 200 字）+ LLM 一句话总结（≤50 字）+ cwd。Prompt 模板：[`internal/llm/prompts/card.go`](internal/llm/prompts/card.go)。

### List（CLI dump）

```bash
llm-recall ls --all                   # 三家全列
llm-recall ls --source claude -n 20
llm-recall ls --no-cache              # 强刷索引
```

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

The W7 commands `card` and `gold` call **your own** LLM API. Never put `api_key` / `key` into `config.toml` — `llm-recall` reads keys only from environment variables and warns if it sees one in the TOML file.

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

## Supported sources

| Source | Path scanned                                  | Resume command                                                  |
| ------ | --------------------------------------------- | --------------------------------------------------------------- |
| claude | `~/.claude/projects/*/*.jsonl`                | `claude --resume <id>`                                          |
| codex  | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`| `codex resume <id>`                                             |
| gemini | `~/.gemini/tmp/<shortid>/chats/session-*.jsonl` | interactive: launch `gemini` then `/chat resume <id>` (\*)    |

(\*) Known limitation: `gemini --resume <UUID>` is rejected by gemini-cli upstream (issues #20480 / #23489); only `latest` / integer index are accepted as flag arg. `llm-recall` falls back to `cd <cwd> && gemini` and prints the `/chat resume` hint.

CWD resolution per source:
- **claude / codex**: read from session header line (`cwd` field)
- **gemini**: fallback chain `metadata.json > workspace.json > .project_root` (single-line abs-path text file); on full miss, title is prefixed `<gemini:<projectHash 前 8 位>>` so it's still findable

## Privacy & Promo

llm-recall 默认对你机器之外的世界**完全静默**，唯一例外是 BYOK 模式下你自己显式触发的 LLM 调用（去你自己配的 endpoint，不经 YCAPI）。

**Onboarding 一次同意流**（首次启动）：

> 跨厂商 LLM CLI 会话搜索 + 恢复终端工具
>
> Sponsored by YCAPI (https://api.youchun.tech)
>
> 营销注入说明（你看到的所有 YCAPI 痕迹）：
> - 启动时顶栏一条金句 banner，5% 概率含加群链接
> - stats 命令底部一行 sponsored 字符串
> - （可选）搜索结果底部讨论关联条
> - gold 功能用你自己的 LLM API key，不走 YCAPI 网关
>
> 关闭方式：
>   `--no-promo`               关 banner / footer / sponsored
>   `config.toml`              细粒度调（详见上文 Configuration）
>
> Enter 接受继续， q 退出

**不上传任何对话内容**：

- 索引：本地 SQLite，落系统 cache 目录（macOS `~/Library/Caches/llm-recall/`、Linux `~/.cache/llm-recall/`、Windows `%LOCALAPPDATA%\llm-recall\Cache\`）
- stats：纯本地聚合，无任何网络出口
- gold / card：调你自己配的 LLM endpoint（默认 Anthropic / OpenAI 官方），调用前 5 类 PII 正则脱敏（API key / OAuth token / email / 手机号 / IPv4）+ token / cost 估算 confirm（`-y` 跳过）+ 7 天结果缓存
- 无 telemetry，无 crash report，无"匿名使用统计"——一行也没有

**关 promo 的所有方式**：

```bash
llm-recall --no-promo                     # 单次
echo 'no_promo = true' >> ~/.config/llm-recall/config.toml   # 永久（写到 [promo] 段下）
```

`--no-promo` 关 banner / search footer / stats sponsored line / gold & card footer 全套，一刀切。

## Contributing

欢迎 issue / PR。

- **Bug report**：请贴 `llm-recall version` 输出 + 复现步骤 + 相关 jsonl 文件头 5 行（脱敏后）
- **PR 流程**：fork → branch → `go test ./... && go vet ./... && gofmt -l .` 全过 → PR；CI 跑 macOS / Linux / Windows × Go 1.22+
- **新 source adapter**：实现 `internal/adapter.SessionAdapter` + 可选 `FileLister` / `FileParser` 增量子接口（详见 [DEVDOC.md §2.1](DEVDOC.md)），开 PR 时附该家 jsonl 头部 schema 样本（脱敏）+ resume 命令实测

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgements

- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) / [bubbles](https://github.com/charmbracelet/bubbles) / [lipgloss](https://github.com/charmbracelet/lipgloss) — TUI 全家桶
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — 纯 Go SQLite，单二进制无 cgo
- [BurntSushi/toml](https://github.com/BurntSushi/toml) — config.toml 解析
- [sahilm/fuzzy](https://github.com/sahilm/fuzzy) — 模糊匹配
- [mattn/go-runewidth](https://github.com/mattn/go-runewidth) — CJK runewidth 对齐
- Sponsored by [YCAPI](https://api.youchun.tech) — LLM API 中转网关
