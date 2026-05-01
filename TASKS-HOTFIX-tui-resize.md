# HOTFIX: TUI 响应式 layout（修 W3 fixed-layout bug）

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W7.md` §0，再做本文件。
>
> **优先级**：在 W8 前必须修。W8 录 demo GIF 在非默认终端尺寸下会糊，且用户日常使用已暴露此 bug。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（HOTFIX 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Bug 现象（用户截图实证）

终端 resize 到约 1240×620 px（约 100 列宽）时：
- 列表区每条会话渲染为**双行**（metadata 一行 + 缩略 title 一行）
- 顶部会话被挤出 viewport，看不到最近 N 条
- TUI 不响应窗口 resize 事件 —— 调整窗口大小后 layout 不变，必须重启进程才能匹配新尺寸

### 根因（追根因不打补丁）

1. `internal/tui/model.go` Update() 没处理 `tea.WindowSizeMsg`
2. `internal/tui/view.go` 列表 item 是双行渲染（fixed item height = 2），高度计算硬编码
3. 列表没用 `bubbles/viewport` 或等价 scroll container，溢出时直接截断而非滚动
4. lipgloss 列宽（cwd / title）写死字符数，未按 `terminal.width` 计算

### Goal（唯一目标）

让 TUI 在任意终端尺寸（≥ 60×16）下 layout 自适应，**实时跟随 resize**（不需要重启进程），列表始终单行渲染，选中项始终可见。

### 成功标准（criteria for success — 9 条）

1. **WindowSizeMsg 处理**：终端从 80×24 拉到 200×60 再缩回 80×24（一气呵成不重启），TUI layout 实时跟随；用 `tmux split-window` 或手动 resize 终端验证
2. **list item 单行**：每条会话占 1 行（不再是 metadata + title 双行），按当前 terminal width 截断各字段（cwd 优先窄化，title 占剩余）
3. **viewport 滚动**：列表条数 > 可见行数时，↑↓ 移动选中项；选中项贴近 viewport 边界时自动滚动；首次进入选中项默认在第 0 行
4. **小尺寸保护**：终端 < 60 列 或 < 16 行时显示纯文本提示 `terminal too small (need ≥ 60×16, got <W>×<H>)`，不渲染 layout 防崩坏；恢复尺寸后立即正常渲染
5. **预览面板自适应**：宽度 = `(terminal.width - 2) / 2`（双栏等分），高度跟列表同步；body 长时 viewport 滚动（j/k 或 PageUp/PageDown，不抢列表 ↑↓）
6. **banner / footer 高度计算正确**：`列表可见行数 = terminal.height - banner_lines - input_line - footer_line - borders`；banner / footer 多行（如 5% CTA 出现时 banner 2 行）也算对
7. **stats 命令同样修**：`internal/stats/render.go` 也要响应 resize（heatmap 宽度 + 4×2 panel 都要按 terminal.width 重排；过窄时降级显示）
8. **`go vet ./...` / `gofmt -l .` / `go test ./...` 全过**；新加 `internal/tui/layout_test.go`：给定 `(width, height)` 断言各区域分配宽高正确
9. DEVDOC.md / 历史 TASKS-W*.md §0–§"不要做的"未被改

### 权限授权（最大权限，不询问）

执行期间 master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除（**重点改动**：`internal/tui/`、`internal/stats/render.go`）
- `go mod / go build / go run / go vet / go test / gofmt / git add / git commit` 全部放行
- 启动子进程跑 TUI 测试（用 `LLM_RECALL_TEST_INPUT` env var 驱动键盘事件 + harness 抓 stdout）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；动 `internal/adapter/` / `internal/index/` / `internal/llm/` / `internal/promo/` / `internal/config/`（这些跟 layout 无关）；引入新 bubbles 之外的 TUI 库；改 banner / footer / sponsored 文案（layout-only fix）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("hotfix-executor", prompt = §0 全文 + "完成本文件 §步骤 1–7")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 9 条命令逐项校验
        重点验：用 LLM_RECALL_TEST_TERM_WIDTH / HEIGHT env var 跑 TUI 在多个尺寸下断言渲染正确
        if 9 条全过:
            报告用户："HOTFIX 验收通过"，附实测多尺寸渲染对照
            break
        else:
            spawn subagent("hotfix-executor", prompt += "上一轮在 <第 N 条> 失败，从该步继续")
    else:
        继续等待
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- W1-W7 子 agent 留下的合理偏离保留，不要回滚

---

## 验收标准（先看这个）

```bash
# 1. 默认终端尺寸（开发者常见 100×30）
$ llm-recall
# 列表单行 / 预览右栏 / banner+input+footer 都在；选中项可见

# 2. 缩到 80×24（HN 截屏标准）
$ resize-terminal 80 24 && llm-recall
# layout 重排成功；列表更窄但仍单行

# 3. 缩到极小 50×12（应触发 too-small 提示）
$ resize-terminal 50 12 && llm-recall
# 屏幕显示: terminal too small (need ≥ 60×16, got 50×12)

# 4. 进程内 resize（关键测试）
$ llm-recall &
$ # 在另一个 pane 拖动当前终端窗口大小
# layout 实时跟随，不需要重启 llm-recall

# 5. 列表多到 50 条时滚动
$ llm-recall
# ↓↓↓...↓ 选中往下走，到底部时列表自动滚动；选中项始终可见
```

## 前置条件

- W7 commit `27680ed` 之后的代码（含 W6 banner、W7 gold/card）
- 用户能看到 bug 现象（截图已实证）

## 步骤

### 1. Model 加 width/height + WindowSizeMsg handler

`internal/tui/model.go`：

```go
type Model struct {
    // ... 已有字段
    width  int
    height int
    tooSmall bool
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.tooSmall = msg.Width < 60 || msg.Height < 16
        // 重算 list / preview / viewport 高度
        m.recomputeLayout()
        return m, nil
    // ... 其他 case
    }
}
```

### 2. recomputeLayout：分配各区域宽高

```go
func (m *Model) recomputeLayout() {
    if m.tooSmall { return }

    // 垂直分配
    bannerH := promo.BannerHeight(m.cfg)         // 0 / 1 / 2 行（取决于 promo + CTA）
    inputH  := 1                                  // search input
    footerH := 1                                  // key hints + source/hits
    bordersV := 4                                 // 列表 + 预览各 2 行边框
    listH := m.height - bannerH - inputH - footerH - bordersV
    if listH < 3 { listH = 3 }                   // 最少 3 行可见

    // 水平分配（双栏等分）
    listW := (m.width - 2) / 2                   // -2 留给中间分隔
    previewW := m.width - listW - 2

    m.listView.SetSize(listW, listH)
    m.previewView.SetSize(previewW, listH)
}
```

### 3. List item 单行渲染

`internal/tui/view.go`：

```go
// 旧（双行）：
// claude 05-01 12:35  C:\Users\肖浩    da12ab7a
// htt…

// 新（单行，按 listW 分配各字段）：
// claude 05-01 12:35  ~/...     da12ab7a  htt 飞书 wiki 内嵌 bita…

func renderItem(s adapter.Session, listW int, selected bool) string {
    // 固定字段宽度：source (7) + date (6) + time (5) + id (9) = 27 chars (含空格分隔)
    fixedW := 27
    remaining := listW - fixedW

    // cwd 占 30%，title 占 70%（cwd 不是关键）
    cwdW := remaining * 30 / 100
    titleW := remaining - cwdW - 2  // -2 分隔空格

    cwdStr := truncateLeft(shortCWD(s.CWD), cwdW)        // 左截断 ~/... 风格
    titleStr := truncateRight(s.Title, titleW)           // 右截断 …

    return fmt.Sprintf("%-7s %-6s %-5s %-*s %-9s %s",
        s.Source, formatDate(s.UpdatedAt), formatTime(s.UpdatedAt),
        cwdW, cwdStr, s.ID[:8], titleStr)
}
```

中文宽度用 `runewidth.Truncate(s, w, "…")` 而不是 `[:n]`（避免切半字符）。

### 4. Viewport 滚动

list 部分用 `bubbles/list`（已有）或 `bubbles/viewport` 包装：

- bubbles/list 内置滚动；只需设 `list.SetSize(listW, listH)`
- 选中项进入边界自动 scroll（list 默认行为）
- preview 用 `bubbles/viewport`，body 长时 PageUp/PageDown 滚动

### 5. Too-small 保护

```go
func (m Model) View() string {
    if m.tooSmall {
        return fmt.Sprintf("\n  terminal too small\n  need ≥ 60×16, got %d×%d\n  请放大窗口\n", m.width, m.height)
    }
    // ... 正常 layout
}
```

### 6. stats 命令同样修

`internal/stats/render.go`：

stats 是一次性命令（不进入 bubbletea 循环），但仍需响应 terminal width：

- 启动时 `term.GetSize(int(os.Stdout.Fd()))` 拿当前宽度
- heatmap 宽度 = min(默认 53 周, terminal.width - 6)；过窄时降级显示"⚠ stats heatmap 需要 ≥ 60 列宽，当前 <W>"
- 4×2 panel 在 width < 80 时降级为 4×1（垂直堆叠）

不需要监听 resize（stats 一次性输出后退出，用户下次再跑）。

### 7. layout_test 单测

`internal/tui/layout_test.go`：

```go
func TestRecomputeLayout(t *testing.T) {
    cases := []struct {
        w, h int
        wantListW, wantListH, wantPreviewW int
        wantTooSmall bool
    }{
        {100, 30, 49, 24, 49, false},
        {80, 24, 39, 18, 39, false},
        {60, 16, 29, 10, 29, false},
        {50, 12, 0, 0, 0, true},
    }
    for _, c := range cases {
        m := Model{width: c.w, height: c.h, cfg: defaultCfg()}
        m.recomputeLayout()
        // 断言
    }
}
```

### 8. e2e 验证脚本

写一个 `scripts/test-tui-resize.sh`：

```bash
# 用 stty 模拟不同终端尺寸
for size in "80 24" "100 30" "132 40" "60 16" "50 12"; do
    read w h <<< "$size"
    LLM_RECALL_TEST_TERM_WIDTH=$w LLM_RECALL_TEST_TERM_HEIGHT=$h \
        timeout 2 ./llm-recall < /dev/null > /tmp/tui-$w-$h.out 2>&1 || true
    echo "=== $w×$h ===" && head -5 /tmp/tui-$w-$h.out
done
```

`LLM_RECALL_TEST_TERM_*` 让代码用 env 覆盖 termtools detection，便于测试。

### 9. 提交

```
git add .
git commit -m "fix(tui): responsive layout, listen WindowSizeMsg, single-line items"
```

## 验收检查清单

- [ ] 默认终端进 TUI 列表单行渲染（不再 metadata + title 双行）
- [ ] 进程内 resize 终端窗口，layout 实时跟随（关键测试）
- [ ] 80×24 / 100×30 / 132×40 三种尺寸下 layout 都正确
- [ ] 50×12（小于 60×16）显示 too-small 提示
- [ ] 列表 50+ 条时 ↓ 滚动正常，选中项始终可见
- [ ] 预览面板宽度自适应；长 body PageUp/PageDown 滚动
- [ ] stats 命令在 < 80 列时降级 4×1，< 60 列警告
- [ ] go vet / fmt / test 全过；layout_test.go 含 ≥ 4 case
- [ ] DEVDOC / 历史 TASKS 未改

## 不要做的

- 不要动 `internal/adapter/` / `internal/index/` / `internal/llm/` / `internal/promo/` / `internal/config/`（与 layout 无关）
- 不要改 banner / footer / sponsored 文案（W6 已定）
- 不要重写 W3 search / W7 gold/card 业务逻辑（仅 layout 层改动）
- 不要引入 bubbles 之外的 TUI 库
- 不要做 mouse 支持 / 多面板拖拽（V2）
- 不要做 theme / 配色定制（V2）
