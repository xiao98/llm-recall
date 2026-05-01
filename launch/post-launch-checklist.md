# Post-launch checklist — v0.2.0 后半周机械执行

> 5 个动作，按顺序跑，预计总耗时 ≤ 2 小时。每个动作含**预计耗时 / 前置依赖 / 逐步 / 验证 / 失败回退 / 成功长什么样**。
>
> 所有内容性创作（README / landing / 公众号 / Reddit / HN / storyboard）已在 `~/llm-recall/launch/` 备齐——本文档只管"按按钮"。

---

## 动作 1：部署 landing 到 GitHub Pages

**预计耗时**：10 分钟（不含 DNS 传播等待）
**前置依赖**：`docs/index.html`、`docs/CNAME` 已 commit 到 main（W8 commit 已搞定）；GitHub 仓 `xiao98/llm-recall` 存在；你拥有 `youchun.tech` DNS 控制权。

**逐步**：

1. 进 GitHub 仓 Settings → Pages → Source 选 **Deploy from a branch** → Branch 选 `main` / Folder 选 `/docs` → Save
2. 等 30 秒，刷新 Settings → Pages 页面，看 "Your site is live at https://xiao98.github.io/llm-recall/"
3. 在 DNS 控制台（Cloudflare / 阿里云 / 腾讯云）加 CNAME 记录：
   - 主机记录：`recall`
   - 记录类型：`CNAME`
   - 记录值：`xiao98.github.io`（**不带斜杠不带子路径**）
   - TTL：600
4. 回 Settings → Pages → Custom domain 填 `recall.youchun.tech` → Save，勾上 "Enforce HTTPS"（DNS 生效后才能勾，可能要等 5-30 分钟）
5. 浏览器访问 `https://recall.youchun.tech` 验证

**验证**：

```bash
curl -sI https://recall.youchun.tech | head -3
# 期望: HTTP/2 200

curl -s https://recall.youchun.tech | grep -o '<h1>llm-recall</h1>'
# 期望: <h1>llm-recall</h1>
```

**失败回退**：

- DNS 24h 不生效 → 临时用 `https://xiao98.github.io/llm-recall/` 访问；landing 的 `<link rel="canonical">` 已指 `recall.youchun.tech`，不影响 SEO
- HTTPS 证书一直 pending → 到 Settings → Pages 把 custom domain 删掉、保存、再加回去，强制重申 cert
- 404 → 检查 `docs/CNAME` 文件内容是不是单行 `recall.youchun.tech`（有 BOM 或多余空行会废）

**成功长什么样**：浏览器访问 `https://recall.youchun.tech` 看到深色背景、橘色标题 `llm-recall`、install 命令块、5 段内容（hero / 它解决什么 / 30 秒上手 / 其他安装 / 隐私）+ footer 含 GitHub / YCAPI 链接。

---

## 动作 2：录 4 段 demo + 转 GIF

**预计耗时**：30 分钟（含装工具）
**前置依赖**：动作 1 完成（landing 上线后 README 里的 GIF 引用才有意义）；本机至少 7 天会话数据；`ANTHROPIC_API_KEY` 或 `OPENAI_API_KEY` 任一可用（gold/card 需要）。

**逐步**：

1. 装录制工具：`brew install asciinema agg`（或 `brew install vhs`）
2. 终端尺寸 100×30、字体 JetBrains Mono 14pt、深色主题（详见 `launch/storyboard.md` 准备工作段）
3. 按 `launch/storyboard.md` §1–§4 顺序录 4 段：
   - §1 stats（30s，无 LLM 调用）
   - §2 TUI search（45s，无 LLM 调用）
   - §3 gold（30s，**真实 LLM 调用，先 dry-run 确认无 PII**）
   - §4 card（20s，**真实 LLM 调用，先 dry-run 确认无 PII**）
4. 每段转 GIF 落 `docs/screenshots/{stats,tui,gold,card}.gif`
5. 删中间产物：`rm demo-*.cast demo-*.tape`
6. commit + push：

```bash
cd ~/llm-recall
git add docs/screenshots/*.gif
git commit -m "docs: add 4 demo GIFs for v0.2.0"
git push origin main
```

**验证**：

```bash
ls -lh docs/screenshots/*.gif
# 期望: 4 个文件，每个 < 5MB

# 浏览器开 https://recall.youchun.tech，README 里 4 个 screenshot 注释还是注释（README 里默认不嵌图，你想嵌的话把 <!-- ... --> 注释换成 ![alt](docs/screenshots/X.gif) 标签）
```

**失败回退**：

- GIF > 5MB → `--speed 2.0` 加速、`--cols 90` 缩列、剪掉 spinner 帧重转
- asciinema ANSI 残留 → 录前 `printf '\033c'` 完整 reset 而不是只 `clear`
- gold/card 输出有 PII → **立即** `rm` 掉 GIF + cast + ~/.cache/llm-recall/llm-cache/，调 PII 正则后重录，**绝不上传未脱敏 GIF**

**成功长什么样**：`docs/screenshots/` 下 4 个 GIF 文件均 < 5MB；GitHub 仓 commit 推上去后，访问 `https://github.com/xiao98/llm-recall/blob/main/docs/screenshots/stats.gif` 能预览动图。

---

## 动作 3：push v0.2.0 tag

**预计耗时**：5 分钟（不含 GitHub Actions 跑 5-10 分钟）
**前置依赖**：动作 2 完成（GIF 已 push）；`CHANGELOG.md` 含 `## [0.2.0]` entry（W8 commit 已搞定）；GitHub Actions `goreleaser` workflow 在 main 分支可见且历史成功过；`HOMEBREW_TAP_GITHUB_TOKEN` / `SCOOP_BUCKET_GITHUB_TOKEN` secrets 已配。

**逐步**：

1. 确认 main 分支干净：

```bash
cd ~/llm-recall
git status            # working tree clean
git log --oneline -3  # 看最新 commit 是 W8 + GIF
```

2. 打 tag：

```bash
git tag -a v0.2.0 -m "release v0.2.0 — stats heatmap + onboarding + gold/card BYOK"
```

3. push tag：

```bash
git push origin v0.2.0
```

4. 浏览器开 `https://github.com/xiao98/llm-recall/actions` 看 goreleaser workflow 起来；等 5-10 分钟跑完
5. 跑完后开 `https://github.com/xiao98/llm-recall/releases/tag/v0.2.0` 验证 5 个 archive：
   - `llm-recall_Darwin_amd64.tar.gz`
   - `llm-recall_Darwin_arm64.tar.gz`
   - `llm-recall_Linux_amd64.tar.gz`
   - `llm-recall_Linux_arm64.tar.gz`
   - `llm-recall_Windows_amd64.zip`
6. brew tap 测试：

```bash
brew update
brew install xiao98/tap/llm-recall
llm-recall version    # 期望: 0.2.0
```

**验证**：

```bash
gh release view v0.2.0 --repo xiao98/llm-recall | head -20
# 期望: tag = v0.2.0, 5 个 asset

curl -sI https://github.com/xiao98/llm-recall/releases/download/v0.2.0/llm-recall_Darwin_arm64.tar.gz | head -3
# 期望: HTTP/2 302 (redirect to S3)
```

**失败回退**：

- Actions 失败 → 看 logs，最常见三类：
  1. brew tap secret 未配 / 过期 → GitHub Settings → Secrets 重配 `HOMEBREW_TAP_GITHUB_TOKEN`，**删 tag 重推**：`git push origin :v0.2.0&& git tag -d v0.2.0`，修后 `git tag v0.2.0 && git push origin v0.2.0`
  2. goreleaser 配置语法错 → 本地 `goreleaser check` 验证后重推 tag
  3. cross-build 编译错 → 本地 `GOOS=windows GOARCH=amd64 go build ./cmd/llm-recall` 复现，修代码（W8 不该改业务代码，但如果是配置问题修了）
- brew install 装不上 → tap 仓 `xiao98/homebrew-tap` 看 formula 文件 sha256 是否对得上 release 的 `checksums.txt`

**成功长什么样**：`brew install xiao98/tap/llm-recall && llm-recall version` 输出 `0.2.0`；release 页面 5 个 archive + checksums.txt 全在；GitHub release notes 自动从 CHANGELOG v0.2.0 entry 拉过来（goreleaser 配置默认行为）。

---

## 动作 4：发公众号 + 朋友圈

**预计耗时**：30 分钟
**前置依赖**：动作 1+2+3 完成（landing 可访问、GIF 已上传、release 已发——文章里 link 才有意义）；公众号编辑权限。

**逐步**：

1. 打开 `~/llm-recall/launch/wechat-draft.md`，**填 3 处 `[作者填: ...]` 占位**：
   - 开篇引子（1-2 句个人化钩子）
   - 个人痛点叙述（2-3 段，必须真实，不要 GPT 编）
   - 结语（个人化 closing，避免油腻话术）
2. 公众号编辑器（mp.weixin.qq.com）→ 新建图文 → 粘贴 markdown（公众号支持基础 markdown，代码块用编辑器自带的代码段）
3. 在 4 处 `[图: ...]` 位置插对应 GIF 的**单帧 PNG**（公众号不自动播动图，需要把 GIF 用 `convert demo.gif[0] preview.png` 抽第一帧或选关键帧上传）
4. 标题再调一遍（公众号标题字数限制 64，副标题 120）
5. 摘要框填一句话（公众号外显，建议直接抄文章第一段重写）
6. 预览 → 检查代码块是否折行、4 个截图位是否对得上、文末链接是否可点
7. 发布
8. 朋友圈：发布后复制 H5 链接 → 朋友圈 → 配文（保持简洁，比如"周末做的小工具，跨厂商 LLM CLI 会话搜索 + 恢复"+链接）

**验证**：

- 公众号文章发布成功，文末 `https://recall.youchun.tech` 链接可点跳转 landing
- 朋友圈 H5 卡片渲染出 OG 图（如有；否则文字也行）
- 24h 内：阅读 / 点赞 / 在看 / 留言一栏有真实数据（无需追求高峰）

**失败回退**：

- 公众号审核驳回（最常见）：
  1. 删油腻话术（"震惊"、"独家"、"99%程序员不知道"等）—— 当前 draft 已规避
  2. 删外链 → 公众号外链需白名单或被识别为引流；GitHub / 个人 landing 一般 OK，但 youchun.tech 域如果是新域可能被卡，改成 `<https://github.com/xiao98/llm-recall>` 兜底
  3. 涉及"营销"字样 → 「Privacy & Promo」段把"营销注入"改成"赞助披露"等中性词
- 朋友圈链接卡片不渲染 → 等 5 分钟微信抓 OG（`<meta property="og:image">` 当前指 `/og.png` 占位，**实际未上传**——后半周需要补一张 1200×630 PNG 落 `docs/og.png` 才能正常出卡片）

**成功长什么样**：公众号文章发布、阅读量 > 0、文末链接跳转 landing；朋友圈 H5 链接可访问。

---

## 动作 5：发 Reddit + Hacker News

**预计耗时**：30 分钟（不含 24h 回复评论时间）
**前置依赖**：动作 1+2+3 完成；Reddit 账号在 r/commandline 有过历史发言（避免 mod queue 卡）；HN 账号 ≥ 1 周龄、有过评论历史。

**逐步**：

### 5a. Reddit r/commandline

1. 打开 `~/llm-recall/launch/reddit-r-commandline.md`，复制 **Title** 字段
2. 进 <https://www.reddit.com/r/commandline/submit?type=LINK>
3. URL 字段填 `https://recall.youchun.tech`
4. Title 粘贴
5. Flair（如有）选 `Self-promotion` 或 `Tool` —— 不同子版规则不同，看右侧规则栏
6. 提交
7. 帖子上线后**立即**复制 draft 里的 **Body** 段，作为帖子第一条评论补完整描述（r/commandline 习惯：link post 配 self-comment 给上下文）
8. 24h 内回评：参考 draft "Comments I expect & how I'll respond" 8 条预演脚本，**别照抄**，按对方语气改

### 5b. Hacker News Show HN

1. 打开 `~/llm-recall/launch/hn-show-hn.md`，复制 **Title** 字段（注意是 en-dash `–` 不是 hyphen `-`）
2. 进 <https://news.ycombinator.com/submit>
3. Title 粘贴
4. URL 填 `https://recall.youchun.tech`（不要用 GitHub 仓 URL，Show HN 偏好可演示 URL）
5. Text 字段留空（HN Show HN 可以 URL + 空 text，正文丢到 self-comment 里更符合习惯；或者把 draft Body 段贴到 text，二选一）
6. 提交
7. 帖子上线后**立刻**（30 秒内）切到自己帖子，发一条评论作为 self-comment：粘贴 draft 里的 **Self-comment** 段（注意是粘到自己 thread 的 reply，不是 top-level）
8. 24h 内监控 HN 排名：如果 30 分钟内没进 new 页面前 30，基本沉了，不要再发；如果进了 front page，盯紧评论按 draft 的 §3 §4 §6 模板回

**验证**：

```bash
# Reddit
# 帖子可访问 → 24h 后 score > 0 即算成功
# 失败信号：帖子被自动删（看不到自己的帖子但能看到 mod 通知）

# HN
# 帖子在 https://news.ycombinator.com/show 列表可见
# 30 分钟内 score > 5 一般就活了
```

**失败回退**：

- Reddit 帖子被删：
  1. 看 mod 通知里的具体规则违反
  2. 90% 是 self-promotion ratio（账号最近 10 条发言里 self-promo 占比 > 10% 就触发）→ 等 1-2 周发更多非 self-promo 内容再发
  3. 10% 是 link post 不配文 → 重发时把 Body 段 copy 进自动出现的 sticky comment
- HN 沉了（30 分钟内 score < 3）：
  1. **不要再发**（HN 反复发同一项目会被 shadowban）
  2. 等 1-2 个月迭代了新功能再以新 angle 发（"v0.3.0 加了 X" 是合法重发）
- Self-comment 漏发（HN 提交后忘贴）：可以补，但越晚效果越弱；超过 1 小时基本就没人回看了，认栽

**成功长什么样**：

- Reddit r/commandline 帖子 24h 内 score > 10、评论 > 3 条
- HN Show HN 进 front page（30 分钟内 score > 30），或至少 new 页面停留 1 小时未沉
- 任一渠道有用户来 GitHub 提 issue / star，即算"传播触发"

---

## 全流程成功长什么样

完成 5 个动作后：

- [ ] `https://recall.youchun.tech` 200 + landing 完整渲染
- [ ] `docs/screenshots/{stats,tui,gold,card}.gif` 4 个文件，每个 < 5MB，GitHub 仓可预览
- [ ] `brew install xiao98/tap/llm-recall` 装得上，`llm-recall version` 输出 `0.2.0`
- [ ] 公众号文章发布、链接可跳转、朋友圈 H5 卡片可访问
- [ ] Reddit 帖子未被删、HN 帖子已上线 + self-comment 紧跟
- [ ] GitHub 仓 24h 内有 star / issue / fork 任一信号

任何一项失败按对应回退执行。**全部 OK = v0.2.0 发布完成**。
