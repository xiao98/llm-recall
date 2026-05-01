"""Pillow renderers for the three template candidates.

Three audacious directions that map to different audiences:

  v1 — minimal data card. White ground, oversized hero number, subdued
       support text. Reads like a fitness app weekly digest. Safe.
  v2 — hype card. Black ground with a vertical purple→cyan gradient,
       bright headline. Built to share on 朋友圈 / 即刻 with brag-energy.
  v3 — quadrant report. Light card split into four blocks (totals, top
       topics, per-source share, longest session). Reads like a tiny
       analytics report.

All three share the same data input, geometry constants, and watermark.
Fonts are loaded from `backend/fonts/` — never the system font cache —
because the production target (Ubuntu 1G) has no CJK fonts installed and
the §4 acceptance criteria require the renderer to keep working when
system fonts are wiped.

Caller passes a `StatsRequest`; we return PNG bytes.
"""

from __future__ import annotations

import io
import os
from typing import Iterable

from PIL import Image, ImageDraw, ImageFont

from schema import StatsRequest


# ---- font loading -------------------------------------------------------

_FONT_DIR = os.path.join(os.path.dirname(os.path.dirname(__file__)), "fonts")
_REGULAR = os.path.join(_FONT_DIR, "NotoSansSC-Regular.otf")
_BOLD = os.path.join(os.path.dirname(_FONT_DIR), "fonts", "NotoSansSC-Bold.otf")

# Cache loaded fonts by (family, size). Pillow's truetype handle is not
# threadsafe for layout, but FastAPI's default worker model is single-thread
# per request inside one uvicorn worker — fine. Avoids re-reading 16MB OTFs
# every request.
_FONT_CACHE: dict[tuple[str, int], ImageFont.FreeTypeFont] = {}


def _font(bold: bool, size: int) -> ImageFont.FreeTypeFont:
    path = _BOLD if bold else _REGULAR
    key = (path, size)
    f = _FONT_CACHE.get(key)
    if f is None:
        f = ImageFont.truetype(path, size=size)
        _FONT_CACHE[key] = f
    return f


def loaded_font_path(bold: bool = False) -> str:
    """Return the path PIL is reading the font from. Used by the test that
    asserts we are NOT using a system font."""
    return _BOLD if bold else _REGULAR


# ---- shared helpers -----------------------------------------------------

def _format_number(n: int) -> str:
    """184 → '184'; 2_345_678 → '2.3M'. Cards have limited width so we
    abbreviate above 10k and 1M boundaries."""
    if n < 10_000:
        return f"{n:,}"
    if n < 1_000_000:
        return f"{n / 1000:.1f}K"
    return f"{n / 1_000_000:.1f}M"


def _draw_watermark(draw: ImageDraw.ImageDraw, w: int, h: int, color: tuple[int, int, int, int]) -> None:
    """Right-bottom one-liner. 24pt grey. No icon (per W5 §不要做的)."""
    text = "llm-recall · sponsored by YCAPI"
    font = _font(bold=False, size=24)
    bbox = draw.textbbox((0, 0), text, font=font)
    tw = bbox[2] - bbox[0]
    th = bbox[3] - bbox[1]
    pad = 32
    draw.text((w - tw - pad, h - th - pad - 8), text, font=font, fill=color)


def _vertical_gradient(size: tuple[int, int], top: tuple[int, int, int], bot: tuple[int, int, int]) -> Image.Image:
    """Plain-Pillow gradient: build column-of-1px tall image then resize.
    No numpy. ~3ms for 1080×1080 — measured."""
    w, h = size
    base = Image.new("RGB", (1, h))
    px = base.load()
    for y in range(h):
        t = y / max(1, h - 1)
        r = int(top[0] * (1 - t) + bot[0] * t)
        g = int(top[1] * (1 - t) + bot[1] * t)
        b = int(top[2] * (1 - t) + bot[2] * t)
        px[0, y] = (r, g, b)
    return base.resize((w, h), resample=Image.NEAREST)


def _truncate_topics(topics: Iterable[str], n: int = 5) -> list[str]:
    out: list[str] = []
    for t in topics:
        if len(out) >= n:
            break
        t = (t or "").strip()
        if not t:
            continue
        if len(t) > 12:
            t = t[:11] + "…"
        out.append(t)
    return out


def _per_source_total(per_source: dict[str, int]) -> int:
    return sum(per_source.values()) if per_source else 0


# ---- v1: minimal data card ---------------------------------------------

def render_v1(req: StatsRequest, w: int, h: int) -> Image.Image:
    """White ground. Hero number centered. Quiet support text. Safe."""
    img = Image.new("RGB", (w, h), (252, 252, 250))
    d = ImageDraw.Draw(img, "RGBA")

    # Top label
    label = f"过去 {req.window_days} 天"
    f_label = _font(bold=False, size=36)
    bbox = d.textbbox((0, 0), label, font=f_label)
    d.text(((w - (bbox[2] - bbox[0])) // 2, int(h * 0.16)), label,
           font=f_label, fill=(120, 120, 120))

    # Hero number — total_sessions
    hero = _format_number(req.total_sessions)
    hero_size = 360 if w == h else 320
    f_hero = _font(bold=True, size=hero_size)
    bbox = d.textbbox((0, 0), hero, font=f_hero)
    hero_x = (w - (bbox[2] - bbox[0])) // 2
    hero_y = int(h * 0.24) if w == h else int(h * 0.20)
    d.text((hero_x, hero_y), hero, font=f_hero, fill=(20, 20, 30))

    # Sub-headline
    sub = "次 LLM CLI 会话"
    f_sub = _font(bold=False, size=44)
    bbox = d.textbbox((0, 0), sub, font=f_sub)
    sub_y = hero_y + (360 if w == h else 320) + 30
    d.text(((w - (bbox[2] - bbox[0])) // 2, sub_y), sub,
           font=f_sub, fill=(80, 80, 90))

    # Token / message line
    if req.total_tokens > 0:
        side_text = f"{_format_number(req.total_tokens)} tokens 消耗"
    elif req.total_messages > 0:
        side_text = f"{_format_number(req.total_messages)} 条消息"
    else:
        side_text = "—"
    f_side = _font(bold=False, size=34)
    bbox = d.textbbox((0, 0), side_text, font=f_side)
    side_y = sub_y + 70
    d.text(((w - (bbox[2] - bbox[0])) // 2, side_y), side_text,
           font=f_side, fill=(150, 150, 160))

    # Top topics row
    topics = _truncate_topics(req.top_topics, 5)
    if topics:
        chip_y = int(h * 0.62) if w == h else int(h * 0.55)
        f_chip = _font(bold=False, size=30)
        # Compute total chip widths to center
        chip_pad_x = 24
        chip_gap = 16
        chip_h = 56
        widths: list[int] = []
        for t in topics:
            bbox = d.textbbox((0, 0), t, font=f_chip)
            widths.append((bbox[2] - bbox[0]) + chip_pad_x * 2)
        total_w = sum(widths) + chip_gap * (len(widths) - 1)
        x = (w - total_w) // 2
        for t, cw in zip(topics, widths):
            d.rounded_rectangle((x, chip_y, x + cw, chip_y + chip_h),
                                radius=28, fill=(240, 240, 245))
            bbox = d.textbbox((0, 0), t, font=f_chip)
            tx = x + (cw - (bbox[2] - bbox[0])) // 2
            ty = chip_y + (chip_h - (bbox[3] - bbox[1])) // 2 - 4
            d.text((tx, ty), t, font=f_chip, fill=(60, 60, 80))
            x += cw + chip_gap

    # Per-source line
    if req.per_source:
        ps_y = int(h * 0.74) if w == h else int(h * 0.66)
        f_ps = _font(bold=False, size=30)
        items = sorted(req.per_source.items(), key=lambda kv: -kv[1])
        line = "  ·  ".join(f"{k} {v}" for k, v in items)
        bbox = d.textbbox((0, 0), line, font=f_ps)
        d.text(((w - (bbox[2] - bbox[0])) // 2, ps_y), line,
               font=f_ps, fill=(110, 110, 120))

    # Longest session
    if req.longest_session_hours > 0:
        ls_y = int(h * 0.82) if w == h else int(h * 0.74)
        f_ls = _font(bold=False, size=28)
        ls_text = f"最长一次会话 {req.longest_session_hours:.1f} 小时"
        bbox = d.textbbox((0, 0), ls_text, font=f_ls)
        d.text(((w - (bbox[2] - bbox[0])) // 2, ls_y), ls_text,
               font=f_ls, fill=(150, 150, 160))

    if req.watermark:
        _draw_watermark(d, w, h, (180, 180, 185, 255))
    return img


# ---- v2: hype card ------------------------------------------------------

def render_v2(req: StatsRequest, w: int, h: int) -> Image.Image:
    """Black ground, purple→cyan gradient stripe, bright headline.
    Built to flex on social. Numbers carry the design."""
    img = Image.new("RGB", (w, h), (12, 12, 18))
    d = ImageDraw.Draw(img, "RGBA")

    # Gradient stripe along the left edge (vertical).
    grad = _vertical_gradient((24, h), (140, 70, 240), (60, 220, 220))
    img.paste(grad, (0, 0))

    # Tag line top
    tag = f"PAST {req.window_days} DAYS"
    f_tag = _font(bold=True, size=30)
    d.text((80, 80), tag, font=f_tag, fill=(140, 220, 220))

    # Headline — punchy
    headline_y = int(h * 0.18)
    line1_text = "我用 LLM CLI"
    f_l1 = _font(bold=True, size=72)
    d.text((80, headline_y), line1_text, font=f_l1, fill=(245, 245, 250))

    # Big sessions number
    big = _format_number(req.total_sessions)
    big_size = 280 if w == h else 240
    f_big = _font(bold=True, size=big_size)
    bbox = d.textbbox((0, 0), big, font=f_big)
    big_x = 80
    big_y = headline_y + 100
    # Use gradient as fill: draw a gradient image, mask the text shape.
    # Pillow: render text on alpha-only canvas, then composite a gradient.
    grad_full = _vertical_gradient((bbox[2] - bbox[0] + 4, bbox[3] - bbox[1] + 20),
                                    (160, 100, 250), (80, 230, 230))
    mask = Image.new("L", grad_full.size, 0)
    mdraw = ImageDraw.Draw(mask)
    mdraw.text((0, 0), big, font=f_big, fill=255)
    img.paste(grad_full, (big_x, big_y), mask)

    # Suffix
    f_unit = _font(bold=False, size=44)
    suf = "次 sessions"
    d.text((80, big_y + big_size + 4), suf, font=f_unit, fill=(180, 180, 200))

    # Token brag
    if req.total_tokens > 0:
        token_main = f"{_format_number(req.total_tokens)}"
        token_sub = "tokens 烧掉"
    elif req.total_messages > 0:
        token_main = f"{_format_number(req.total_messages)}"
        token_sub = "条消息往返"
    else:
        token_main = ""
        token_sub = ""
    if token_main:
        tk_y = big_y + big_size + 100
        f_tk_main = _font(bold=True, size=84)
        d.text((80, tk_y), token_main, font=f_tk_main, fill=(245, 245, 250))
        bbox = d.textbbox((0, 0), token_main, font=f_tk_main)
        f_tk_sub = _font(bold=False, size=36)
        d.text((80 + (bbox[2] - bbox[0]) + 24, tk_y + 28), token_sub,
               font=f_tk_sub, fill=(140, 220, 220))

    # Per-source bars (proportional)
    if req.per_source:
        items = sorted(req.per_source.items(), key=lambda kv: -kv[1])
        total = max(1, _per_source_total(req.per_source))
        bar_x = 80
        bar_y = int(h * 0.72) if w == h else int(h * 0.78)
        bar_w = w - 160
        bar_h = 18
        gap = 14
        f_bar_label = _font(bold=False, size=26)
        for k, v in items[:4]:
            pct = v / total
            d.rounded_rectangle((bar_x, bar_y, bar_x + bar_w, bar_y + bar_h),
                                radius=9, fill=(40, 40, 60))
            fill_w = int(bar_w * pct)
            grad_bar = _vertical_gradient((max(2, fill_w), bar_h),
                                           (160, 100, 250), (80, 230, 230))
            mask_bar = Image.new("L", grad_bar.size, 0)
            mdb = ImageDraw.Draw(mask_bar)
            mdb.rounded_rectangle((0, 0, fill_w, bar_h), radius=9, fill=255)
            img.paste(grad_bar, (bar_x, bar_y), mask_bar)
            label = f"{k}  {v}"
            d.text((bar_x, bar_y - 30), label, font=f_bar_label, fill=(200, 200, 220))
            bar_y += bar_h + gap + 24

    if req.watermark:
        _draw_watermark(d, w, h, (140, 140, 160, 255))
    return img


# ---- v3: quadrant report -----------------------------------------------

def render_v3(req: StatsRequest, w: int, h: int) -> Image.Image:
    """Light card. Four quadrants. Reads like a tiny analytics report."""
    img = Image.new("RGB", (w, h), (245, 246, 248))
    d = ImageDraw.Draw(img, "RGBA")

    # Title bar
    f_title = _font(bold=True, size=44)
    title = f"会话统计 · 过去 {req.window_days} 天"
    d.text((60, 60), title, font=f_title, fill=(30, 30, 40))
    f_subtitle = _font(bold=False, size=26)
    d.text((60, 116), "llm-recall stats", font=f_subtitle, fill=(140, 140, 150))

    # Quadrant geometry
    pad = 60
    gap = 24
    top = 200
    bottom = h - 140
    cell_w = (w - pad * 2 - gap) // 2
    cell_h = (bottom - top - gap) // 2

    cells = [
        (pad, top),
        (pad + cell_w + gap, top),
        (pad, top + cell_h + gap),
        (pad + cell_w + gap, top + cell_h + gap),
    ]

    for x, y in cells:
        d.rounded_rectangle((x, y, x + cell_w, y + cell_h),
                            radius=22, fill=(255, 255, 255))

    # Q1: total sessions
    x, y = cells[0]
    f_q_label = _font(bold=False, size=26)
    d.text((x + 28, y + 24), "TOTAL SESSIONS", font=f_q_label, fill=(140, 140, 160))
    f_q_main = _font(bold=True, size=140)
    val = _format_number(req.total_sessions)
    d.text((x + 28, y + 64), val, font=f_q_main, fill=(30, 30, 40))
    f_q_sub = _font(bold=False, size=28)
    if req.total_tokens > 0:
        sub = f"{_format_number(req.total_tokens)} tokens"
    elif req.total_messages > 0:
        sub = f"{_format_number(req.total_messages)} 消息"
    else:
        sub = ""
    if sub:
        d.text((x + 28, y + cell_h - 60), sub, font=f_q_sub, fill=(120, 120, 140))

    # Q2: top topics
    x, y = cells[1]
    d.text((x + 28, y + 24), "TOP TOPICS", font=f_q_label, fill=(140, 140, 160))
    topics = _truncate_topics(req.top_topics, 5)
    f_topic = _font(bold=True, size=34)
    f_topic_idx = _font(bold=False, size=26)
    ty = y + 76
    for i, t in enumerate(topics, start=1):
        d.text((x + 28, ty), f"{i:>2}", font=f_topic_idx, fill=(180, 180, 200))
        d.text((x + 80, ty - 6), t, font=f_topic, fill=(30, 30, 40))
        ty += 52

    # Q3: per-source share
    x, y = cells[2]
    d.text((x + 28, y + 24), "PER SOURCE", font=f_q_label, fill=(140, 140, 160))
    items = sorted(req.per_source.items(), key=lambda kv: -kv[1])
    total = max(1, _per_source_total(req.per_source))
    f_src = _font(bold=False, size=30)
    palette = [(120, 90, 240), (60, 200, 200), (240, 140, 80), (160, 200, 80)]
    by = y + 80
    bar_x0 = x + 28
    bar_w = cell_w - 56
    for i, (k, v) in enumerate(items[:4]):
        pct = v / total
        d.text((bar_x0, by), f"{k}", font=f_src, fill=(60, 60, 80))
        d.text((bar_x0 + cell_w - 100, by), f"{v}", font=f_src, fill=(60, 60, 80))
        d.rounded_rectangle((bar_x0, by + 38, bar_x0 + bar_w, by + 50),
                            radius=6, fill=(232, 232, 240))
        d.rounded_rectangle((bar_x0, by + 38, bar_x0 + int(bar_w * pct), by + 50),
                            radius=6, fill=palette[i % len(palette)])
        by += 76

    # Q4: longest session
    x, y = cells[3]
    d.text((x + 28, y + 24), "LONGEST SESSION", font=f_q_label, fill=(140, 140, 160))
    f_q4 = _font(bold=True, size=120)
    d.text((x + 28, y + 76), f"{req.longest_session_hours:.1f}",
           font=f_q4, fill=(30, 30, 40))
    f_q4_unit = _font(bold=False, size=36)
    d.text((x + 28, y + 220), "小时连续会话",
           font=f_q4_unit, fill=(120, 120, 140))

    if req.watermark:
        _draw_watermark(d, w, h, (160, 160, 170, 255))
    return img


# ---- dispatch -----------------------------------------------------------

def render(req: StatsRequest) -> bytes:
    """Render → PNG bytes per req.template + req.format."""
    if req.format == "square":
        w, h = 1080, 1080
    else:
        w, h = 1080, 1920

    if req.template == "v1":
        img = render_v1(req, w, h)
    elif req.template == "v2":
        img = render_v2(req, w, h)
    elif req.template == "v3":
        img = render_v3(req, w, h)
    else:  # pragma: no cover — pydantic enforces literal
        img = render_v1(req, w, h)

    buf = io.BytesIO()
    img.save(buf, format="PNG", optimize=True)
    return buf.getvalue()
