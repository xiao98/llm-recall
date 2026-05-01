# backend — llm-recall imggen

Tiny FastAPI service that turns a JSON stats payload into a PNG card. Lives
behind the Go `llm-recall stats` command; not a public API.

## Local dev

```
cd backend
"C:/veighna_studio/python.exe" -m venv .venv      # Windows
# or: python3 -m venv .venv                       # Linux/macOS
.venv/Scripts/pip.exe install -r requirements.txt # Windows
# or: .venv/bin/pip install -r requirements.txt   # Linux/macOS

.venv/Scripts/uvicorn.exe main:app --port 8001    # Windows
# or: .venv/bin/uvicorn main:app --port 8001
```

Then from the project root:

```
go run ./cmd/llm-recall stats --backend http://localhost:8001
```

## Endpoints

- `GET  /health` → `{"status":"ok"}` for liveness/monitoring
- `POST /v1/stats-card` → `image/png`

Request body matches `schema.py:StatsRequest`:

```json
{
  "window_days": 30,
  "total_sessions": 184,
  "total_tokens": 2345678,
  "total_messages": 921,
  "top_topics": ["claude", "历史", "wiki", "quant", "feishu"],
  "longest_session_hours": 4.2,
  "per_source": {"claude": 120, "codex": 31, "gemini": 33},
  "watermark": true,
  "format": "square",
  "template": "v1"
}
```

`format` ∈ `square` (1080×1080) | `vertical` (1080×1920).
`template` ∈ `v1` (minimal) | `v2` (hype) | `v3` (quadrant report). Sample
PNGs of all three live at `templates/sample_v{1,2,3}.png`.

## Production deploy

The service is meant to run as a systemd `--user` unit on the shared
RackNerd box (1G / 1C). See `deploy/install.sh` and the `.service` file.

## Fonts

Noto Sans CJK SC Regular + Bold are bundled under `fonts/` (Apache-2.0).
The renderer reads ONLY from this directory so production hosts without
system CJK fonts still render Chinese correctly.
