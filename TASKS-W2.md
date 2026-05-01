# W2 执行任务 — Codex/Gemini adapter + SQLite 增量缓存

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W1.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Goal（唯一目标）
让 `go run ./cmd/llm-recall ls` 在本机产出**三家**（Claude / Codex / Gemini）的会话列表，且二次执行命中 SQLite cache 显著加速。

### 成功标准（criteria for success）
本文件末尾"验收检查清单"7 条全部通过，以**实际命令输出**为准（不是 subagent 自述）：

1. `go run ./cmd/llm-recall ls --all` 输出包含 `claude` / `codex` / `gemini` 三个 source 的会话（**前提**：本机有这些 CLI 装过；如果用户机器某家从未跑过，对应 source 自然为 0 行，不算失败 —— 但 adapter 必须实例化、Discover 必须调用、必须 graceful 处理"目录不存在"返回空切片）
2. 任意已有 source 的会话条数 ≥ 1，且 sessionId / cwd / mtime / title 四字段非空
3. 二次执行 `time go run ./cmd/llm-recall ls` wall-clock 时间 < 首次的 30%（cache 生效证据）
4. `--no-cache` 标志强扫，无视 cache
5. `--source claude|codex|gemini` 标志只列单源
6. W1 遗留 title 含 `\n` 破坏对齐的瑕疵已修（title 中 `\n` `\r` `\t` 全部替换为空格，连续空白合并）
7. `go vet ./...` 无报错；`gofmt -l .` 无输出；`go test ./...` 全过

### 权限授权（最大权限，不询问）
执行期间，master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go run / go vet / go test / gofmt / git init / git add / git commit` 全部放行
- 在 `~/.cache/llm-recall/`（macOS/Linux）或 `%LocalAppData%\llm-recall\cache\`（Windows）下创建 SQLite 文件
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` 与本文件 §0–§"不要做的"区段；推送到任何远端；`rm -rf` 跨出 `~/llm-recall/`；引入 cobra/viper/任何 cgo 依赖；把 SQLite 装进 `~/llm-recall/` 仓内（必须放系统 cache 目录）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环（伪代码即行为规约）

```
spawn subagent("w2-executor", prompt = §0 全文 + "完成本文件 §步骤 1–8")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 7 条命令逐项校验
        if 7 条全过:
            报告用户："W2 验收通过"，附 7 条命令实际输出
            break
        else:
            spawn subagent("w2-executor", prompt += "上一轮在 <第 N 条> 失败，实际输出为 <…>，从该步继续")
    elif subagent 卡死/崩溃:
        spawn subagent("w2-executor", prompt += "前一个 subagent 在 <最后步骤> 中断，从此处继续")
    else:
        继续等待

# 唯一退出条件：7 条全过，或用户从外部手动停止
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`，无需展开过程
- 单次工具失败：自查 → 修 → 再试，最多 3 次；3 次仍失败才回传 master
- 完成后回传**实际命令输出**而非"我已完成"
- W1 子 agent 留下的合理偏离（Session 加 Title 字段、不递归扫 subagents、Read() 推迟）保留，不要回滚

---

## 验收标准（先看这个）

```
$ time go run ./cmd/llm-recall ls --all                              # 首次：可能 1-3s
SOURCE  UPDATED           CWD                    SESSION   TITLE
claude  2026-05-01 12:28  ~/sandbox              432cf667  W1 验收通过 …
codex   2026-04-29 10:11  ~/quant-learning       a91d7c2e  bumpchart for ETH backtest …
gemini  2026-04-22 09:30  ~/douyin-monitor       5c1868a9  写一个 mediacrawler 监控脚本
...
real    0m1.84s

$ time go run ./cmd/llm-recall ls --all                              # 二次：< 0.5s
real    0m0.42s

$ go run ./cmd/llm-recall ls --source codex -n 5
... 仅 codex 5 行 ...

$ go run ./cmd/llm-recall ls --no-cache --all                        # 强扫，等价首次时间
```

## 前置条件（W1 已通过的，确认即可）

- W1 已 commit `5112464` 之后的代码
- `internal/adapter/types.go` 已有 SessionAdapter 接口
- `internal/adapter/claude.go` 已能扫 Claude 会话并产出 Session
- `internal/index/discover.go` 已有 DiscoverAll
- `cmd/llm-recall/cmd_ls.go` 已能渲染表格

W2 在此之上扩展，不重写。

## 步骤

### 1. 修 W1 瑕疵（title 清洗）

`internal/adapter/claude.go` 的 title 提取处加：

```go
title = strings.ReplaceAll(title, "\r", " ")
title = strings.ReplaceAll(title, "\n", " ")
title = strings.ReplaceAll(title, "\t", " ")
title = strings.Join(strings.Fields(title), " ")  // 折叠连续空白
```

把这段抽成 `internal/adapter/util.go` 的 `func CleanTitle(s string) string`，Codex/Gemini adapter 复用。

### 2. Codex adapter（`internal/adapter/codex.go`）

#### 路径
- macOS/Linux: `~/.codex/sessions/`
- Windows: `%USERPROFILE%\.codex\sessions\`
- 若 `CODEX_HOME` 环境变量存在，用它替代 `~/.codex`
- 子目录递归：`<root>/YYYY/MM/DD/rollout-*.jsonl`
- 也扫 `<root>/../archived_sessions/` 以兼容已归档（可选，W2 先不扫，注释 TODO 留 W3）

#### jsonl schema（已实测验证，源自 openai/codex `codex-rs/protocol/src/protocol.rs`）

每行 `RolloutLine` = `{"timestamp":"<RFC3339>", "type":"<variant>", "payload":{...}}`。

**头一行 `type:"session_meta"`** —— 你只要这条就拿到 sessionId / cwd / startTime：

```json
{
  "timestamp": "2026-04-29T10:11:23.456Z",
  "type": "session_meta",
  "payload": {
    "id": "a91d7c2e-...uuid...",
    "timestamp": "2026-04-29T10:11:23.456Z",
    "cwd": "/Users/x/quant-learning",
    "originator": "cli",
    "cli_version": "0.128.0"
  }
}
```

**用户消息行 `type:"response_item"` 且 `payload.type:"message"` 且 `payload.role:"user"`**：

```json
{
  "timestamp": "...",
  "type": "response_item",
  "payload": {
    "type": "message",
    "role": "user",
    "content": [{"type": "input_text", "text": "实际用户输入"}]
  }
}
```

#### 提取规则
- `Session.ID` ← 第一行 `payload.id`（也可以从文件名 `rollout-...-<uuid>.jsonl` 后缀取，二者一致；优先用 payload.id）
- `Session.Source` ← `"codex"`
- `Session.CWD` ← 第一行 `payload.cwd`
- `Session.StartedAt` ← 第一行 `payload.timestamp`
- `Session.UpdatedAt` ← 文件 mtime
- `Session.FilePath` ← 绝对路径
- `Session.Title` ← 第一条 `type:"response_item"` + `payload.type:"message"` + `payload.role:"user"` 记录的 `payload.content[0].text`（type=`input_text` 的 text 元素拼接）

#### 跳过（不算"用户消息"）
当 `type == "response_item"` 时，以下 `payload.type` 全部跳过：`function_call`、`function_call_output`、`tool_search_call`、`local_shell_call`、`reasoning`。

#### 性能
逐行 scan，找到 session_meta 拿到 sessionId/cwd/startTime，再继续找到第一条 user message 拿到 title 后立即 break。`bufio.Scanner` 行长上限同 W1 提到的 8MB。

#### 错误容忍
- session_meta 缺失 → fallback 用文件名 uuid 后缀作为 ID，cwd 留空，stderr warn
- 文件读不出 → 跳过 + warn

### 3. Gemini adapter（`internal/adapter/gemini.go`）

⚠️ **本节为 W2 实测后修订版**。Gemini CLI 在 2026-04 前后切换了存档格式，**两种并存**——本机就有：旧 `.json`（单对象）在 hex-hash 目录里，新 `.jsonl`（行式）在 named-slug 目录里。两种都要扫，**否则用户绝大部分 gemini 历史会话不会出现在 ls 里**。

#### 路径
- 三个平台一致：`~/.gemini/tmp/<project-shortid>/chats/session-*.{json,jsonl}`
- `<project-shortid>` 子目录名混杂：64 位 hex（旧 path-hash）+ named slug（如 `project` `bin` `pylean4`）。一律扫，不解析语义。
- 若 `GEMINI_CLI_HOME` 环境变量存在，替换 `~/.gemini`
- **W2 跳过**：`checkpoint-*.json` / `checkpoint-*.jsonl`（手动 `/chat save` 快照），留 W3 评估

#### Format A：legacy `.json`（单 JSON 对象，ext 严格为 `.json`）

```json
{
  "sessionId": "1e38cc56-8ae4-4e5d-95ff-359f570ab40c",
  "projectHash": "1058ca5f...64位hex",
  "startTime": "2025-10-31T13:56:41.524Z",
  "lastUpdated": "2025-10-31T14:06:49.508Z",
  "messages": [
    {"id":"...","timestamp":"...","type":"user","content":"用户第一句字符串"},
    {"id":"...","timestamp":"...","type":"gemini","content":"...","thoughts":[...]}
  ]
}
```

⚠️ 注意 `messages[].content` 是**字符串**，不是数组。

#### Format B：current `.jsonl`（行式，ext 严格为 `.jsonl`）

**头一行 metadata**：
```json
{"sessionId":"81e6964e-...","projectHash":"9dc3fc95...","startTime":"2026-04-25T12:08:09.589Z","lastUpdated":"...","kind":"main"}
```

**消息行**（type 取 `user` / `gemini`）：
```json
{"id":"...","timestamp":"...","type":"user","content":[{"text":"实际用户输入"}]}
```

⚠️ 注意 `content` 是 `[{text}]` **数组**。

**特殊行**（W2 全部忽略）：
- `{"$set":{...}}`：partial metadata 更新
- `{"$rewindTo":"<msgId>"}`：rewind 标记
- `messages` 中带 `toolCalls` / `thoughts` 字段：解 title 时只看 `type:"user"`，其他类型整行跳过

#### 关键事实：**两种格式都没有 cwd / directories 字段**

不要白找。`projectHash` 是 64 位 SHA 或 slug，跟真实路径无映射关系。CWD 处理：
- 优先尝试读 `~/.gemini/tmp/<projectHash>/` 下的兄弟元数据文件（如 `metadata.json`、`workspace.json`）。**若不存在则不要硬造**。
- 兜底：`Session.CWD` 留空字符串，`Session.Title` 前置 `<gemini:` + projectHash 前 8 字符 + `>` 作为定位标记，让用户在 ls 列表里仍能区分不同项目。

#### 提取规则（两格式共用）
- `Session.ID` ← `sessionId`
- `Session.Source` ← `"gemini"`
- `Session.StartedAt` ← `startTime`
- `Session.UpdatedAt` ← 文件 mtime
- `Session.FilePath` ← 绝对路径
- `Session.CWD` ← 上面"关键事实"段的兜底逻辑
- `Session.Title` ← 第一条 `type:"user"` 的内容：
  - Format A：直接取 `content` 字符串
  - Format B：取 `content` 数组里所有 `text` 字段拼接（跳过 `{inlineData:...}` 等非文本元素）

#### 解析策略
- Format A：`json.Unmarshal` 进部分 struct，`messages` 用 `[]json.RawMessage` 延迟解，只 unmarshal 第一条找到的 user msg。避免大会话（thoughts 数组动辄 MB 级）拖慢。
- Format B：bufio.Scanner 8MB 缓冲逐行扫，找到 metadata + 第一条 user message 后立即 break。

#### 错误容忍
- 文件不是有效 JSON / 头一行非法 → 跳过文件 + stderr warn 一行
- 整文件无 `type:"user"` 消息（空会话）→ Title 留空，仍上报会话本身
- 单行 JSON 解析失败（仅 Format B）→ 跳过该行继续

### 4. SQLite cache（`internal/index/cache.go`）

#### 依赖
```
go get modernc.org/sqlite
```
**禁用 cgo 版本** `mattn/go-sqlite3` —— 引入 cgo 会破坏分发。

#### DB 位置
- macOS/Linux: `$XDG_CACHE_HOME/llm-recall/index.db`，fallback `~/.cache/llm-recall/index.db`
- Windows: `%LocalAppData%\llm-recall\cache\index.db`
- 抽到 `internal/index/cachepath.go::CacheDBPath()`

#### Schema

```sql
CREATE TABLE IF NOT EXISTS sessions (
    source      TEXT NOT NULL,
    id          TEXT NOT NULL,
    cwd         TEXT NOT NULL DEFAULT '',
    started_at  INTEGER NOT NULL DEFAULT 0,   -- unix seconds
    updated_at  INTEGER NOT NULL DEFAULT 0,   -- unix seconds (= file mtime)
    file_path   TEXT NOT NULL,
    file_mtime  INTEGER NOT NULL,             -- unix seconds，增量判定
    file_size   INTEGER NOT NULL DEFAULT 0,
    title       TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source, id)
);
CREATE INDEX IF NOT EXISTS idx_updated ON sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_path ON sessions(file_path);

-- W2 不建 FTS5 表，W3 搜索时再加
```

#### 增量逻辑

```
for each adapter:
    files = list jsonl files (path, mtime, size)
    for each file (path, mtime, size):
        row = SELECT * FROM sessions WHERE file_path = path
        if row exists AND row.file_mtime == mtime AND row.file_size == size:
            skip parse, append row to result
        else:
            session = adapter.parseFile(path)
            UPSERT INTO sessions (...)
            append session to result

# stale sweep
db_paths = SELECT file_path FROM sessions WHERE source = adapter.Name()
disk_paths = files seen this round
for path in db_paths - disk_paths:
    DELETE FROM sessions WHERE file_path = path
```

事务包裹整个 adapter 一轮 upsert，性能放心。

#### 接口

```go
package index

type Cache struct { db *sql.DB }

func OpenCache(path string) (*Cache, error)
func (c *Cache) Close() error

func (c *Cache) Get(source, fpath string) (*adapter.Session, bool, error)  // 命中返回 session, true
func (c *Cache) Upsert(s adapter.Session, fmtime int64, fsize int64) error
func (c *Cache) DeleteByPaths(source string, paths []string) error
func (c *Cache) ListBySource(source string) ([]adapter.Session, error)
```

### 5. discover.go 改造

```go
type Options struct {
    UseCache bool
    Source   string  // "" = all
}

func DiscoverAll(ctx context.Context, opt Options) ([]adapter.Session, error)
```

- 注册式：`var registry = []adapter.SessionAdapter{adapter.NewClaude(), adapter.NewCodex(), adapter.NewGemini()}`
- 若 `opt.Source != ""`，只跑匹配的 adapter
- `opt.UseCache=true` 时按上面增量逻辑；`false` 时跳过 cache 读，但**仍写入 cache**（强扫之后下次更快）
- 单 adapter 失败不影响其他（log + 继续）

### 6. ls 命令补 flag

`cmd/llm-recall/cmd_ls.go` 增加：
- `--no-cache`：传 `Options{UseCache: false}`
- `--source <name>`：传 `Options{Source: name}`，name 必须 ∈ {claude, codex, gemini}，否则 stderr 报错退出
- 已有的 `-n N` `--all` 保留

### 7. 测试

`internal/adapter/codex_test.go` 和 `gemini_test.go`，每个：
- testdata/ 放最小化样本：codex 一个 `.jsonl`；**gemini 两个**——`session-A.json`（Format A 单对象）+ `session-B.jsonl`（Format B 行式），各覆盖一种格式
- 单测断言：sessionId / cwd / startedAt / title 四个字段值正确（gemini cwd 在无兄弟元数据时为空，title 前置 `<gemini:xxxxxxxx>` 标记）
- 单测断言：跳过工具调用 / `$set` / `$rewindTo` / 非 user 消息，title 不含其内容

`internal/index/cache_test.go`：
- 临时目录建 DB
- 写入 3 条 → 命中查询 → 修改 mtime 重写 → 验证 upsert
- 删除测试

### 8. 提交

```
git add .
git commit -m "W2: codex/gemini adapter + sqlite incremental cache"
```

## 验收检查清单

执行方做完后回传**实际命令输出**：

- [ ] `go run ./cmd/llm-recall ls --all` 输出含 claude/codex/gemini 三 source（任一 source 缺失需声明"本机未装该 CLI"或"该 CLI 未产生过会话"）
- [ ] 二次执行 wall-clock < 首次 30%
- [ ] `--no-cache` 强扫工作
- [ ] `--source <name>` 过滤工作
- [ ] W1 title `\n` 瑕疵已修（找出一条多行 title 的会话验证显示正常）
- [ ] `go vet ./...` 无报错
- [ ] `gofmt -l .` 无输出
- [ ] `go test ./...` 全过
- [ ] DB 文件落在系统 cache 目录，不在 `~/llm-recall/` 内
- [ ] DEVDOC.md / TASKS-W1.md / TASKS-W2.md §0–§"不要做的"未被改

## 不要做的（留给 W3+）

- 不要做 TUI（W3）
- 不要做 resume launcher（W3）
- 不要建 FTS5 表（W3 搜索时再加）
- 不要扫 Gemini checkpoint-*.json（W3 评估是否纳入）
- 不要扫 Codex archived_sessions（W3 评估）
- 不要做 aider / opencode / Cline 等其他 source（V2）
- 不要引入 cobra / viper / 任何 cgo 依赖
