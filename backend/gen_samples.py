"""One-shot script to regenerate the three template sample PNGs.

Run: backend/.venv/Scripts/python.exe gen_samples.py

Uses a single fake dataset across all three templates so the user can
compare apples to apples and pick a winner.
"""

import os
import sys

# Allow `from schema import ...` even when run from outside backend/.
HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

from schema import StatsRequest
from templates.stats_card import render

SAMPLE = StatsRequest(
    window_days=30,
    total_sessions=184,
    total_tokens=2_345_678,
    total_messages=921,
    top_topics=["claude", "历史", "wiki", "quant", "feishu"],
    longest_session_hours=4.2,
    per_source={"claude": 120, "codex": 31, "gemini": 33},
    watermark=True,
    format="square",
    template="v1",
)


def main() -> None:
    out_dir = os.path.join(HERE, "templates")
    for v in ("v1", "v2", "v3"):
        req = SAMPLE.model_copy(update={"template": v})
        png = render(req)
        path = os.path.join(out_dir, f"sample_{v}.png")
        with open(path, "wb") as f:
            f.write(png)
        print(f"wrote {path} ({len(png)} bytes)")


if __name__ == "__main__":
    main()
