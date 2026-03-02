#!/usr/bin/env python3
"""Generate the dfinstall SVG logo.

Go-themed: uses the official Go color palette with a stylised gopher
silhouette, symlink arrow motif, and dotfile dots.

Usage:
    python3 assets/gen_logo.py          # writes assets/logo.svg
"""

import math
import os

# ── Go brand palette ──────────────────────────────────────────────
GO_BLUE      = "#00ADD8"
GO_DARK_BLUE = "#007D9C"
GO_LIGHT     = "#5DC9E2"
GO_WHITE     = "#FFFFFF"
BG_DARK      = "#1A1B26"   # Tokyo Night-ish background
TEXT_DIM     = "#7AA2F7"
ACCENT       = "#9ECE6A"   # green for the "ok" checkmark feel

W, H = 800, 260


def n(v):
    """Round a float to 1 decimal, drop trailing .0 for integers."""
    r = round(v, 1)
    return int(r) if r == int(r) else r


def circle(cx, cy, r, fill, opacity=1.0):
    op = f' opacity="{opacity}"' if opacity < 1.0 else ""
    return f'<circle cx="{n(cx)}" cy="{n(cy)}" r="{n(r)}" fill="{fill}"{op}/>'


def rounded_rect(x, y, w, h, rx, fill, opacity=1.0):
    op = f' opacity="{opacity}"' if opacity < 1.0 else ""
    return f'<rect x="{n(x)}" y="{n(y)}" width="{n(w)}" height="{n(h)}" rx="{n(rx)}" fill="{fill}"{op}/>'


def text(x, y, content, size, fill, anchor="start", weight="normal", family="monospace"):
    return (
        f'<text x="{x}" y="{y}" font-family="{family}, \'SF Mono\', Consolas, monospace" '
        f'font-size="{size}" font-weight="{weight}" fill="{fill}" text-anchor="{anchor}">'
        f'{content}</text>'
    )


def symlink_arrow(x, y, length=50, color=GO_BLUE):
    """A stylised symlink arrow: line with a small arrowhead."""
    x2 = x + length
    head = 8
    return (
        f'<line x1="{x}" y1="{y}" x2="{x2 - head}" y2="{y}" '
        f'stroke="{color}" stroke-width="2.5" stroke-linecap="round"/>'
        f'<polygon points="{x2},{y} {x2 - head},{n(y - head/2)} {x2 - head},{n(y + head/2)}" '
        f'fill="{color}"/>'
    )


def gopher_silhouette(cx, cy, scale=1.0):
    """Minimal gopher head silhouette built from ellipses & circles."""
    s = scale
    parts = []

    # Head (main ellipse)
    parts.append(
        f'<ellipse cx="{n(cx)}" cy="{n(cy)}" rx="{n(28*s)}" ry="{n(32*s)}" fill="{GO_BLUE}"/>'
    )
    # Ears
    for dx in [-20, 20]:
        parts.append(circle(cx + dx * s, cy - 30 * s, 10 * s, GO_DARK_BLUE))
        parts.append(circle(cx + dx * s, cy - 30 * s, 5 * s, "#F0C674"))
    # Eyes (white circles with black pupils)
    for dx in [-10, 10]:
        parts.append(circle(cx + dx * s, cy - 8 * s, 8 * s, GO_WHITE))
        parts.append(circle(cx + dx * s + 1.5 * s, cy - 8 * s, 4 * s, "#1A1B26"))
    # Nose / mouth area
    parts.append(
        f'<ellipse cx="{n(cx)}" cy="{n(cy + 10*s)}" rx="{n(14*s)}" ry="{n(10*s)}" fill="{GO_LIGHT}"/>'
    )
    # Tooth
    parts.append(rounded_rect(cx - 3 * s, cy + 6 * s, 6 * s, 8 * s, 1.5 * s, GO_WHITE))

    return "\n    ".join(parts)


def dot_pattern(x_start, y_start, cols, rows, spacing, r, color, base_opacity=0.15):
    """Grid of faint dots — the 'dotfiles' visual motif."""
    dots = []
    for row in range(rows):
        for col in range(cols):
            # Fade opacity based on distance from top-left
            dist = math.sqrt(row**2 + col**2) / math.sqrt(rows**2 + cols**2)
            op = round(base_opacity * (1 - dist * 0.6), 3)
            cx = x_start + col * spacing
            cy = y_start + row * spacing
            dots.append(circle(cx, cy, r, color, op))
    return "\n    ".join(dots)


def build_svg():
    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">',
        f'  <defs>',
        f'    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">',
        f'      <stop offset="0%" stop-color="{BG_DARK}"/>',
        f'      <stop offset="100%" stop-color="#24283B"/>',
        f'    </linearGradient>',
        f'  </defs>',
        "",
        f'  <!-- background -->',
        f'  {rounded_rect(0, 0, W, H, 16, "url(#bg)")}',
        "",
        f'  <!-- dot pattern (dotfiles motif) -->',
        f'  <g>',
        f'    {dot_pattern(580, 30, 12, 12, 20, 2.5, GO_LIGHT, 0.18)}',
        f'  </g>',
        "",
        f'  <!-- gopher -->',
        f'  <g>',
        f'    {gopher_silhouette(80, 120, 1.6)}',
        f'  </g>',
        "",
        f'  <!-- title -->',
        f'  {text(175, 100, "dfinstall", 52, GO_WHITE, weight="bold")}',
        "",
        f'  <!-- subtitle -->',
        f'  {text(175, 135, "dotfiles manager", 22, GO_LIGHT)}',
        "",
        f'  <!-- symlink arrows -->',
        f'  <g opacity="0.9">',
        f'    {symlink_arrow(175, 168, 55, GO_BLUE)}',
        f'    {symlink_arrow(240, 168, 55, GO_DARK_BLUE)}',
        f'    {symlink_arrow(305, 168, 55, ACCENT)}',
        f'  </g>',
        "",
        f'  <!-- tag line -->',
        f'  {text(175, 205, "Symlink configs. Install tools. One command.", 15, TEXT_DIM)}',
        "",
        f'  <!-- language badge -->',
        f'  <g>',
        f'    {rounded_rect(175, 220, 52, 22, 6, GO_DARK_BLUE, 0.85)}',
        f'    {text(201, 236, "Go", 13, GO_WHITE, anchor="middle", weight="bold")}',
        f'    {rounded_rect(233, 220, 56, 22, 6, "#3B4261", 0.7)}',
        f'    {text(261, 236, "CLI", 13, GO_LIGHT, anchor="middle")}',
        f'    {rounded_rect(295, 220, 66, 22, 6, "#3B4261", 0.7)}',
        f'    {text(328, 236, "WSL2", 13, GO_LIGHT, anchor="middle")}',
        f'  </g>',
        "",
        f'</svg>',
    ]
    return "\n".join(parts)


if __name__ == "__main__":
    svg = build_svg()
    out = os.path.join(os.path.dirname(__file__), "logo.svg")
    with open(out, "w") as f:
        f.write(svg)
    print(f"wrote {out}  ({len(svg)} bytes)")
