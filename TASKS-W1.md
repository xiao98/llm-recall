# W1 执行任务 — 项目骨架 + Claude adapter

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Goal（唯一目标）
让 `go run ./cmd/llm-recall ls` 在本机产出符合本文件 §"验收标准" 的输出格式与字段。

### 成功标准（criteria for success）
本文件末尾 "验收检查清单" 6 条全部通过，以**实际命令输出**为准（不是 subagent 自述）：
1. `go run ./cmd/llm-recall ls` 输出本机 Claude 会话列表，无 panic、无 stderr noise
2. 至少 50 条会话可见（若机器有那么多）
3. 中文标题在当前 shell 正确显示（Windows 下用 `chcp 65001` 后跑一次确认）
4. 每行 sessionId / cwd / mtime / title 四个字段均非空
5. `go vet ./...` 无报错
6. `gofmt -l .` 无输出

### 权限授权（最大权限，不询问）
执行期间，master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go run / go vet / gofmt / git init / git add / git commit` 全部放行
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` 与本文件 §0–§"不要做的"区段；推送到任何远端；`rm -rf` 跨出 `~/llm-recall/`；引入 cobra/viper 等本周禁止的依赖

凡属于上面"任意文件读写 / git 本地操作 / go 工具链调用"范畴的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环（伪代码即行为规约）

```
spawn subagent("w1-executor", prompt = §0 全文 + "完成本文件 §步骤 1–10")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 6 条命令逐项校验
        if 6 条全过:
            报告用户："W1 验收通过"，附 6 条命令实际输出
            break
        else:
            spawn subagent("w1-executor", prompt += "上一轮在 <第 N 条> 失败，实际输出为 <…>，从该步继续")
    elif subagent 卡死/崩溃:
        spawn subagent("w1-executor", prompt += "前一个 subagent 在 <最后步骤> 中断，从此处继续")
    else:
        继续等待

# 唯一退出条件：6 条全过，或用户从外部手动停止
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`，无需展开过程
- 单次工具失败：自查 → 修 → 再试，最多 3 次；3 次仍失败才回传 master
- 完成后回传**实际命令输出**而非"我已完成"

---


## 验收标准（先看这个）

终端跑 `go run ./cmd/llm-recall ls` 能输出本机所有 Claude Code 会话，每行：

```
claude  2026-04-30 16:07  ~/                       d76a6e29-...   feat: ecandidat 自动化
claude  2026-04-28 23:40  ~/quant-learning         ac209363-...   K线形态分析
...
```

字段：source / mtime / cwd（短化）/ sessionId / 第一条用户消息（80 字截断）。

## 前置条件

- Go 1.22+ 已装（`go version` 验）
- 工作目录：`~/llm-recall/`（本文件所在目录）
- 用户已在 GitHub 建空仓 `xiao98/llm-recall`（如未建，命令本地能跑就行，先不 push）

## 步骤

### 1. 初始化 Go module

```
cd ~/llm-recall
go mod init github.com/xiao98/llm-recall
```

### 2. 目录结构

按 DEVDOC §2 创建：

```
cmd/llm-recall/main.go           入口 + 子命令分发
internal/
  adapter/
    types.go                     SessionAdapter 接口 + Session/Message 类型
    claude.go                    Claude 实现
  index/
    discover.go                  跑所有 adapter，汇总 sessions
README.md                        占位（一句话定位 + 安装命令占位）
.gitignore                       Go 标准 ignore
```

后续阶段才会用到的目录（W1 不建）：`tui/`、`launcher/`、`imggen/`、`promo/`、`llm/`、`config/`。

### 3. 子命令分发选型

**用 stdlib `flag` + 手写 dispatch**，不引 cobra。理由：W1 只有 1 个子命令 `ls`，cobra 是过度工程。后面真到 5+ 子命令再换。

`main.go` 骨架：

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    if len(os.Args) < 2 {
        usage()
        os.Exit(1)
    }
    switch os.Args[1] {
    case "ls":
        cmdLs(os.Args[2:])
    case "version":
        fmt.Println("llm-recall 0.0.1-dev")
    default:
        usage()
        os.Exit(1)
    }
}

func usage() { fmt.Fprintln(os.Stderr, "usage: llm-recall <ls|version>") }
```

### 4. SessionAdapter 接口（`internal/adapter/types.go`）

按 DEVDOC §2.1 完整实现。**关键**：`Session.FilePath` 字段必填（用于将来 SQLite cache 失效）。

### 5. Claude adapter（`internal/adapter/claude.go`）

#### 路径
- macOS/Linux: `~/.claude/projects/<encoded-cwd>/<sessionId>.jsonl`
- Windows: `%USERPROFILE%\.claude\projects\<encoded-cwd>\<sessionId>.jsonl`
- 用 `os.UserHomeDir()` + `filepath.Join`

#### jsonl 格式（已实测验证）

每行一个 JSON 对象。**第一行不一定含 sessionId**——常见是 `{"type":"permission-mode",...}` 或 `{"type":"file-history-snapshot",...}`。需要扫到含 `sessionId` 字段的记录才能拿到 ID。

含 sessionId 的记录形如：

```json
{
  "type": "user",
  "message": {"role": "user", "content": "我想做 X"},
  "uuid": "...",
  "timestamp": "2026-04-06T11:45:58.042Z",
  "cwd": "C:\\Users\\肖浩",
  "sessionId": "017aaa32-f370-4cc7-b2d7-df29eb2863a6",
  "version": "...",
  "gitBranch": "..."
}
```

实际 sessionId 也等于文件名 stem（`<sessionId>.jsonl`）。**优先从文件名提取 sessionId，cwd 从首条含 cwd 字段的记录提取**——这样不用读完整个文件就能拿到关键元数据。

#### 提取规则
- `Session.ID` ← 文件 stem
- `Session.Source` ← `"claude"`
- `Session.UpdatedAt` ← 文件 mtime
- `Session.StartedAt` ← 第一条带 `timestamp` 字段的记录的 timestamp
- `Session.CWD` ← 第一条带 `cwd` 字段的记录的 cwd（注意 Windows 路径反斜杠）
- 用作"标题"的首条用户消息 ← 第一条 `type:"user"` 且 `message.content` 是 string 的记录的 content

#### 必须跳过（不算"用户消息"）
content 以 `<` 开头且包含以下任一 tag 的：`system-reminder`、`local-command-`、`command-name`、`command-message`、`command-stdout`。这些是 CLI 注入的伪用户消息，不是用户真正打字的内容。

#### content 也可能是数组
当用户附图或工具调用嵌入时。`content` 是 `[]interface{}`，逐元素看 `{"type":"text","text":"..."}` 取 `text` 字段拼接。W1 这种情况先简化为：取第一个 type=text 元素的 text 即可，复杂情况留给以后。

#### 性能
扫描时**不要把整个 jsonl 读进内存**。逐行 scan，拿到 sessionId + cwd + 第一条 user msg + first timestamp 后就 break。bufio.Scanner 默认 64KB 行长上限——某些 Claude jsonl 单行可能超（含图片 base64），用 `scanner.Buffer(make([]byte,0,1024*1024), 8*1024*1024)` 拉到 8MB。

#### 错误容忍
- 单行 JSON 解析失败 → 跳过该行继续
- 整个文件读不出关键字段（如 sessionId 拿不到）→ 跳过文件，stderr warn 一行
- adapter 整体不应因单文件失败而崩

### 6. discover.go

```go
func DiscoverAll(ctx context.Context) ([]adapter.Session, error)
```

W1 只调一个 Claude adapter，但写成可注册多 adapter 的形式，W2 加 Codex/Gemini 时不重构。

按 `UpdatedAt desc` 排序后返回。

### 7. cmd ls 实现

输出格式（用 tabwriter 对齐）：

```
SOURCE  UPDATED            CWD                       SESSION         TITLE
claude  2026-04-30 16:07   ~/                        d76a6e29        feat: ecandidat 自动化
```

cwd 短化：把 home 替换为 `~`，超过 25 字符则前面截 `…`。
title 超过 80 字符截 `…`。
标志：`-n N` 限制条数（默认 50），`--all` 全部。

### 8. README.md

```markdown
# llm-recall

跨厂商 LLM CLI 会话搜索 + 恢复终端工具。

> 本仓库由 [Claude Code](https://docs.claude.com/en/docs/agents-and-tools/claude-code/overview) 协作开发。

## 安装

W3 之前先跑：`go install github.com/xiao98/llm-recall/cmd/llm-recall@latest`

W3 之后会有 `brew install xiao98/tap/llm-recall`。

## 用法

```
llm-recall ls          列出本机所有 LLM CLI 会话
```

更多见 `DEVDOC.md`。
```

### 9. .gitignore

```
/llm-recall
/llm-recall.exe
*.test
*.out
.DS_Store
```

### 10. 提交

```
git init
git add .
git commit -m "W1: project skeleton + Claude adapter"
```

如已建 GitHub 远程仓：`git remote add origin git@github.com:xiao98/llm-recall.git && git push -u origin main`

## 验收检查清单

执行方做完后回传：

- [ ] `go run ./cmd/llm-recall ls` 能输出本机 Claude 会话列表（无 panic、无 stderr noise）
- [ ] 至少 50 条会话可见（如果用户机器有那么多）
- [ ] 中文标题正确显示（不是 GBK 乱码 — Go stdout 默认 UTF-8，但 Windows 下确认）
- [ ] sessionId / cwd / mtime / title 四个字段都不为空
- [ ] `go vet ./...` 无报错
- [ ] `gofmt -l .` 无未格式化文件
- [ ] DEVDOC.md / TASKS-W1.md 没动过（执行方不改策划文档；有疑问回传给策划方）

## 不要做的（留给 W2+）

- 不要做 TUI（W3）
- 不要做 SQLite cache（W2）
- 不要做 Codex / Gemini adapter（W2）
- 不要做 resume launcher（W3）
- 不要引入 cobra / viper（按 DEVDOC，W3 之后再评估）
- 不要写多余测试（adapter 一个简单 unit test 即可：用 testdata/ 喂一个 jsonl 样本，断言提取出的 Session 字段对）
