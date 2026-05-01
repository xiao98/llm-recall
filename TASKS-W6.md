# W6 执行任务 — 营销注入：banner / footer / onboarding 同意流

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W5.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（W6 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### Goal（唯一目标）
让 **首次启动** 弹一次 onboarding 同意流，之后每次 TUI 启动顶栏出金句 banner（5% 概率含加群 CTA），搜索 footer 默认关可配开，stats 底部 sponsored 字符串受 `--no-promo` 统一控制。**透明度作护盾**：onboarding 文本明告所有营销点 + 一键关。

### 成功标准（criteria for success — 11 条）

1. 首次启动（`~/.config/llm-recall/onboarding-accepted` 不存在）渲染 onboarding 单屏：title / 4 条营销点说明 / 2 个开关说明（`--no-promo` / `config.toml`）/ `Enter` 接受 / `q` 退出
2. 按 Enter → 写 `~/.config/llm-recall/onboarding-accepted` 含 ISO timestamp + version → 落地后进入 TUI 主屏
3. 按 q → 不写文件 → exit 0 → 下次启动仍弹 onboarding
4. 第二次启动（accepted 文件存在）→ 不再弹 onboarding，直接进 TUI
5. TUI 顶栏 banner：每次启动随机一条金句（30 条池），**5% 概率**金句下追加 `→ 加入 YCAPI 群: <短链>` CTA 行（`crypto/rand` 取，跑 100 次 ratio 在 [0.02, 0.10] 区间）
6. 搜索 footer：默认关；`config.toml` 设 `[promo] search_footer = true` → 列表底部出 `🔍 YCAPI 群里有人在讨论「<query 第一个词>」 →` 行
7. `--no-promo` flag：banner / footer / stats sponsored 字符串**全部**不渲染（一处开关三个口）
8. config 文件位置：macOS/Linux `~/.config/llm-recall/config.toml`，Windows `%APPDATA%\llm-recall\config.toml`；不存在用 default；不可读 stderr warn 但不崩
9. `internal/promo/quotes.go` 含 30 条金句：subagent 用 WebFetch + WebSearch 抓 YCAPI 风格金句（详见 §步骤 3），凑齐 ≥ 20 条优先；不足用通用开发者金句补齐到 30；每条金句行尾注释数据源；抓取整体失败兜底全 30 条通用金句 + stderr warn
10. `go vet ./...` / `gofmt -l .` / `go test ./...` 全过；新加 `internal/promo/promo_test.go` 测：onboarding 状态机 / 5% CTA 概率范围 / `--no-promo` 三处全关 / quotes 加载
11. DEVDOC.md / 历史 TASKS-W*.md §0–§"不要做的"未被改

### 权限授权（最大权限，不询问）
执行期间 master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go run / go vet / go test / gofmt / git add / git commit` 全部放行
- **WebFetch** 抓取金句：最多 10 个 URL，5 分钟时间预算；起点 `https://api.youchun.tech`
- **WebSearch** 抓取金句：最多 5 个 query；用 §步骤 3 给的关键词
- 写 `~/.config/llm-recall/` 测试用文件（测完清理）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；动 W1-W5 业务代码（W6 是注入周，已有 TUI / stats 仅在 banner/footer 占位处接钩子，不重写主循环）；引入 cobra / viper / cgo；调用真 LLM API（W7 才做）；写图片 / 二进制资源到 binary（quotes 是字符串字面量，编译进 binary 即可）；把 onboarding-accepted 写到 `~/llm-recall/` 仓内（必须落用户 config 目录）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("w6-executor", prompt = §0 全文 + "完成本文件 §步骤 0–8")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 11 条命令逐项校验
        if 11 条全过:
            报告用户："W6 验收通过"，附 11 条实际输出（重点：onboarding 截屏、CTA 概率统计、--no-promo 三处全关验证）
            break
        else:
            spawn subagent("w6-executor", prompt += "上一轮在 <第 N 条> 失败，从该步继续")
    else:
        继续等待
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- onboarding 文本必须**逐字采用** §步骤 4 给的版本（这是策划方权威，不能"优化"措辞 —— 这关乎合规和用户感知）
- W1-W5 子 agent 留下的合理偏离保留，不要回滚（特别：DEVDOC §3 P0-1 实测补丁段、Session.Title 字段、stats heatmap 终端原生路线）

---

## 验收标准（先看这个）

```
$ rm -f ~/.config/llm-recall/onboarding-accepted
$ llm-recall
┌─ Welcome to llm-recall ────────────────────────────────────────┐
│ 跨厂商 LLM CLI 会话搜索 + 恢复终端工具                          │
│                                                                 │
│ Sponsored by YCAPI (https://api.youchun.tech)                          │
│ Homepage: https://recall.youchun.tech                           │
│                                                                 │
│ 营销注入说明（你看到的所有 YCAPI 痕迹）：                       │
│   • 启动时顶栏一条金句 banner，5% 概率含加群链接                │
│   • stats 命令底部一行 sponsored 字符串                         │
│   • （可选）搜索结果底部讨论关联条                              │
│   • gold 功能用你自己的 LLM API key，不走 YCAPI 网关            │
│                                                                 │
│ 关闭方式：                                                      │
│   --no-promo               关 banner / footer / sponsored       │
│   config.toml              细粒度调（详见 README）              │
│                                                                 │
│ Enter 接受继续， q 退出                                          │
└─────────────────────────────────────────────────────────────────┘

$ # 按 Enter
$ cat ~/.config/llm-recall/onboarding-accepted
{"accepted_at":"2026-05-15T10:23:45Z","version":"0.2.0"}

$ llm-recall    # 第二次直接进 TUI，顶栏出 banner
─────────────────────────────────────────────────────────────────
  💡  show, don't tell                                  YCAPI ❤
─────────────────────────────────────────────────────────────────
search: _
...

$ llm-recall --no-promo    # banner / footer / stats sponsored 全无
search: _
...

$ llm-recall stats
... heatmap + 4×2 panel ...
─────────────────────────────────────────────────────────────────
                                  llm-recall · sponsored by YCAPI
─────────────────────────────────────────────────────────────────

$ llm-recall stats --no-promo
... heatmap + 4×2 panel ...     # 末尾 sponsored 行不渲染
```

## 前置条件

- W5-rev1 commit `b8ed306` 之后的代码
- W4 后半周（push tag / brew install）状态不影响 W6 开发
- 用户**可选**已建 `~/llm-recall/quotes-draft.md`（30 条 YCAPI 群金句草稿）；未建则 subagent 自填占位

## 步骤

### 0. 项目元数据同步（一次性，写代码前做）

#### 0.1 写入项目主页 URL `https://recall.youchun.tech`：
1. `.goreleaser.yml` → `brews[0].homepage` 和 `scoops[0].homepage` 都改为 `"https://recall.youchun.tech"`
2. `README.md` → 顶部一句话定位下加一行 `Homepage: https://recall.youchun.tech`
3. `internal/promo/banner.go` → CTA 5% 概率显示行的 URL 用 `https://recall.youchun.tech`（不是 api.youchun.tech；CTA 落项目主页，主页再引导加群）

#### 0.2 全仓 ycapi.com → api.youchun.tech 替换

YCAPI 实际域名是 `api.youchun.tech`，旧 spec 用 `ycapi.com` 是占位。subagent 跑：

```bash
grep -rln 'ycapi\.com' . --include='*.go' --include='*.md' --include='*.yml' --include='*.yaml' --include='*.toml'
# 然后对每个命中文件做 ycapi.com → api.youchun.tech 替换
```

注意：本任务文档（TASKS-W6.md）和策划方文档（DEVDOC.md / 历史 TASKS-W*.md）已经被策划方手动改过，subagent **跳过这些文件**，只改业务代码 / README / goreleaser config / CHANGELOG 等。

### 1. config 模块（`internal/config/`）

```go
package config

type Config struct {
    Promo PromoConfig
}

type PromoConfig struct {
    NoPromo         bool    `toml:"no_promo"`         // 总开关，等价 --no-promo
    SearchFooter    bool    `toml:"search_footer"`    // 默认 false
    BannerFreq      float64 `toml:"banner_freq"`      // default 1.0
    CTAProbability  float64 `toml:"cta_probability"`  // default 0.05
}

// 加载顺序：default → config.toml → flag 覆盖
func Load(flagNoPromo bool) (*Config, error)
func ConfigPath() string  // mac/linux ~/.config/llm-recall/config.toml ; win %APPDATA%\llm-recall\config.toml
```

依赖：`go get github.com/BurntSushi/toml`（无 cgo，单文件解析器）。

### 2. promo 模块（`internal/promo/`）

```go
package promo

// quotes.go: var Quotes []string  (≥30 条，编译进 binary)

func Banner(cfg *config.Config) string         // 30 条池随机选 + 5% CTA；NoPromo=true 返回空
func SearchFooter(cfg *config.Config, query string) string  // 默认空；SearchFooter=true 渲染
func StatsFooter(cfg *config.Config) string    // "llm-recall · sponsored by YCAPI"；NoPromo=true 返回空

func OnboardingPath() string                    // ~/.config/llm-recall/onboarding-accepted
func OnboardingAccepted() bool                  // 文件存在即真
func WriteOnboardingAccepted(version string) error
```

CTA 概率：`crypto/rand` 取 0-99 整数，< 5 视为命中。**不要用 math/rand 默认源**（受 Go 版本影响 + 测试不可控；用 crypto/rand 后测试用 monkey-patch 接口注入种子）。

### 3. quotes 自动抓取（subagent 启动前一次性做）

用户不手写 quotes-draft.md。subagent 自己从公开 web 抓取 YCAPI 风格金句，时间预算 5 分钟。

#### 抓取流程

1. **WebFetch** 起点优先级：`https://recall.youchun.tech`（项目主页）→ `https://api.youchun.tech`（赞助方）→ 两者链接到的子页；总共最多 8 个 URL，单页 < 1MB；提取所有 slogan / 短句 / 标语类内容（一句话 ≤ 60 中文字符 / ≤ 120 英文字符；剔除导航 / 版权 / 错误页 / 法律条款 / 价格表）
2. **WebSearch** 关键词：`site:recall.youchun.tech`、`site:youchun.tech`、`"YCAPI" 中文 AI`、`YCAPI 实战派`、`site:api.youchun.tech`、用户公开 ID（如 `@xiao98 即刻` / `@xiao98 微博`）；最多 5 个外部 URL，每个最多挑 5 条
3. 候选去重 → 按以下打分排序，取 Top 20 进 YCAPI 风格池：
   - 是否短句（≤ 60 字符 +2）
   - 是否含 AI / LLM / 开发者文化关键词（+1）
   - 是否过度营销硬广（→ -3 删除）
   - 是否含 PII / 联系方式（→ 整条删除）

#### 凑数策略

- YCAPI 风格池 ≥ 20 条 → 用通用金句补齐到 30 条
- YCAPI 风格池 ＜ 20 条 → 抓到几条用几条，剩下全部用通用开发者金句（Linus / Knuth / Brooks / Carmack 等）兜底

通用金句池示例（subagent 自由发挥到所需数量）：
- "Talk is cheap. Show me the code." — Linus
- "Premature optimization is the root of all evil." — Knuth
- "Make it work, make it right, make it fast."
- "Simplicity is the ultimate sophistication."
- "First, solve the problem. Then, write the code." — Johnson

#### 写入 `internal/promo/quotes.go`

每条金句**注释标注数据源**：

```go
var Quotes = []string{
    "金句1",  // from api.youchun.tech/about
    "金句2",  // from <抓取 URL>
    "Talk is cheap. Show me the code.",  // generic: Linus
    ...
}
```

文件头注释：

```go
// Auto-fetched at W6 by subagent.
// YCAPI-style: N from <抓取来源汇总>
// Generic developer quotes: M (Linus/Knuth/Brooks/etc.)
// 用户审核后可手动替换为更贴切的 YCAPI 群真实金句。
```

#### 失败兜底

抓取超时 / 网络挂 / WebFetch 全 0 条 → 30 条全用通用金句 + stderr 一行 warn：
```
warn: 自动抓取 YCAPI 金句失败，已用 30 条通用开发者金句占位。后续可手动编辑 internal/promo/quotes.go。
```

### 4. onboarding 文本（**hardcode**，**逐字采用**）

```
┌─ Welcome to llm-recall ────────────────────────────────────────┐
│ 跨厂商 LLM CLI 会话搜索 + 恢复终端工具                          │
│                                                                 │
│ Sponsored by YCAPI (https://api.youchun.tech)                          │
│ Homepage: https://recall.youchun.tech                           │
│                                                                 │
│ 营销注入说明（你看到的所有 YCAPI 痕迹）：                       │
│   • 启动时顶栏一条金句 banner，5% 概率含加群链接                │
│   • stats 命令底部一行 sponsored 字符串                         │
│   • （可选）搜索结果底部讨论关联条                              │
│   • gold 功能用你自己的 LLM API key，不走 YCAPI 网关            │
│                                                                 │
│ 关闭方式：                                                      │
│   --no-promo               关 banner / footer / sponsored       │
│   config.toml              细粒度调（详见 README）              │
│                                                                 │
│ Enter 接受继续， q 退出                                          │
└─────────────────────────────────────────────────────────────────┘
```

边框用 lipgloss `Border(NormalBorder())`。中英混排宽度用 runewidth 算。

### 5. 主入口接 onboarding 检查（`cmd/llm-recall/main.go`）

```
进入流程伪代码：

main():
    cfg = config.Load(flagNoPromo)
    if onboarding 子命令 OR onboarding-accepted 不存在：
        accepted = runOnboarding()
        if not accepted: exit 0
        promo.WriteOnboardingAccepted(version)
    dispatch to ls / stats / TUI
```

`onboarding` 子命令显式启动（用户想重看）：`llm-recall onboarding`。

### 6. TUI banner 接钩子（`internal/tui/banner.go`）

W3 已有 `banner.go` 占位返回空字符串 → W6 改为：

```go
func Banner(cfg *config.Config) string {
    return promo.Banner(cfg)
}
```

TUI Init() 调用，渲染到顶栏（lipgloss horizontal divider 上方）。

### 7. TUI search footer 接钩子

`internal/tui/view.go` 的 list footer slot：

```go
if footer := promo.SearchFooter(cfg, model.query); footer != "" {
    rendered = append(rendered, footer)
}
```

### 8. stats footer 接钩子（`cmd/llm-recall/cmd_stats.go`）

W5-rev1 已有"sponsored by YCAPI"字符串 → 改成走 `promo.StatsFooter(cfg)`，受 `--no-promo` 控制。

### 9. `--no-promo` flag

主入口 `flag.BoolVar(&flagNoPromo, "no-promo", false, "...")`，传入 config.Load 覆盖 PromoConfig.NoPromo。

子命令 ls / stats / TUI 都接受这个 flag。

### 10. 测试 + 提交

`internal/promo/promo_test.go` 关键 case：
- onboarding 状态机：未 accepted → 调 WriteOnboardingAccepted → Accepted 返 true
- CTA 概率：调 Banner 1000 次，统计 CTA 行出现 ratio，应 ∈ [0.03, 0.07]
- `--no-promo` 三处全关：Banner / SearchFooter / StatsFooter 都返空字符串
- quotes 加载：mock quotes-draft.md 测有 / 无 / 不足 30 条三种路径

```
git add .
git commit -m "W6: marketing injection (banner/footer/onboarding) with --no-promo kill switch"
```

## 验收检查清单

- [ ] onboarding 首次启动渲染（截屏对照 §步骤 4 文本逐字一致）
- [ ] Enter 写 accepted 文件 + ISO timestamp + version
- [ ] q 退出不写文件 exit 0
- [ ] 第二次启动跳过 onboarding 直接进 TUI
- [ ] TUI 顶栏 banner（截屏含金句）
- [ ] CTA 概率统计：跑 1000 次 banner，ratio ∈ [0.03, 0.07]
- [ ] search footer 默认关；config 设 true 后开（截屏前后对照）
- [ ] `--no-promo` 三处全关（banner / search footer / stats footer 截屏验证）
- [ ] config 路径正确（mac/linux/win 至少在当前平台验）
- [ ] quotes.go ≥ 30 条；行尾注释标数据源；文件头注释汇总（YCAPI 风格 N 条 + 通用 M 条 + 抓取来源 URL 列表）；抓取失败时全通用 + stderr warn
- [ ] go vet / fmt / test 全过
- [ ] DEVDOC / 历史 TASKS 未改

## 不要做的（留给 W7+）

- 不要做 gold 命令 / card 命令（W7）
- 不要做 share / UTM 后端（已 cancel）
- 不要做 LLM API 调用（W7 BYOK）
- 不要做语义 / embedding 搜索（V2）
- 不要重写 W3 TUI 主循环或 W5 stats 主 logic（仅在已暴露的 banner/footer 钩子处接入）
- 不要引入图标 / emoji 字体素材（终端 ASCII / utf-8 即可，💡 ❤ 🔍 等基础 emoji OK，依赖系统 emoji 字体的复杂图标不要）
- 不要把 onboarding 文本写成 i18n 资源文件（W6 hardcode 中文 + 英文混排即可，国际化 V2）
