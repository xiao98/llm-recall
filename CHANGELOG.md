# Changelog

All notable changes documented here, [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) style.

## [0.3.0] - 2026-05-XX

### Added
- W9: `llm-recall login` 交互式 LLM provider 配置（vendor / api key / base url / model / storage）。Key 走 `golang.org/x/term.ReadPassword` 隐藏输入或 stdin pipe，**永远不进 CLI flag**（shell history 风险）。默认落 `~/.config/llm-recall/credentials.toml` chmod 600；`--use-keyring` 改入系统钥匙串（macOS Keychain / Win Credential Manager / Linux Secret Service via `zalando/go-keyring`）。
- W9: stats 顶部加一行 `📚 Top topics: ...`，5 个 EN/ZH 关键词（去停用词，brand name 自动过滤）；`stats --json` 输出含 `top_topics` 字段。
- W9: stats 右上角加 16×16 像素小幽灵（lipgloss + unicode block + 24-bit RGB），按活跃数据切 7 个表情：`Idle / Happy / Pumped / Sad / Sleeping / Confused / Cheering`。终端 ≥ 100 列时与 heatmap 横向并排；< 100 列自动隐藏；`--no-pet` 强制关闭。

### Changed
- **Rebrand**: `YCAPI` → `YC TECH`，`ycapi` → `yctech`（README / landing `docs/index.html` / 启动 banner / search footer / stats footer 全套）。**真实服务地址 `api.youchun.tech` / `dash.youchun.tech` 不变**（rebrand 仅作用于品牌名字面量）。
- **Onboarding 流程**：W6 的首次启动 bubbletea consent box（`internal/promo/onboarding.go`）整体删除。首次启动直接进 TUI；LLM 配置改由 `llm-recall login` 显式触发。README / docs/index.html 加 attribution 行 `Created within the YC TECH community`（中性表述，不含 "Sponsored" / "Powered by"）。
- stats / card / gold footer 文案：`── llm-recall · sponsored by YCAPI ──` → `── llm-recall · Created within the YC TECH community ──`。`--no-promo` kill switch 行为不变。
- 凭据优先级链：`credentials.toml` > 系统 keyring（当 `use_keyring = true` 时）> `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` env var > error → `Run: llm-recall login`。`gold` / `card` 在无凭据时退出 2 + stderr 友好提示。

### Fixed
- W9: `llm.Resolve` 在 vendor 显式但 key 缺失时报 `ErrNoCredentials`（W7 之前是字符串 match，rebrand 后更稳）。
- W9: `credentials.Save` 写新 vendor 段不会清掉另一 vendor 的现有段（atomic temp-file rename + 0600）。

### Known limitations
- 像素宠物当前为静态渲染，不做帧动画（lipgloss 单帧足够，动画会持续占用刷新成本）。
- Top 5 话题用简单 bigram 切 CJK，无 jieba/MeCab 级分词；少数情况会切出无意义 bigram。停用词表可在 `internal/stats/stopwords.go` 加。
- `--use-keyring` 在无图形会话的 Linux server（无 Secret Service）会自动降级到 credentials.toml + stderr warn。

### Compatibility
- v0.2.0 → v0.3.0：W7 用户的 env-var 配置 100% 向后兼容（`ANTHROPIC_API_KEY` / `OPENAI_API_KEY` 仍工作）。已存在的 `~/.config/llm-recall/onboarding-accepted` sentinel 不再读取也不会被删，留给用户自行清理。

---

## [0.2.0] - 2026-05-XX

### Added
- W5: `llm-recall stats` 终端原生 ASCII heatmap (GitHub-contribution-graph 风格 + 4×2 stats panel)。三窗口切换：All time / Last 7 days / Last 30 days；`q` 退出；`--json` 给 pipe；纯本地聚合，无网络出口。
- W5-rev2: stats 4×2 panel 加 per-source 行（claude / codex / gemini 各自 sessions / tokens 占比）。
- W6: Onboarding 一次同意流（首次启动 bubbletea consent screen，落 `~/.config/llm-recall/onboarding-accepted` sentinel）。
- W6: 启动 banner（`internal/promo/quotes.go` 30+ 条 YCAPI 群语录，5% 概率含加群 CTA 一行）。
- W6: 搜索 footer（默认关；`config.toml` `[promo] search_footer = true` 开启后 TUI 列表底部一行讨论关联条）。
- W6: stats / card / gold sponsored footer line（`── llm-recall · sponsored by YCAPI ──`）。
- W6: `--no-promo` 全局 kill switch（关 banner / search footer / stats sponsored / card & gold footer 全套）+ `config.toml` `[promo]` 段细粒度配置（`no_promo` / `search_footer` / `banner_freq` / `cta_probability`）。
- W7: `llm-recall card <session-id>` 命令 — lipgloss 圆角名片卡（会话头 + 首条用户消息截 200 字 + LLM 一句话总结 ≤ 50 字 + cwd），BYOK，prompt 模板 `internal/llm/prompts/card.go`。
- W7: `llm-recall gold` 命令 — LLM 抽 Top 10 用户金句 + 一句话点评，默认扫 7 天（`--days N` 覆盖），单次 LLM 调用，total > 100KB 自动 sample 50 会话，prompt 模板 `internal/llm/prompts/gold.go`；`--md` 输出纯 markdown 给 pipe。
- W7: BYOK 调用链 — env `ANTHROPIC_API_KEY` 优先 → `OPENAI_API_KEY` 兜底；config.toml `[llm]` 段（`vendor` / `model` / `base_url`）；CLI flag `--vendor` / `--model` / `--llm-base-url` 覆盖；`LLM_RECALL_BASE_URL` env 兜底；4 级优先级链。
- W7: PII 脱敏 — 调 LLM 前 5 类正则（API key / OAuth token / email / 手机号 / IPv4）客户端脱敏。
- W7: token / cost 估算 confirm — gold/card 调用前显示估算 cost，`-y` flag 跳过。
- W7: LLM 结果本地 cache — `~/.cache/llm-recall/llm-cache/`，7 天 TTL，`--no-cache` 强刷。

### Changed
- **Breaking** (stats only): stats 输出从 PNG 双尺寸 (1080×1080 / 1080×1920) 落盘到 `~/Pictures/` 改为**终端原生**渲染。Python + Pillow 后端、Scaleway 部署、imggen Go 模块、`~/Pictures/` 落盘逻辑、`--no-watermark` flag 全部删除。聚合逻辑（token / session / streak）+ TOKEN-AUDIT.md 保留迁至 `internal/stats/`。
  - **迁移**：之前依赖 `~/Pictures/llm-recall-stats-*.png` 的脚本失效，改为终端截屏分享或 `llm-recall stats --json` pipe。
- README v2 — 整体重写：4 大段 (What / Install / Usage / Privacy & Promo) + Configuration + Supported sources + Contributing + Acknowledgements；4 段命令各配 GIF 占位（`docs/screenshots/{stats,tui,gold,card}.gif`，由 release 流程录制）。
- 项目首页上线 — `https://recall.youchun.tech`（GitHub Pages + 自定义域名 + 单文件 `docs/index.html`，零依赖）。

### Fixed
- W7: gold 输出 JSON 解析失败 retry once with stricter system prompt（避免单次调用浪费）。
- W7: card body 引用 200 字截断 utf-8 安全（不切碎多字节字符）。

### Known limitations
- Gemini resume 仅支持交互式 `/chat resume <id>`，CLI flag 不接受 UUID（gemini-cli 上游 #20480 / #23489，未变化）。
- aider / opencode / cline adapter 留 V2，欢迎 PR。

### Compatibility
- v0.1.0 → v0.2.0 仅 stats 输出形态有 breaking change（PNG → 终端原生）。其他命令（ls / TUI 搜索 / resume launcher / 三家 adapter / SQLite cache schema）100% 向后兼容；既有 cache DB 无需重建。

---

## [0.1.0] - 2026-04-XX

### Added
- W1: 项目骨架 + Claude adapter (`~/.claude/projects/*/*.jsonl`)；`llm-recall ls` 列会话。
- W2: Codex adapter (`~/.codex/sessions/`) + Gemini adapter（双格式 .json/.jsonl + cwd fallback 链）+ SQLite 增量缓存（modernc.org/sqlite，纯 Go，DB 落系统 cache 目录）。
- W2: ls 命令 `--no-cache` / `--source <name>` flag。
- W3: TUI 实时搜索（bubbletea + lipgloss + sahilm/fuzzy；多词 AND；中文 unicode 处理；命中片段预览高亮）。
- W3: Resume launcher（claude / codex 直接 `--resume <id>`；gemini 退化为交互式提示用户 `/chat resume <id>`；跨平台 Unix syscall.Exec / Windows cmd.Run）。
- W3: cache schema v2，sessions 表加 `body` 字段（拼接所有用户消息，截断到 64KB）。
- W4: goreleaser 分发流水线（mac/linux/win × amd64/arm64）；Homebrew tap + Scoop bucket 自动发布；GitHub Actions release workflow。

### Fixed
- W2: Title 含 `\n` `\r` `\t` 破坏 tabwriter 列对齐 → CleanTitle 清洗。
- W3: Codex 顶部"会话"实为 `<environment_context>` / `[Imported from Claude]` CLI 注入伪用户消息 → 过滤。
- W3: Gemini cwd fallback 链 metadata.json > workspace.json > .project_root > 留空 + title 标记。
- W3: 时间解析容忍 .NET 7 位 fractional（`2026-04-25T10:55:00.0000000Z`）。

### Known limitations
- Gemini resume 仅支持交互式 `/chat resume <id>`，CLI flag 不接受 UUID（gemini-cli 上游 #20480 / #23489）。
- TUI 截图 / GIF / landing 留 W7+（v0.2.0 落地）。
