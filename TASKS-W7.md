# W7 执行任务 — gold（金句挖掘）+ card（一键出卡） · BYOK 终端原生

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W6.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（W7 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Goal（唯一目标）
让 `llm-recall card <session-id>` 和 `llm-recall gold` 两个 LLM 命令工作，全 BYOK（用户自己 ANTHROPIC_API_KEY / OPENAI_API_KEY），输出**终端原生**（lipgloss 卡片 / markdown 列表），用户截屏即传播。无后端调用、无 PNG 渲染（保持 W5-rev1 终端原生路线）。

### 成功标准（criteria for success — 14 条）

1. `llm-recall card <session-id>` 工作：拿 cache 中的 session → 脱敏后调 LLM → 渲染 lipgloss ASCII 卡片到 stdout（含会话首条用户消息 + LLM 一句话总结 + 时间 + cwd）
2. `llm-recall gold` 工作：扫 N 天（默认 7，`--days N`）所有会话 → 单次 LLM 调用 → 输出 Top 10 金句 + 一句话点评，markdown 风格列表带颜色高亮
3. BYOK 探测顺序：`ANTHROPIC_API_KEY` 优先 → `OPENAI_API_KEY` 兜底 → 都没有友好错误引导：`set ANTHROPIC_API_KEY=sk-ant-... or OPENAI_API_KEY=sk-...`，退出码 2
4. `--vendor <anthropic|openai>` flag 强制选；`--model <id>` flag 覆盖默认 model（default: anthropic 用 `claude-haiku-4-5-20251001`，openai 用 `gpt-4o-mini`）
5. `--llm-base-url <url>` flag：力用户的 escape hatch（如指向 `https://api.youchun.tech/v1`），不影响默认（默认走各家官方 endpoint）
6. **PII 脱敏**：调 LLM 前扫 prompt 内容，正则命中 API key（`sk-[a-zA-Z0-9]{20,}` / `gho_*` / `xoxb-*`）/ 邮箱 / 大陆手机号 / IPv4 → 全部替换为 `<redacted>`，stderr 一行 `redacted N item(s) before LLM call`
7. **调用前 confirm**：card / gold 在真发请求前打印 token 估算 + 预估成本（按 model pricing 表，hardcode 当前价格） + 提示 `[y/N]`；`-y` flag 跳过；估算用 `len(prompt)/4` 简化（不引 tiktoken）
8. **结果缓存**：`~/.cache/llm-recall/llm-cache/<sha256(model+prompt)>.json` 7 天 TTL，命中直接返。`--no-cache` 强刷
9. `--md` flag（仅 gold）：输出纯 markdown 列表给 pipe（自动化用）；不带 lipgloss 颜色 / 边框 / footer
10. `--no-promo` 时 card / gold 输出底部 sponsored 字符串不渲染（W6 promo.StatsFooter 同款机制）
11. **调用失败容忍**：rate limit (429) / timeout / 4xx / 5xx → 友好错误（含 vendor + status + 简短建议）+ 退出码 1，不 panic
12. `go vet ./...` / `gofmt -l .` / `go test ./...` 全过；新加 `internal/llm/llm_test.go` 含 PII 脱敏 + cache 命中 + vendor 探测三类单测
13. `README.md` 含 `## Configuration` 段：`[llm]` config.toml 完整示例 + 中转用法（用 `https://dash.youchun.tech/v1` 当**真实** base_url 示例，不要写 placeholder URL）
14. DEVDOC.md / 历史 TASKS-W*.md §0–§"不要做的"未被改

### 权限授权（最大权限，不询问）
执行期间 master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go run / go vet / go test / gofmt / git add / git commit` 全部放行
- **真调 LLM API**：仅用于 e2e 验收。优先用 mock server（`net/http/httptest` 起本地 mock）跑 unit + integration 测试；只有最终 §成功标准 1/2 验收时**真调一次**确认 e2e 通；用户的 API key 通过 env var 传入（subagent 自己不写 key 到文件）。**单次 e2e 不超过 0.10 USD 成本**（gold 用 haiku/mini default，约 0.01 USD；card 单次约 0.001 USD）
- 写 `~/.cache/llm-recall/llm-cache/` 测试用文件（测完清理 / 留 cache 给后续命令复用）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；动 W1-W6 业务代码（W7 是新增 `card` / `gold` 命令 + `internal/llm/`，已有模块仅在新命令钩子处接入，不重写）；引入 cobra / viper / cgo / tiktoken（token 估算用 len/4 简化）；引入 PNG 渲染依赖（如 fogleman/gg）；调用 YCAPI 网关或任何中转（默认走官方 endpoint，用户可用 `--llm-base-url` 自己指）；把 API key 写入任何文件 / commit / log

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("w7-executor", prompt = §0 全文 + "完成本文件 §步骤 0–8")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 14 条命令逐项校验
        for §1, §2 真调 e2e：用 mock 模式（LLM_RECALL_LLM_MOCK=1 走 fake response）通过即视为通过；
        真 LLM e2e 由用户在 W7 收尾时自己跑一次（避免 master 重复消耗 token）
        if 14 条全过:
            报告用户："W7 验收通过"，附 13 条实际输出 + 让用户用自己 key 跑一次真 e2e
            break
        else:
            spawn subagent("w7-executor", prompt += "上一轮在 <第 N 条> 失败，从该步继续")
    else:
        继续等待
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- LLM mock：在所有测试中默认走 mock；真调用仅 §验收 1/2 一次
- W1-W6 子 agent 留下的合理偏离保留，不要回滚

---

## 验收标准（先看这个）

```
$ llm-recall gold --days 7 -y
扫描 32 个会话, 估算 token: 11200 input / ~600 output, 预估成本: $0.012 USD (claude-haiku-4-5)
调用 anthropic claude-haiku-4-5-20251001...
redacted 2 item(s) before LLM call

╭─ 你的 7 天金句 Top 10 ─────────────────────────────────╮
│  1.  做事都搞到一半就停                                │
│      → 半成品=收益最大化位置（自我观察）               │
│                                                         │
│  2.  AI 是这一代人的电力                               │
│      → 框架性判断，传播力强                            │
│                                                         │
│  ...                                                    │
│                                                         │
│ 10.  规约的演进只能由策划方 Edit 增量                  │
│      → 工程纪律的元规则                                │
╰─ llm-recall · sponsored by YCAPI ──────────────────────╯

$ llm-recall card 26348a6c-154a-4efc-958b-bee80e8a4bdc -y
╭─ session 26348a6c · claude · 2026-05-01 12:17 ─────────╮
│                                                         │
│ "claude code的历史会话管理太垃圾了..."                 │
│                                                         │
│ 在做：抱怨 claude code 历史搜索差劲，开搞 cli 工具     │
│                                                         │
│ cwd: ~/                                                 │
╰─ llm-recall · sponsored by YCAPI ──────────────────────╯

$ llm-recall gold --md > gold.md            # 纯 markdown，无颜色无边框
$ llm-recall gold --vendor openai --model gpt-4o
$ llm-recall card <id> --no-cache --no-promo
```

## 前置条件

- W6 commit `39aaf71` 之后的代码
- 用户已自行 commit `chore: rename ycapi.com → api.youchun.tech in spec docs`（不影响开发）
- 用户至少有 ANTHROPIC_API_KEY 或 OPENAI_API_KEY 之一在 env（W7 e2e 时用）

## 步骤

### 1. LLM 抽象（`internal/llm/`）

#### 1.0 config.toml 扩展

W6 的 `internal/config/` 加 `LLMConfig`（与 `PromoConfig` 平级）：

```toml
[llm]
vendor   = ""                              # "anthropic" | "openai" | ""（空=按 env 自动探测；ANTHROPIC_API_KEY 优先）
model    = ""                              # "" = vendor 默认（haiku-4-5 / 4o-mini）
base_url = ""                              # "" = 官方 endpoint；用户中转填如 "https://dash.youchun.tech/v1"
```

```go
type Config struct {
    Promo PromoConfig
    LLM   LLMConfig
}

type LLMConfig struct {
    Vendor  string `toml:"vendor"`
    Model   string `toml:"model"`
    BaseURL string `toml:"base_url"`
}
```

**优先级（高 → 低）**：CLI flag（`--vendor` / `--model` / `--llm-base-url`） > env（`ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `LLM_RECALL_BASE_URL`） > config.toml `[llm]` 段 > hardcoded default（vendor 走 env 探测；model 按 vendor；base_url 走官方 endpoint）。

**API key 永远不写 config.toml**（只走 env var，防 commit 泄露）。subagent 加载 config.toml 时如发现 `api_key` / `key` 等字段直接 stderr warn 并忽略。

#### 1.1 抽象本体

```go
package llm

type Vendor string
const (
    Anthropic Vendor = "anthropic"
    OpenAI    Vendor = "openai"
)

type Client interface {
    Vendor() Vendor
    Model() string
    Complete(ctx context.Context, req Request) (Response, error)
}

type Request struct {
    System    string
    Prompt    string
    MaxTokens int
    BaseURL   string  // 空 = 官方 endpoint
}

type Response struct {
    Text       string
    InputToks  int
    OutputToks int
}

func DetectKey() (vendor Vendor, key string, err error)  // ANTHROPIC_API_KEY > OPENAI_API_KEY
func NewClient(vendor Vendor, key string, model string, baseURL string) Client
```

文件：
- `internal/llm/types.go`
- `internal/llm/anthropic.go`：调 `https://api.anthropic.com/v1/messages`（或 baseURL）
- `internal/llm/openai.go`：调 `https://api.openai.com/v1/chat/completions`（或 baseURL）
- `internal/llm/detect.go`：DetectKey 实现
- `internal/llm/cache.go`：sha256 key + JSON 落盘 + 7d TTL
- `internal/llm/redact.go`：PII 正则 + 脱敏

mock 支持：`LLM_RECALL_LLM_MOCK=1` 环境变量下，所有 client 走本地 fake response（`internal/llm/mock.go`，返一段 hardcoded JSON 的 gold / 一句话 card）。

### 2. PII 脱敏（`internal/llm/redact.go`）

正则集合（按顺序扫，命中替换为 `<redacted>`）：

```go
var redactPatterns = []*regexp.Regexp{
    regexp.MustCompile(`sk-[a-zA-Z0-9-_]{20,}`),                    // OpenAI / Anthropic API key
    regexp.MustCompile(`sk-ant-[a-zA-Z0-9-_]{20,}`),                // Anthropic
    regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),                      // GitHub OAuth
    regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                      // GitHub Personal
    regexp.MustCompile(`xoxb-[a-zA-Z0-9-]{50,}`),                   // Slack bot
    regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),  // email
    regexp.MustCompile(`\b1[3-9]\d{9}\b`),                          // CN mobile (11 digits)
    regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),              // IPv4
}

func Redact(s string) (clean string, count int)
```

count 报给调用方 → stderr `redacted <N> item(s) before LLM call`（N=0 时不打印）。

### 3. Prompt 模板（`internal/llm/prompts/`）

#### card.go

```
SYSTEM:
You are a concise summarizer. Given a developer's chat with an LLM, produce ONE short sentence (≤ 50 chars, prefer action-oriented Chinese if input is Chinese, else English) describing what the user is doing in this session. Do NOT start with "用户在..." or "The user is...". Just say the action.

PROMPT:
Session content (脱敏后):
<<<
{body}
>>>

Output: just the sentence. No quotes, no markdown.
```

#### gold.go

```
SYSTEM:
You are a quote curator for a developer's LLM chat history. Pick the Top 10 most quotable lines from the user's own messages — opinions, sharp expressions, principle statements, or witty observations. Each ≤ 60 chars. Skip generic questions like "how do I..." or "what does X mean".

PROMPT:
Session bodies (脱敏后, 时间排序):
<<<
{bodies}
>>>

Output JSON array (no markdown wrapper):
[
  {"quote": "用户原话", "comment": "≤ 30 char 入选理由"},
  ...
]

Strictly 10 items. quote 必须来自上面的内容，不要编造。
```

### 4. card 命令（`cmd/llm-recall/cmd_card.go`）

```
llm-recall card <session-id> [-y] [--no-cache] [--vendor X] [--model X] [--llm-base-url X] [--no-promo]
```

逻辑：
1. 从 cache 拿 session（含 body）；找不到报错引导 `先跑 llm-recall ls 触发扫描`
2. body redact
3. 估算 token，打印预估成本，等 confirm（除非 -y）
4. 调 LLM cache 检查 → 命中直接渲染；未命中 → call → 写 cache
5. lipgloss 卡片渲染：title (`session <id8> · <source> · <time>`) + 首条用户消息（截 200 字）+ "在做：" + LLM 回复 + cwd 行 + footer (受 `--no-promo`)
6. stdout 输出（用户截屏）

### 5. gold 命令（`cmd/llm-recall/cmd_gold.go`）

```
llm-recall gold [--days N] [-y] [--md] [--no-cache] [--vendor X] [--model X] [--llm-base-url X] [--no-promo]
```

逻辑：
1. 从 cache 拿 sessions where `updated_at >= now - N days`（N default 7）
2. 每 session body 取 first 1KB（utf-8 安全截断）→ 用 `\n--- session <id> ---\n` 拼接
3. 总字符 ≥ 100KB（约 25K token）→ 警告 + 自动 sample（随机抽 50 会话）
4. redact 拼后总文本
5. 估算 token + cost，confirm
6. 调 LLM（System gold prompt）→ 拿 JSON array
7. 解析 JSON；解析失败：retry once with stricter "ONLY output valid JSON" 提示；二次失败报错
8. 渲染：
   - 默认：lipgloss 圆角边框 + 编号 + quote 高亮 + comment 灰色
   - `--md`：纯 markdown 列表，无颜色无边框，pipe 友好

### 6. lipgloss 渲染（`internal/llm/render.go`）

card 卡片样式：
- 圆角边框（`RoundedBorder()`）
- 标题居左带横线连接边框
- 内容缩进 2，行间空 1
- footer 居右带横线连接

gold 列表样式：
- 同样圆角边框 + 标题"你的 N 天金句 Top 10"
- 每条编号占 4 字符（`  1. `）+ quote（白色加粗）+ 换行 + 缩进 6 + `→ ` + comment（灰色）

宽度：80 字符（终端宽度大于 80 时不撑满，居中或左对齐）。

### 7. 测试（mock 主导）

- `internal/llm/redact_test.go`：8 类正则全覆盖 + 边界（空字符串、纯 PII、混合）
- `internal/llm/cache_test.go`：写入 → 命中 → TTL 过期失效
- `internal/llm/detect_test.go`：env 三种状态（仅 anthropic / 仅 openai / 都没）
- `internal/llm/mock_test.go`：mock client 返预设 fixture
- `cmd/llm-recall/cmd_card_test.go`：mock 模式跑 card → 断言输出含预期字段
- `cmd/llm-recall/cmd_gold_test.go`：mock 模式跑 gold → 解析 JSON → 渲染断言

### 8. 提交

```
git add .
git commit -m "W7: gold + card commands (BYOK, terminal-native, lipgloss)"
```

## 验收检查清单

- [ ] `llm-recall gold -y` mock 模式工作，渲染 Top 10 列表（lipgloss 边框 + 编号）
- [ ] `llm-recall card <id> -y` mock 模式工作，渲染卡片
- [ ] BYOK 探测：env 仅 anthropic / 仅 openai / 都无 三种状态行为正确
- [ ] `--vendor` `--model` `--llm-base-url` flag 接通
- [ ] PII 脱敏：5 类正则全测过（API key / email / 手机 / IP / token），stderr warn 计数正确
- [ ] confirm prompt：默认弹问，`-y` 跳过
- [ ] cache 命中：第一次写 cache，第二次跳过 LLM 调用（mock 模式可观察 mock 计数）
- [ ] `--no-cache` 强刷
- [ ] `--md` gold 输出纯 markdown，pipe 友好
- [ ] `--no-promo` 卡片底部 sponsored 字符串不渲染
- [ ] 调用失败：mock 返 429 → 友好错误 + exit 1
- [ ] go vet / fmt / test 全过
- [ ] README.md `## Configuration` 段含 `[llm]` 完整示例（`https://dash.youchun.tech/v1` 真实示例）
- [ ] DEVDOC / 历史 TASKS 未改

W7 验收通过后，**策划方动作**（master agent 不做）：
- 更新 DEVDOC §3 P0-8 / P0-9 替换 ⚠️ stale 段为 W7 落实的最终 spec
- 起 W8 任务文档（README + landing + 公众号文 + Reddit/HN 发车）

## 不要做的（留给 W8 / V2）

- 不要做 share / UTM 后端（已 cancel）
- 不要做 PNG 渲染（fogleman/gg 不引）
- 不要做 Pillow 后端（W5-rev1 已 pivot）
- 不要做语义 / embedding 搜索（V2）
- 不要做 aider / opencode 等其他 source（V2）
- 不要做多语言（中英混排够用，i18n 资源文件 V2）
- 不要做 streaming（card / gold 一次返回；UX 不需要流式）
- 不要在 binary 里 bundle tiktoken / 真 token counter（len/4 估算够用，差 ±20% 不影响 confirm 决策）
- 不要在 prompt 模板里加"YCAPI 风格"等业务私货（保持 prompt 中性，让 LLM 客观判断）
- 不要把 prompt 模板做成可外部覆盖（i18n / 用户自定义 prompt 是 V2）
