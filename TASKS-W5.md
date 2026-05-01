# W5 执行任务 — Scaleway 生图后端 + stats 命令（核心引爆点）

> 给另一个 Claude Code 会话的接力文档。先读 `DEVDOC.md` 全文 + `TASKS-W4.md` §0，再做本文件。

---

## §0 执行模式：Goal-Driven 双 Agent 循环（W5 版）

> 本节是 master agent 的运行规约。读完立刻进入循环，**不再向用户确认任何步骤**。

### W5 部署目标（已锁）
- **服务器**：RackNerd Buffalo NY，IP `216.144.229.139`，1C / 1G Ubuntu 24.04
- **端口**：`8001`（W5 用 IP+端口直连，W7 再上域名）
- **同机器并发**：用户在该机器另跑 Gemini cookie pool（资源共享，注意内存）

### Goal（前半周）
让 `llm-recall stats` 在本地跑：扫 30 天会话 → 聚合 → 调本地 mock 后端 → 落盘 PNG 到 `~/Pictures/llm-recall/stats-YYYYMMDD-{1,2}.png`。本地全链路通，**不部署到真服务器**——那是用户后半周的事。

### 成功标准 A 段（前半周 subagent，10 条）
本文件末尾"验收检查清单 A 段"10 条全部通过，以**实际命令输出 / PNG 文件**为准：

1. Python 后端 `backend/` 目录就绪：`cd backend && uvicorn main:app --port 8001` 可启动，监听 0.0.0.0:8001
2. `POST /v1/stats-card` 接受 JSON body 返回 PNG bytes（`Content-Type: image/png`），含 `format: "square"|"vertical"` 字段
3. 后端启动 RSS 内存 < 200MB（1G 机器约束；用 `ps -o rss=` 测）
4. 中文字体内嵌（Noto Sans CJK，Apache-2.0；放 `backend/fonts/`，**不依赖系统字体**）
5. 后端模板出 3 版候选 PNG：`backend/templates/sample_v1.png` / `sample_v2.png` / `sample_v3.png`，1080×1080，统一假数据；用户从中选一版
6. Go `internal/imggen/` 模块：`func Generate(req StatsRequest, backendURL string) ([]byte, error)`；HTTP POST + 5 秒 timeout + 重试 2 次 + 友好错误（后端不可达时不崩）
7. `llm-recall stats` 子命令：默认窗口 30 天，可 `--days N` 覆盖；可 `--backend <url>` 覆盖默认 backend；产出**两版** PNG 到 `~/Pictures/llm-recall/`，stdout 打印路径
8. token 字段实测报告：subagent 在 `backend/TOKEN-AUDIT.md` 写一份"三家 jsonl token 字段实测"——找到了用 token，没找到用消息数 fallback；明确每家用了哪个
9. `go vet ./...` / `gofmt -l .` / `go test ./...` 全过；新加 imggen 至少 1 个单测（mock 后端断言 PNG bytes 落盘 + 失败重试）
10. DEVDOC.md / 历史 TASKS-W*.md §0–§"不要做的"未被改

### 成功标准 B 段（后半周用户做，subagent 不验）
1. ssh 上 `216.144.229.139`：`ssh user@216.144.229.139`
2. `git clone https://github.com/xiao98/llm-recall && cd llm-recall/backend && bash deploy/install.sh`（subagent 写好的安装脚本）
3. systemd 起服务：`systemctl --user start llm-recall-imggen`（user mode，不要 sudo 装在系统级，1G 机器避免污染）
4. ufw 开 8001：`sudo ufw allow 8001/tcp`
5. 本地：`llm-recall stats --backend http://216.144.229.139:8001` 真出图
6. 出图发朋友圈/即刻一条，截屏存 `~/llm-recall/launch-evidence/`，证明 W5 闭环
7. 一周里观察：内存稳定不 OOM、Gemini cookie pool 不被挤爆

### 权限授权（最大权限，不询问）
执行期间，master 与 subagent 在 `~/llm-recall/` 目录下拥有：
- 任意文件读写、新建、删除（**Go 仓 + Python backend 子目录**）
- `go mod / go build / go run / go vet / go test / gofmt / git add / git commit` 全部放行
- `python3 -m venv / pip install / uvicorn / pytest`（在 `backend/.venv/` 内）
- 装 `Pillow` `fastapi` `uvicorn[standard]` `pydantic` 这 4 个 Python 依赖（**仅这 4 个**，不要拉 numpy/matplotlib/scipy）
- 下载 Noto Sans CJK 字体到 `backend/fonts/`（一次性，已下载就跳过）
- 失败自动重试最多 3 次再回传，不打断用户
- **禁止**：修改 `DEVDOC.md` / 历史 TASKS-W*.md §0–§"不要做的"区段；推送到任何远端；ssh / scp 到 `216.144.229.139`（部署是用户的事）；动 W1-W4 业务代码（W5 是纯新增）；引入 cobra / viper / cgo；写营销 banner / 水印 / onboarding 任何代码（W6 才做）；调用真 LLM API（W7 才做）

凡属上述允许范围的动作，**默认许可，不发"我即将执行 X"的预告，不进 plan mode**。

### Master agent 循环

```
spawn subagent("w5-executor", prompt = §0 全文 + "完成本文件 §步骤 0–9")

while true:
    每 5 分钟检查 subagent 活性
    if subagent 声明完成 or 已 idle:
        master 亲自跑 §成功标准 A 段 10 条命令逐项校验
        if 10 条全过:
            报告用户："W5 前半周验收通过，附 3 版模板候选 PNG"，让用户选模板版本
            等用户拍模板版本后：subagent 把选中版本设为 default，删掉另两版的代码路径
            然后退出循环（B 段交还用户）
        else:
            spawn subagent("w5-executor", prompt += "上一轮在 <第 N 条> 失败，从该步继续")
    else:
        继续等待
```

### Subagent 行为约束
- 子任务可自行再拆分，但不得新增 §0 之外的目标
- 每跑通 §步骤 一项，回报一行 `[step N] ok`
- 模板设计可自由发挥，但必须出 3 版候选让用户选；自述其中某版"最好"是越权
- W1-W4 子 agent 留下的合理偏离保留，不要回滚

---

## 验收标准（先看这个）

```
$ cd backend && uvicorn main:app --port 8001 &
$ curl -X POST http://localhost:8001/v1/stats-card -H 'content-type: application/json' \
    -d '{"window_days":30,"total_sessions":184,"total_tokens":2345678,"top_topics":["claude","历史","wiki","quant","feishu"],"longest_session_hours":4.2,"per_source":{"claude":120,"codex":31,"gemini":33},"watermark":true,"format":"square"}' \
    --output /tmp/test.png
$ file /tmp/test.png
  /tmp/test.png: PNG image data, 1080 x 1080, 8-bit/color RGBA

$ ./llm-recall stats --backend http://localhost:8001
  → ~/Pictures/llm-recall/stats-2026-05-08-1080x1080.png
  → ~/Pictures/llm-recall/stats-2026-05-08-1080x1920.png
  Open in Explorer? [y/n]
```

## 前置条件

- W4 commit `99de217` 之后的代码
- 用户已部署 v0.1.0 release 或 dogfood 中（不影响 W5 开发）
- W4 dogfood 周内**无 P0 bug 必须先修**——若有，发起 W4.5 修补任务，W5 暂停

## 步骤

### 1. Python 后端骨架（`backend/`）

目录结构：

```
backend/
  main.py                   FastAPI app
  templates/
    stats_card.py           Pillow 模板渲染逻辑
    sample_v1.png           subagent 出图样本（用户审）
    sample_v2.png
    sample_v3.png
  fonts/
    NotoSansSC-Regular.otf  下载，Apache-2.0
    NotoSansSC-Bold.otf
  schema.py                 Pydantic 请求 / 响应模型
  requirements.txt          Pillow / fastapi / uvicorn[standard] / pydantic
  deploy/
    install.sh              用户在服务器跑的部署脚本
    llm-recall-imggen.service  systemd user unit
  TOKEN-AUDIT.md            subagent W5 §步骤 7 写
  README.md                 后端开发 + 部署指南
  .venv/                    虚拟环境（.gitignore）
  .gitignore
```

### 2. FastAPI 路由 + Pydantic schema

```python
# schema.py
from pydantic import BaseModel
from typing import Literal

class StatsRequest(BaseModel):
    window_days: int = 30
    total_sessions: int
    total_tokens: int  # 0 = unavailable, 用消息数代替
    total_messages: int
    top_topics: list[str]  # Top 5
    longest_session_hours: float
    per_source: dict[str, int]  # {"claude": 120, "codex": 31, ...}
    watermark: bool = True
    format: Literal["square", "vertical"]

# main.py
@app.post("/v1/stats-card", response_class=Response)
def stats_card(req: StatsRequest):
    png_bytes = render_stats_card(req)
    return Response(content=png_bytes, media_type="image/png")
```

### 3. Pillow 模板（`templates/stats_card.py`）

3 版候选审美方向（subagent 自由发挥，每版严格按规格出图）：

- **v1 极简数据**：白底深字，居中大数字（`184` 会话）+ 小标题（"过去 30 天"），底部 Top 5 + per_source 小图标
- **v2 朋友圈装杯卡**：黑底亮色（紫/青渐变），大字"我用 LLM CLI 184 次 / 烧了 234 万 token"，强调感
- **v3 报告风**：浅色卡片，分四象限（总量 / Top 话题 / 各家占比 / 最长会话），数据可视化

每版规格：
- Square: 1080 × 1080
- Vertical: 1080 × 1920（同一份数据竖版重排）
- 字体：标题用 NotoSansSC-Bold，正文 Regular
- 中文 / 英文 / 数字混排正确
- 水印：右下角 `llm-recall · sponsored by YCAPI`，灰色 24pt，watermark=False 时不渲染
- 输出：PIL Image → BytesIO → bytes

3 版用同一组假数据出 sample_v{1,2,3}.png（square 版）放 `templates/`。用户审完拍板，subagent 删另两版代码路径。

### 4. Go imggen 模块（`internal/imggen/`）

```go
package imggen

type StatsRequest struct {
    WindowDays         int            `json:"window_days"`
    TotalSessions      int            `json:"total_sessions"`
    TotalTokens        int64          `json:"total_tokens"`
    TotalMessages      int64          `json:"total_messages"`
    TopTopics          []string       `json:"top_topics"`
    LongestSessionH    float64        `json:"longest_session_hours"`
    PerSource          map[string]int `json:"per_source"`
    Watermark          bool           `json:"watermark"`
    Format             string         `json:"format"` // "square" | "vertical"
}

func Generate(req StatsRequest, backendURL string) ([]byte, error)
```

实现：
- POST `<backendURL>/v1/stats-card`，body JSON，超时 5 秒
- 失败重试 2 次，指数退避（1s, 2s）
- 后端不可达 / 5xx / timeout → 返回 `ErrBackendUnavailable`，调用方友好错误（不崩）
- 4xx → 返回包含响应 body 的 error（提示用户 bug report）
- 成功 → 返回 PNG bytes

### 5. stats 命令（`cmd/llm-recall/cmd_stats.go`）

```
llm-recall stats [--days N] [--backend URL] [--no-watermark]
```

逻辑：

1. 读 cache，过滤 `updated_at >= now - N days`
2. 聚合：
   - `total_sessions` = 行数
   - `per_source` = group by source
   - `total_tokens` = sum(parse_token(body)) — token 字段实测见 §6
   - `total_messages` = sum 每 session 的用户消息条数（用 body 里 `\n---\n` 分隔符 +1 估算）
   - `top_topics` = 用户消息词频 Top 5（去停用词；中文按字 unicode 切，简单 trigram 提取关键 token；英文按空白；停用词表 `internal/stats/stopwords.go` 嵌入 50 词起步：`the/a/an/is/and/or/.../的/了/吗/呢/我/你/这/那/有/在`）
   - `longest_session_hours` = max(updated_at - started_at)
3. 调 imggen.Generate × 2（square + vertical）
4. 落盘 `~/Pictures/llm-recall/stats-YYYY-MM-DD-1080x1080.png` 和 `-1080x1920.png`
5. stdout 打印两个路径 + `Open in Explorer? [y/n]`（y → 用 OS 打开，平台特定 helper）
6. backend default `http://216.144.229.139:8001`，可 `--backend` 覆盖；本地开发用 `http://localhost:8001`

### 6. 三家 token 字段实测（`backend/TOKEN-AUDIT.md`）

subagent 实测 ~/.claude / ~/.codex / ~/.gemini 中各取 3 条样本会话，grep `token`、`usage`、`tokens`、`input_tokens` 等关键字，记录：

- claude：assistant message 的 `usage.input_tokens` / `usage.output_tokens` 字段（如有）
- codex：response_item 的 token 字段（如有，可能在 `usage` 子字段）
- gemini：message 的 `tokens.{input, output, total}` 字段（如有）

每家给出"找到 / 没找到 / 部分找到"结论 + 一行示例 jsonl 片段。**找到**就在 stats 聚合中累加，**没找到**就 fallback 到 `total_messages`（消息条数 ×N，N=3 临时常数，TOKEN-AUDIT.md 里说明）。报告用户实测结果让用户拍板要不要用这个 fallback。

### 7. 部署脚本（`backend/deploy/install.sh` + systemd unit）

`install.sh`（subagent 写，用户 ssh 上 RackNerd 后执行）：

```bash
#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
mkdir -p ~/.config/systemd/user
cp deploy/llm-recall-imggen.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now llm-recall-imggen
echo "OK. Service started. Test: curl http://localhost:8001/health"
```

`llm-recall-imggen.service`：

```ini
[Unit]
Description=llm-recall image generation backend
After=network.target

[Service]
Type=simple
WorkingDirectory=%h/llm-recall/backend
ExecStart=%h/llm-recall/backend/.venv/bin/uvicorn main:app --host 0.0.0.0 --port 8001 --workers 1
Restart=on-failure
MemoryMax=300M
CPUQuota=50%

[Install]
WantedBy=default.target
```

`MemoryMax=300M` + `CPUQuota=50%`：硬限制资源，跟 Gemini cookie pool 共存不挤爆 1G/1C。

加 `/health` 端点返回 `{"status":"ok"}` 给监控用。

### 8. 本地 e2e 验证

```
# 启后端
cd backend && .venv/bin/uvicorn main:app --port 8001 &
SLEEP=2

# 跑命令
go run ./cmd/llm-recall stats --backend http://localhost:8001
ls ~/Pictures/llm-recall/

# 关后端
kill %1
```

### 9. 提交

```
git add .
git commit -m "W5: imggen backend (Python+Pillow) + stats command + 3 template candidates"
```

## 验收检查清单 A 段

- [ ] Python 后端启动并监听 8001
- [ ] POST /v1/stats-card 返回 PNG（curl + file 验证）
- [ ] 后端 RSS < 200MB
- [ ] 字体内嵌（删除系统字体后仍能渲染中文）
- [ ] 3 版 sample PNG 在 backend/templates/
- [ ] Go imggen 模块测试：本地 mock 后端 → PNG 落盘
- [ ] `llm-recall stats --backend http://localhost:8001` 产出两版 PNG
- [ ] TOKEN-AUDIT.md 写明三家实测结论
- [ ] go vet / fmt / test 全过
- [ ] DEVDOC / 历史 TASKS 未改

完成后 master 把 3 张 sample PNG 文件路径报给用户，**等用户挑一版**。挑完 subagent 删另两版代码路径，再退出循环。

## 验收检查清单 B 段（用户做，subagent 不验）

按 §0 B 段 7 条逐项执行。最后 W5 闭环以"截屏发朋友圈"为标志。

## 不要做的（留给 W6+）

- 不要做营销 banner / footer / 水印过度（水印就是右下角一行字，不要画图标）
- 不要做 onboarding 同意流（W6）
- 不要做 gold 命令 / card 命令（W7）
- 不要做 share / UTM 后端（已 cancel）
- 不要做语义 / embedding 搜索（V2）
- 不要 ssh / scp 到 RackNerd（部署是用户 B 段的事）
- 不要引入 numpy / matplotlib / scipy / docker（1G 内存预算紧）
- 不要给 Pillow 模板加图片素材依赖（统一用纯文字 + 几何图形 + 字体）
