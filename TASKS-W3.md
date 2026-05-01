# W3 执行任务 — TUI 实时搜索 + Resume launcher（全流程闭环）

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W2.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Goal（唯一目标）
让用户在终端打 `llm-recall` 不带参数 → 进入 TUI → 输入关键词实时筛选三家 source 会话 → 回车选中 → 自动 cd 到原 cwd 并 exec 对应 CLI 进入该会话。**全流程闭环**。

### 成功标准（criteria for success）
本文件末尾"验收检查清单"10 条全部通过，以**实际命令输出 / 录屏文本**为准（不是 subagent 自述）：

1. `go run ./cmd/llm-recall` 不带参数启动 TUI（搜索框聚焦、列表预填最近会话、右侧预览首条消息）
2. 输入"飞书"立即过滤出含此关键词的会话；删除关键词后列表恢复
3. 多关键词 AND："claude 历史" 命中既含 claude 又含 历史 的会话
4. 预览面板显示选中会话的首条用户消息全文 + 命中片段高亮（颜色 / 反白 / 任意视觉区分均可）
5. 回车选中后 TUI 退出，stdout 打印 `→ exec: <cmd> [args] in <cwd>` 一行后实际 exec 子进程（W3 默认 dry-run，详见 §步骤 6）
6. `--no-dry-run` 标志下回车真启动子进程：claude / codex / gemini 任一选中后能进入对应会话（master 选 1 个 claude session 实测一次即可）
7. ESC / Ctrl-C 退出 TUI 不启动任何子进程，进程退出码 0
8. **W2 遗留偏离已纳入实施**：
   - codex 顶部不再有 title 以 `<environment_context>` 或 `[Imported from Claude]` 开头的会话
   - gemini cwd 优先级链按 DEVDOC §3 P0-1"已纳入官方的实测补丁"实施（metadata.json > workspace.json > .project_root > 留空+title 前置 `<gemini:xxxxxxxx>`）
9. cache schema 已升级到 v2（含 body 字段），首次 `llm-recall` 启动检测旧版本自动 backfill；之后增量逻辑仍生效（W2 性能验收命令二跑 < 30% 不退化）
10. `go vet ./...` 无报错；`gofmt -l .` 无输出；`go test ./...` 全过

### 权限授权（最大权限，不询问）
执行期间，master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go run / go vet / go test / gofmt / git init / git add / git commit` 全部放行
- 启动 / 终止本机的 `claude` `codex` `gemini` CLI 子进程（仅用于验收 §6）
- 写入 SQLite cache 文件（系统 cache 目录）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` 与本文件 §0–§"不要做的"区段；推送到任何远端；`rm -rf` 跨出 `~/llm-recall/`；引入 cobra / viper / 任何 cgo 依赖（包括 mattn/go-sqlite3、SQLite FTS5 native extension）；写营销 banner / 水印 / onboarding 任何代码（W6 才做）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环（伪代码即行为规约）

```
spawn subagent("w3-executor", prompt = §0 全文 + "完成本文件 §步骤 0–8")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 10 条命令逐项校验
        if 10 条全过:
            报告用户："W3 验收通过"，附 10 条实际输出 / 录屏文本
            break
        else:
            spawn subagent("w3-executor", prompt += "上一轮在 <第 N 条> 失败，实际输出为 <…>，从该步继续")
    elif subagent 卡死/崩溃:
        spawn subagent("w3-executor", prompt += "前一个 subagent 在 <最后步骤> 中断，从此处继续")
    else:
        继续等待

# 唯一退出条件：10 条全过，或用户从外部手动停止
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`，无需展开过程
- 单次工具失败：自查 → 修 → 再试，最多 3 次；3 次仍失败才回传 master
- TUI 验收需要伪交互：用 `expect` / `tmux send-keys` / 录屏脚本，或在代码里加 `LLM_RECALL_TEST_INPUT` 环境变量驱动键盘事件，自行选最稳路径
- 完成后回传**实际命令输出 / 关键截屏文字**而非"我已完成"
- W1/W2 子 agent 留下的合理偏离保留，不要回滚

---

## 验收标准（先看这个）

```
$ llm-recall                                      # 进 TUI
┌─ search: 飞书_                                                          ┐
│ claude  2026-04-28  ~/douyin-monitor   1f3a...  飞书 wiki 内嵌 bitable  │
│ gemini  2026-04-26  ~/                 7d071...  你可以链接notion mcp吗 │
│ ...                                                                      │
│                                                                          │
│ Preview ──────────────────────────────────                              │
│ 用户首条消息全文，命中关键词【飞书】高亮                                  │
└──────────────────────────────────────────────────────────────────────────┘
↑↓ select   ⏎ resume   esc quit                                            

$ # 选中第 1 条 + 回车
→ exec: claude --resume 1f3a8d2e in C:\Users\肖浩\douyin-monitor

$ llm-recall --no-dry-run                         # 真启动子进程
... TUI 同上 ... 选中回车 → 实际进入 claude 会话 ...
```

## 前置条件（W2 已通过的，确认即可）

- W2 commit `1e6dd84` 之后的代码
- 三家 adapter 都能产出 Session（不含 body 字段）
- SQLite cache 增量工作
- `--source` / `--no-cache` flag 已可用

W3 在此之上扩展。

## 步骤

### 0. 前置补丁（W2 遗留偏离纳入官方）

参照 DEVDOC §3 P0-1"已纳入官方的实测补丁"小节实施：

#### 0.1 Codex 伪用户消息过滤（`internal/adapter/codex.go`）

提取 title 时，跳过 text 以下面任一前缀开头的"user message"：
- `<environment_context>`
- `[Imported from Claude]`

实现方式同 Claude adapter 的 system-reminder 跳过逻辑。验证：W2 报告里出现的 `019dc486 <environment_context> <cwd>...` 这条会话，W3 之后 title 应是它真正第二条用户消息（如不存在，session 仍出现但 title 为空字符串）。

#### 0.2 Gemini CWD fallback 链（`internal/adapter/gemini.go`）

替换现有的 `.project_root` 单点 fallback，改为优先级链：

```go
// 在 ~/.gemini/tmp/<projectHash>/ 下按顺序尝试
1. metadata.json   { "rootDir": "/abs/path" } 或 { "directories": ["/abs/path"] }
2. workspace.json  同上 schema 兼容
3. .project_root   纯文本，单行存绝对路径，trim 空白
4. 都不存在 → CWD = ""，title 前置 `<gemini:` + projectHash[:8] + `> ` 标记
```

读不出 / 解析失败时静默继续下一档（不要 stderr noise）。

#### 0.3 时间解析容忍（`internal/adapter/util.go`）

抽 `func ParseTime(s string) (time.Time, error)`，依次尝试：

```go
time.RFC3339Nano
time.RFC3339
"2006-01-02T15:04:05.0000000Z07:00"   // .NET 7 位 fractional
```

全失败返回零时 + error；调用方决定是 warn 还是 silent。三家 adapter 替换原有 `time.Parse` 调用统一走这里。

#### 0.4 实测三家 resume CLI 语法（**先做，不然 §5 表是猜的**）

§5 现有的命令表（`claude --resume <id>` / `codex resume <id>` / `gemini --resume <id>`）只有 claude 一家有把握。其余两家**必须先实测**再写代码，否则 §6 验收"真启动子进程"会挂在错语法上。

跑这些命令获取实际 flag：

```powershell
claude --help     | Select-String -Pattern "resume|session"
codex --help      | Select-String -Pattern "resume|session"
codex resume --help 2>&1
gemini --help     | Select-String -Pattern "resume|session|chat"
gemini chat --help 2>&1
```

记录每家**实际能用**的 resume 调用形式，把 §5 命令表替换为实测结果。可能落到这几种形态之一：

| 形态 | 含义 | launcher 处理 |
|---|---|---|
| `<cli> --resume <id>` 或 `<cli> resume <id>` | 顶层 flag 直接还原 | 正常实现 |
| 仅 `<cli>` 启动后交互 `/chat resume <id>` | 只有交互式 | **退化路径**：launcher 仅 chdir + 启动 cli 不带 flag，stdout 多打一行 `→ 进入后请运行：/chat resume <id>`，由用户手动续接 |
| 完全无 resume | 该家不支持 | `ResumeCommand` 返回 `(nil, "", ErrNoResume)`；TUI 选中时弹 toast `<source> 不支持 CLI resume，已复制 sessionId 到剪贴板` |

实测如果 gemini 落到"仅交互式"或"无 resume"——**预期就是这个结果**——把 §6 验收第 6 条调整为："claude 必须真启动子进程；codex / gemini 按实测能力，最低标准是 chdir 正确 + sessionId 可见，进入后续接动作可由用户完成"。这条调整记录到 commit message 里，不要静默改验收标准。

### 1. cache schema v2 升级（`internal/index/cache.go`）

加 schema_version 表 + body 字段：

```sql
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY);

-- 新增字段
ALTER TABLE sessions ADD COLUMN body TEXT NOT NULL DEFAULT '';
-- body：所有用户消息文本拼接，UTF-8，截断到 64KB
```

OpenCache 启动时：
- 读 schema_version；空 → 视为 v0，写入 v1（W2 状态）
- 若 < 2 → ALTER TABLE 加 body → INSERT INTO schema_version(2)
- ALTER 后 body 全空，触发一次"forced rescan"：cache.Get 不命中，强制走 ParseFile 路径，把 body 写入

不需要 DROP TABLE 重建（避免用户首次 W3 启动时所有 184 条全消失再扫的体验断层）。

### 2. adapter body 提取（三家）

为每个 adapter 实现"扫整个文件、拼接所有用户消息文本到 body"的功能。**两阶段提取**：

```go
type FileParser interface {
    ParseFile(fm FileMeta) (Session, error)        // 已有，含 title
    ParseFileFull(fm FileMeta) (Session, string, error)  // 新增，返回 (session, body, err)
}
```

`body` 拼接规则（三家共用）：
- 仅取 user 角色消息的 text content
- 跳过 codex 伪上下文 / claude system-reminder / 工具调用记录（与 title 提取规则一致）
- 多条消息以 `\n---\n` 分隔
- 总长度截断到 65536 bytes（约 22K 中文字符）—— 用 utf8 安全截断，不切半字符

cache.Upsert 接收 `(Session, body, mtime, size)` 并写入 body 字段。

### 3. discover.go 升级

`Options` 加：

```go
type Options struct {
    UseCache bool
    Source   string
    NeedBody bool   // 新增：true 时强制走 ParseFileFull
}
```

ls 命令传 `NeedBody: false`（不需要 body，省时间）；TUI 传 `NeedBody: true` 第一次冷启 → cache hit 后续都不重读。

cache miss 走 ParseFileFull；hit 直接返回（body 已在 cache 里）。

### 4. TUI 模块（`internal/tui/`）

**依赖**：

```
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
go get github.com/charmbracelet/lipgloss
go get github.com/sahilm/fuzzy
go get github.com/mattn/go-runewidth
```

**布局（lipgloss 三栏）：**

```
┌─ search: <query>_                                    ┐
│ <list, 50% width>          │ <preview, 50% width>    │
│ ...                        │ ...                     │
└─ ↑↓ select  ⏎ resume  esc quit  source: all  N hits ─┘
```

**模块文件：**
- `internal/tui/model.go`     bubbletea Model 主结构
- `internal/tui/search.go`    query → SQL LIKE → fuzzy rank
- `internal/tui/keys.go`      keymap
- `internal/tui/view.go`      lipgloss 渲染
- `internal/tui/banner.go`    banner 占位（W6 实装），W3 返回空字符串

**搜索逻辑（`search.go`）**：

```go
func Search(db *sql.DB, query string, source string, limit int) ([]adapter.Session, error)
```

1. query 按空白分词 → 每词 LIKE `%word%` 拼 AND
2. SQL：`SELECT ... FROM sessions WHERE title LIKE ? AND title LIKE ? AND body LIKE ? ... ORDER BY updated_at DESC LIMIT ?`
   或：分别在 (title OR body) 内匹配每个词
3. `source != ""` 时加 `AND source = ?`
4. 取出最多 200 候选 → sahilm/fuzzy 对 (title + body[:500]) 重排 → 取 limit
5. 空 query 时跳过 LIKE，直接 ORDER BY updated_at DESC LIMIT

**Debounce**：tea.Cmd 模式，每次输入框变化触发 `searchMsg{q}`；用 timestamp guard，落后的 searchMsg 丢弃。50ms 节流。

**预览面板**：
- 选中会话取 body 字段（已在 cache）
- 命中关键词用 lipgloss 反白渲染
- viewport 可滚动（j/k 或 ↑↓）

**中文宽度**：所有截断 / 列宽计算用 `runewidth.StringWidth`，不要用 `len()`。

### 5. resume launcher（`internal/launcher/launcher.go`）

#### 接口

```go
type Launcher struct { DryRun bool }

func (l *Launcher) Run(s adapter.Session) error
```

#### 命令构造

按 source 调对应 CLI（写在 adapter 里更干净 → 复用 DEVDOC §2.1 的 `ResumeCommand(s) ([]string, string)`）。

⚠️ **下表是占位假设，必须用 §0.4 的实测结果覆盖。** 若实测发现某家无 CLI resume（最可能是 gemini），改走 §0.4 表的"退化路径"，**不要硬编占位命令进生产代码**。

| source | cmd | args | cwd |
|---|---|---|---|
| claude | `claude` | `--resume <id>` | session.CWD |
| codex  | `codex`  | `resume <id>`  | session.CWD（codex 不强制 cwd 一致，但还原最贴近原环境）|
| gemini | `gemini` | `--resume <id>` | session.CWD（gemini 要求 cwd 与原 project 匹配；CWD 为空时退化为当前 cwd + warn）|

`ResumeCommand` 返回签名扩展：

```go
ResumeCommand(s Session) (argv []string, cwd string, mode ResumeMode, err error)

type ResumeMode int
const (
    ResumeDirect      ResumeMode = iota  // 直接 exec argv，会话自动续接
    ResumeInteractive                    // exec argv 进入交互，hint 用户敲 /chat resume <id>
    ResumeUnsupported                    // 该家无 resume，仅复制 id 到剪贴板
)
```

launcher 按 mode 分支：Direct → 正常 exec；Interactive → exec + stdout 多打一行 `→ 进入后请运行：/chat resume <id>`；Unsupported → 不 exec，stdout 打 `<source> 不支持 CLI resume，sessionId: <id>` 后退出。

#### 执行

DryRun=true：打印 `→ exec: <cmd> <args...> in <cwd>` 到 stdout，return nil

DryRun=false：
1. `os.Chdir(cwd)`（cwd 不存在 → 不 chdir，warn 后用当前 cwd）
2. Unix: `syscall.Exec(absPath, args, os.Environ())` 替换进程
3. Windows: `cmd := exec.Command(...); cmd.Stdin/out/err = os.Stdin/out/err; cmd.Run(); os.Exit(cmd.ProcessState.ExitCode())`
4. CLI 不在 PATH → stderr 打印 `<cmd> not found in PATH; install it to resume <source> sessions` 退出码 127

### 6. 主入口改造（`cmd/llm-recall/main.go`）

```
llm-recall                       → TUI（默认）
llm-recall ls [...]              → 同 W1/W2
llm-recall version               → 同 W1
llm-recall --no-dry-run          → TUI 默认 dry-run，加此 flag 真 exec
```

子命令分发逻辑：`os.Args[1]` 不在已知子命令集合中（`ls`/`version`）则视为 TUI（连同剩余 args 一起当 TUI flag 传入）。

### 7. 测试

- `internal/launcher/launcher_test.go`：DryRun 模式下断言三家的 cmd/args/cwd 构造正确
- `internal/tui/search_test.go`：喂 mock cache，断言多词 AND / source 过滤 / 中文匹配 / fuzzy 排序
- adapter 三家加 body 提取的单测（在 W2 已有 testdata 上扩展）

不需要 TUI 视图层 e2e 测试（成本高收益低，验收 §1-§7 实际跑代替）。

### 8. 提交

```
git add .
git commit -m "W3: TUI search + resume launcher (full loop)"
```

## 验收检查清单

执行方做完后回传**实际命令输出 / 关键截屏文字**：

- [ ] `go run ./cmd/llm-recall` 不带参数进 TUI（截屏文字：搜索框 / 列表 / 预览三栏可见）
- [ ] 输入"飞书"实时过滤（前后两屏对比）
- [ ] "claude 历史" 多词 AND 命中（截屏）
- [ ] 预览面板高亮命中片段（截屏含命中词反白或颜色块）
- [ ] DryRun 默认开：选中回车打印 `→ exec: claude --resume ... in ...` 后退出
- [ ] `--no-dry-run` 真启动子进程（master 录一次：选 claude session → 进入 → 立即 `/exit` 回到 shell，exit code 0）
- [ ] ESC / q 退出，exit code 0，无任何子进程启动
- [ ] codex 不再有 `<environment_context>` title 的会话（grep 验证）
- [ ] gemini cwd fallback 链生效（找一个用户机器上有 metadata.json / workspace.json / .project_root 的 project，验证三档 cwd 都能正确取到）
- [ ] cache schema v2 已写入：`SELECT version FROM schema_version` 返回 2
- [ ] 二跑 wall-clock < 首跑 30%（cache 仍生效，body 字段加入未拖垮性能）
- [ ] `go vet ./...` 无报错；`gofmt -l .` 无输出；`go test ./...` 全过
- [ ] DB 文件落点不变（系统 cache 目录），不在 `~/llm-recall/` 内
- [ ] DEVDOC.md / 历史 TASKS-*.md §0–§"不要做的"未被改

## 不要做的（留给 W4+）

- 不要做 banner / footer / 水印 / onboarding（W6）
- 不要做 stats / card / gold（W5/W7）
- 不要做 goreleaser / brew tap / scoop（W4）
- 不要做 aider / opencode / Cline 等其他 source（V2）
- 不要做 FTS5 / 中文 trigram tokenizer（W3 SQL LIKE 双匹配已够，等 1000+ 会话再说）
- 不要引入 cobra / viper / 任何 cgo 依赖
- 不要扫 codex archived_sessions / gemini checkpoint-*.json（V2 评估）
