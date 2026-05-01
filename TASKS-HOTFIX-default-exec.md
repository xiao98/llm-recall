# HOTFIX: 翻转 dry-run default + gemini 提示可见性

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` + `TASKS-W3.md` §0 + `TASKS-HOTFIX-tui-resize.md`。
>
> **优先级**：W8 录 demo 之前必须修。当前默认 dry-run 是反人类 UX，dogfood 直接暴露。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（HOTFIX 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Bug 现象

```
$ llm-recall              # 用户期望：选中后直接进入对应会话
→ exec: codex resume <id> in C:\Users\肖浩
$                          # 实际：只打印一行就退出，没 exec
```

### 根因

`cmd/llm-recall/main.go` 的 dry-run flag default = `true`，是 W3 验收期约束（avoid master 进程被替换），日常用应反向。

附带问题：gemini 退化路径下，提示文字（`→ 进入后请运行：/chat resume <id>`）紧跟着 exec gemini，REPL 覆盖屏幕导致用户**看不到** sessionId。

### Goal

让 `llm-recall` 默认行为符合用户预期：选中会话回车 → **直接** 进入对应 CLI（claude / codex / gemini）。`--dry-run` 显式打开干跑模式（开发 / 调试用）。

### 成功标准（criteria for success — 6 条）

1. `llm-recall` 不带 flag → 选中 claude session 回车 → 真启动 `claude --resume <id>` 替换进程；`/exit` 后回 shell 退出码 0
2. 同上选中 codex session → 真启动 `codex resume <id>`；exit 0
3. 同上选中 gemini session → 真启动 `gemini`（退化路径），且 stderr 提示 `→ 进入后请运行：/chat resume <id>` **在 exec 前**清晰可见（sleep 1.5s 让 REPL 起来前用户能读到）
4. `llm-recall --dry-run` → 旧行为：只打印 `→ exec: ...` 一行不真 exec，退出码 0
5. `--no-dry-run` flag 移除（已不需要，旧名字还接受但 stderr 一行 deprecation warn）
6. `go vet ./...` / `gofmt -l .` / `go test ./...` 全过；`internal/launcher/launcher_test.go` 改测试名 + 加 `TestRunRealExec` 用 `LLM_RECALL_LAUNCHER_FAKE_EXEC` env mock 断言 cmd/args/cwd 构造

### 权限授权

执行期间 master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写
- `go build / go run / go vet / go test / gofmt / git add / git commit` 全部放行
- 启动 `claude` / `codex` / `gemini` 子进程（仅 §1/§2/§3 e2e 验收一次，立即 `/exit`）
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；动 launcher / TUI / adapter 的非 default-flag 部分（仅改 default 翻转 + gemini sleep 提示，其他不动）

凡属上述允许范围的动作，**默认许可，不进 plan mode**。

---

## 步骤

### 1. 翻转 dry-run default

`cmd/llm-recall/main.go`（或 `cmd_tui.go`）：

```go
// 旧
flag.BoolVar(&flagDryRun, "no-dry-run", false, "execute resume target instead of dry-run")
flagDryRun = !flagDryRun  // 倒置

// 新
flag.BoolVar(&flagDryRun, "dry-run", false, "print exec line without spawning subprocess (for testing)")

// 保留 --no-dry-run 兼容（一行 deprecation warn）
flag.BoolVar(&flagLegacyNoDry, "no-dry-run", false, "deprecated alias (now default behavior)")
if flagLegacyNoDry {
    fmt.Fprintln(os.Stderr, "warn: --no-dry-run is now default; flag is a no-op and will be removed in v0.3")
}
```

### 2. gemini 退化路径提示可见性

`internal/launcher/launcher.go`（或 `internal/adapter/gemini.go::ResumeCommand` 调用处）：

gemini 走交互式退化时，在 `os.Chdir` + `syscall.Exec`（Unix）/ `cmd.Run`（Windows）之前：

```go
if source == "gemini" {
    fmt.Fprintf(os.Stderr, "\n→ 进入 gemini 后请运行：/chat resume %s\n", session.ID)
    fmt.Fprintln(os.Stderr, "  或先 `/chat list` 看所有 session")
    fmt.Fprintln(os.Stderr, "")
    time.Sleep(1500 * time.Millisecond)  // 让用户看清提示
}
```

claude / codex 不需要 sleep（`--resume <id>` 直接进对应会话，无歧义）。

### 3. 测试

`internal/launcher/launcher_test.go` 加 mock exec case：

```go
func TestRunRealExec_Claude(t *testing.T) {
    t.Setenv("LLM_RECALL_LAUNCHER_FAKE_EXEC", "1")
    s := adapter.Session{Source: "claude", ID: "abc-123", CWD: "/tmp"}
    err := launcher.Run(s, false /* dryRun=false */)
    // 断言 fake exec 收到 cmd=claude, args=[--resume, abc-123], cwd=/tmp
}
```

### 4. README + onboarding 文本

`README.md` 的 Usage 段：

```diff
- llm-recall --no-dry-run       # TUI 选中后真启动子进程进入会话
+ llm-recall                     # 默认：选中后真启动对应 CLI
+ llm-recall --dry-run           # 调试模式：只打印不真启动
```

如有其他文档（landing / launch drafts）提到 `--no-dry-run`，全部改为新表达。

### 5. 提交

```
git add .
git commit -m "fix(launcher): default to real exec; --dry-run as opt-in; gemini hint visibility"
```

## 验收检查清单

- [ ] `llm-recall`（无 flag）选 claude → 真进 claude，/exit 后 exit 0
- [ ] 同上选 codex → 真进 codex，/exit 后 exit 0
- [ ] 同上选 gemini → 提示行 stderr 可见（截屏含 `/chat resume` 行）+ 1.5s 后 gemini REPL 起来
- [ ] `--dry-run` → 旧行为
- [ ] `--no-dry-run` → 接受 + deprecation warn
- [ ] go vet / fmt / test 全过
- [ ] README + onboarding 文本里所有 `--no-dry-run` 已替换
- [ ] DEVDOC / 历史 TASKS 未改

## 不要做的

- 不要重写 launcher / TUI / adapter 的其他逻辑（仅 default 翻转 + gemini sleep）
- 不要 break `--dry-run` 的旧测试（开发期还要用 dry-run 跑 mock）
- 不要给 gemini 加"自动 inject /chat resume"魔法（已知 gemini-cli 上游不支持 stdin pre-inject 此命令，强行做会脆弱）
- 不要动 W6 onboarding 的"Enter 接受 / q 退出" 文本（W6 验收过的）
