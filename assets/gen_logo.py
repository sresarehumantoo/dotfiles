#!/usr/bin/env python3
"""Generate the dfinstall SVG logo.

A convergence mark: three config files (YAML, JSON, .env) symlinked down into a
single terminal, next to the wordmark. It says what the tool does -- link your
configs into place on every machine -- and it is platform-neutral (Linux, WSL,
macOS), unlike the old Go-gopher banner.

Usage:
    python3 assets/gen_logo.py            # writes assets/logo.svg
    python3 assets/gen_logo.py out.svg    # writes elsewhere
"""

import os
import sys

# Tokyo Night palette, matching config/ and the terminal themes.
BG_TOP   = "#1A1B26"
BG_BOT   = "#24283B"
GRID     = "#2A2E42"
MINT     = "#73DACA"   # primary: file outlines, arrows
LIME     = "#9ECE6A"   # accent: link nodes, prompt glyph
BLUE     = "#7AA2F7"   # wordmark gradient end
CYAN     = "#2AC3DE"
TEXT     = "#C0CAF5"
DIM      = "#565F89"
TERM_BG  = "#16161E"

W, H = 820, 300


def file_glyph(x, y, label, w=70, h=86):
    """A rounded document with a folded corner and a couple of content lines."""
    fold = 18
    body = (
        f'<path d="M{x} {y+8} q0 -8 8 -8 H{x+w-fold} L{x+w} {y+fold} '
        f'V{y+h-8} q0 8 -8 8 H{x+8} q-8 0 -8 -8 Z" '
        f'fill="{TERM_BG}" stroke="{MINT}" stroke-width="3"/>'
    )
    corner = (
        f'<path d="M{x+w-fold} {y} V{y+fold} H{x+w} Z" '
        f'fill="none" stroke="{MINT}" stroke-width="3" stroke-linejoin="round"/>'
    )
    text = (
        f'<text x="{x+w/2}" y="{y+h*0.44}" font-family="monospace" '
        f'font-size="15" font-weight="700" fill="{MINT}" '
        f'text-anchor="middle">{label}</text>'
    )
    lines = ""
    for i, lw in enumerate((w * 0.62, w * 0.5, w * 0.4)):
        ly = y + h * 0.58 + i * 10
        col = LIME if i == 0 else DIM
        lines += (
            f'<line x1="{x+14}" y1="{ly}" x2="{x+14+lw}" y2="{ly}" '
            f'stroke="{col}" stroke-width="3" stroke-linecap="round"/>'
        )
    return f'<g>{body}{corner}{text}{lines}</g>'


def symlink(x1, y1, x2, y2, bend):
    """Dashed elbow link from a file down to the terminal, with a link node."""
    midy = (y1 + y2) / 2 + bend
    path = (
        f'<path d="M{x1} {y1} V{midy} H{x2} V{y2-10}" fill="none" '
        f'stroke="{MINT}" stroke-width="3" stroke-dasharray="2 7" '
        f'stroke-linecap="round" opacity="0.9"/>'
    )
    # arrowhead into the terminal
    head = (
        f'<path d="M{x2-6} {y2-12} L{x2} {y2-2} L{x2+6} {y2-12}" '
        f'fill="none" stroke="{MINT}" stroke-width="3" '
        f'stroke-linecap="round" stroke-linejoin="round"/>'
    )
    # link node (git-style) on the horizontal run
    node = (
        f'<circle cx="{(x1+x2)/2}" cy="{midy}" r="5" '
        f'fill="{BG_BOT}" stroke="{LIME}" stroke-width="2.5"/>'
    )
    return path + node + head


def terminal(cx, y, w=150, h=84):
    x = cx - w / 2
    chrome = (
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="12" '
        f'fill="{TERM_BG}" stroke="{MINT}" stroke-width="3"/>'
        f'<rect x="{x}" y="{y}" width="{w}" height="22" rx="12" fill="{MINT}"/>'
        f'<rect x="{x}" y="{y+11}" width="{w}" height="11" fill="{MINT}"/>'
    )
    dots = "".join(
        f'<circle cx="{x+16+i*16}" cy="{y+11}" r="3.5" fill="{TERM_BG}"/>'
        for i in range(3)
    )
    prompt = (
        f'<text x="{x+16}" y="{y+58}" font-family="monospace" font-size="26" '
        f'font-weight="700" fill="{LIME}">&#10095;</text>'
        f'<rect x="{x+34}" y="{y+40}" width="13" height="22" fill="{TEXT}"/>'
    )
    return f'<g filter="url(#glow)">{chrome}{dots}{prompt}</g>'


def chip(x, y, label, w):
    return (
        f'<g>'
        f'<rect x="{x}" y="{y}" width="{w}" height="26" rx="13" '
        f'fill="none" stroke="{DIM}" stroke-width="1.5"/>'
        f'<text x="{x+w/2}" y="{y+18}" font-family="monospace" font-size="13" '
        f'fill="{TEXT}" text-anchor="middle">{label}</text>'
        f'</g>'
    )


def build():
    # Icon layout (left square, ~0..300).
    f1 = file_glyph(40, 44, "YAML")
    f2 = file_glyph(118, 24, "JSON")
    f3 = file_glyph(196, 48, ".env")
    term_cx, term_y = 150, 196
    l1 = symlink(75, 130, term_cx - 36, term_y, 6)
    l2 = symlink(153, 110, term_cx, term_y, 22)
    l3 = symlink(231, 134, term_cx + 36, term_y, 6)
    term = terminal(term_cx, term_y)

    # Wordmark + tagline + platform chips (right).
    word = (
        f'<text x="332" y="150" font-family="Helvetica,Arial,sans-serif" '
        f'font-size="78" font-weight="800" fill="url(#word)" '
        f'letter-spacing="-2">dfinstall</text>'
    )
    tag = (
        f'<text x="336" y="188" font-family="monospace" font-size="18" '
        f'fill="{MINT}">symlink '
        f'<tspan fill="{DIM}">&#183;</tspan> install '
        f'<tspan fill="{DIM}">&#183;</tspan> converge</text>'
    )
    sub = (
        f'<text x="336" y="214" font-family="monospace" font-size="13.5" '
        f'fill="{DIM}">one command, every machine stays identical</text>'
    )
    chips = (
        chip(336, 234, "Linux", 62)
        + chip(406, 234, "WSL", 54)
        + chip(468, 234, "macOS", 64)
    )

    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}" role="img" aria-label="dfinstall">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="{BG_TOP}"/>
      <stop offset="1" stop-color="{BG_BOT}"/>
    </linearGradient>
    <linearGradient id="word" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="{MINT}"/>
      <stop offset="0.55" stop-color="{CYAN}"/>
      <stop offset="1" stop-color="{BLUE}"/>
    </linearGradient>
    <filter id="glow" x="-40%" y="-40%" width="180%" height="180%">
      <feGaussianBlur stdDeviation="5" result="b"/>
      <feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge>
    </filter>
    <pattern id="grid" width="26" height="26" patternUnits="userSpaceOnUse">
      <path d="M26 0H0V26" fill="none" stroke="{GRID}" stroke-width="1"/>
    </pattern>
  </defs>

  <rect width="{W}" height="{H}" rx="18" fill="url(#bg)"/>
  <rect width="{W}" height="{H}" rx="18" fill="url(#grid)" opacity="0.5"/>
  <line x1="306" y1="40" x2="306" y2="260" stroke="{GRID}" stroke-width="1.5"/>

  <!-- convergence mark -->
  {l1}{l2}{l3}
  {f1}{f2}{f3}
  {term}

  <!-- wordmark -->
  {word}{tag}{sub}{chips}
</svg>
'''
    return svg


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "logo.svg")
    with open(out, "w") as fh:
        fh.write(build())
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
