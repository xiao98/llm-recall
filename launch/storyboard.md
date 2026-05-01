# Recording storyboard — 4 段 demo GIF

> 目标：用户照着这份脚本一段段录，每段 ≤ 60s，输出 4 个 GIF 落 `docs/screenshots/`：
> `stats.gif` / `tui.gif` / `gold.gif` / `card.gif`。
>
> 总耗时预算：30 分钟（含装工具）。

## 准备工作（只做一次）

### 装录制工具（任选其一）

**asciinema + agg**（推荐，转 GIF 控制力强）：

```bash
# macOS
brew install asciinema agg

# Linux (Debian/Ubuntu)
sudo apt install asciinema
cargo install --git https://github.com/asciinema/agg
```

**vhs**（Charm 家纯 Go 替代，用 `.tape` 脚本驱动；适合追求"每次录都一致"）：

```bash
# macOS
brew install vhs

# 任何平台 (Go)
go install github.com/charmbracelet/vhs@latest
```

### 终端设置（每次录前确认）

- 尺寸：**100 列 × 30 行**（iTerm2 Cmd+i / WezTerm `font_size = 14`）
- 字体：**JetBrains Mono** 或 **Cascadia Code**，14pt，无 ligature 也行
- 主题：dark（推荐 GitHub Dark / Catppuccin Mocha 等带 #FF6B35 友好色）
- 录前清屏：`clear`
- **隐藏 onboarding**：确认 `~/.config/llm-recall/onboarding-accepted` 已存在（不存在就先 `llm-recall onboarding` 跑一次）
- **隐藏 banner**：每段录制都加 `--no-promo` flag（除非你想录 banner 的样子；当前版本通过 flag 控制，无 env var）
- 录制前 dry-run 一次确认无 PII：`llm-recall card <id> --no-promo` 看输出，有 PII 别录

---

## §1 stats（30s）

**目标**：展示 GitHub 贡献日历 + 4×2 stats panel 的视觉冲击。

**录前**：清屏；确认本机至少有 7 天会话数据（无数据时 heatmap 全空，没看头）。

**asciinema 命令**：

```bash
asciinema rec demo-stats.cast --idle-time-limit 1.5 -t "llm-recall stats" -c "bash"
```

**vhs 替代**（写一个 `demo-stats.tape`）：

```tape
Output docs/screenshots/stats.gif
Set Theme "GitHub Dark"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 720
Type "llm-recall stats --no-promo" Sleep 500ms Enter
Sleep 3s
Type "2"  Sleep 2s
Type "3"  Sleep 2s
Type "1"  Sleep 1s
Type "q"
```

**逐字脚本**（asciinema 录时手动敲）：

```
0:00  $ clear
0:01  $ llm-recall stats --no-promo
0:03  [TUI 渲染：贡献日历 + 4×2 panel 全显]
0:08  按 2 → 切到 Last 7 days 视图
0:13  按 3 → 切到 Last 30 days 视图
0:20  按 1 → 切回 All time
0:25  按 q 退出
0:30  END
```

**转 GIF**（asciinema 路径）：

```bash
agg demo-stats.cast docs/screenshots/stats.gif \
  --speed 1.5 --theme github-dark --font-size 14 --cols 100 --rows 30
```

**验证**：`ls -lh docs/screenshots/stats.gif` < 5MB；浏览器打开能看到日历 + panel 切换。

---

## §2 TUI search（45s）

**目标**：展示 fuzzy 输入即筛 + 右侧预览高亮 + resume dry-run 输出。

**录前**：选好一个**真实存在**的关键词（建议从 `llm-recall ls --all | head -20` 里挑一个会话标题里的词）。下面用 `飞书` 占位，你换成你机器上有命中的词。

**asciinema 命令**：

```bash
asciinema rec demo-tui.cast --idle-time-limit 1.5 -t "llm-recall TUI" -c "bash"
```

**vhs 替代** (`demo-tui.tape`)：

```tape
Output docs/screenshots/tui.gif
Set Theme "GitHub Dark"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 720
Type "llm-recall --no-promo" Sleep 500ms Enter
Sleep 1500ms
Type "飞书"
Sleep 2s
Down  Sleep 800ms
Down  Sleep 800ms
Up    Sleep 1s
Enter Sleep 2s
```

**逐字脚本**：

```
0:00  $ clear
0:01  $ llm-recall --no-promo
0:03  [TUI 起，输入框聚焦，列表显示最新 20 条]
0:05  输入 "飞书"（替换为你的真实命中关键词）
0:09  [列表实时筛掉到 1-3 行]
0:13  ↓ 选第 2 行 → 右侧预览面板显示完整 body + 命中片段高亮
0:25  ↑ 回到第 1 行 → 预览切换
0:33  按 Enter
0:36  [stdout: → exec: claude --resume 26348a6c (cwd: /Users/.../proj)]
0:42  END
```

**转 GIF**：

```bash
agg demo-tui.cast docs/screenshots/tui.gif \
  --speed 1.3 --theme github-dark --font-size 14 --cols 100 --rows 30
```

**验证**：GIF < 5MB；能看到输入筛选 + 高亮 + resume dry-run 输出三段。

---

## §3 gold（30s）

**目标**：展示 LLM 抽 Top 10 金句的 lipgloss 圆角输出。

**重要**：本段需要**真实 LLM 调用**。subagent 不能代跑——必须用你自己的 `ANTHROPIC_API_KEY` 或 `OPENAI_API_KEY`。

**录前**：

1. 确认 env 有 key：`echo $ANTHROPIC_API_KEY | head -c 10`
2. **dry-run 看输出无 PII**：`llm-recall gold --days 7 -y --no-promo --no-cache | less`，确认没有 email / 手机号 / API key 漏网（PII 脱敏不是 100% 的）
3. 确认无 PII 后，删 cache 让录制走真调用：`rm -rf ~/.cache/llm-recall/llm-cache/`

**asciinema 命令**：

```bash
asciinema rec demo-gold.cast --idle-time-limit 2 -t "llm-recall gold" -c "bash"
```

**vhs 替代** (`demo-gold.tape`)：

```tape
Output docs/screenshots/gold.gif
Set Theme "GitHub Dark"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 800
Type "llm-recall gold --days 7 -y --no-promo" Sleep 500ms Enter
Sleep 8s
```

**逐字脚本**：

```
0:00  $ clear
0:01  $ llm-recall gold --days 7 -y --no-promo
0:03  [stdout: scanning 7 days... N sessions, ~XK tokens, est cost $X.XX]
0:06  [LLM 调用 spinner / "calling anthropic..."]
0:14  [lipgloss 圆角边框输出 Top 10 + 编号 + comment]
0:25  END
```

**转 GIF**：

```bash
agg demo-gold.cast docs/screenshots/gold.gif \
  --speed 1.5 --theme github-dark --font-size 14 --cols 100 --rows 36
```

**验证**：GIF < 5MB；能看到圆角边框 + 10 条编号 + 灰色 comment 三层结构。

---

## §4 card（20s）

**目标**：展示单会话 lipgloss 圆角卡片。

**重要**：同 §3，需要真实 LLM 调用。

**录前**：

1. 挑一个**无敏感**的真实 session id：`llm-recall ls --all | head -5` 选一个标题不含 PII 的
2. dry-run 一次：`llm-recall card <short-id> -y --no-promo --no-cache`，确认输出无 PII
3. 删 cache：`rm -rf ~/.cache/llm-recall/llm-cache/`

**asciinema 命令**：

```bash
asciinema rec demo-card.cast --idle-time-limit 1.5 -t "llm-recall card" -c "bash"
```

**vhs 替代** (`demo-card.tape`)：

```tape
Output docs/screenshots/card.gif
Set Theme "GitHub Dark"
Set FontFamily "JetBrains Mono"
Set FontSize 14
Set Width 1200
Set Height 600
Type "llm-recall card 26348a6c -y --no-promo" Sleep 500ms Enter
Sleep 5s
```

**逐字脚本**（替换 `26348a6c` 为你选的 short id）：

```
0:00  $ clear
0:01  $ llm-recall card 26348a6c -y --no-promo
0:03  [cost confirm 已被 -y 跳过, LLM 调用]
0:08  [lipgloss 圆角卡片：session 头 + body 引用 + 在做 + cwd]
0:18  END
```

**转 GIF**：

```bash
agg demo-card.cast docs/screenshots/card.gif \
  --speed 1.3 --theme github-dark --font-size 14 --cols 100 --rows 24
```

**验证**：GIF < 5MB；能看到圆角卡片完整四段结构。

---

## 录制后清单

- [ ] `docs/screenshots/stats.gif` < 5MB，渲染无截断
- [ ] `docs/screenshots/tui.gif` < 5MB，能看到输入筛选效果
- [ ] `docs/screenshots/gold.gif` < 5MB，**确认无 PII 漏网**
- [ ] `docs/screenshots/card.gif` < 5MB，**确认无 PII 漏网**
- [ ] README 4 个 `<!-- screenshot: ... -->` 注释位置和文件名对得上
- [ ] 删除 `*.cast` / `*.tape` 中间产物（`rm demo-*.cast demo-*.tape`）

## 失败回退

- **GIF 太大** (>5MB)：降速 `--speed 2.0`、缩 cols `--cols 90`、缩 font-size 12pt、剪掉 spinner 帧
- **asciinema 录到 ANSI 残留**：录制时先 `printf '\033c'` 完整 reset 而不是只 `clear`
- **vhs 中文显示窄了**：`Set FontFamily "Cascadia Code"`（CJK 友好）或装 Sarasa Mono
- **gold/card 真实调用 PII 漏网**：立即删 GIF + cast 文件 + cache，调试 PII 正则后重录；**绝不上传未脱敏的 GIF**
