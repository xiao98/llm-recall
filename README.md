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

## Privacy & Promo

W4 阶段：仅本地读 jsonl，不上传任何对话内容到任何后端。

W6 起会加：启动 banner / 生图水印 / `gold` 命令（BYOK 调用你自己的 LLM key）。届时首次启动会有一次性同意流，可用 `--no-promo` 关 banner / footer。详见 DEVDOC §4。

（占位 / W6 起加完整版）

## License

MIT

## Screenshot

(W7 出截屏 GIF；W4 占位)
