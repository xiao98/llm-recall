"""Sanity check that the renderer reads fonts ONLY from backend/fonts/.

Run: backend/.venv/Scripts/python.exe test_fonts.py

This is a hard requirement (W5 §0 acceptance #4): production runs on a 1G
Ubuntu box that has no system CJK fonts. If anyone wires the renderer to
PIL's default font lookup the deploy will silently produce tofu boxes for
every Chinese character.
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

from templates.stats_card import _font, loaded_font_path


def main() -> None:
    fonts_dir = os.path.join(HERE, "fonts")
    f_reg = _font(bold=False, size=36)
    f_bold = _font(bold=True, size=72)

    # PIL FreeTypeFont exposes .path
    reg_path = os.path.realpath(f_reg.path)
    bold_path = os.path.realpath(f_bold.path)

    print(f"regular: {reg_path}")
    print(f"bold:    {bold_path}")

    assert reg_path.startswith(os.path.realpath(fonts_dir)), \
        f"regular font NOT under backend/fonts/: {reg_path}"
    assert bold_path.startswith(os.path.realpath(fonts_dir)), \
        f"bold font NOT under backend/fonts/: {bold_path}"

    # And the helper API agrees.
    assert loaded_font_path(False) == os.path.join(fonts_dir, "NotoSansSC-Regular.otf")
    assert loaded_font_path(True) == os.path.join(fonts_dir, "NotoSansSC-Bold.otf")

    print("OK — all fonts come from backend/fonts/")


if __name__ == "__main__":
    main()
