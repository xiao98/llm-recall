# Changelog

All notable changes documented here, [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) style.

## [0.1.0] - 2026-05-XX

### Added
- W1: 项目骨架 + Claude adapter (`~/.claude/projects/*/*.jsonl`)；`llm-recall ls` 列会话
- W2: Codex adapter (`~/.codex/sessions/`) + Gemini adapter（双格式 .json/.jsonl + cwd fallback 链）+ SQLite 增量缓存（modernc.org/sqlite，纯 Go，DB 落系统 cache 目录）
- W2: ls 命令 `--no-cache` / `--source <name>` flag
- W3: TUI 实时搜索（bubbletea + lipgloss + sahilm/fuzzy；多词 AND；中文 unicode 处理；命中片段预览高亮）
- W3: Resume launcher（claude / codex 直接 `--resume <id>`；gemini 退化为交互式提示用户 `/chat resume <id>`；跨平台 Unix syscall.Exec / Windows cmd.Run）
- W3: cache schema v2，sessions 表加 `body` 字段（拼接所有用户消息，截断到 64KB）
- W4: goreleaser 分发流水线（mac/linux/win × amd64/arm64）；Homebrew tap + Scoop bucket 自动发布；GitHub Actions release workflow

### Fixed
- W2: Title 含 `\n` `\r` `\t` 破坏 tabwriter 列对齐 → CleanTitle 清洗
- W3: Codex 顶部"会话"实为 `<environment_context>` / `[Imported from Claude]` CLI 注入伪用户消息 → 过滤
- W3: Gemini cwd fallback 链 metadata.json > workspace.json > .project_root > 留空+title 标记
- W3: 时间解析容忍 .NET 7 位 fractional（`2026-04-25T10:55:00.0000000Z`）

### Known limitations
- Gemini resume 仅支持交互式 `/chat resume <id>`，CLI flag 不接受 UUID（gemini-cli 上游 #20480 / #23489）
- TUI 截图 / GIF / landing 留 W7+
