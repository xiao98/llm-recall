# llm-recall — 开发文档 v0.1

## 0. 一句话定位

> 一个跨厂商 LLM CLI 会话的搜索 + 恢复终端工具，自带营销基因和"装杯"传播钩子。

**目标用户**：同时用 2 家以上 LLM CLI 的开发者（Claude Code + Codex / Gemini / Aider 任意组合）。

**差异化（vs `Dicklesworthstone/coding_agent_session_search`，725★）**：
- 它是技术工具，没人主动传播 → 我们做"自带传播基因"的版本
- 锚点：装杯生图 / 金句挖掘 / 透明营销 → 用户截图就在帮我们传

## 1. 技术栈

| 层 | 选型 | 理由 |
|---|---|---|
| 主程序 | Go 1.22 + Charm bubbletea | 单二进制，跨平台，TUI 生态成熟 |
| 模糊搜索 | sahilm/fuzzy | 简单可控；后续可换 fzf-rs |
| 缓存索引 | SQLite (modernc.org/sqlite，纯 Go) | 无 cgo，分发零依赖 |
| 后端生图 | Scaleway 上 Python FastAPI + Pillow | 模板可热更新 |
| LLM 调用（gold） | **用户自己的 API key**（BYOK），自动检测 ANTHROPIC_API_KEY / OPENAI_API_KEY | 隐私安全 |
| 分发 | goreleaser → brew tap / scoop bucket / GitHub Releases | 一键发版 |

**显式不选**：Electron / Tauri / Python TUI。分发即地狱。

## 2. 架构

```
internal/
  adapter/         每家一个 parser，统一接口 SessionAdapter
    claude.go      ~/.claude/projects/*/*.jsonl
    codex.go       ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
    gemini.go      ~/.gemini/tmp/<shortid>/chats/session-*.jsonl
  index/           扫描 + SQLite 缓存（mtime-based 增量）
  search/          多关键词 AND + fuzzy（含中文）
  tui/             bubbletea 屏幕：搜索框 / 列表 / 预览
  launcher/        resume：os.Chdir + os.StartProcess <vendor-cli> --resume <id>
  imggen/          HTTP 调 Scaleway 后端，下载 PNG
  promo/           banner / footer / 水印 / onboarding 同意流
  llm/             gold 调用（按 vendor 抽象，BYOK）
  config/          ~/.config/llm-recall/config.toml
```

### 2.1 SessionAdapter 接口

```go
type SessionAdapter interface {
    Name() string                                // "claude" / "codex" / "gemini"
    Discover(ctx context.Context) ([]Session, error)  // 扫描根目录
    Read(s Session) ([]Message, error)            // 读消息流（lazy）
    ResumeCommand(s Session) ([]string, string)   // ([cmd, args...], cwd)
}

type Session struct {
    Source    string    // adapter name
    ID        string
    CWD       string
    StartedAt time.Time
    UpdatedAt time.Time
    FilePath  string    // for invalidation
}

type Message struct {
    Role string  // "user" | "assistant" | "tool"
    Text string  // 已过滤工具调用的纯文本
    Time time.Time
}
```

### 2.2 数据流

```
adapters → discover sessions → SQLite (id, source, cwd, mtime, title, body_fts)
                                  ↓
                              fts5 + fuzzy → TUI 列表
                                  ↓
                              用户回车 → launcher.Run() → exec
```

## 3. P0 功能清单

### P0-1 多源 parser
- 三家 schema 已摸清（见 `~/llm-recall/RESEARCH.md`，下个 commit 写）
- 共同点：jsonl + 头一行 metadata（含 sessionId / cwd / startTime）
- 用户文本提取规则各家不同（见各 adapter 注释）
- **坑**：Codex/Gemini 文件按 mtime 增量扫；Claude 是扁平目录可全扫

### P0-2 TUI 实时搜索
- 输入即筛（go routine + debounce 50ms）
- 列表显示：source 标识 + 时间 + cwd 短名 + 首条用户消息（80 字截断）
- 右侧预览：完整首条 + 命中片段高亮
- 多关键词 AND + 模糊匹配；中文按字 unicode 处理（不分词）

### P0-3 Resume launcher
- 选中回车 → adapter 给出 `(cmd, args, cwd)` → `os.Chdir(cwd)` → `syscall.Exec`（Unix）/ `os.StartProcess` (Windows)
- TUI 退出码 = 子进程退出码
- 跨家共用一个入口：用户不用记 `claude --resume` / `codex resume` / `gemini --resume`

### P0-4 单二进制分发
- goreleaser config：mac/linux/windows × amd64/arm64
- brew tap：`<user>/homebrew-tap`
- scoop bucket：`<user>/scoop-bucket`
- 安装命令在 README 头一段：`brew install <user>/tap/llm-recall`

### P0-5 装杯模式（核心引爆点）`llm-recall stats`
- 扫 30 天会话，本地聚合：对话数 / 总 token / Top 5 话题（用户消息词频，去停用词）/ 最长会话 / 每家 CLI 占比
- POST 数据到 Scaleway 生图后端：`POST /v1/stats-card { stats, watermark, format: "square"|"vertical" }`
- 后端用 Pillow 套模板返 PNG，本地落盘 `~/Pictures/llm-recall/stats-YYYYMMDD-{1,2}.png`
- 命令完成后打印图片路径 + "Open in Finder/Explorer? [y/n]"
- **水印**：右下角 `llm-recall · sponsored by YCAPI`（默认开，`--no-watermark` 关）
- 输出两版：1080×1080 / 1080×1920

### P0-6 启动 banner
- 每次 TUI 启动顶栏一行金句（YCAPI 群语录，30 条起步，存 `internal/promo/quotes.go`）
- 5% 概率金句下追加 `→ 加入 YCAPI 群: <短链>` CTA
- `--no-promo` / `config.no_promo = true` 关
- 实现：`promo.RandomQuote()` 在 TUI Init() 调一次

### P0-7 搜索 footer
- **默认关**（搜索区是专注区，少打扰）
- 开启后：每次出搜索结果，列表底部一行 `🔍 YCAPI 群里有人在讨论「<关键词1>」 →`
- 关键词从用户当前 query 取
- 实现：在 TUI list footer slot 渲染

### P0-8 一键出图（取代 share）
- 在搜索结果上选中后按 `s` → `llm-recall card <session-id>`
- 内容：会话脱敏摘要（首条 user msg + LLM 1 句话总结，BYOK 调）
- 同样调 Scaleway 后端 → 出图 → 落盘
- 提示用户：「Saved to <path>. 截图发朋友圈/即刻吧」（不做 share 后端）

### P0-9 金句挖掘 `llm-recall gold`
- 扫 7 天会话（默认窗口可配置）
- BYOK：自动探测 `ANTHROPIC_API_KEY` 或 `OPENAI_API_KEY`，没有就报错引导配置
- 调 LLM 抽 Top 10 用户金句（prompt 模板见 `internal/llm/gold_prompt.go`）
- 输出长图（Scaleway 后端模板 `gold-list`）
- 默认水印开

## 4. 隐私 + 透明度（防被骂指南）

### Onboarding（首次启动一次）
```
llm-recall — 跨厂商 LLM CLI 会话搜索

This tool is sponsored by YCAPI (https://ycapi.com).
- 启动时显示一条金句 banner，5% 概率含加群链接
- stats / gold 生成的图片右下角带水印
- gold 功能调用你自己的 LLM API key（不上传任何对话内容到 YCAPI）

可以用以下开关关闭：
  --no-promo        关 banner / footer
  --no-watermark    关图片水印

按 Enter 继续，按 q 退出。
```
- 同意写入 `~/.config/llm-recall/onboarding-accepted`
- README 第一段照搬本声明

### 不做的隐私雷
- 不上传任何对话内容到任何后端（gold 走 BYOK，stats 只上传聚合数字）
- Scaleway 后端只接收：聚合统计数字 + 渲染参数。**不接收原文**
- 用户脱敏：stats 不展示具体对话片段；gold 输出在用户本地（图先落盘，要不要发是用户的事）

## 5. 命名 + 仓库

- repo: `github.com/xiao98/llm-recall`
- bin: `llm-recall`（不加 alias）
- brew tap: `xiao98/homebrew-tap`
- scoop bucket: `xiao98/scoop-bucket`

## 6. 8 周时间线（粗）

| 周 | 目标 | 验收 |
|---|---|---|
| W1 | 项目骨架 + Claude adapter | `llm-recall ls` 能扫出本机 Claude 会话列表（CLI dump） |
| W2 | Codex/Gemini adapter + SQLite cache | 三家全扫 + 增量 |
| W3 | TUI 搜索 + resume launcher | 闭环：搜 → 选 → 进入会话 |
| W4 | goreleaser + brew tap + scoop + dogfood 一周 | 自己装自己用 |
| W5 | Scaleway 生图后端 + stats 命令 | 出图能发朋友圈 |
| W6 | banner / footer / onboarding / `--no-promo` | 透明度声明就位 |
| W7 | gold 命令（BYOK） + 一键出图 | 全部 P0 闭环 |
| W8 | README + landing + 公众号文 + Reddit/HN 发车 | 公开发布 |

## 7. 风险

- **Dicklesworthstone 抢跑**：他们 Rust + 11 家适配器在更新。我们靠营销/生图/中文社区差异化，技术深度不打算追平。
- **格式变更**：Codex / Gemini 都还在快速迭代 schema，要做 graceful degradation（每个字段 Option-typed）。
- **YCAPI 营销过度被喷**：靠 onboarding 透明 + `--no-promo`。出问题就把默认 banner 频率降到每天一次而不是每次启动。
- **生图后端单点**：Scaleway 挂了 stats/card/gold 全废。短期可接受（有重试 + 友好报错）；后期可加 fallback 到 Go 本地渲染。

## 8. 交付给执行方

文档由策划方写完。执行方（另开 Claude Code 会话）从这里接力：

1. 当前阶段任务详见 `~/llm-recall/TASKS-W1.md`（每周一份，做完一份再写下一份）
2. 全局架构 / 接口约束 / 隐私边界以本 DEVDOC 为准
3. 执行方完成 W1 验收后，回传给策划方，再发起 W2 任务文档
4. 用户负责并行：在 GitHub 建空仓 `xiao98/llm-recall`、维护 `internal/promo/quotes.go` 的 30 条 YCAPI 金句草稿

**执行模式**：自 W1 起，每周任务文档以 goal-driven 双 agent 模式运行（master + subagent，5 分钟轮询，最大权限不询问）。规约见 `TASKS-W*.md` 顶部"§0 执行模式"段，DEVDOC 不再重复。
