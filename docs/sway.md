# Sway Desktop

The `sway` module links a complete [sway](https://swaywm.org/) Wayland desktop:
the compositor config, a [waybar](https://github.com/Alexays/Waybar) status bar,
[swaync](https://github.com/ErikReider/SwayNotificationCenter) notifications, and
three helper scripts.

**During `install all`, the whole module no-ops when `sway` is not on `PATH`** —
including `Status()`, which reports nothing rather than "missing". Installing on
a WSL box or a headless server is therefore safe and silent: it will never
apt-get a compositor onto a machine that did not ask for one.

**Naming the module explicitly is the opt-in.** `dfinstall install sway` treats
the request as "set this desktop up" and will install any missing packages from
the list below before linking, so it bootstraps a bare machine. The two paths
are distinguished by `core.InstallingAll`, which `BeginInstall` sets from the
session options — so the CLI and the MCP server cannot answer it differently.

## Package requirements

The configs name binaries directly — in keybinds, in waybar `exec`/`on-click`,
in swaync's button grid — and **a missing one does not announce itself**. It
produces a key that does nothing, a bar module that silently hides, or a panel
button that no-ops. So the dependency set is declared as data in
`src/modules/sway.go` (`swayPackages`) rather than living in someone's memory.

| Group | Packages | What breaks without it |
|---|---|---|
| Compositor / session | `sway` `swaybg` `swayidle` `swaylock` `swaynag` `xwayland` | wallpaper, idle-lock chain, `$lock`, the exit prompt, X11 clients |
| Launcher / backlight | `fuzzel` `brightnessctl` | `$mod+d`; brightness keys when swayosd is absent |
| Bar / notifications | `waybar` `sway-notification-center` `mako-notifier` | the bar; the panel and bell; the documented fallback daemon |
| Status-area features | `swayosd` `pavucontrol` `playerctl` `pipewire-pulse` `wireplumber` | the on-screen volume/brightness overlay, the mixer on right-click, the mpris module and media keys |
| Screenshots | `grim` `slurp` `wl-clipboard` | `Print` / `Shift+Print` and the panel's two capture buttons |
| Applets | `network-manager-gnome` `blueman` | clicking the network module; Bluetooth pairing |

Two deliberate omissions: **no terminal** (`set $term ghostty`, which has its own
module and is not an apt package on Debian), and **no fonts** (the `fonts` module
owns `IosevkaTerm`, which every stylesheet here names).

Presence is checked by probing for each package's **binary on `PATH`**, not by
asking dpkg. A package query is per-distro and says nothing about a compositor
built from source or installed to `/opt` — which is this machine's own case
(`/opt/sway-next/bin`). What the configs require is that the command is
callable, so that is what gets tested. `dfinstall status` reports any gap in its
INFO column:

```
MODULE            LINKED  MISSING  INFO
sway                   9        0  1 package(s) missing: pavucontrol
```

## What gets linked

| Source | Destination |
|--------|-------------|
| `config/sway/config` | `~/.config/sway/config` |
| `config/sway/sway-portals.conf` | `~/.config/xdg-desktop-portal/sway-portals.conf` |
| `config/waybar/config` | `~/.config/waybar/config` |
| `config/waybar/style.css` | `~/.config/waybar/style.css` |
| `config/swaync/config.json` | `~/.config/swaync/config.json` |
| `config/swaync/style.css` | `~/.config/swaync/style.css` |
| `config/mako/config` | `~/.config/mako/config` |
| `config/swayosd/style.css` | `~/.config/swayosd/style.css` |
| `config/sway/sway-powermenu` | `~/.local/bin/sway-powermenu` |
| `config/sway/sway-quickpanel` | `~/.local/bin/sway-quickpanel` |
| `config/sway/sway-brightness` | `~/.local/bin/sway-brightness` |

The three scripts are `chmod 0755` at the **source** before linking, because a
symlink inherits its target's mode.

## Manual steps the module deliberately does not do

Both write outside `$HOME` and need root, so they are documented rather than
automated.

**1. Backlight permissions.** `brightnessctl` 0.5 has no logind support and
writes sysfs directly, and Debian ships no udev rule granting that access. Until
you install one, the `XF86MonBrightness*` keys silently no-op:

```bash
sudo install -m 0644 config/sway/90-backlight.rules /etc/udev/rules.d/
sudo udevadm control --reload && sudo udevadm trigger -s backlight
sudo usermod -aG video "$USER"     # then log out and back in
```

> **Largely superseded by `swayosd`.** Its package ships
> `/usr/lib/udev/rules.d/99-swayosd.rules`, which does exactly what the rule
> above does (`chgrp video` + `chmod g+w` on `backlight/*/brightness`). If
> swayosd is installed you can skip this step — you still need to be in the
> `video` group. The vendored copy stays for machines without swayosd, and
> because a package-managed rule can vanish on removal.
>
> Note swayosd's shipped polkit rule grants only the **`wheel`** group, which
> Debian does not use by default. That affects `swayosd-libinput-backend`
> (the caps-lock overlay) only; volume and brightness work without it.

**2. Output layout.** See below.

## Host-specific output layout

`config/sway/config` ends its Outputs section with:

```
include $HOME/.config/sway/outputs.conf
```

That file is **not tracked in this repo and not linked by the module**, because
sway matches an output by identifier (`"make model serial"`) — which embeds each
panel's serial number and is therefore machine identity, not shareable config.
Copy `config/sway/outputs.conf.example` to `~/.config/sway/outputs.conf` and fill
in your own. A missing include is silently tolerated by sway, so a machine
without one simply gets default auto-placement.

Discover your identifiers with:

```bash
swaymsg -t get_outputs | grep -E 'make|model|serial|Output'
```

> [!IMPORTANT]
> Match external displays by **identifier, not connector name**. A
> DisplayLink/evdi output's connector is assigned in attach order, so it
> enumerates as `DVI-I-1` one session and `DVI-I-2` the next — a connector-name
> match silently stops applying after a replug. Built-in laptop panels are the
> exception: `eDP-1` is stable.

## Keybindings

`$mod` is **Mod4 (Super)**. See [Keybindings](keybindings.md) for the full table
and the reasoning behind the Alt reservation.

Notifications and media:

| Key | Action |
|---|---|
| `$mod+n` | Toggle the swaync control centre |
| `$mod+Shift+n` | Push the newest popup off screen (`--hide-latest`; it stays in history) |
| `XF86Audio{Raise,Lower}Volume` | Volume ±, with the swayosd overlay |
| `XF86AudioMute` / `XF86AudioMicMute` | Mute sink / source |
| `XF86MonBrightness{Up,Down}` | Brightness ±, floored — see *Brightness has a floor* |

> The **volume** keys are written as `swayosd-client … || <fallback>`, so a
> machine without swayosd falls back to `wpctl` rather than going silent. The
> **brightness** keys go through `sway-brightness` instead, which owns the
> floor and degrades to `brightnessctl` by itself. Test the fallback by renaming `swayosd-client`, not by trusting the
> `||`. Do not drop the `sh -c` wrapper: sway tokenizes the bindsym before the
> shell sees it, so a bare `||` is parsed by sway, not `/bin/sh`.
>
> `$mod+Shift+n` is deliberately **not** `swaync-client -C` (close-all) — that
> drops the notifications from history rather than dismissing the popup.

> [!WARNING]
> This config binds exactly **one** Alt key (`Mod1+space`, launcher). Sway grabs
> keys at the compositor, *before* the terminal sees them, so any further Alt
> binding silently steals a tmux binding — and `config/tmux/tmux.conf` uses
> `M-a` as prefix plus no-prefix binds on `M-1`…`M-9`, `M-n`, `M-Enter`, `M--`,
> `M-q`, `M-m`, `M-s`, `M-c`. Audit with:
> ```bash
> grep -nE '^bindsym.*(Mod1|Alt)' config/sway/config   # expect exactly one line
> ```

## Two traps worth knowing before you edit

**`~/.local/bin` is not on the session `PATH`.** A display manager starts sway
without a login shell, so a bare command name in a waybar `on-click` or a sway
`exec` **fails silently**. Write `$HOME/.local/bin/<name>` — waybar runs
`on-click` through `/bin/sh -c`, so `$HOME` expands.

**Autostart must reload in place, never kill-and-respawn.** Sway forks `exec`
lines asynchronously, so the common `exec_always pkill -x waybar` +
`exec_always waybar` pair races itself and kills the instance it just started.
The working shape is *reload the running instance, and only spawn one if the
reload could not reach it*:

```
exec_always pkill -SIGUSR2 -x waybar || waybar
```

## The bar shows almost no numbers, on purpose

The status area follows the GNOME/macOS model rather than a dashboard: **at rest
each module is a single glyph; the exact value lives in its tooltip; the
adjustable things live in the panel behind them.** Before "fixing" this by
re-adding percentages, note what each removal bought:

- **Brightness has no module at all.** It has no meaningful at-rest state —
  nobody needs telling the screen is on — and the value only matters while you
  are changing it, which is when swayosd puts it centre-screen. Set it
  deliberately from the swaync panel's slider.
- **Volume is icon-only.** Same reasoning; the glyph still carries muted-or-not,
  which is the part you read at a glance.
- **Network is icon-only.** The glyph is a signal-strength ramp; SSID, address
  and dBm are one hover away.
- **Battery keeps its percentage**, and the clock keeps the date. These are read
  at rest rather than adjusted, so a number earns its place.

Colour is reserved to *mean* something: the accent marks the focused workspace,
the clock and the hover underline, and yellow/red are left free for warning and
critical states. An earlier scheme gave every module its own hue, and a red
battery warning had to compete with five other colours.

Modules that hide themselves entirely when idle: `privacy` (mic/camera/
screenshare), `mpris`, and `network#vpn`. An indicator that is always lit is one
you stop seeing.

## Brightness has a floor, and lies about it

**The screen can never be driven fully black**, and "all the way down" reads as
`0%` anyway — the same contract Android and GNOME offer. A dark panel is
indistinguishable from a laptop that is off, and the only way back is a key you
cannot see to find.

The hardware range is `5%..100%`; the user-facing range is `0..100` mapped onto
it:

```
real = 5 + user × 95 / 100          user = (real − 5) × 100 / 95
```

so user `0` is a real `5%` — dim but visibly lit. Both paths implement it:

| Path | Floor | Shows |
|---|---|---|
| `XF86MonBrightness{Up,Down}` → `sway-brightness` | yes | swayosd OSD, user scale |
| `sway-quickpanel` slider | yes (`BRIGHTNESS_MIN`) | user scale |
| swaync panel slider | **removed** — see below | — |

> The formula is duplicated in `config/sway/sway-brightness` and
> `sway-quickpanel`'s `brightness_to_user()`. **Change one and you must change
> the other**, or the keys and the slider will disagree about where the bottom
> is.

Two implementation notes worth keeping:

- **The user value is remembered in a state file, never re-derived per press.**
  This is not an optimisation; deriving it caused a real bug where **brightness
  could not be taken below 91% at all**. The two scales are integer maths and
  are not mutual inverses, so they have fixed points:

  ```
  user 90 -> real 91      (5 + round(90 × 95 / 100))
  real 91 -> user 91      (round((91 − 5) × 100 / 95))
  ```

  Pressing "down" at real 91 snapped 91→90, wrote real 91, and read back 91 —
  forever, with no error. `$XDG_RUNTIME_DIR/sway-brightness.state` holds
  `<user> <real>`; the recorded real is an integrity check, so if anything else
  moves the backlight (the quickpanel, a lid event) the stale user value is
  discarded and re-derived. A corrupt file is ignored.

  Steps also snap to the 5% grid, and a final guard forces the value to move at
  least one percent per press, so a coarse device can never produce a dead key.
- **swayosd is not asked to set the brightness.** Debian trixie ships swayosd
  **0.1.0**, which has no minimum and will happily drive the backlight to zero.
  Upstream added `min_brightness` in **0.3.x** (defaulting to 5) along with
  `--custom-progress`, which can draw a bar at an arbitrary fraction — that
  combination would let the OSD show a proper *bar* at the user value. Debian
  stable has neither and there is no backport, so `sway-brightness` sets the
  value itself and asks swayosd only to display it. **It probes for
  `--custom-progress` at runtime**, so if swayosd is ever upgraded the numeric
  OSD becomes a bar with no change here.

### Why swaync's backlight widget was removed

swaync's slider writes raw sysfs with no minimum and exposes no `min` option, so
dragging it to the bottom blanks the screen — defeating the floor entirely.
Fixing it properly means forking swaync (Vala) and carrying a build, which is
not worth it for a slider that already exists, floored, one middle-click away on
the volume module. The config block is kept in `config/swaync/config.json` as a
comment with the exact JSON to restore it.

## Two GTK dialects, one desktop

This is the sharpest edge in the module. The stylesheets it links are **not all
the same language**:

| File | Toolkit | Checked with |
|---|---|---|
| `config/waybar/style.css` | GTK **3** | `Gtk 3.0` provider |
| `config/swaync/style.css` | GTK **3** | `Gtk 3.0` provider |
| `config/swayosd/style.css` | GTK **4** | `Gtk 4.0` provider |

Confirmed by linkage, not by assumption — `swayosd-server` pulls `libgtk-4.so.1`
while `waybar` and `swaync` pull `libgtk-3.so.0`. It matters because:

- GTK4 supports `var()` and custom properties; **GTK3 does not** (this is why
  most attractive swaync themes on GitHub cannot simply be copied in — a
  `tokens/variables.css` full of `var()` is the giveaway that a theme is GTK4).
- GTK3 rejects `transform`, `filter` and `text-transform` outright.
- **On an unknown property GTK3 discards the whole stylesheet** — including
  rules above the offending line — and the app falls back to raw Adwaita. GTK4
  drops only that declaration.

So validating a GTK3 file with a GTK4 parser (or the reverse) gives a confidently
wrong answer. Parse each against its own toolkit; see *Validating a change*.

## The waybar icons are fragile

`config/waybar/config` contains literal Nerd Font glyphs in the Material Design
private-use plane (U+F0000+). They are invisible in most diffs and easy to
destroy — an editor or script that is not UTF-8 clean will strip them and the
bar renders icon-less without erroring. After editing, check:

```bash
python3 -c "print(sum(1 for c in open('config/waybar/config',encoding='utf-8').read() if ord(c)>0xF0000))"
```

A healthy file reports 40+.

## Validating a change

A broken sway config leaves you at a black screen, so validate before logging in:

```bash
sway --validate -c config/sway/config
# Off a graphical session, which needs a backend hint:
env WLR_BACKENDS=headless sway --validate -c config/sway/config
```

`--validate` does more than parse: it checks that the background file actually
exists, and it reads `include`d files (errors name the included path and line).

Reload a running session without logging out:

```bash
swaymsg reload                    # sway + anything on exec_always
pkill -SIGUSR2 waybar             # waybar only
swaync-client --reload-config && swaync-client --reload-css
pkill -x swayosd-server           # restarted by exec_always on the next reload
```

**Parse a stylesheet before you ship it** — and against the right toolkit (see
*Two GTK dialects* above). A GTK3 file with one bad property renders as raw
Adwaita rather than erring, so "it still looked styled" is not evidence:

```bash
# GTK3 — waybar, swaync.  Swap 3.0 for 4.0 to check config/swayosd/style.css.
python3 - config/waybar/style.css <<'PY'
import sys, gi
gi.require_version("Gtk", "3.0")
from gi.repository import Gtk, Gio
p = Gtk.CssProvider()
p.connect("parsing-error", lambda _p, _s, e: print("ERROR:", e.message))
try:
    p.load_from_file(Gio.File.new_for_path(sys.argv[1]))
    print("parsed clean")
except Exception as exc:
    print("REJECTED —", exc)
PY
```

Under sway you can also verify the result rather than trusting it: `grim -c -g
"<x>,<y> <w>x<h>" out.png` captures a region with the cursor drawn, which is how
the tooltips and the swayosd overlay were checked. Note `grim -g` takes **global**
coordinates while `swaymsg seat seat0 cursor set` takes **output-relative** ones —
on a multi-head layout those differ by the output's origin, which is an easy hour
to lose.

## Notifications: swaync, with mako as fallback

swaync is the active daemon. mako stays installed and its config stays managed,
but mako 1.10 has **no fade, animation, transition or opacity option of any
kind**, so notifications can only blink out instantly — that is why it lost. To
switch back, replace the swaync line in the autostart block with:

```
exec_always makoctl reload || mako
```

> [!NOTE]
> `mako`'s built-in `default-timeout` is `0`, meaning *never expire*. With no
> mako config present at all, every notification stays on screen until clicked.
> `config/mako/config` exists primarily to fix that.

If your distro packages `waybar.service` / `mako.service` / `swaync.service` as
`WantedBy=graphical-session.target`, mask them — that target is reached by GNOME
too, so they fire and fail on every GNOME login. Sway launches all three itself
via `exec_always`:

```bash
systemctl --user mask waybar.service mako.service swaync.service
```
