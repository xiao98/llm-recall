**English** | [中文](README.zh-CN.md)

# llm-recall

> Cross-vendor LLM CLI session search and resume — fzf-style. Claude Code, Codex, Gemini CLI. Single Go binary, zero deps, no telemetry.

[![Release](https://img.shields.io/github/v/release/xiao98/llm-recall)](https://github.com/xiao98/llm-recall/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/xiao98/llm-recall.svg)](https://pkg.go.dev/github.com/xiao98/llm-recall)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Homepage: <https://recall.youchun.tech>

> Created within the YC TECH community: <https://recall.youchun.tech>

Promo (banner / footer / community CTA) is on by default — see [Privacy & Promo](#privacy--promo); `--no-promo` disables everything.

<!-- screenshot: docs/screenshots/tui.gif | 录制脚本见 launch/storyboard.md §2 TUI search -->

## What it does

If you use Claude Code, Codex, and Gemini side by side, your sessions live in three different directories under three different CLIs with three different `--resume` semantics. Searching history means searching each one separately. `llm-recall` indexes all three jsonl stores into one local SQLite cache:

- **Live fuzzy TUI search** — type to filter, multi-keyword AND, CJK-aware
- **Enter to resume** — auto-`cd` back to the original cwd, then dispatch the right CLI's `--resume` (Gemini falls back to interactive)
- **Terminal-native stats / gold / card** — lipgloss rendering, screenshot-friendly, no PNG backend
- **BYOK** — `gold` / `card` use your own LLM key; conversation content never goes through any relay

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

### Go install (any platform)

```bash
go install github.com/xiao98/llm-recall/cmd/llm-recall@latest
```

### From source

```bash
git clone https://github.com/xiao98/llm-recall && cd llm-recall
go build -o llm-recall ./cmd/llm-recall
```

First launch goes straight into the TUI. To use `gold` / `card` (LLM-powered), run `llm-recall login` once to set up your provider (API key lands in `~/.config/llm-recall/credentials.toml`, chmod 600; opt-in `--use-keyring` stores it in the OS keyring instead).

## Usage

### Fuzzy search TUI (default)

```bash
llm-recall                    # default: pick a row → spawn the matching CLI in the original cwd
llm-recall --dry-run          # debug: print the resume command instead of executing
llm-recall --source codex     # restrict to one adapter
```

<!-- screenshot: docs/screenshots/tui.gif | 录制脚本见 launch/storyboard.md §2 TUI search -->

Type to filter → arrow keys to select → preview pane on the right with hit highlights → Enter to resume.

### Stats heatmap

```bash
llm-recall stats              # GitHub-style 7-row contribution calendar + 4×2 stats panel
llm-recall stats --json       # pipe-friendly snapshot
```

<!-- screenshot: docs/screenshots/stats.gif | 录制脚本见 launch/storyboard.md §1 stats -->

`1/2/3` switches All time / Last 7 days / Last 30 days, `q` quits. Terminal-native rendering (`⋅ ▒ ▓ █` four levels), lipgloss truecolor — screenshot it directly.

### Gold (LLM mines your Top 10 quotes)

```bash
llm-recall gold                       # last 7 days, BYOK
llm-recall gold --days 30 -y          # 30 days, skip cost confirm
llm-recall gold --md > gold.md        # plain markdown, pipe-friendly
llm-recall gold --vendor openai --model gpt-4o-mini
```

<!-- screenshot: docs/screenshots/gold.gif | 录制脚本见 launch/storyboard.md §3 gold -->

A single LLM call surfaces your top 10 most quotable lines plus a one-sentence comment each. > 100KB total auto-samples down to 50 sessions. Results cached at `~/.cache/llm-recall/llm-cache/`, 7-day TTL, `--no-cache` forces a refresh. Prompt template: [`internal/llm/prompts/gold.go`](internal/llm/prompts/gold.go).

### Card (single-session card)

```bash
llm-recall card 26348a6c              # short id prefix-matches
llm-recall card 26348a6c -y           # skip cost confirm
llm-recall card 26348a6c --no-cache
```

<!-- screenshot: docs/screenshots/card.gif | 录制脚本见 launch/storyboard.md §4 card -->

A lipgloss rounded card: session header + first user message (truncated to 200 chars) + LLM one-sentence summary (≤50 chars) + cwd. Prompt template: [`internal/llm/prompts/card.go`](internal/llm/prompts/card.go).

### List (CLI dump)

```bash
llm-recall ls --all                   # all three sources
llm-recall ls --source claude -n 20
llm-recall ls --no-cache              # rebuild the index
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
base_url = ""             # "" = official endpoint; e.g. "https://dash.youchun.tech/v1" for the YC TECH relay
```

### LLM (BYOK)

The W7 commands `card` and `gold` call **your own** LLM API. Never put `api_key` / `key` into `config.toml` — `llm-recall` reads keys only from `credentials.toml` (W9), the system keyring, or environment variables. It warns if it sees a key in `config.toml`.

#### W9: `llm-recall login`

```bash
llm-recall login                                          # interactive
llm-recall login --vendor openai --base-url <url>         # non-interactive (key on stdin)
echo "$KEY" | llm-recall login --vendor openai --pipe-key
llm-recall login --use-keyring                            # store in OS keyring instead
```

The key **never** goes through a CLI flag (shell history risk). Hidden input via `golang.org/x/term`.

| Source                                            | Purpose                                                |
| ------------------------------------------------- | ------------------------------------------------------ |
| `~/.config/llm-recall/credentials.toml`           | W9 default; chmod 600; one section per vendor          |
| OS keyring (Keychain / Credential Manager / SS)   | W9 opt-in via `--use-keyring`                          |
| `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` env vars   | Fallback for CI / scripted use                         |
| `LLM_RECALL_BASE_URL`                             | Optional escape hatch; overrides `[llm] base_url`      |

**Credential resolution priority (high → low)**:

1. `credentials.toml` (vendor section matching the resolved vendor)
2. System keyring (when `credentials.toml` has `use_keyring = true` for the vendor)
3. `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` environment variable
4. Error → `Run: llm-recall login`

**Vendor / model / base URL priority (high → low)**:

1. CLI flag (`--vendor`, `--model`, `--llm-base-url`)
2. Environment variable (`LLM_RECALL_BASE_URL` for base URL only)
3. `credentials.toml` (W9)
4. `config.toml` `[llm]` section (legacy)
5. Hardcoded defaults (`anthropic` → `claude-haiku-4-5-20251001`, `openai` → `gpt-4o-mini`)

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

llm-recall is **completely silent** to the world outside your machine — the only exception is BYOK LLM calls you explicitly trigger (which go to whichever endpoint you configured).

**Marketing surfaces** (W9 onwards; first launch goes straight to TUI, no popup):

- TUI startup banner with one quote, 5% chance of a community-link CTA line (`https://recall.youchun.tech`)
- One-line attribution at the bottom of stats / card / gold (`Created within the YC TECH community`)
- (optional) discussion-link line at the bottom of search results
- gold / card use your own LLM API key; no relay gateway is involved

**No conversation content is ever uploaded**:

- Index: local SQLite under the system cache dir (macOS `~/Library/Caches/llm-recall/`, Linux `~/.cache/llm-recall/`, Windows `%LOCALAPPDATA%\llm-recall\Cache\`)
- stats: pure local aggregation, no network egress
- gold / card: hits the LLM endpoint you configured (default Anthropic / OpenAI official); 5 PII regex classes are redacted client-side before the call (API key / OAuth token / email / phone / IPv4); cost estimate prompts for confirm (`-y` skips); 7-day result cache
- No telemetry, no crash reports, no "anonymous usage statistics" — not a single line

**Disabling promo**:

```bash
llm-recall --no-promo                                        # one-off
echo 'no_promo = true' >> ~/.config/llm-recall/config.toml   # permanent (write into [promo])
```

`--no-promo` kills banner / search footer / stats attribution / gold & card footer in one shot.

## Contributing

Issues and PRs welcome.

- **Bug reports**: include `llm-recall version` output + reproduction steps + the first 5 lines of the relevant jsonl (redacted)
- **PR flow**: fork → branch → `go test ./... && go vet ./... && gofmt -l .` clean → PR; CI runs macOS / Linux / Windows × Go 1.22+
- **New source adapter**: implement `internal/adapter.SessionAdapter` + optional `FileLister` / `FileParser` incremental sub-interfaces (see [DEVDOC.md §2.1](DEVDOC.md)); attach a redacted jsonl header sample + resume command verification when opening the PR

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgements

- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) / [bubbles](https://github.com/charmbracelet/bubbles) / [lipgloss](https://github.com/charmbracelet/lipgloss) — TUI stack
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) — pure-Go SQLite, single binary, no cgo
- [BurntSushi/toml](https://github.com/BurntSushi/toml) — config.toml parser
- [sahilm/fuzzy](https://github.com/sahilm/fuzzy) — fuzzy matching
- [mattn/go-runewidth](https://github.com/mattn/go-runewidth) — CJK runewidth alignment
- Created within the [YC TECH](https://recall.youchun.tech) community
