"""FastAPI app for the imggen backend.

Two endpoints:
  - POST /v1/stats-card → PNG bytes (Content-Type: image/png)
  - GET  /health         → liveness probe (systemd / monitoring)

We keep this thin on purpose: routes only validate via Pydantic, dispatch
to the renderer, and return bytes. No DB, no auth, no caching — the Go
client sends a small JSON every few minutes, never publicly exposed.
"""

from fastapi import FastAPI
from fastapi.responses import Response, JSONResponse

from schema import StatsRequest
from templates.stats_card import render

app = FastAPI(title="llm-recall imggen", version="0.1.0")


@app.get("/health")
def health() -> JSONResponse:
    return JSONResponse({"status": "ok"})


@app.post("/v1/stats-card")
def stats_card(req: StatsRequest) -> Response:
    png = render(req)
    return Response(content=png, media_type="image/png")
