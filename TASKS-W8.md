# W8 执行任务 — 发布周（README + landing + 公众号 + Reddit/HN）

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W7.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（W8 版 / 收口周）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### W8 与前几周的差异（最重要）

W8 是**发布收口周**。用户记忆里有 [Half-finished Pattern]（做事容易搞到一半就停）—— 这是这个项目最容易卡住的一周。**subagent 的核心任务是把所有"硬编码部分"准备到 95%，让用户只剩 5 个机械动作就完成发布**。任何"等用户决策内容"的环节都是失败信号。

W8 分两段：
- **前半周（subagent 主导）**：README 完整版 / landing 静态页 / 公众号草稿 / Reddit / HN 草稿 / 录像 storyboard / CHANGELOG
- **后半周（用户 5 个机械动作）**：起 landing → 录 4 段 demo → 推 v0.2.0 tag → 发公众号 → 发 Reddit + HN

### Goal（前半周）
让用户在后半周只需做 5 件**纯执行不思考**的事就能完成 v0.2.0 发布。所有内容性创作（文案、storyboard、配图描述、self-comment 范本）由 subagent 完成；用户只做"贴帖、按按钮、对着脚本录像"。

### 成功标准 A 段（前半周 subagent，11 条）
本文件末尾"验收检查清单 A 段"11 条全部通过：

1. `README.md` 完整版（替换 W4 v1）：4 大段 What / Install / Usage / Privacy & Promo + Configuration + Supported sources + Contributing + License；含每个命令的实际输出截图占位（4 个 png/gif 占位 placeholder：`docs/screenshots/{stats,tui,gold,card}.gif`）+ 截图录制指引
2. `docs/index.html`（landing 单页）：semantic HTML + 内联 CSS（< 200 行总），含 hero / install / quick start / privacy / GitHub link 五段；可用 `python3 -m http.server` 本地预览；为 GitHub Pages + custom domain `recall.youchun.tech` 优化（含 `<base>` / OG 标签 / favicon 占位）
3. `docs/CNAME` 含 `recall.youchun.tech`（GitHub Pages 自定义域名所需）
4. `launch/wechat-draft.md` 公众号草稿（中文）：标题 / 摘要 / 正文 800-1500 字 / 配图说明 / 文末加群 CTA → recall.youchun.tech；叙事风格"为什么做 + 怎么做 + 给谁用"，**留 3 处 `[作者填: ...]` 占位**让用户加个人化叙述（不让 subagent 编个人故事）
5. `launch/reddit-r-commandline.md` 草稿（英文）：title ≤ 100 字符 / body 200-400 字 / TL;DR / link / "Comments I expect & how I'll respond"（5-8 条预演）
6. `launch/hn-show-hn.md` 草稿（英文）：title 严格 `Show HN: llm-recall – fzf-style search across Claude/Codex/Gemini CLI sessions` 格式 / body 200-300 字 / **第一条 self-comment** 范本（作者立即在自己帖子下补充技术细节 / 隐私 / 与现有工具对比，是 Show HN 标准动作）
7. `launch/storyboard.md` 录像分镜：4 段 demo（stats / TUI search / gold / card），每段 30-60s，含**逐字命令脚本**（用户照着 `pbcopy` 一段段贴）+ 录制工具建议（`asciinema rec` + `agg` 转 GIF；或 `vhs` Charm 家的纯 Go 替代）
8. `CHANGELOG.md` v0.2.0 entry：列 W5/W6/W7 主要变更（按 Added / Changed / Fixed 分组）+ 注明 v0.1.0 → v0.2.0 是 breaking only on stats（PNG export → 终端原生）
9. `launch/post-launch-checklist.md`：用户后半周 5 个动作的逐步清单（每个动作含具体命令 / 预计耗时 / 失败回退），让用户照着 cua-tab 完成
10. `go vet ./...` / `gofmt -l .` / `go test ./...` 全过；W8 不动业务代码（如发现 W7 遗留 bug 提 dogfood 到 DOGFOOD.md，不在 W8 修）
11. DEVDOC.md / 历史 TASKS-W*.md §0–§"不要做的"未被改

### 成功标准 B 段（后半周用户做，subagent 不验）

5 个机械动作（详见 `launch/post-launch-checklist.md`）：

1. **部署 landing**：`docs/` 推 `gh-pages` 分支 → GitHub Pages → DNS CNAME `recall.youchun.tech` 指 `xiao98.github.io`（10 分钟）
2. **录 4 段 demo**：按 `launch/storyboard.md` 逐段录制；放 `docs/screenshots/`（30 分钟）
3. **推 v0.2.0**：`git tag v0.2.0 && git push origin v0.2.0` → 等 GitHub Actions 跑完（5 分钟）
4. **发公众号**：把 `launch/wechat-draft.md` 填完 3 处占位 → 公众号编辑器 → 发布；朋友圈截屏配文（30 分钟）
5. **发 Reddit + HN**：`launch/reddit-r-commandline.md` 贴 r/commandline；`launch/hn-show-hn.md` 贴 Show HN 后立即贴 self-comment（30 分钟）

总耗时 ≤ 2 小时，全机械动作。

### 权限授权（最大权限，不询问）
执行期间 master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除（**重点：`docs/` 和 `launch/` 是新建目录**）
- `go vet / go test / gofmt / git add / git commit` 放行
- `python3 -m http.server` 本地预览 landing 页（端口 8000）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；动 W1-W7 业务代码（W8 是纯文档/营销周；如发现 bug 仅在 DOGFOOD.md 记录）；引入 Tailwind / Bootstrap / Vue / React / 任何 JS 框架（landing 页**纯 HTML + 内联 CSS**，零依赖）；写中文叙事时编造作者个人故事（`[作者填: ...]` 占位标出）；编造 Reddit / HN 上的"虚假反响"或假数据；引入图片素材依赖（`docs/screenshots/*.gif` 是 placeholder，用户后半周录）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("w8-executor", prompt = §0 全文 + "完成本文件 §步骤 1–10")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 A 段 11 条命令逐项校验
        if 11 条全过:
            报告用户："W8 前半周验收通过，后半周交还用户。预计用户耗时 ≤ 2 小时。"
            附 launch/post-launch-checklist.md 全文
            break
        else:
            spawn subagent("w8-executor", prompt += "上一轮在 <第 N 条> 失败，从该步继续")
    else:
        继续等待
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- 文案创作必须：**真实 / 具体 / 不浮夸**。HN / Reddit 用户对营销话术敏感，过度"changelog 用力推"会反噬
- 中文叙事保留 `[作者填: ...]` 占位，不假装是作者
- W1-W7 子 agent 留下的合理偏离保留，不要回滚

---

## 验收标准（先看这个）

```
$ tree -L 2 docs launch
docs/
├── CNAME
├── index.html
└── screenshots/  (placeholder, 用户后半周录)
launch/
├── wechat-draft.md
├── reddit-r-commandline.md
├── hn-show-hn.md
├── storyboard.md
└── post-launch-checklist.md

$ python3 -m http.server -d docs 8000
# 浏览器访问 localhost:8000，看到 landing 页正确渲染

$ wc -l README.md docs/index.html launch/*.md
   180 README.md
   195 docs/index.html
   320 launch/wechat-draft.md
   ...

$ go vet ./... && gofmt -l . && go test ./...    # 全过
```

## 前置条件

- W7 commit `27680ed` 之后的代码
- 用户已自行跑过真 LLM e2e（gold + card 各一次，截图给团队看过没有 PII 泄露）
- W4 后半周（push v0.1.0 tag + brew install 测试）状态不影响 W8（W8 直接奔 v0.2.0）

## 步骤

### 1. README.md 完整版（替换 W4 v1）

结构（约 180 行）：

```markdown
# llm-recall

> 一个跨厂商 LLM CLI 会话搜索 + 恢复终端工具。fzf-style，支持 Claude Code / Codex / Gemini CLI。

[![Release](badge)](release-page) [![Go Reference](badge)](pkg.go.dev) [![License: MIT](badge)](LICENSE)

Homepage: https://recall.youchun.tech · Sponsored by YCAPI: https://api.youchun.tech

![demo](docs/screenshots/tui.gif)

## What it does

3-4 句话点出痛点 + 解法。

## Install

[Homebrew / Scoop / go install / from source 四段]

## Usage

### TUI mode (default)
llm-recall                    截图: docs/screenshots/tui.gif

### Stats heatmap
llm-recall stats              截图: docs/screenshots/stats.gif

### Gold (LLM-curated quotes)
llm-recall gold               截图: docs/screenshots/gold.gif

### Card (single session summary)
llm-recall card <id>          截图: docs/screenshots/card.gif

### List mode (CLI dump)
llm-recall ls --all

## Configuration

[W7 已有的 [llm] 段示例]

## Supported sources

[Claude / Codex / Gemini 三家详情 + 路径表]

## Privacy & Promo

[W6 onboarding 完整文本搬过来 + --no-promo 说明]

## Contributing

[贡献指南占位 — 三段：bug report / PR 流程 / 新 source adapter PR template]

## License

MIT

## Acknowledgements

- bubbletea / lipgloss / bubbles (Charm)
- modernc.org/sqlite
- BurntSushi/toml
- sahilm/fuzzy
- Sponsored by YCAPI (https://api.youchun.tech)
```

每段截图位置写 `<!-- screenshot: docs/screenshots/X.gif | 录制脚本见 launch/storyboard.md §X -->`，引用录像分镜。

### 2. docs/index.html landing 页

**纯 HTML + 内联 CSS，零依赖**。Hero 段 + Install + Quick Start + Privacy + GitHub link。

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <title>llm-recall — 跨厂商 LLM CLI 会话搜索</title>
  <meta name="description" content="...">
  <meta property="og:title" content="...">
  <meta property="og:description" content="...">
  <meta property="og:image" content="https://recall.youchun.tech/og.png">
  <link rel="canonical" href="https://recall.youchun.tech">
  <style>
    /* 内联 CSS，约 80 行，深色模式 + 响应式 + 等宽字体强调 */
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <h1>llm-recall</h1>
      <p class="tagline">跨厂商 LLM CLI 会话搜索 + 恢复</p>
      <pre><code>brew install xiao98/tap/llm-recall</code></pre>
    </section>
    <section class="features">...</section>
    <section class="install">...</section>
    <section class="quick-start">...</section>
    <section class="privacy">[W6 onboarding 文本简化版]</section>
    <footer>
      <a href="https://github.com/xiao98/llm-recall">GitHub</a> ·
      Sponsored by <a href="https://api.youchun.tech">YCAPI</a>
    </footer>
  </main>
</body>
</html>
```

**design constraints**：
- 单文件，CSS 内联
- 字体用 system stack（`-apple-system, BlinkMacSystemFont, "Helvetica Neue", sans-serif` + 等宽 `ui-monospace, SF Mono, Menlo, monospace` 给代码块）
- 深色背景 `#0d1117`（GitHub dark style），高亮色 `#FF6B35`（与 stats heatmap 一致）
- 响应式：手机端单列，桌面 max-width 720px 居中
- 不引入字体文件 / 不引入 JS / 不引入图标库

### 3. docs/CNAME

```
recall.youchun.tech
```

GitHub Pages 自定义域名必需文件。

### 4. launch/wechat-draft.md（公众号草稿）

```markdown
# [标题占位 — 80 字内打动开发者读者]

[作者填: 开篇 1-2 句个人化引子。例如"上个月我同时在用 4 家 LLM CLI 工具..."]

## 起因

[作者填: 个人痛点叙述，2-3 段。subagent 写一个版本作起点，标 "/* 这段你改掉 */" ]

## 解法

llm-recall 是一个...

[功能演示 4 张图：stats / tui / gold / card]

## 怎么用

[安装 + 命令示例]

## 隐私 & 营销透明

[搬 README]

## 一些技术细节

[简短：Go 单二进制 / 适配器架构 / 中转支持]

## 结语

[作者填: 个人化 closing + 加群 CTA]

---

⌐ 加入讨论群：https://recall.youchun.tech
```

### 5. launch/reddit-r-commandline.md（Reddit r/commandline 草稿）

```markdown
**Title**: llm-recall – fuzzy search & resume across Claude / Codex / Gemini CLI sessions

**Body**:
[400 字英文，含：
- 问题陈述（each LLM CLI's history search is bad / siloed）
- 解法（unified TUI, fuzzy search, resume into session in original cwd）
- Tech（pure Go, single binary, no daemon, no telemetry, no cloud）
- Install（brew / scoop / go install）
- Repo + landing
- Sponsorship 透明（一句话）
]

**Comments I expect & how I'll respond**:
1. "But isn't this just X" → diff vs Dicklesworthstone (technical) and 'X' if X is mentioned
2. "Gemini CLI session ID issue" → known limitation, link to upstream issue
3. "Why no fzf integration" → comment on tradeoff, point to --md flag
... (5-8 条预演)
```

### 6. launch/hn-show-hn.md（Show HN 草稿）

严格遵守 HN Show HN 格式：

```markdown
**Title**: Show HN: llm-recall – fzf-style search across Claude/Codex/Gemini CLI sessions

**URL**: https://recall.youchun.tech

**Body** (200-300 词):
[
- 第一段：what problem
- 第二段：what it does
- 第三段：tech stack + 单二进制 + 隐私（无 telemetry，无 cloud，本地 SQLite）
- 第四段：known limitations / contributing
]

**Self-comment** (作者立即在帖子下贴的第一条评论):
[
- "Some technical details that didn't fit in the post:" 开头
- 适配器架构 / SQLite incremental cache / lipgloss TUI
- BYOK gold / card 隐私边界
- 与 Dicklesworthstone/coding_agent_session_search 的差异（人家做底层覆盖，我做出图引爆）
- Sponsored by YCAPI 透明（1 句话，链接）
]
```

### 7. launch/storyboard.md（4 段录像分镜）

**§1 stats** (30s)
```
0:00  $ llm-recall stats
0:05  [TUI 渲染完，heatmap + 4×2 panel 全显]
0:10  按 2 切到 Last 7 days
0:15  按 3 切到 Last 30 days
0:25  按 q 退出
0:30  END
```

**§2 TUI search** (45s)
```
0:00  $ llm-recall
0:03  [TUI 起，输入框聚焦]
0:05  输入 "飞书"
0:08  [列表实时筛掉到 1-3 行]
0:12  ↓ 选中第 1 行
0:15  右侧预览面板显示完整 body
0:25  按回车
0:28  [stdout 出 → exec: claude --resume ...]
0:35  END
```

**§3 gold** (30s)
**§4 card** (20s)

每段含：
- 录制工具命令：`asciinema rec demo-stats.cast --idle-time-limit 1.5 -t "llm-recall stats"`
- 转 GIF：`agg demo-stats.cast docs/screenshots/stats.gif --speed 1.5 --theme github-dark`
- 终端尺寸：`100×30` 字符
- 字体：JetBrains Mono / Cascadia Code（推荐）
- 录制前清屏：`clear` + 隐藏 banner（用 `LLM_RECALL_TEST_MODE=1` 跳过）

### 8. launch/post-launch-checklist.md

5 个机械动作的逐步清单。每个动作：

```markdown
## 动作 N: <标题>

**预计耗时**：X 分钟
**前置依赖**：N-1 完成

**逐步**：
1. ...
2. ...
3. ...

**验证**：[一行 curl / 浏览器访问 / 看到什么 = 成功]

**失败回退**：[最常见的错 + 修复方式]
```

5 个动作：
1. 部署 landing 到 GitHub Pages
2. 录 4 段 demo + 转 GIF
3. push v0.2.0 tag
4. 发公众号 + 朋友圈
5. 发 Reddit + HN

### 9. CHANGELOG.md v0.2.0 entry

按 Keep a Changelog 格式追加：

```markdown
## [0.2.0] - 2026-05-XX

### Added
- W5: Stats command with terminal-native ASCII heatmap (GitHub-contribution-graph style); 3 windows (all/7d/30d); 4×2 panel
- W6: Onboarding 同意流 + banner / footer / `--no-promo` kill switch + config.toml `[promo]` 段
- W7: `gold` (LLM-curated Top 10 quotes) and `card` (single-session summary) commands; BYOK; lipgloss rendering; PII redaction

### Changed
- **Breaking**: Stats output changed from PNG (1080×1080 / 1080×1920) to terminal-native rendering. Pillow backend deprecated. Users截屏 directly.
- README expanded to v2 with Configuration / Supported sources / Privacy sections

### Fixed
- (W7) ...
```

### 10. 测试 + 提交

```
go vet ./... && gofmt -l . && go test ./...

git add .
git commit -m "W8: README v2 + landing + launch drafts (Reddit/HN/wechat) + storyboard"
```

## 验收检查清单 A 段

- [ ] README.md ≥ 150 行，含 4 大段 + 4 个截图占位 + Configuration + Privacy
- [ ] docs/index.html 本地 `python3 -m http.server -d docs 8000` 渲染正常
- [ ] docs/CNAME 含 recall.youchun.tech
- [ ] launch/wechat-draft.md 800-1500 字，含 ≥ 3 处 `[作者填: ...]` 占位
- [ ] launch/reddit-r-commandline.md 含 title / body / 5-8 条预演 comments
- [ ] launch/hn-show-hn.md 含 title / body / self-comment 范本
- [ ] launch/storyboard.md 含 4 段录像分镜，每段含逐字命令脚本 + 录制工具命令
- [ ] launch/post-launch-checklist.md 含 5 个机械动作，每个含耗时 / 步骤 / 验证 / 回退
- [ ] CHANGELOG v0.2.0 entry 列 W5/W6/W7 主要变更，标注 breaking change
- [ ] `go vet ./...` / `gofmt -l .` / `go test ./...` 全过
- [ ] DEVDOC / 历史 TASKS 未改

## 验收检查清单 B 段（用户后半周做，subagent 不验）

详见 `launch/post-launch-checklist.md`。最低标准：
- [ ] recall.youchun.tech 可访问，渲染 landing 页
- [ ] 4 个 GIF 在 docs/screenshots/ 且 README 中能看到
- [ ] v0.2.0 release 在 GitHub 上，brew install 装得上
- [ ] 公众号文章发出
- [ ] HN Show HN 帖出，self-comment 紧跟

## 不要做的

- 不要碰 W1-W7 业务代码（W8 是纯文档/营销周）
- 不要做语义 / embedding 搜索（V2）
- 不要做 aider / opencode 等其他 source（V2）
- 不要引入 Tailwind / Bootstrap / Vue / React / 任何 JS 框架
- 不要写中文叙事时编造作者个人故事（用 `[作者填: ...]` 占位）
- 不要在 Reddit / HN 草稿里编造虚假反响 / 假数据
- 不要把"sponsored by YCAPI"藏起来（透明度护盾，README 头一段 + landing footer + onboarding 三处至少都要有）
- 不要做 i18n（中文 README + 英文外站文，二者并存即可）
- 不要在 launch 文档里编造其他用户的 testimonial / 评价 / 引用（无中生有 = HN 死刑）
