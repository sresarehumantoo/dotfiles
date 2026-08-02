# Sway Desktop

The `sway` module links a complete [sway](https://swaywm.org/) Wayland desktop:
the compositor config, a [waybar](https://github.com/Alexays/Waybar) status bar,
[swaync](https://github.com/ErikReider/SwayNotificationCenter) notifications, and
two helper scripts.

The whole module **no-ops when `sway` is not on `PATH`** — including `Status()`,
which reports nothing rather than "missing". Installing on a WSL box or a
headless server is therefore safe and silent.

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
| `config/sway/sway-powermenu` | `~/.local/bin/sway-powermenu` |
| `config/sway/sway-quickpanel` | `~/.local/bin/sway-quickpanel` |

The two scripts are `chmod 0755` at the **source** before linking, because a
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
```

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
