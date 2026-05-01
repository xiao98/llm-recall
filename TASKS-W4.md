# W4 执行任务 — 分发流水线 + Dogfood 周

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W3.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（W4 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### W4 与前三周的差异

W4 分两段：
- **前半周（subagent 主导）**：分发流水线就绪 —— `.goreleaser.yml` / `.github/workflows/release.yml` / README v1 / CHANGELOG / DOGFOOD 模板，跑通 `goreleaser release --snapshot --clean` 本地验证
- **后半周（用户主导）**：推 tag 触发真发版 + 实战用一周 + 写 dogfood 日志

§0 双 agent 循环**仅覆盖前半周**。后半周用户手动推进，subagent 不再值守。

### Goal（前半周）
让 `goreleaser release --snapshot --clean` 在本机跑成功，产出 5 个平台 binary archive，所有分发配置 + 文档就绪。**不推 tag、不真发版**——那是用户的事。

### 成功标准（criteria for success — 仅前半周）
本文件末尾"验收检查清单 A 段"9 条全部通过，以**实际命令输出**为准（不是 subagent 自述）：

1. `.goreleaser.yml` 存在，`goreleaser check` 无报错
2. `goreleaser release --snapshot --clean` 跑成功，`dist/` 下含 5 份 archive：`darwin_amd64` / `darwin_arm64` / `linux_amd64` / `linux_arm64` / `windows_amd64`（archive 后缀：mac/linux 是 `.tar.gz`，windows 是 `.zip`）
3. `dist/` 已加入 `.gitignore`，仓内无 `dist/` 提交痕迹
4. `.github/workflows/release.yml` 存在且 YAML 合法（`python -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` 通过），含 `goreleaser/goreleaser-action@v6` 引用
5. `README.md` 已升级为 v1：含三平台安装命令 / 用法（TUI / ls / `--no-dry-run`）/ 隐私占位区块（"W6 完整版"标注）/ 截图占位
6. `CHANGELOG.md` 含 `v0.1.0` entry，列 W1-W4 主要变更（按 `### Added/Changed/Fixed` 分组）
7. `DOGFOOD.md` 模板就绪：含 7 天结构 + 1 条 W4 boot entry（subagent 自填，证明流水线打通）
8. `go vet ./...` 无报错；`gofmt -l .` 无输出；`go test ./...` 全过
9. DEVDOC / 历史 TASKS-*.md §0–§"不要做的"未被改

### 权限授权（最大权限，不询问）
执行期间，master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除
- `go mod / go build / go install / go run / go vet / go test / gofmt / git init / git add / git commit / goreleaser` 全部放行
- `go install github.com/goreleaser/goreleaser/v2@latest` 装 goreleaser 工具（如本机未装）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端（**包括不要推 tag**）；`rm -rf` 跨出 `~/llm-recall/`；引入 cobra / viper / 任何 cgo 依赖；写营销 banner / 水印 / onboarding 任何代码（W6 才做）；改 W1-W3 已交付的业务代码（W4 是纯分发周，业务零改动）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("w4-executor", prompt = §0 全文 + "完成本文件 §步骤 1–7")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 9 条命令逐项校验
        if 9 条全过:
            报告用户："W4 前半周验收通过，后半周交还用户"，附 9 条实际输出
            break
        else:
            spawn subagent("w4-executor", prompt += "上一轮在 <第 N 条> 失败，实际输出为 <…>，从该步继续")
    elif subagent 卡死/崩溃:
        spawn subagent("w4-executor", prompt += "前一个 subagent 在 <最后步骤> 中断，从此处继续")
    else:
        继续等待

# 唯一退出条件：9 条全过，或用户从外部手动停止
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- 完成后回传**实际命令输出**而非"我已完成"
- W1-W3 子 agent 留下的合理偏离保留，不要回滚（特别注意：DEVDOC §3 P0-1 "已纳入官方的实测补丁" 段是策划方权威，**禁止 git checkout / git revert 还原**——W3 出过这个事故）

---

## 验收标准（先看这个）

```
$ goreleaser check
  • loading                                    path=.goreleaser.yml
  • config is valid

$ goreleaser release --snapshot --clean
  ...
  • building binaries
  • archives
  • calculating checksums
  • storing release metadata
  • release succeeded after 23s

$ ls dist/
  llm-recall_0.1.0-snapshot_darwin_amd64.tar.gz
  llm-recall_0.1.0-snapshot_darwin_arm64.tar.gz
  llm-recall_0.1.0-snapshot_linux_amd64.tar.gz
  llm-recall_0.1.0-snapshot_linux_arm64.tar.gz
  llm-recall_0.1.0-snapshot_windows_amd64.zip
  checksums.txt
  ...
```

## 前置条件

- W3 commit `15fbd42` 之后的代码（TUI + resume launcher 闭环）
- 用户已手动验过 `--no-dry-run` 选 claude session 真启动
- 用户已在 GitHub 建空仓 `xiao98/llm-recall` / `xiao98/homebrew-tap` / `xiao98/scoop-bucket`（可选；本周 subagent 不 push，但 release.yml 引用这些仓的名字必须正确）

## 步骤

### 1. 装 goreleaser

```
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser --version   # 期望 v2.x
```

如本机已装跳过。

### 2. `.goreleaser.yml`

放在仓根。完整模板：

```yaml
version: 2

project_name: llm-recall

before:
  hooks:
    - go mod tidy

builds:
  - id: llm-recall
    main: ./cmd/llm-recall
    binary: llm-recall
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: llm-recall
    ids: [llm-recall]
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - LICENSE*
      - README.md
      - CHANGELOG.md

checksum:
  name_template: "checksums.txt"

snapshot:
  version_template: "{{ incpatch .Version }}-snapshot"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "^Merge pull request"

brews:
  - name: llm-recall
    repository:
      owner: xiao98
      name: homebrew-tap
      branch: main
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/xiao98/llm-recall"
    description: "Cross-vendor LLM CLI session search & resume TUI"
    license: "MIT"
    install: |
      bin.install "llm-recall"
    test: |
      system "#{bin}/llm-recall version"

scoops:
  - name: llm-recall
    repository:
      owner: xiao98
      name: scoop-bucket
      branch: main
      token: "{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}"
    homepage: "https://github.com/xiao98/llm-recall"
    description: "Cross-vendor LLM CLI session search & resume TUI"
    license: MIT
```

跑 `goreleaser check` 验证。**字段名以 goreleaser v2 当前文档为准**——若 check 报错，按报错调整。常见 v2 改名：
- `format_overrides[].format` → `format_overrides[].formats: [zip]`
- `archives[].builds` → `archives[].ids`
- 其他差异以 `goreleaser check` 输出为准

### 3. `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_BUCKET_GITHUB_TOKEN: ${{ secrets.SCOOP_BUCKET_GITHUB_TOKEN }}
```

YAML 合法性 + secrets 名字一致即可。这两个 secret 由用户在 GitHub 仓 Settings 配（W4 后半周用户的事）。

### 4. `README.md` v1

替换 W1 占位版。结构：

```markdown
# llm-recall

跨厂商 LLM CLI 会话搜索 + 恢复终端工具。一个命令找回任意 Claude Code / Codex / Gemini 历史会话并直接进入。

> Sponsored by [YCAPI](https://ycapi.com)（详见 [Privacy & Promo](#privacy--promo) 段；W6 起会有 onboarding 一次同意流）

## Install

### macOS / Linux (Homebrew)
brew install xiao98/tap/llm-recall

### Windows (Scoop)
scoop bucket add xiao98 https://github.com/xiao98/scoop-bucket
scoop install llm-recall

### From source
go install github.com/xiao98/llm-recall/cmd/llm-recall@latest

## Usage

llm-recall                    # 进 TUI，输入即筛三家会话
llm-recall ls --all           # 列出所有会话（CLI dump）
llm-recall ls --source codex  # 只列 codex
llm-recall --no-dry-run       # TUI 选中后真启动子进程进入会话

## Supported sources (W4)

claude / codex / gemini —— 自动扫 `~/.claude/`、`~/.codex/sessions/`、`~/.gemini/tmp/*/chats/`，无需配置。

## Privacy & Promo

W4 阶段：仅本地读 jsonl，不上传任何对话内容到任何后端。

W6 起会加：启动 banner / 生图水印 / `gold` 命令（BYOK 调用你自己的 LLM key）。届时首次启动会有一次性同意流，可用 `--no-promo` 关 banner / footer。详见 DEVDOC §4。

## License

MIT

## Screenshot

(W7 出截屏 GIF；W4 占位)
```

### 5. `CHANGELOG.md`

```markdown
# Changelog

All notable changes documented here, [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) style.

## [0.1.0] - 2026-05-XX

### Added
- W1: 项目骨架 + Claude adapter (`~/.claude/projects/*/*.jsonl`)；`llm-recall ls` 列会话
- W2: Codex adapter (`~/.codex/sessions/`) + Gemini adapter（双格式 .json/.jsonl + cwd fallback 链）+ SQLite 增量缓存（modernc.org/sqlite，纯 Go，DB 落系统 cache 目录）
- W2: ls 命令 `--no-cache` / `--source <name>` flag
- W3: TUI 实时搜索（bubbletea + lipgloss + sahilm/fuzzy；多词 AND；中文 unicode 处理；命中片段预览高亮）
- W3: Resume launcher（claude / codex 直接 `--resume <id>`；gemini 退化为交互式提示用户 `/chat resume <id>`；跨平台 Unix syscall.Exec / Windows cmd.Run）
- W3: cache schema v2，sessions 表加 `body` 字段（拼接所有用户消息，截断到 64KB）
- W4: goreleaser 分发流水线（mac/linux/win × amd64/arm64）；Homebrew tap + Scoop bucket 自动发布；GitHub Actions release workflow

### Fixed
- W2: Title 含 `\n` `\r` `\t` 破坏 tabwriter 列对齐 → CleanTitle 清洗
- W3: Codex 顶部"会话"实为 `<environment_context>` / `[Imported from Claude]` CLI 注入伪用户消息 → 过滤
- W3: Gemini cwd fallback 链 metadata.json > workspace.json > .project_root > 留空+title 标记
- W3: 时间解析容忍 .NET 7 位 fractional（`2026-04-25T10:55:00.0000000Z`）

### Known limitations
- Gemini resume 仅支持交互式 `/chat resume <id>`，CLI flag 不接受 UUID（gemini-cli 上游 #20480 / #23489）
- TUI 截图 / GIF / landing 留 W7+
```

### 6. `DOGFOOD.md`

```markdown
# llm-recall — Dogfood Log

> W4 后半周 + W5 用户实战周。每日 ≥ 1 条 entry。一周 ≥ 5 条进入 W5。

## Format

```
[YYYY-MM-DD][bug | impr | obs] <短描述> -- <复现/上下文> -- <action: fix-now | W5+ | wontfix>
```

## Entries

- [2026-05-XX][obs] W4 boot entry: `goreleaser release --snapshot --clean` 本地跑通，5 个 archive 就位；release.yml YAML 合法。等用户配 GitHub Secrets + 推 tag v0.1.0 触发真发版。 -- action: 用户手动

(以下由用户填)

- [ ][bug | impr | obs] ...
```

### 7. `.gitignore` 补 dist/ + goreleaser

在已有 .gitignore 末尾加：

```
dist/
```

### 8. 跑通本地 snapshot

```
goreleaser check
goreleaser release --snapshot --clean
ls -la dist/
```

确认 §成功标准 #2 列出的 5 个 archive 全在。

### 9. 提交

```
git add .
git commit -m "W4: goreleaser pipeline + brew/scoop tap + README v1 + dogfood log"
```

## 验收检查清单 A 段（subagent 完成 → master 亲验）

- [ ] `goreleaser check` 通过
- [ ] `goreleaser release --snapshot --clean` 完成，dist/ 含 5 份 archive
- [ ] dist/ 在 .gitignore，仓内无大文件
- [ ] `.github/workflows/release.yml` YAML 合法且引用 goreleaser-action@v6
- [ ] README.md v1 含三平台安装命令 + Usage + Privacy & Promo 段
- [ ] CHANGELOG.md v0.1.0 entry 列 W1-W4 变更
- [ ] DOGFOOD.md 模板 + W4 boot entry 就绪
- [ ] `go vet ./...` / `gofmt -l .` / `go test ./...` 全过
- [ ] DEVDOC.md / TASKS-W1/2/3.md 未改

## 验收检查清单 B 段（W4 后半周 + W5 用户做）

不在 subagent 验收范围。用户日历项：

- [ ] 在 GitHub 仓 `xiao98/llm-recall` Settings → Secrets 配 `HOMEBREW_TAP_GITHUB_TOKEN` 和 `SCOOP_BUCKET_GITHUB_TOKEN`（GitHub Personal Access Token，需要对应 tap/bucket 仓的 `contents: write` 权限）
- [ ] `git remote add origin git@github.com:xiao98/llm-recall.git && git push -u origin main`
- [ ] `git tag v0.1.0 && git push origin v0.1.0` 触发首次发版
- [ ] Actions 跑完后，干净环境（或同机器换路径）测 `brew install xiao98/tap/llm-recall` 装上能跑
- [ ] 一周里每日填 ≥ 1 条 DOGFOOD entry
- [ ] 周末把 dogfood 日志回传给策划方，以决定 W5（生图后端 + stats）是否照原计划，或要先 backlog 修 dogfood 暴露的 P0 bug

## 不要做的（留给 W5+ 或用户）

- 不要推 tag / 真发 release（用户手动）
- 不要在 GitHub 上配 Secrets（用户手动）
- 不要碰 W1-W3 的业务代码（W4 是纯分发周，业务零改动）
- 不要做营销 banner / 水印 / onboarding（W6）
- 不要做 stats / card / gold（W5/W7）
- 不要做 aider / opencode 等其他 source（V2）
- 不要引入 cgo 依赖
