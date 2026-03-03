#!/usr/bin/env python3
"""Generate the dfinstall SVG logo.

Go-themed: embeds the Go gopher SVG paths inline, symlink arrow motif, and dotfile dots.

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

# ── Gopher SVG elements (from gopher.svg, 402x559 original) ─────
# Extracted paths grouped logically for embedding in a <g> with transform.
GOPHER_PATHS = """\
<!-- left hand -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#F6D2A2" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M10.634,300.493c0.764,15.751,16.499,8.463,23.626,3.539c6.765-4.675,8.743-0.789,9.337-10.015
  c0.389-6.064,1.088-12.128,0.744-18.216c-10.23-0.927-21.357,1.509-29.744,7.602C10.277,286.542,2.177,296.561,10.634,300.493"/>
<path fill-rule="evenodd" clip-rule="evenodd" fill="#C6B198" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M10.634,300.493c2.29-0.852,4.717-1.457,6.271-3.528"/>

<!-- left ear -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#6AD7E5" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M46.997,112.853C-13.3,95.897,31.536,19.189,79.956,50.74L46.997,112.853z"/>
<!-- right ear -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#6AD7E5" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M314.895,44.984c47.727-33.523,90.856,42.111,35.388,61.141L314.895,44.984z"/>

<!-- right foot -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#F6D2A2" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M325.161,494.343c12.123,7.501,34.282,30.182,16.096,41.18c-17.474,15.999-27.254-17.561-42.591-22.211
  C305.271,504.342,313.643,496.163,325.161,494.343z"/>
<path fill-rule="evenodd" clip-rule="evenodd" fill="none" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M341.257,535.522c-2.696-5.361-3.601-11.618-8.102-15.939"/>

<!-- left foot -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#F6D2A2" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M108.579,519.975c-14.229,2.202-22.238,15.039-34.1,21.558c-11.178,6.665-15.454-2.134-16.461-3.92
  c-1.752-0.799-1.605,0.744-4.309-1.979c-10.362-16.354,10.797-28.308,21.815-36.432C90.87,496.1,100.487,509.404,108.579,519.975z"/>
<path fill-rule="evenodd" clip-rule="evenodd" fill="none" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M58.019,537.612c0.542-6.233,5.484-10.407,7.838-15.677"/>

<!-- ear inner darks -->
<path fill-rule="evenodd" clip-rule="evenodd" d="M49.513,91.667c-7.955-4.208-13.791-9.923-8.925-19.124
  c4.505-8.518,12.874-7.593,20.83-3.385L49.513,91.667z"/>
<path fill-rule="evenodd" clip-rule="evenodd" d="M337.716,83.667c7.955-4.208,13.791-9.923,8.925-19.124
  c-4.505-8.518-12.874-7.593-20.83-3.385L337.716,83.667z"/>

<!-- right hand -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#F6D2A2" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M392.475,298.493c-0.764,15.751-16.499,8.463-23.626,3.539c-6.765-4.675-8.743-0.789-9.337-10.015
  c-0.389-6.064-1.088-12.128-0.744-18.216c10.23-0.927,21.357,1.509,29.744,7.602C392.831,284.542,400.932,294.561,392.475,298.493"/>
<path fill-rule="evenodd" clip-rule="evenodd" fill="#C6B198" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M392.475,298.493c-2.29-0.852-4.717-1.457-6.271-3.528"/>

<!-- body -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#6AD7E5" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M195.512,13.124c60.365,0,116.953,8.633,146.452,66.629c26.478,65.006,17.062,135.104,21.1,203.806
  c3.468,58.992,11.157,127.145-16.21,181.812c-28.79,57.514-100.73,71.982-160,69.863c-46.555-1.666-102.794-16.854-129.069-59.389
  c-30.826-49.9-16.232-124.098-13.993-179.622c2.652-65.771-17.815-131.742,3.792-196.101
  C69.999,33.359,130.451,18.271,195.512,13.124"/>

<!-- right eye white -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" stroke="#000" stroke-width="2.908" stroke-linecap="round" d="
  M206.169,94.16c10.838,63.003,113.822,46.345,99.03-17.197C291.935,19.983,202.567,35.755,206.169,94.16"/>
<!-- left eye white -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" stroke="#000" stroke-width="2.821" stroke-linecap="round" d="
  M83.103,104.35c14.047,54.85,101.864,40.807,98.554-14.213C177.691,24.242,69.673,36.957,83.103,104.35"/>

<!-- nose bridge -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M218.594,169.762c0.046,8.191,1.861,17.387,0.312,26.101c-2.091,3.952-6.193,4.37-9.729,5.967c-4.89-0.767-9.002-3.978-10.963-8.552
  c-1.255-9.946,0.468-19.576,0.785-29.526L218.594,169.762z"/>

<!-- left pupil -->
<ellipse fill-rule="evenodd" clip-rule="evenodd" cx="107.324" cy="95.404" rx="14.829" ry="16.062"/>
<ellipse fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" cx="114.069" cy="99.029" rx="3.496" ry="4.082"/>

<!-- right pupil -->
<ellipse fill-rule="evenodd" clip-rule="evenodd" cx="231.571" cy="91.404" rx="14.582" ry="16.062"/>
<ellipse fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" cx="238.204" cy="95.029" rx="3.438" ry="4.082"/>

<!-- nose left -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#FFF" stroke="#000" stroke-width="3" stroke-linecap="round" d="
  M176.217,168.87c-6.47,15.68,3.608,47.035,21.163,23.908c-1.255-9.946,0.468-19.576,0.785-29.526L176.217,168.87z"/>

<!-- mouth -->
<path fill-rule="evenodd" clip-rule="evenodd" fill="#F6D2A2" stroke="#231F20" stroke-width="3" stroke-linecap="round" d="
  M178.431,138.673c-12.059,1.028-21.916,15.366-15.646,26.709c8.303,15.024,26.836-1.329,38.379,0.203
  c13.285,0.272,24.17,14.047,34.84,2.49c11.867-12.854-5.109-25.373-18.377-30.97L178.431,138.673z"/>
<path fill-rule="evenodd" clip-rule="evenodd" d="M176.913,138.045c-0.893-20.891,38.938-23.503,43.642-6.016
  C225.247,149.475,178.874,153.527,176.913,138.045C175.348,125.682,176.913,138.045,176.913,138.045z"/>
"""


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


def dot_pattern(x_start, y_start, cols, rows, spacing, r, color, base_opacity=0.15):
    """Grid of faint dots — the 'dotfiles' visual motif."""
    dots = []
    for row in range(rows):
        for col in range(cols):
            dist = math.sqrt(row**2 + col**2) / math.sqrt(rows**2 + cols**2)
            op = round(base_opacity * (1 - dist * 0.6), 3)
            cx = x_start + col * spacing
            cy = y_start + row * spacing
            dots.append(circle(cx, cy, r, color, op))
    return "\n    ".join(dots)


def gopher_group():
    """Embed gopher SVG paths scaled and positioned for the left side of the logo.

    Original gopher: 402x559. Target area: ~160x220, centered vertically.
    Scale = 220/559 ≈ 0.394.  Translate so the gopher sits in the left ~200px.
    """
    scale = 0.394
    # Center the scaled gopher vertically: (260 - 559*0.394) / 2 ≈ 20
    # Horizontally: offset ~20px from left edge
    tx = 20
    ty = 20
    return (
        f'<g transform="translate({tx},{ty}) scale({scale})">\n'
        f'{GOPHER_PATHS}'
        f'</g>'
    )


def gopher_glow():
    """Radial gradient glow behind the gopher."""
    # Center of the gopher area (roughly 100, 130)
    return (
        '<radialGradient id="gopherGlow" cx="100" cy="130" r="120" '
        'gradientUnits="userSpaceOnUse">\n'
        f'  <stop offset="0%" stop-color="{GO_BLUE}" stop-opacity="0.18"/>\n'
        f'  <stop offset="100%" stop-color="{GO_BLUE}" stop-opacity="0"/>\n'
        '</radialGradient>'
    )


def gopher_overlays():
    """Overlay elements on/near the gopher: symlink arrow + dotfile dots."""
    parts = []

    # Symlink arrow near the right hand — positioned to the right of the gopher
    # Right hand is at roughly x=392*0.394+20 ≈ 174, y=298*0.394+20 ≈ 137
    # Place arrow starting just past the hand
    parts.append(
        f'<g opacity="0.9">'
        f'  <text x="170" y="138" font-family="monospace, \'SF Mono\', Consolas, monospace" '
        f'font-size="18" font-weight="bold" fill="{GO_WHITE}" opacity="0.95">-&gt;</text>'
        f'</g>'
    )

    # Three small colored dots on the gopher's belly (the "dotfiles" motif)
    # Belly center is roughly x=195*0.394+20 ≈ 97, y=350*0.394+20 ≈ 158
    belly_cx, belly_cy = 97, 162
    dot_colors = [GO_BLUE, ACCENT, TEXT_DIM]
    dot_spacing = 14
    for i, color in enumerate(dot_colors):
        dx = belly_cx + (i - 1) * dot_spacing
        parts.append(circle(dx, belly_cy, 4, color, 0.9))

    return "\n  ".join(parts)


def build_svg():
    # Text area starts after the gopher region
    text_x = 210

    parts = [
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">',
        f'  <defs>',
        f'    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">',
        f'      <stop offset="0%" stop-color="{BG_DARK}"/>',
        f'      <stop offset="100%" stop-color="#24283B"/>',
        f'    </linearGradient>',
        f'    {gopher_glow()}',
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
        f'  <!-- gopher glow -->',
        f'  <circle cx="100" cy="130" r="120" fill="url(#gopherGlow)"/>',
        "",
        f'  <!-- gopher -->',
        f'  {gopher_group()}',
        "",
        f'  <!-- gopher overlays (symlink arrow + dotfile dots) -->',
        f'  {gopher_overlays()}',
        "",
        f'  <!-- title -->',
        f'  {text(text_x, 100, "dfinstall", 52, GO_WHITE, weight="bold")}',
        "",
        f'  <!-- subtitle -->',
        f'  {text(text_x, 135, "dotfiles manager", 22, GO_LIGHT)}',
        "",
        f'  <!-- symlink arrows -->',
        f'  <g opacity="0.9">',
        f'    {symlink_arrow(text_x, 168, 55, GO_BLUE)}',
        f'    {symlink_arrow(text_x + 65, 168, 55, GO_DARK_BLUE)}',
        f'    {symlink_arrow(text_x + 130, 168, 55, ACCENT)}',
        f'  </g>',
        "",
        f'  <!-- tag line -->',
        f'  {text(text_x, 205, "Symlink configs. Install tools. One command.", 15, TEXT_DIM)}',
        "",
        f'  <!-- language badges -->',
        f'  <g>',
        f'    {rounded_rect(text_x, 220, 52, 22, 6, GO_DARK_BLUE, 0.85)}',
        f'    {text(text_x + 26, 236, "Go", 13, GO_WHITE, anchor="middle", weight="bold")}',
        f'    {rounded_rect(text_x + 58, 220, 56, 22, 6, "#3B4261", 0.7)}',
        f'    {text(text_x + 86, 236, "CLI", 13, GO_LIGHT, anchor="middle")}',
        f'    {rounded_rect(text_x + 120, 220, 66, 22, 6, "#3B4261", 0.7)}',
        f'    {text(text_x + 153, 236, "WSL2", 13, GO_LIGHT, anchor="middle")}',
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
