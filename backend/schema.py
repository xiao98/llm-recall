"""Pydantic request/response schemas for the imggen backend.

Why split out: keeps `main.py` route code thin and lets the Pillow templates
import the same types without pulling in FastAPI.
"""

from typing import Literal

from pydantic import BaseModel, Field


class StatsRequest(BaseModel):
    """Aggregated stats payload from the Go `llm-recall stats` command.

    `total_tokens` may be 0 when the underlying jsonl files don't carry usage
    metadata (see backend/TOKEN-AUDIT.md). In that case the renderer falls
    back to `total_messages` * a constant heuristic.
    `template` selects which of the three candidate designs to render. Defaults
    to v1 (clean) — once the user picks a winner we collapse the other two.
    """

    window_days: int = 30
    total_sessions: int
    total_tokens: int = 0
    total_messages: int = 0
    top_topics: list[str] = Field(default_factory=list)
    longest_session_hours: float = 0.0
    per_source: dict[str, int] = Field(default_factory=dict)
    watermark: bool = True
    format: Literal["square", "vertical"] = "square"
    template: Literal["v1", "v2", "v3"] = "v1"
