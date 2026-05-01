# 我开了个跨厂商 LLM CLI 会话搜索工具，单 Go 二进制，BYOK，无 telemetry

> 公众号草稿。发布前需要：① 填掉 3 处 `[作者填: ...]` 占位；② 在 4 处 `[图: ...]` 位置贴对应截图（GIF 压成单帧 PNG，公众号不支持动图自动播放）；③ 标题可在公众号编辑器里再调。

---

[作者填: 开篇 1-2 句个人化引子。例如"上个月我同时打开 Claude Code、Codex、Gemini 三个终端，想找一周前我让 Claude 改过的那段 SQL，结果三家 CLI 各搜一遍才翻到"——这一段是公众号"为什么我做这个"的钩子，要真实，不要用力。]

## 痛点

如果你和我一样同时在用两家以上 LLM CLI 工具——Claude Code 写代码、Codex 跑脚本、Gemini 大模型对比、aider 改库——你应该熟悉这套尴尬：

- **会话散在三处目录**：`~/.claude/projects/`、`~/.codex/sessions/YYYY/MM/DD/`、`~/.gemini/tmp/<shortid>/chats/`，每家一套 jsonl schema，搜历史得各家 CLI 各搜一次。
- **resume 语义不一致**：Claude `claude --resume <id>`、Codex `codex resume <id>`、Gemini `gemini --resume` 不接受 UUID 只接受 `latest` 或整数 index（gemini-cli 上游 #20480 / #23489 还没修），交互式 `/chat resume` 兜底。
- **没有跨家全局视图**：上周写了什么、本月用哪家最多、最长一次会话在哪天，三家 CLI 都没有原生 stats。

[作者填: 这里说一段你的个人痛点叙述，2-3 段。比如"我试过手写 fzf + jq pipeline，但 Codex 的 jsonl 头部 metadata 格式跟 Claude 不一样，Gemini 还有双格式问题（旧 .json 单对象 / 新 .jsonl）。维护起来比写一个工具还累。"——这段决定文章可信度，必须是你真实经历，不是 GPT 编的。]

## 解法：llm-recall

一个跨厂商 LLM CLI 会话搜索 + 恢复终端工具，fzf 风格，单 Go 二进制，跨平台，零依赖。

四个核心命令：

### 1. `llm-recall` — TUI 实时模糊搜索

[图: tui demo —— 输入框筛选三家会话 + 右侧预览面板]

输入即筛，多关键词 AND，中文按 unicode 字处理。↑↓ 选中右侧实时预览原文 + 命中片段高亮。回车直接 resume：自动 `cd` 到原 cwd 再调对应 CLI 的 `--resume`。

### 2. `llm-recall stats` — 贡献日历

[图: stats heatmap —— GitHub 风格 7 行贡献日历 + 4×2 stats 面板]

GitHub-style 贡献日历，`⋅ ▒ ▓ █` 四档色阶，lipgloss 24-bit 色（accent `#FF6B35`）。`1/2/3` 切 All time / 7 days / 30 days。4×2 面板含 Sessions / Total tokens / Active days / Longest streak / Favorite source / Longest session / Most active day / Current streak。**纯本地聚合，无任何网络出口**。

### 3. `llm-recall gold` — LLM 抽 Top 10 金句

[图: gold demo —— lipgloss 圆角边框 + 编号 + quote 高亮]

默认扫 7 天会话（`--days N` 覆盖），单次 LLM 调用挑出你说过的 Top 10 金句 + 一句话点评。total > 100KB 自动 sample 50 会话。**BYOK**：调你自己的 `ANTHROPIC_API_KEY` 或 `OPENAI_API_KEY`，对话内容不经任何中转。`--md` 输出纯 markdown 给 pipe。

### 4. `llm-recall card <session-id>` — 单会话名片

[图: card demo —— lipgloss 圆角卡片，session 头 + 首条 + LLM 总结 + cwd]

lipgloss 圆角卡片：会话头 + 首条用户消息（截 200 字）+ LLM 一句话总结（≤50 字）+ cwd。截屏发朋友圈传播友好。

## 怎么用

```bash
# macOS / Linux
brew install xiao98/tap/llm-recall

# Windows
scoop bucket add xiao98 https://github.com/xiao98/scoop-bucket
scoop install llm-recall

# 任何平台
go install github.com/xiao98/llm-recall/cmd/llm-recall@latest
```

首次启动会进 onboarding 同意流（一次性），按 Enter 接受。然后三家 CLI 的会话就被自动扫到了，敲 `llm-recall` 进 TUI 即可。

## 隐私 & 营销透明

工具由 YCAPI（<https://api.youchun.tech>，LLM API 中转网关）赞助。透明度声明保留三处可见：onboarding 同意流 / README 头一段 / landing footer。营销注入的全部位置：

- 启动 banner 一行金句，5% 概率含加群链接
- `stats` 命令底部一行 sponsored 字符串
- 可选的搜索结果底部讨论关联条（**默认关**）

`--no-promo` 一刀切关全部，或 `~/.config/llm-recall/config.toml` `[promo]` 段细粒度调。

**不上传任何对话内容**：

- 索引：本地 SQLite，落系统 cache 目录
- stats：纯本地聚合
- gold / card：调你自己配的 LLM endpoint（默认 Anthropic / OpenAI 官方），调用前 5 类 PII 正则脱敏（API key / OAuth token / email / 手机 / IPv4）+ token 估算 cost confirm + 7 天结果缓存
- 无 telemetry，无 crash report，无"匿名使用统计"

## 一些技术细节

- **Go 1.22 + Charm bubbletea**：单二进制，跨平台，TUI 生态成熟
- **modernc.org/sqlite 纯 Go**：无 cgo，分发零依赖
- **适配器架构**：`SessionAdapter` 接口 + 可选 `FileLister` / `FileParser` 增量子接口，新加一家 CLI 只要实现接口 + 提交头部 schema 样本
- **mtime-based 增量缓存**：Codex / Gemini 文件按 mtime 增量扫，Claude 是扁平目录全扫
- **LLM 调用**：BYOK，vendor 自动检测（`ANTHROPIC_API_KEY` 优先 → `OPENAI_API_KEY` 兜底），可 config 走中转

代码：<https://github.com/xiao98/llm-recall>，主页：<https://recall.youchun.tech>。

## 已知限制

- Gemini `--resume <UUID>` 上游不接受，工具退化为交互式提示用户 `/chat resume <id>`
- 暂未支持 aider / opencode / cline，欢迎 PR adapter（`internal/adapter/` 看 claude.go 抄一份即可）

## 结语

[作者填: 个人化 closing。例如"做这个工具的初衷其实就是给自己用，意外发现 stats 截屏发朋友圈很多人来问。如果你也同时用两家以上 LLM CLI，欢迎试试，issue / PR 都欢迎"——这段是和读者建立连接的最后一段，要真实，避免"如果对你有帮助希望点个 star"这种油腻话术。]

---

主页：<https://recall.youchun.tech>
代码：<https://github.com/xiao98/llm-recall>

> 加入讨论 / 反馈 issue：访问主页扫码或直接 GitHub Issues。
