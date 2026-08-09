# Sway Desktop

The `sway` module links a complete [sway](https://swaywm.org/) Wayland desktop:
the compositor config, a [waybar](https://github.com/Alexays/Waybar) status bar,
[swaync](https://github.com/ErikReider/SwayNotificationCenter) notifications, and
four helper scripts.

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
| Applets | `network-manager-gnome` `blueman` | **right**-clicking the network module (left-click opens the control centre); Bluetooth pairing. nm-applet is also NetworkManager's secret agent, so without it wifi password prompts never appear |
| Calendar | `gir1.2-gtklayershell-0.1` | `sway-calendar` dies on import — and since waybar launches it from a click, the clock just silently does nothing |

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

> [!NOTE]
> **`gir1.2-gtklayershell-0.1` is the one entry that is not probed on `PATH`.** It
> ships a GObject-introspection typelib and no binary, so `exec.LookPath` would
> report it missing forever and `install sway` would reinstall it on every run.
> `swayPkgGlobs` in `src/modules/sway.go` checks for the typelib file instead,
> globbing both Debian's multiarch directory and the plain one so the pattern is
> not host-specific. Adding another binary-less package means adding a glob there.

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
| `config/sway/sway-calendar` | `~/.local/bin/sway-calendar` |
| `config/fuzzel/fuzzel.ini` | `~/.config/fuzzel/fuzzel.ini` |
| `config/gtk/gtk3-settings.ini` | `~/.config/gtk-3.0/settings.ini` |
| `config/gtk/gtk4-settings.ini` | `~/.config/gtk-4.0/settings.ini` |
| `config/sway/sway-fx` | `~/.local/bin/sway-fx` |
| `config/sway/sway-workspaces` | `~/.local/bin/sway-workspaces` |

The six scripts are `chmod 0755` at the **source** before linking, because a
symlink inherits its target's mode.

⚠ Two GTK destinations **rename**: GTK requires `settings.ini` under a
version-named directory, so the repo names the sources by version instead to
keep both readable side by side in `config/gtk/`.

⚠ **Not linked, and deliberately so:** `config/sway/swayfx-session` and
`config/sway/swayfx.desktop`. They install to `/opt/swayfx/bin/` and
`/usr/share/wayland-sessions/`, which need root and sit outside the dotfiles
model — the recipe in *SwayFX — the effects build* copies them. Same
arrangement as the three swayosd/swaync patches.

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

> [!NOTE]
> **`$mod+q` closes the focused window**, alongside the i3 default
> `$mod+Shift+q` which stays bound for muscle memory. It is the FOCUSED window,
> not the hovered one, and `focus_follows_mouse no` makes those different here.
> A keyboard binding cannot target the hovered window at all — sway exposes no
> pointer coordinates over IPC (`get_seats` carries `focus` and the device list
> and nothing else). Mouse bindings are the exception, since they act on the
> container under the pointer: `bindsym --whole-window $mod+button2 kill`.


`$mod` is **Mod4 (Super)**. See [Keybindings](keybindings.md) for the full table
and the reasoning behind the Alt reservation.

Notifications and media:

| Key | Action |
|---|---|
| `$mod+n` | Toggle the swaync control centre |
| click the clock | Open the month calendar (`sway-calendar`) — see *The clock is the centre module* |
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
| swaync panel slider | yes (`min`) | user scale — see below |

> The floor is written in **three** places: `FLOOR` in `config/sway/sway-brightness`,
> `BRIGHTNESS_MIN` in `sway-quickpanel`, and `widget-config.backlight.min` in
> `config/swaync/config.json`. **Change one and you must change all three**, or
> the keys and the sliders will disagree about where the bottom is.

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

  Steps also snap to the `STEP` grid, and a final guard forces the value to move
  at least one percent per press, so a coarse device can never produce a dead
  key.
- **swayosd is not asked to set the brightness**, only to display it.
  `sway-brightness` owns the floor and the user scale; swayosd draws the bar.
  See *swayosd is a local build* below — trixie's 0.1.0 could not draw a bar at
  an arbitrary value at all, which is why this used to be a numeric OSD.
- **`STEP` is a function of how fast a press completes, not a taste setting.**
  `repeat_rate 40` fires a press every 25ms while the key is held, and sway
  `exec`s each independently, so a press costing more than 25ms overlaps the
  next and they race on the state file. Measured, this used to cost **~80ms**:

  | | per press | effect of holding the key |
  |---|---|---|
  | `brightnessctl -m` read ×2 | 33.8ms | |
  | `brightnessctl set` | 18.5ms | |
  | `--help` capability probe | 11.0ms | |
  | `awk` fraction | 9.3ms | |
  | **before** | **~80ms** | ~12 eff. presses/s → 62%/s at STEP 5 |
  | **after** | **19.4ms** | 40 presses/s → 80%/s at STEP 2 |

  The reads and the write now go straight to sysfs (`read` builtin / `printf >`,
  fork-free, ~0.1ms), mirroring what `sway-quickpanel` already did. **Lowering
  the per-press cost without lowering `STEP` makes the key worse, not better**:
  at 14ms and STEP 5 a held key crosses the whole range in half a second. The
  two numbers move together.
- **Do not reintroduce a `--help` capability probe.** Trying the modern call and
  falling back on its exit code is one spawn instead of two, and self-heals
  across an upgrade instead of caching an answer that outlives the binary it
  described. Unknown options are a hard error in both argument parsers:
  `swayosd 0.1.0` (GLib) exits 1, `0.3.2` (clap) exits 0.

### swaync's backlight widget gives the floor *and* the false zero, for free

This module briefly shipped without the widget, on the belief that swaync writes
raw sysfs with no minimum and exposes no `min` option. **That was wrong** —
`min` is in the 0.11.0 schema and in the shipped Vala:

```vala
int min = int.max (0, get_prop<int> (config, "min"));
slider.set_range (min, 100);      // src/controlCenter/widgets/backlight/backlight.vala
```

The part that isn't obvious is that this buys the *whole* contract, not just a
clamp. `min` moves the GtkAdjustment's **lower bound**, and GtkScale places its
handle at `(value − lower) / (upper − lower)` — so the track itself becomes the
user scale. Hard left is real `5%`, hard right is `100%`, linear between:
algebraically the same map as `real = 5 + user × 95 / 100`. No fork, no build.

Measured rather than assumed — force the hardware under the floor and screenshot
the panel:

```sh
brightnessctl -q set 3% && swaync-client -op && sleep 1.2 && grim /tmp/t.png
swaync-client -cp && brightnessctl -q set 100%
```

The handle clamps to `lower` and lands at **x=1501** — the exact pixel where the
volume slider (range `0..100`) sits at `0`.

The one thing left un-remapped is the **drag tooltip**, which prints the real
percent (`5..100`). Nothing else shows a number; the widget calls
`set_draw_value(false)`.

> ⚠ **Do not reach for a newer swaync for that tooltip.** Upstream `main` is
> identical here apart from the GTK4 port (`append` vs `pack_start`) — same
> `min`, same raw write, same real-percent tooltip. Fixing one tooltip means
> forking Vala and carrying a build. Verified by diffing the 0.11.0 source
> (`apt-get source sway-notification-center`) against upstream `main`.

## ⚠ Three local builds carry patches — read this before upgrading anything

Two packages on this box are **local builds installed to `/usr/local/bin`,
shadowing the Debian package**, and they carry patches that live in this repo.
An `apt upgrade` does not touch `/usr/local`, so it will not silently revert
them — but rebuilding from a new upstream release will, unless the patches are
re-applied.

| Patch | Applies to | What it adds |
|---|---|---|
| `config/sway/swayosd-fade.patch` | SwayOSD v0.3.2 | fade in/out for the OSD overlay |
| `config/sway/swaync-system-stats.patch` | swaync 0.11.0 | the `system-stats` widget |
| `config/sway/swaync-cc-fade.patch` | swaync 0.11.0 | fade in/out for the control centre |

All three were verified to apply cleanly to a pristine upstream tree, and the
two swaync patches touch disjoint files so their order does not matter.

**Where the stock packages still matter:** both Debian packages stay installed.
They supply `/etc/xdg/swaync/*`, the D-Bus service files and the systemd units,
and they are the fallback if `/usr/local/bin` is ever cleared.

> [!NOTE]
> **There is a fourth local build, and it is not in this table because it
> carries no patches:** SwayFX, in `/opt/swayfx`, as an additional GDM session.
> It is a fork of sway rather than a patched package, so it upgrades on its own
> schedule and none of the three patches above apply to it. See
> *SwayFX — the effects build* below.

### Rebuilding

```bash
# swayosd — Rust + meson, needs sassc + libgtk4-layer-shell-dev
curl -sfLO https://github.com/ErikReider/SwayOSD/archive/refs/tags/v0.3.2.tar.gz
tar xzf v0.3.2.tar.gz && cd SwayOSD-0.3.2
patch -p1 < <dotfiles>/config/sway/swayosd-fade.patch
meson setup build --prefix=/usr/local --buildtype=release && ninja -C build
sudo install -m0755 build/src/swayosd-{server,client} /usr/local/bin/

# swaync — Vala + meson; `apt build-dep` installs the exact Build-Depends
sudo apt build-dep -y sway-notification-center
apt-get source sway-notification-center && cd sway-notification-center-0.11.0
patch -p1 < <dotfiles>/config/sway/swaync-system-stats.patch
patch -p1 < <dotfiles>/config/sway/swaync-cc-fade.patch
meson setup build --prefix=/usr/local && ninja -C build
sudo install -m0755 build/src/swaync{,-client} /usr/local/bin/
```

> [!WARNING]
> **Install to `/usr/local/bin`, never `~/.local/bin`.** sway's PATH is
> `/opt/sway-next/bin:~/.cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin`
> — GDM starts sway without a login shell, so `~/.local/bin` is absent. A binary
> installed there is invisible to every `exec_always` and every keybinding, and
> the failure is silent. This was hit for real: a `~/.local` swayosd install
> left a **0.1.0 client talking to a 0.3.2 server**, which exits 0 and draws
> nothing, so both the volume and brightness OSDs went dead with no error.
> Verify with
> `env PATH="$(tr '\0' '\n' </proc/$(pgrep -x sway)/environ | grep ^PATH= | cut -d= -f2-)" command -v <binary>`.

### swayosd is a local build (v0.3.2 vs trixie's 0.1.0)

Trixie ships **0.1.0**, which cannot draw a progress bar at an arbitrary value —
`--custom-progress` arrived in **0.2.1** and `min_brightness` in **0.3.0**. That
is why the brightness OSD used to be a bare number. `sway-brightness` calls
`--custom-progress` / `--custom-progress-text` / `--custom-icon`, all of which
exist in 0.3.2 under exactly those names.

The **fade** is a patch because upstream has none at all: `run_timeout` is a bare
`window.show()` / `window.hide()`, with no opacity or transition anywhere in the
widget. The patch animates window opacity on a frame-clock tick callback and
cancels any in-flight fade so a repeat press cannot strobe the pill. Verified by
differential — unpatched renders **2** distinct opacity levels, patched renders
**4** in a monotonic ramp.

> [!NOTE]
> `min_brightness` in 0.3.x is **not** used. swayosd never sets the brightness
> here, so it would be inert config. The floor lives in `sway-brightness`.

### swaync is a local build (0.11.0 + two patches)

**`system-stats`** is the cpu/memory/temperature readout that used to be waybar's
`group/system` drawer. It exists because swaync 0.11 has no widget that can show
a *changing* value — `label` takes a static `text` and there is no
exec-and-refresh anywhere in the widget set. Three things it deliberately gets
right:

- **Polling is bound to `on_cc_visibility_change`**, so a closed panel costs
  nothing. Same contract the backlight widget uses for its sysfs monitor.
- **CPU is a delta between two `/proc/stat` samples.** A single read reports the
  cumulative average since boot, which barely moves and looks frozen.
- **Memory uses `MemAvailable`, not `MemFree`.** Free excludes reclaimable page
  cache, so it reports a machine with a warm cache as nearly full.

It reuses the stable `/sys/devices/platform/coretemp.0/hwmon` path rather than
`/sys/class/hwmon`, for the reason recorded in `config/waybar/config`.

> [!WARNING]
> **Vala has no `\U` escape** (only `\u`, four hex digits) and every Nerd Font
> icon here is in Plane 15, above the BMP — so `"\U000F0EE0"` is a *compile
> error*, not a silent fallback. The glyphs are built from codepoints via a
> `glyph(unichar)` helper, which also keeps the patch file free of raw Plane-15
> bytes that a re-encoding could eat.

**The control-centre fade** reuses swaync's own `Animation` class (frame-clock
driven, has easing, and no-ops when `gtk-enable-animations` is off, so it honours
the accessibility setting for free). 180ms of `ease_out_cubic` rather than the
400ms `ANIMATION_DURATION` — this fires every time the panel opens, and 400ms
reads as lag on something you opened to click. Two non-obvious hazards:

- **`Animation.start()` no-ops when the widget is not mapped**, jumping straight
  to the end value — so starting the fade in the same turn as `set_visible(true)`
  produces no fade at all on the open path. It is deferred to an idle.
- **A closing window is still mapped**, so `visible` cannot answer "is the panel
  open?". Left alone, a toggle pressed mid-fade computes "it is open, so close
  it" and the panel refuses to reopen until the fade lands. `cc_is_open()`
  accounts for the closing state, and `on_visibility_change` uses it — which is
  also what stops `system-stats` polling at the right moment.

Verified by differential against the stock Debian binary: stock renders **2**
distinct opacity levels, patched renders **6** in and **5** out.

## SwayFX — the effects build (blur, rounded corners, animations)

**Status 2026-08-08: built, validated, NOT YET LOGGED INTO.** The one untested
thing is DisplayLink; see the go/no-go at the end of this section.

This is the fourth local build, and unlike the three above it is not a patch —
it is a **fork of sway itself**, in its own prefix, offered as a **third GDM
session**. Plain sway cannot blur or round corners at any setting; that has been
recorded elsewhere in this file as the only thing that would ever justify a
fork. SwayFX **0.6** (2026-08-05) is what made it worth doing, because it added
the other half: `animation_duration_ms` drives open/close, move/resize and
workspace-change animations. Every 0.5.x write-up you will find says SwayFX has
no animations — that stopped being true days before this was built.

| | |
|---|---|
| Prefix | `/opt/swayfx` (self-contained, 30 MB) |
| Session | `/usr/share/wayland-sessions/swayfx.desktop` → `swayfx-session` |
| Sources | `config/sway/swayfx-session`, `config/sway/swayfx.desktop` |
| Prebuilt | `~/projects/sway-migration/swayfx-0.6/` + `install.sh` |
| Versions | SwayFX 0.6 (sway 1.12.0 base) · wlroots 0.20.1 · scenefx 0.5 |

**Nothing is replaced.** `/usr/bin/sway` and `/opt/sway-next` are untouched, so
GDM offers three entries and a bad SwayFX session is one logout from recovery.
That is the entire reason this was affordable.

> [!IMPORTANT]
> **Six of trixie's libraries are too old for wlroots 0.20, and all six are
> built into the prefix rather than upgraded.** `PLAN.md` in the sway-migration
> repo rejected 0.20.1 in July on the grounds that it "cannot build here" —
> true of a system-dependency build, **false** once wlroots' own `.wrap`
> fallbacks are used. It also under-counted: the note names two, there are six.
>
> | | trixie | wlroots 0.20.1 needs |
> |---|---|---|
> | libdrm | 2.4.124 | 2.4.129 |
> | wayland-server | 1.23.1 | 1.24.0 |
> | wayland-protocols | 1.44 | 1.47 |
> | libdisplay-info | 0.2.0 | 0.3.0 |
> | pixman | 0.44.0 | 0.46.0 |
> | libxkbcommon | 1.7.0 | 1.8.0 |
>
> Upgrading these system-wide (sid pinning, or a forky dist-upgrade) was
> considered and rejected: libdrm and libwayland are the floor of the whole
> graphics stack, and that is a large permanent change to the host in exchange
> for a session you might log out of. **No apt package is touched.**

> [!CAUTION]
> **DO NOT export `LD_LIBRARY_PATH` in `swayfx-session`.** `sway-next-session`
> does, and copying that line over is the obvious move and a real bug. That
> prefix holds exactly ONE library, with a soname nothing else on the box links,
> so leaking it to children is inert — its own comment says so. `/opt/swayfx`
> holds **twelve**, six of which collide with system sonames (`libdrm.so.2`,
> `libwayland-{client,server,cursor,egl}.so.*`, `libxkbcommon.so.0`,
> `libpixman-1.so.0`). `LD_LIBRARY_PATH` is inherited by everything sway spawns,
> so it would force Firefox, Ghostty and every GTK dialog onto this prefix's
> libwayland and libxkbcommon for the whole session — while Mesa still links the
> system libdrm.
>
> The binaries instead carry an absolute **RUNPATH**, set at build time with
> `LDFLAGS`. Meson preserves it through `ninja install`. Verify:
> `readelf -d /opt/swayfx/bin/sway | grep RUNPATH`.

### Rebuilding

```bash
# Sources: swayfx 0.6 + scenefx 0.5 tarballs, wlroots 0.20.1 tarball.
# Copies of all three are kept in ~/projects/sway-migration/swayfx-0.6/.
tar xf swayfx-0.6.tar.gz && tar xf scenefx-0.5.tar.gz
cd swayfx-0.6 && mkdir -p subprojects
cp -r ../scenefx-0.5 subprojects/scenefx
cp -r ../wlroots-0.20.1 subprojects/wlroots

# ⚠ HOIST wlroots' OWN WRAPS TO THE TOP LEVEL. Meson resolves subprojects only
# from the root subprojects/ dir, so with the wraps left inside
# subprojects/wlroots/subprojects/ the scenefx subproject fails with
# "Neither a subproject directory nor a wayland.wrap file was found" and is
# silently disabled — the visible error is then the misleading
# "Subproject subprojects/scenefx required but not found".
cp -n subprojects/wlroots/subprojects/*.wrap subprojects/

# ⚠ LDFLAGS, not a wrapper env var — see the CAUTION above.
# ⚠ The libdrm vendor helpers must be disabled EXPLICITLY. wlroots asks for
#   `auto_features=disabled` on its libdrm fallback and it does not take; the
#   Intel bufmgr then builds and GCC 14 rejects the kernel header outright
#   (`error: packed attribute is unnecessary ... [-Werror=packed]`).
#   `-Dlibdrm:auto_features=...` is NOT a settable option; name each one.
#   Do not include `install-test-programs` — it is boolean, not a feature, and
#   one bad option aborts the whole `meson configure` without applying any.
LDFLAGS="-Wl,-rpath,/opt/swayfx/lib/x86_64-linux-gnu" \
meson setup build --prefix=/opt/swayfx \
  --force-fallback-for=libdrm,wayland,wayland-protocols,libdisplay-info,pixman \
  $(for o in intel radeon amdgpu nouveau vmwgfx omap exynos tegra vc4 \
             etnaviv cairo-tests man-pages valgrind; do
      printf -- "-Dlibdrm:%s=disabled " $o; done)

ninja -C build
doas ninja -C build install
doas install -m0755 <dotfiles>/config/sway/swayfx-session /opt/swayfx/bin/
doas install -m0644 <dotfiles>/config/sway/swayfx.desktop /usr/share/wayland-sessions/
```

> [!WARNING]
> **Never pipe `ninja` into `tail`.** The pipeline's exit status is `tail`'s, so
> a failed build reports success. This wasted a round here: `ninja` had stopped
> on the libdrm error and the harness recorded exit 0. Redirect to a log file
> and check `$?`, then `grep FAILED:` the log.

### What the fork actually adds

All of these were validated against the built binary (`sway --validate`, exit 0)
rather than taken from the README:

- `blur enable` + `blur_passes` `blur_radius` `blur_noise` `blur_brightness`
  `blur_contrast` `blur_saturation` `blur_xray`
- `corner_radius`, `smart_corner_radius`
- `shadows` + `shadow_blur_radius` `shadow_color` `shadow_inactive_color`
  `shadow_offset` `shadows_on_csd`
- `default_dim_inactive`, per-window `dim_inactive`, `dim_inactive_colors.*`
- `animation_duration_ms <0-5000>` — **one knob, no easing curve.** Upstream
  recommends 250. Do not go looking for a bezier; there isn't one.
- `layer_effects "<namespace>" { … }` — blur/shadows/corners on panels. Accepts
  `blur`, `blur_xray`, `blur_ignore_transparent`, `shadows`, `corner_radius`
  only; the blur *strength* knobs are global.

Layer namespaces on this box, read out of the installed binaries rather than
guessed — `waybar`, `swaync-control-center`, `swaync-notification-window`,
`launcher` (fuzzel), `wallpaper` (swaybg — do **not** blur that one), and
`sway-calendar`, whose `GLib.set_prgname` call already anticipates exactly this.
swayosd's namespace did not fall out of `strings`; under SwayFX it can simply be
read live, since `swaymsg -t get_outputs` gains a `layer_shell_surfaces` array
that plain sway does not have.

⚠ **Waybar tooltips will not blur.** They are separate GTK surfaces, not part of
the `waybar` layer, so `layer_effects` cannot reach them (upstream Waybar #3450,
unanswered). Keep them opaque — a half-transparent unblurred tooltip reads as
broken in a way a solid one does not. This desktop has already been bitten once
by tooltips-are-their-own-surface, in `custom/notification`.

### Turning the effects on: `config/sway/sway-fx`, not the config

**The FX directives are applied over IPC from an `exec_always`, and putting them
in `config/sway/config` is a mistake that looks correct.** That config is shared
by all three GDM sessions, and plain sway does not skip an unknown command —
measured against `/opt/sway-next`:

```
[ERROR] Error on line 1 'blur enable': Unknown/invalid command 'blur'
[ERROR] Error(s) loading config!            # exit 1
```

That is a swaynag *"There are errors in your config file"* on every login to the
fallback session — the recovery path, which is the last place to put a banner.
An `include` of a glob matching nothing **is** silent (exit 0, no output), so a
conditional include would work, but nothing populates the directory per-session
without a wrapper and the bare **Sway** entry has none.

`sway-fx` keys on **`sway_original_version`** in `swaymsg -t get_version` — a
field plain sway does not emit — and exits 0 having sent nothing. ⚠ Do **not**
key on `variant`, which is the string `"sway"` on both. The config validates
exit 0 on SwayFX *and* on plain sway 1.11; keep it that way.

`exec_always`, not `exec`: `swaymsg reload` resets runtime effects to whatever
the config says, i.e. none, so re-running on reload is exactly what makes reload
the right way to re-apply after editing. Every value also applies live —
`swaymsg blur_passes 3` — which is how these were chosen.

> [!IMPORTANT]
> **The blur settings are a function of the wallpaper, not of taste.** Glass
> renders what is behind it. The strip under the bar on the wallpaper in place
> 2026-08-08 measures **13/255 mean luminance — 5%**, effectively black, and at
> `blur_brightness 1.0` the pills were nearly invisible. 1.25 lifts them to a
> readable frosted grey.
>
> That is **compensation for a dark backdrop, not the intended look.** Proven by
> temporarily swapping in a colourful wallpaper: at 1.0 the same settings make
> each pill tint to the colour behind it, which is the whole point of the
> effect. **If the wallpaper changes, re-judge `blur_brightness`/`blur_saturation`
> first and expect to drop them.**
>
> The waybar pill alpha is the coupled half — it went 0.88 → 0.62, because blur
> is only visible through whatever the alpha leaves transparent. Raising it back
> towards opaque removes the glass *while still paying the GPU cost for it*.
> Change both together or neither.

Two settings deliberately **not** taken: `blur_xray` (it blurs only the
wallpaper, so a bar over a maximised terminal would blur something it is not in
front of — off matches macOS and GNOME) and `default_dim_inactive` (this desktop
hides the focused border, so dimming would become the only focus cue *and* would
dim the terminal you are reading the moment a dialog takes focus).

⚠ **`blur_ignore_transparent` matters most on waybar**: the bar window is fully
transparent between the three pills, and without it the compositor blurs those
gaps too — a faint full-width band where the design calls for three islands.

### ⚠ `blur_brightness` has a hard ceiling of 1.0

**The blur region is a rectangle.** It does not follow a pill's rounded corners,
and `blur_ignore_transparent` does not confine it. At `blur_brightness 1.0` that
costs nothing — the blurred area and its surroundings are the same backdrop, so
the boundary is invisible. Push it above 1.0 and the rectangle lights up: a grey
block with **square corners sitting behind a rounded pill**, which is the
straight-edge-against-a-curve mismatch this desktop has been burned by before.

Luminance of the strip just outside the pill, minus bare wallpaper:

| `blur_brightness` | delta |
|---|---|
| 1.0 | **−2.8** (invisible) |
| 1.1 | +16.3 |
| 1.25 | **+45.0** (a grey block on every pill) |

Isolated by elimination: the halo survives `shadows disable` and
`corner_radius 0`, and disappears only under `blur disable`.

> [!IMPORTANT]
> **That diagnosis was incomplete, and the follow-up matters more.** Brightness
> above 1.0 makes the blur region *lit*, but it does not create it — and on a
> **detailed** wallpaper the region is visible at 1.0 anyway, as blurred
> rectangles beside each pill, because blur shows as loss of detail whether or
> not it is brightened. The original conclusion ("invisible at 1.0") was true
> only of the near-black wallpaper it was measured on.
>
> **What creates the region is the pills' own drop shadows.** The compositor
> blurs every pixel that is not fully transparent, and a soft `0 6px 16px`
> shadow is ~16 px of barely-transparent pixels ringing the pill. Measured
> spill into the gap between the workspace and clock pills:
>
> | pill `box-shadow` | spill |
> |---|---|
> | `0 1px 2px` + `0 6px 16px` | 18 / 28 px |
> | `0 1px 3px` + `0 2px 6px` | 2 / 7 px |
> | **`0 1px 2px` only** | **2 / 0 px** |
> | none at all | 2 / 0 px |
>
> The tight shadow costs nothing over having none, so the pill still reads as
> seated. ⚠ **Never put a soft or wide shadow on a surface the compositor
> blurs.** Reducing `blur_passes`/`blur_radius` does *not* fix it — even at
> 1/1 the spill was still 13-21 px, because the region is set by the shadow's
> extent, not the blur's strength.

This was briefly set to 1.25 to compensate for a near-black wallpaper. **That
trade is not available** — it buys pill definition with a visible rectangle.
Saturation *is* safe to spend: it scales colour rather than lifting the region.

> [!NOTE]
> **The wallpaper was the fix, and it was applied.** The old one measured
> **13/255** in the strip under the bar, so the glass had nothing to render at
> `blur_brightness 1.0` and the pills were nearly invisible. It was replaced
> (2026-08-08) with an ESA/Hubble galaxy image measuring **~57**, and the pills
> now genuinely tint to what is behind them.
>
> ⚠ **A wallpaper is therefore a functional choice here, not decoration.**
> `~/Pictures/wallpapers/README.md` carries the candidates with their measured
> bar-strip luminance and the script's criterion. Anything below ~30 puts the
> glass back to invisible. Changing it means re-judging three coupled values:
> the pill `@pill` alpha and `.empty` opacity in `config/waybar/style.css`, and
> `blur_saturation` here.

### The right cluster is two pills, and `.modules-right` paints nothing

⚠ **Reading `config/waybar/config` alone will mislead you here.** Every module
sets `background-color: transparent`; the pill is painted on the *container*. So
"merge volume/network/battery into one pill" was already true — the GNOME
merged-quick-settings look came for free from `.modules-right`.

What one pill could not do is separate the two *kinds* of thing in it. Privacy,
media and the VPN shield hide themselves entirely when idle, so they grew and
shrank the surface the permanent status icons live on. GNOME keeps its privacy
indicator apart from the status pill; macOS does the same with its recording dot.
So the pill moved onto two groups:

- `group/indicators` — privacy · mpris · network#vpn. **Every member must be
  self-hiding**; an always-visible module in here pins the pill open and
  collapses the distinction it exists to draw.
- `group/status` — idle_inhibitor · pulseaudio · network · battery ·
  custom/notification (hub still last, at the corner).

> [!WARNING]
> **`#indicators` must carry no `border` and no horizontal `padding`.** Zeroing
> padding and `min-width` takes the empty group's *content* to zero width, but
> border and padding both have intrinsic width and keep painting — the artifact
> is a single bright column beside the status pill (measured x=1676, luminance
> **46.9** against a ~6 wallpaper) that reads as a smudge rather than a widget.
> The hairline is an **inset box-shadow**, clipped to the box's own area and so
> invisible at zero width (6.1 after). Padding stays on the children, where the
> shared chip rule already puts it. Verify both states — empty *and* populated,
> the latter by temporarily borrowing an always-visible module into the group.

### Layer namespaces — read them live, do not guess

Under SwayFX, `swaymsg -t get_outputs` gains a `layer_shell_surfaces` array that
plain sway does not have. A surface only appears while it is mapped, so open the
thing first:

```bash
swaymsg -t get_outputs | jq -r '.[].layer_shell_surfaces[]?.namespace'
```

`waybar` · `swaync-control-center` · `swaync-notification-window` · `launcher`
(fuzzel) · `swayosd` · `sway-calendar` · `wallpaper` (swaybg — **never blur
this**, it is what everything else samples).

This method resolved **swayosd**, which `strings` on the binary could not find,
and corrected the calendar:

> [!WARNING]
> **prgname does NOT set a layer-shell namespace.** `config/sway/sway-calendar`
> asserted it did — *"sway derives a layer surface's namespace from prgname
> too… what any future `layer_effects`-style rule would match"* — and the first
> rule that actually needed it proved otherwise: the panel reported as the
> generic `gtk-layer-shell`. GtkLayerShell supplies its own default and never
> consults prgname. Fixed at source with `GtkLayerShell.set_namespace()`; that
> call and the rule in `sway-fx` must stay in step, or the calendar silently
> becomes the one panel with no blur.

### The go/no-go: DisplayLink

`/opt/sway-next` exists **because** trixie's wlroots 0.18 cannot drive the
DisplayLink dock, and 0.19 turned that abort into a fallback. SwayFX 0.6 moves
to wlroots **0.20**, which nothing here has ever run.

The fix is present — `backend/drm/backend.c:182` in the 0.20.1 source, and the
string `falling back to scanning out from primary GPU` is in the built
`libwlroots-0.20.so`. That is necessary, not sufficient.

**Test docked, and from a bare VT** (`~/projects/sway-migration/test-displaylink.sh`
is the existing harness). If 0.20 regresses DisplayLink, the fallbacks are
SwayFX 0.5.3 on the proven wlroots 0.19.3 — all the glass, no animations — or
nothing. Recovery from a bad login is always: pick **Sway (0.19 / DisplayLink)**.

## The workspace row is nine custom modules, not `sway/workspaces`

`config/sway/sway-workspaces` + `custom/ws1..ws9`. Added when the row needed a
**travelling highlight**: jumping 9 → 1 sweeps back through 8,7,6…2 before
landing, without visiting those workspaces.

**Why the built-in module could not do it.** `sway/workspaces` derives every
button's CSS class from sway's own state, so an intermediate workspace is never
*anything* and nothing outside the module can light it up. That is structural,
not a missing option. GTK3 cannot fake it either — sliding one highlight across
sibling widgets needs `transform`, which GTK3 does not have, and a stray
`transform:` invalidates the **entire** stylesheet.

**Why nine widgets and not one Pango row.** A single `custom/` module rendering
markup was the smaller build, and it costs both things the row had just been
tuned for: Pango backgrounds are **rectangles** and cannot be rounded, and one
module has one `on-click`, so per-workspace clicking goes away.

> [!IMPORTANT]
> **Why a server plus `tail`, and not nine subscribers.** waybar instantiates
> every module **once per bar**, so on the dock that is 9 × 3 = **27 processes**.
> Nine independent Python subscribers was the obvious build and would cost
> ~10 MB each — **~300 MB of RSS for a status bar**. One server holds the single
> sway subscription and appends each slot's state to its own file; the clients
> are a bare `tail -F`, ~1 MB each.
>
> ⚠ **The module `exec` must stay a plain `tail`.** Anything carrying its own
> interpreter gets multiplied by 27. `-F` (not `-f`) follows by name, so it
> survives the server truncating the file and waits politely if waybar starts
> first; `-n1` replays the last line, which is what gives a freshly-started bar
> its state instead of nine blanks.

Two bugs found by measuring, both of which look correct in the source:

> [!WARNING]
> **Take the jump from the EVENT, not from polled state.** Refreshing from sway
> and then comparing silently disables the whole effect: sway emits `init`
> before `focus` when the target workspace does not exist yet, and the poll on
> that first event **already returns the new focus**. By the time `focus`
> arrives, `old == new` and the sweep never fires — which is exactly the common
> case, because an empty row is when you jump furthest. The event's own
> `old`/`current` are authoritative and immune to the ordering.

> [!WARNING]
> **The destination must stay dark until the sweep arrives.** Otherwise the
> focused chip lights the instant the key is pressed and the trail runs toward
> an already-lit target — two highlights on screen at once, which reads as a
> glitch rather than as travel. `arriving` suppresses it for the ~250 ms the
> sweep is in flight.

Other things that are deliberate: the trail is **weaker than focused and is not
grown** (giving it the focused chip's padding reflowed the row nine times per
jump, and the bar visibly convulsed); `MIN_JUMP` means an adjacent switch does
not animate, so the common case pays no latency; and the server guards itself
with a **pidfile, not `pgrep`** — `pgrep -f sway-workspaces` matches the `sh -c`
that launches it, the same self-match trap recorded at `exec autotiling`.

Verify by capturing frames, since 52 ms/frame is fast enough to sample a 245 ms
sweep:

```sh
mkdir -p /tmp/sweep
( sleep 0.15; swaymsg workspace number 1 ) &
for i in $(seq -w 1 10); do grim -g "0,120 400x52" /tmp/sweep/$i.png; done
```

## ⚠ Two bar modules poll, and both defaulted to 60s

`network` and `battery` are **poll-only**, and waybar's default `interval` is
**60 seconds** for both. Neither failure looks like a stale readout — it looks
like the bar is broken:

- **network** appeared frozen: everything nl80211-derived (essid, signal
  strength, the icon ramp) refreshes only on that timer.
- **battery** took up to a minute to show `format-charging` after the charger
  went in — and the one moment anyone looks at the battery icon is the moment
  they plug in.

Both are now `"interval": 5`. The cost is reading a few small sysfs files.

> [!IMPORTANT]
> **There is no event-driven alternative for battery, and the binary makes it
> look like there is.** waybar links `libudev`/`libgudev` and carries the whole
> `udev_monitor` API — but that monitor belongs to the **backlight** module:
> `backlight` is the only subsystem string in the binary, and there is no
> `power_supply` string at all. `waybar-battery(5)` documents exactly one knob,
> `interval`.
>
> Nor could it be inotify-driven, which is the other instinct: **the kernel does
> not emit inotify events for sysfs *attribute* changes**, so watching
> `/sys/class/power_supply/*/status` can never work. Polling is the only
> mechanism available.

## Bar geometry is one set: gaps, margins and pill height

⚠ **These values are coupled and must be changed together and re-measured.**
The alignment is the point, not the individual numbers.

| what | where | value |
|---|---|---|
| bar height | `config/waybar/config` | 40 |
| bar margin top / bottom | `config/waybar/config` | 6 / 6 |
| bar margin left / right | `config/waybar/config` | 10 |
| pill height | `config/waybar/style.css` `min-height` | 32 (+2 border = 34) |
| inner gap | `config/sway/config` | 8 |
| outer gap | `config/sway/config` | 2 → visible 10 |
| **top** outer gap | `config/sway/config` | **−8** → visible 0 |

Measured result: every visible gap is **10 px** — window to each screen edge,
and window to the bar's pill — and a window starts at y=52 whether there is one
of them or nine.

### The three things that were wrong, none of which the raw numbers showed

**The pills were different heights.** Measured 34 / 34 / **28** for left /
centre / right, with the right one also sitting 3 px lower. `.modules-left` and
`.modules-center` are direct children of the bar so GTK stretches them to the
full zone; `#status` and `#indicators` are **groups** nested inside
`.modules-right`, and a group hugs its children instead. Fixed with `min-height`
on the pill rule — *and* by moving the `margin: 3px 0` inset up to
`.modules-right`, because it was being applied a second time inside an
already-positioned container. Fixing only the first took it to 40/40/34.

**Windows did not line up with the bar.** Windows sat at x=12 while the pills
start at x=10. `gaps outer 2` puts the screen-edge gaps at 10, matching
`margin-left`.

**The top gap jumped when a second window spawned.** With one window
`smart_gaps` holds it flush to the exclusive zone, 10 px below the pill; with
two, the outer gap stacked on top of the bar's own `margin-bottom` and the
pill's CSS margin — both of which live *inside* the zone — giving 22 px. So it
visibly moved on spawn. `gaps top -8` cancels the inner gap at that edge only;
outer gaps are *in addition to* inner, and sway accepts negative values for
exactly this.

> [!NOTE]
> **`margin-bottom` is the only lever that moves where windows start.** The
> exclusive zone is `height + margin-top + margin-bottom`. The pill's own CSS
> margin cannot do it — that is inside a surface whose size sway has already
> reserved.

### `smart_gaps on`: alone means full bleed

A lone window takes the whole usable area; gaps return on split. **Three
settings agree on this and move together**: `smart_gaps on`,
`hide_edge_borders --i3 smart`, and `smart_corner_radius` in `sway-fx`. The last
is load-bearing under SwayFX — rounded corners on a window flush to the screen
edge would cut visible notches out of the display corners.

## The glass tint, and where the accent went

The selected and hover states are a **dark pigment plus a lit edge**, not a
colour fill. Apple's Liquid Glass guidance is that a tint modulates the
*material* rather than filling the shape, and that where a tint muddies the
scene you lower saturation and **let edge cues carry the material**. A saturated
fill is the specific thing it warns against — and the old sapphire→blue gradient
on the focused workspace was exactly that: a painted chip sitting in a glass
pill.

- `@glass-tint` — a dark pigment, not a hue; it deepens the pill it sits in
- `@glass-edge` — the inset hairline that does the actual work of saying *this one*
- `@glass-veil` — the same idea at hover strength

⚠ **Legibility decides these numbers, not taste.** Apple's floor is 4.5:1 for
text over the material after blur; the focused numeral measures **12.8:1**.

**`@blue` is now unused by any resting state.** It came off the focused
workspace, the hover cue and finally the clock — one remaining blue thing reads
as a leftover rather than a decision. **Urgent deliberately keeps its saturated
fill**: drain that too and nothing in the bar can interrupt.

⚠ **Even chip spacing needs measuring, not eyeballing.** The optical corrections
centre each glyph *within its own chip*, but the gap you see is ink-to-ink and
every glyph's side bearing differs — nominally identical padding measured
**22 / 27 / 25 / 31 px**. Two rounds against the ink-span script brought it to
**29 / 29 / 29 / 31**. Re-derive both halves if the font or size changes.

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

## The clock is the centre module, and the calendar is a real window

The clock sits in waybar's **`modules-center`**, where `sway/window` used to be.
Consequence, stated plainly because it has already misled two comments in
`config/sway/config`: **the window title is not displayed anywhere on this
desktop** — no titlebars, and now nothing in the bar either. Putting
`"sway/window"` back in `modules-center` restores it, and the clock then has to
move back to `modules-right`.

Left-clicking the clock opens `sway-calendar`.

### Why the calendar is a separate window and not waybar's

waybar's clock *can* render a calendar — `tooltip-format` accepts `{calendar}`,
and `actions.on-scroll-up: shift_up` flips months. That is what this used to do,
and it cannot be browsed, because **a GTK tooltip is hover-only and waybar has no
lever to open one on click or keep it open**:

- reaching for a month arrow moves the pointer off the clock, which dismisses the
  surface you are reaching into;
- scrolling to flip months works, but only while hovering — you are scrolling
  blind on something that disappears if you drift off it;
- a tooltip cannot take keyboard focus at all, so no key ever reaches it.

So it is a real surface of its own — a **layer-shell panel**, the same mechanism
waybar and swaync use, for the reasons in the next section. GNOME's shell calendar
is the model: month arrows, today picked out, adjacent months shown dimmed rather
than as blanks. It closes on Escape, on a second click of the clock, or on focus
loss once armed.

### It is a layer-shell panel, which is what stops it jumping

**First version was a plain xdg_toplevel like `sway-quickpanel`, and that was the
wrong shape.** A Wayland client cannot position its own toplevel — there is no
`move()`; sway owns the position. So sway placed it mid-screen and a `swaymsg move
absolute position` dragged it under the clock a frame or two later, which is
**visible**: the calendar appeared in the middle of the screen and slid. That is
not tunable, only shrinkable, because the move can by definition only happen once
the surface already exists.

`zwlr_layer_shell_v1` expresses position as anchors and margins that the
compositor applies in the **initial configure**, so the first frame is already in
place. It is what waybar and swaync themselves use. Verified top edge y=**52** —
the same row as the control centre, so the two line up when both are open.

It also deletes a pile of incidental machinery: no `for_window` rules, no `app_id`
criteria, no IPC round-trip, no retry loop, no output arithmetic, and no
`client.focused` border (a layer surface gets none), so the blue focus ring the
toplevel version had is simply gone.

```python
GtkLayerShell.init_for_window(win)                  # before the first map
set_layer(OVERLAY)          # not TOP — that is waybar's layer; stacking would be arbitrary
set_keyboard_mode(ON_DEMAND)  # not EXCLUSIVE, which would steal every key in the session
set_exclusive_zone(0)         # not -1: 0 respects waybar's zone, -1 slides under the bar
set_anchor(TOP, True); set_margin(TOP, GAP)
# anchoring NEITHER left nor right is what centres it; there is no "centre" anchor
```

`GAP` is only 6 because a layer surface's margins resolve against the **usable
area**, so waybar's 46px exclusive zone is already subtracted — no bar arithmetic.
`--anchor left|center|right` must match which module list holds the clock.

> [!CAUTION]
> **A layer surface fires a spurious `focus-in` → `focus-out` immediately after
> map, and a bare close-on-focus-loss handler therefore kills it before it
> draws.** Symptom: the script exits **0 instantly** — no window, no output, no
> error, nothing to grep for. The same handler is correct on an xdg_toplevel
> (`sway-quickpanel` still uses one), so the port to layer-shell is what broke it.
> Gating on "have we ever had focus" does **not** help — the spurious `focus-in`
> already set that. Traced with signal logging:
>
> ```
> SIGNAL: map-event
> SIGNAL: focus-in
> SIGNAL: focus-out, _had_focus= True     <- quits here
> ```
>
> Fix is a `FOCUS_ARM_MS` (500ms) arming delay: ignore focus events through the
> startup churn, then behave normally.

### It runs as a resident server, because spawn-per-click cannot feel responsive

Measured cold start of the one-shot version, exec to first pixel:

| stage | cumulative |
|---|---|
| python interpreter up | 35 ms |
| **gi imports** | **256 ms** |
| CSS + window build | 318 ms |
| first draw | **366 ms** |

**~260ms of that is fixed Python+GTK init that runs before any of the script's own
code**, so there was nothing in the calendar to optimise — a spawn-per-click design
sits permanently past the ~300ms where a click starts feeling sluggish. The window
is therefore built once by a resident server (autostarted from
`config/sway/config`, the same shape as `swayosd-server`) and a click just signals
it: **SIGUSR1 toggles visibility**.

Measured after: **~58-65ms** per click, and the server costs ~55 MB RSS resident.

> [!CAUTION]
> **The client path must not import GTK, and defining helpers above `import gi`
> does not achieve that.** Module-level imports run at exec time, before `main()`
> is entered — so the first version of this optimisation put the dispatch inside
> `main()` and measured **~280ms per click**, i.e. no better than spawning a fresh
> calendar. The dispatch itself has to run at module level, above the import:
>
> ```python
> if __name__ == "__main__":
>     _rc = client_fast_path(sys.argv[1:])
>     if _rc is not None:
>         sys.exit(_rc)
>
> import gi  # noqa: E402
> ```
>
> `--help` and the duplicate-`--server` decline are handled there too, so neither
> costs a GTK init.

Consequences worth knowing:

- **Escape and focus-loss now `hide()`, they do not quit** — quitting would throw
  away the pre-built window that makes the next click instant.
- **`toggle_visible()` re-derives today and re-arms the focus guard on every
  show.** A resident server easily outlives midnight, which would otherwise leave
  the cached `self.today` highlighting yesterday; and the post-map focus-out churn
  happens on *every* map, not just the first, so an un-rearmed guard would dismiss
  the second and later opens instantly.
- **`--anchor` now lives in two places.** The server owns the geometry and a
  signal cannot carry an argument, so the value in waybar's `on-click` applies
  *only* on the fallback path where no server was running. Change both together.
- **The autostart deliberately has no `pgrep` guard.** `pgrep -f sway-calendar`
  would match the `sh -c` running the line itself and never fire — the same
  self-match trap the `autotiling` comment records. The script guards itself via
  its pidfile instead, which is what makes it safe under `exec_always` re-running
  on every reload. Verified: two consecutive `swaymsg reload`s leave exactly one
  resident server.

### The font is Cantarell, the one place here that is not IosevkaTerm

Everything else on this desktop is monospace because it shows aligned columns of
data. A calendar is UI chrome, and in a terminal face it reads as a printout —
cramped weekday headings, typewriter digits, a month name that looks like log
output. Cantarell is the desktop's actual UI font
(`gsettings get org.gnome.desktop.interface font-name` → `'Cantarell 11'`) and is
what GNOME's own shell calendar uses, which is what this is modelled on.

Alignment does not suffer: Cantarell carries `tnum` (verified in its `GSUB`), so
digits are tabular. `button.nav` keeps the Nerd Font — the chevrons are
`md-chevron_left/right` in Plane 15 and Cantarell has neither (verified against its
cmap).

> [!CAUTION]
> **GTK3's font-feature syntax is not the web's.** Measured against the real
> parser:
>
> | declaration | result |
> |---|---|
> | `font-feature-settings: "tnum" 1;` | **rejected** — `Junk at end of value` |
> | `font-feature-settings: "tnum";` | OK |
> | `font-variant-numeric: tabular-nums;` | **rejected** — not a valid property |
>
> And one bad property discards the **whole** stylesheet, so verify before
> shipping. Note `Gtk.CssProvider().load_from_data()` **raises `GError`** rather
> than merely emitting `parsing-error`, so a try/except is the check.

### Clicking anywhere off the panel dismisses it

**This required making the surface cover the whole usable area.** A layer surface
sized to the panel receives only the clicks that land *on* the panel — a click
anywhere else goes to whatever is underneath and the calendar never hears about it,
so it could not dismiss itself. The surface is therefore anchored on **all four
edges** with a transparent background, and the visible panel is a box inset within
it. Every click then arrives, and `_on_button_press` decides.

Consequences, all intended:

- **While open, the calendar is modal**: the dismissing click does not pass through
  to the window beneath. GNOME's shell popovers behave the same way.
- **The offsets moved off the surface and onto the box.** `GAP` / `EDGE_MARGIN` are
  now GTK margins on the panel widget, because surface margins would inset the
  *clickable region* instead. The surface's top edge is the usable area's top
  (waybar's zone already subtracted), so `GAP` 6 still lands the panel on y52 —
  verified: visible panel top 52, allocation y 6 within the surface, therefore
  surface top 46.
- **The bar stays clickable.** `set_exclusive_zone(0)` means the surface starts at
  46, and waybar occupies 0-45, so the clock still receives its own clicks and the
  toggle keeps working. That is the number to check if the clock ever goes dead.
- **The window background must be transparent and the *box* opaque**, or the panel
  paints the entire screen. `Gdk.Screen.get_rgba_visual()` is non-null and the
  screen is composited here, so `background-color: transparent` on `window` is
  enough.

> [!CAUTION]
> **Hit-test against the panel's allocation, not "did a child handle it".** The
> panel's own 12px padding is not a child — a `Gtk.Box` has no event window, so a
> click on the padding arrives at the toplevel handler exactly like a click on the
> wallpaper. Only geometry separates them, and dismissing on a click just inside
> your own border feels broken. Clicks on the day and nav buttons never reach the
> handler at all, which is correct: they are consumed by the buttons.

> [!NOTE]
> The `CSS` constant is a Python **bytes** literal, so it must stay pure ASCII —
> a warning glyph inside it fails at import with `bytes can only contain ASCII
> literal characters`, which `py_compile` *does* catch but only after you have
> written it. GTK3 CSS comments are also C-style: a `#` line in there is a parse
> error, and one bad token discards the whole stylesheet. Keep prose in Python
> comments outside the literal.

### Clicking the clock again closes it

**This needs an explicit single-instance check and cannot be left to focus-out.**
waybar's surface requests no keyboard focus, so clicking the clock does not
deliver a focus-out to the calendar — an early version just stacked a second
calendar on top of the first. A pidfile in `$XDG_RUNTIME_DIR` names the resident
server, and a click sends it **`SIGUSR1`**, which toggles visibility.

- **A stale pidfile is treated as "not running"**, not as an error — probed with
  `os.kill(pid, 0)` — so a crash or a reboot can never wedge the toggle into never
  opening again; a click in that state starts a fresh server instead.
- **`SIGTERM` is handled via `GLib.unix_signal_add`** rather than left to the
  default disposition, so the `finally` runs and the pidfile is removed.
  `unix_signal_add` defers to the main loop instead of running Python inside a
  signal handler.

Verified: open → close → open → close, exactly one resident server across repeated
`swaymsg reload`s, and the pidfile cleaned up on exit.

The clock's `tooltip` is now `false`. Keeping both was considered and rejected:
two calendars, one of which vanishes when you try to use it, is worse than one
that works. It also removes a second problem for free — a clock tooltip is its own
surface and overlapped the swaync panel, exactly as `custom/notification`'s did.

| Input | Action |
|---|---|
| click the arrows | previous / next month |
| scroll wheel | previous / next month |
| **Shift** + scroll | previous / next **year** |
| `PageUp` / `PageDown` | previous / next month |
| `Shift+PageUp/PageDown` | previous / next year |
| arrow keys | move the selected day, crossing month boundaries |
| `t`, or click the title | back to today |
| `Escape` | close |

> [!IMPORTANT]
> **First weekday is a constant, and that is not laziness.** glibc has the answer
> in `_NL_TIME_FIRST_WEEKDAY`, but Python's `locale` module cannot reach it:
> `nl_langinfo()` validates its argument against Python's own table, so the glibc
> item numbers (`(LC_TIME << 16) | 17`) do not get through — they land on
> unrelated entries and return abbreviated **month** names. Measured: item
> `0x20011` returned `'Apr'`. Do not re-derive this. The default is Sunday (this
> desktop is `en_US`); override with `SWAY_CALENDAR_FIRST_WEEKDAY=monday|sunday`.
> Day and month *names* are locale-aware, via `setlocale(LC_TIME, "")`.

> [!CAUTION]
> **`gi.require_version` must pin `Gdk` as well as `Gtk`.** Without it Gdk is
> resolved first, finds no requirement, loads the newest on the system (GTK **4**),
> and then `require_version("Gtk", "3.0")` fails with `RepositoryError: Requiring
> namespace 'Gdk' version '3.0', but '4.0' is already loaded`. `py_compile` does
> **not** catch it — this file shipped with that bug until its first real run, and
> a test harness importing the module hit the same thing from the other side.

> [!TIP]
> **Two traps when testing this from a script rather than by hand.**
> - **`pkill -f sway-calendar` kills your own shell.** `-f` matches full command
>   lines and yours contains the string. Use `swaymsg '[app_id="sway-calendar"] kill'`.
>   Same self-match trap the `autotiling` comment in `config/sway/config` records.
> - **`swaymsg -t get_tree` rects are GLOBAL; screenshots are output-local.**
>   `eDP-1` is at global y=120 here, so cropping a window's tree rect out of a
>   `grim` capture needs y−120. Getting it wrong crops a band of terminal and
>   looks like the window rendered without its header.
>
> Synthetic clicks via `swaymsg seat - cursor press button1` report
> `"success": true` but did **not** activate GTK buttons here, even with the
> pointer walked in via `cursor move`. Verify the widgets by emitting `clicked`
> and feeding `Gdk.EventKey` to the handler instead; that is what the checks in
> *Validating a change* do.

## Notifications: swaync, with mako as fallback

> [!WARNING]
> **`swaync-client` BLOCKS when no daemon is running — it does not fail.** This
> caused a real outage: the autostart line read
> `if swaync-client --reload-config; then …; else swaync; fi`, on the assumption
> that the reload fails when nothing is listening. It does not. It prints
> `Could not connect to CC service. Will wait for connection...` and waits
> forever, so the `else` is unreachable and **swaync never starts at all** —
> observed still hung 24 minutes into a fresh session. Symptom: the control
> centre "stops opening" and the waybar bell does nothing, silently.
>
> The condition must be `pgrep`, never the client's exit code:
>
> ```
> exec_always sh -c 'command -v swaync >/dev/null 2>&1 || exit 0; if pgrep -x swaync >/dev/null 2>&1; then swaync-client --reload-config -sw && swaync-client --reload-css -sw; else swaync & fi'
> ```
>
> Two further traps in the same two lines:
> - **`-sw` does not start swaync.** Its help is *"Doesn't wait when swaync
>   hasn't been started"* — it opts out of the blocking wait and gives up. The
>   waybar bell's `-sw` cannot recover a dead daemon; that is the autostart's job.
> - **`-sw` must FOLLOW the action flag.** `swaync-client` takes exactly one
>   option, so `-sw --reload-config` prints its usage and exits **0** — a silent
>   no-op that looks like success. `--reload-config -sw` is correct.

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

## One destination: every status readout left-clicks to the control centre

`pulseaudio`, `network`, `battery` and `custom/notification` all run
`swaync-client -t -sw` on a plain left click. That is a rule, not four independent
choices — keep it when adding a readout.

It is the other half of *The bar shows almost no numbers*: if the numbers live one
click away, that click has to be predictable. A cluster where each glyph opens a
different tool reintroduces exactly the friction the restrained bar removes — you
end up reading the icons to remember which one goes where.

Secondary buttons are where the specific tools live, and each is the **only** route
to that tool from the bar:

| module | left | middle | right |
|---|---|---|---|
| `pulseaudio` | control centre | `sway-quickpanel` | `pavucontrol` |
| `network` | control centre | — | `nm-connection-editor` |
| `battery` | control centre | — | — |
| `custom/notification` | control centre | — | DND toggle |
| `clock` | `sway-calendar` | — | — |

Two things not to undo by accident:

- **`nm-connection-editor` moved to right-click; it was not dropped.** It is what
  justifies `network-manager-gnome` in `swayPackages`, so check there before
  deleting that line. nm-applet's *other* job — being NetworkManager's secret
  agent, i.e. the wifi password prompt — is independent of any click and comes
  from the `exec nm-applet` line in `config/sway/config`.
- **`idle_inhibitor` is deliberately exempt.** It is not a readout; it toggles a
  Wayland object it holds on waybar's own surface (see the `modules-right`
  comment), so sending it to the control centre would break the one thing it does.

`battery` had no click action at all before, so nothing was displaced there.

## The control hub (what used to be "the notification bell")

`custom/notification` is the **last** entry in `modules-right` and its glyph is
`md-tune` (U+F062E, two horizontal sliders) — the same shorthand macOS uses for
Control Centre. It stopped being "notifications" when it took the corner: behind
it are the volume and brightness sliders, the system readouts, the session
actions and the notification list. DND swaps to `md-weather_night` (U+F0594),
the crescent macOS uses for Focus. Unread count still reads out via `{icon}{}`.

Moved *into* the panel: **power** (it was already there — swaync's buttons-grid
ran the same `sway-powermenu`, so the bar entry was a second door to one room)
and the **hardware readouts** (now the `system-stats` widget).

Deliberately **left on the bar**: `idle_inhibitor`. It is not a readout — it is
the thing that actually holds the Wayland `zwp_idle_inhibitor_v1` object on
waybar's own surface. A swaync button can only run a shell command and nothing
on `PATH` holds that object; the nearest shell equivalent (killing `swayidle`)
would also drop `before-sleep "$lock"`, so a lid-close suspend would stop locking
the screen. Correct where it is.

`custom/power` is still **defined** but out of `modules-right` — re-adding it is
one array entry, and its comment is the only written copy of the
`$HOME`-not-a-bare-name / GDM-PATH trap.

### ✅ RESOLVED: the "disjointed corners" at the top right of the panel

**There were two independent causes, and the one you actually see in normal use
was the waybar tooltip, not the window border.** Both are fixed:

| cause | fix | measured |
|---|---|---|
| waybar's `custom/notification` **tooltip** rendering over the panel's corner | `"tooltip": false` in `config/waybar/config` | tooltip px over the corner **458 → 0** |
| `client.focused`'s border/indicator exposed by the panel's **corner radius** | `border-radius: 0` (all four corners) in `config/swaync/style.css` | leaks per corner **7/3/30/0 → 0/0/18*/0** |

<sub>* the residual 18 is a 1px edge bleed, not a corner cut-out — see below.</sub>

**The tooltip is the one that mattered, and it hid the real story for months.**
`custom/notification` had `"tooltip": true`, commented as showing swaync's own
text ("3 Notifications"). That field is **empty when idle**, which is most of the
time, and waybar then falls back to rendering the module's *own label* — so the
tooltip was a rounded box containing nothing but a copy of the md-tune glyph you
were already pointing at. A GTK tooltip is its own surface and is **not** confined
to waybar's 40px height: this one drew at x≈1855-1910, **y≈46-93**, straight over
the control centre's top-right corner and part of "Clear all". Its rounded corners
meeting the panel's edge is precisely the "two corners that don't line up" look.

And it was not an edge case — **it was the default view of the panel.** Opening
the control centre by clicking that button leaves the pointer parked on it, so the
tooltip was on screen *every time the panel was opened by hand*. It vanished only
when opening via `$mod+n` or after moving the mouse, which is exactly what every
diagnostic screenshot did — which is why the region kept measuring clean while
the artifact was plainly visible in use.

> [!TIP]
> Other modules' tooltips can overlap the panel too — the clock's lands in the
> same band. That one is left alone: it carries a real calendar, and the clock is
> not the button that opens the panel, so you have to go out of your way to see
> them together. The rule is only that **the module which opens the panel must
> not have a tooltip.**

The rest of this section is the border half. Three of the four things previously
written about it were wrong, and the corrections are the valuable part.

**The geometry, which is simple and was not.** waybar's exclusive zone is
`height 40 + margin-top 6` = **46**, so `usable_area.y = 46`. Then:

```
panel top edge y  =  46 + control-center-margin-top      (linear, no clamp)
window top edge y =  46 + gaps outer 4 + gaps inner 8  =  58
```

The focused window's 2px `client.focused` border therefore occupies rows
**y=58-59**, and its right border occupies **x=1906-1907** (window rect
`x=12 w=1896`, confirmed via `swaymsg -t get_tree`).

**What was actually wrong with the panel, and what was not:**

- ❌ *"`control-center-margin-top` clamps the panel at y=58 for any value ≥14."*
  It does not clamp at all. Measured: 0→47, 2→49, 4→51, 6→53, 8→55, 10→57,
  14→61 — exactly linear. The old measurement was reading **the window's border
  row at y=58**, which never moves, so every margin value looked the same.
  ⚠ **Identify the panel by its colour** (`@panel` over dark content reads
  `(24,24,36)`) **and across a wide row run, not one column.** A single-column
  probe at x=1700 manufactured a *second* fake clamp later in this same work
  (margins 6 and 8 both reporting y=60) — the row-wide test
  (`>300 panel-coloured px across x=1500..1900`) is what gave the clean line
  above. Two different bad measurements, same wrong conclusion.
- ❌ *"At margin 8 the panel intruded into waybar's exclusive zone."* At 8 the
  panel top is y=55, well *below* the boundary at 46.
- ❌ *"14 is the right value, because it keeps the panel clear of the border
  rows so the corner radius cannot leak them."* True while a corner was rounded
  — **obsolete now that all four are square.** There is no cut-out, so the panel
  just covers the border uniformly, and covering it is *better*: at 14 a bright
  blue line sat in the gap between bar and panel (y=58 reads **177** in blue); at
  5 the panel covers it and it reads **49**. See *Margins* below for the values
  that replaced it.
- ❌ *"A straight-edged stub pokes out of the rounded corners."* That shape was
  a **GTK tooltip** — see the caution below. With the pointer parked away, row
  y=58 is continuous full-strength `#89b4fa` `(137,180,250)` across the whole
  screen, and under the open panel it is a smooth gradient (250 → 177 under the
  body, easing back to 218 at the corner) with no discontinuity anywhere.
- ✅ *"It is sway's own window decoration, not drawn by swaync."* Correct — but
  it was the **indicator**, not the border: an ~11px cream patch at
  x=1906-1907, y=60-67, peak `(204,187,185)`, i.e. `client.focused`'s
  `indicator` `#f5e0dc` on the window's right border. The left corner has no
  equivalent because the window's left border is at x=12, nowhere near the
  panel.

**The sign was backwards.** This used to be written up as "a translucent panel
with rounded corners *reveals* what is behind the cut-out". In fact `@panel` is
94% opaque and is what **conceals** that border, dimming it from
`(245,224,220)` to `(36,35,47)` everywhere it covers it. The radius leaves the
first few rows uncovered, and those are the tick. Which is why **both margin
levers make it worse, measured**:

| change | cream px in corner | peak |
|---|---|---|
| `margin-top 14`, `margin-right 8` | 11 | 204 |
| `margin-right` 14 / 16 / 20 | 60 | 218 / 221 / 224 |
| `margin-top 26` | indicator exposed 15px instead of 7 | — |

Squaring covers those rows instead of moving them, which is why it works — and it
had to be applied to **all four** corners, not just the top two.

**Squaring only the top was an incomplete fix.** The first pass reasoned that the
bottom corners were far from any border, on the assumption that the panel is
`control-center-height` tall. It is not:

> [!IMPORTANT]
> **`control-center-height` is inert while `fit-to-screen: true`.** Measured at
> 300, 600 and 900: the panel is **1130px** every time (y=61..1190), filling the
> output minus the margins. So `"control-center-height": 600` describes nothing
> — do not reason about the panel's extent from it.

At 1130px the panel's bottom edge (y=1190) sits just past the window's bottom
border (y=1186-1187), so the bottom corners clipped it under exactly the same
geometry as the top. That exposure was measured, judged "faint enough to keep the
rounding" — and it was still plainly visible in use as a bright block welded to a
rounded corner. **The mismatch is what the eye catches, not the brightness**: a
straight bright segment against a curve reads as a defect at a fraction of the
contrast that the same segment needs to be noticed against a straight edge. Pixel
counts are the wrong metric for deciding whether to keep a radius.

Bright pixels strictly inside the panel, per corner:

| `border-radius` | top-right | bottom-right | bottom-left | top-left |
|---|---|---|---|---|
| `18px` (original) | 7 | 3 | 30 | 0 |
| `0 0 18px 18px` (first pass) | 0 | 3 | 30 | 0 |
| **`0`** (shipped) | **0** | **0** | **18** | **0** |

The bottom-left residue of 18 is **not** a corner cut-out — it is the border
bleeding through the panel's own `1px solid @surface0` edge at x=1443, a 1px-wide
effect no radius change can reach. Every actual cut-out leak is zero.

At full height against the screen's corner the squared panel reads as a docked
sidebar, and inner elements (buttons, notifications, the Clear-all pill) keep
their own radius so the panel still feels soft — only its outer edge is hard.

> [!TIP]
> **Parse the stylesheet with the real GTK engine before trusting a reload.** One
> bad property discards the *entire* file (see the header of
> `config/swaync/style.css`), so a radius edit that silently broke something else
> would present as "the panel looks like a stock GTK dialog", not as an error.
> The one-liner in that header returns `NONE` for the shipped file; `swaync-client
> --reload-css` then confirms `CSS reload success: true`.

### Margins: the panel is a sibling of the bar island, and both values are derived

Squaring the corners removed the constraint that had pinned `margin-top` at 14,
which freed the panel to sit where it belongs relative to the bar. It had been
hanging **17px** below the island with the window's bright blue border line
stranded in the gap. Both margins are now derived from the bar's own geometry in
`config/waybar/config` — **re-derive them if that changes**:

| value | why |
|---|---|
| `control-center-margin-top: 5` | The island's visible rows are y=9-42, i.e. it floats **9px** below the screen top. A 9px gap below it (43 → panel top y=52) mirrors that exactly. |
| `control-center-margin-right: 10` | Puts the panel's right edge at x=1908 — **the island's own right edge** — and 10 *is* waybar's `margin-right`, so the two share one inset. Measured: 8 and 9 land 2px and 1px outside it. |

`panel top y = 47 + margin-top`, linear. The gap options, if the balance is ever
revisited: margin 0 → 4px gap (reads as docked to the bar), 2 → 6px, 5 → **9px
(shipped)**, 14 → 18px (the old detached look). Corner leaks stay at 0 for every
one of these, so this is now purely an aesthetic dial.

> [!WARNING]
> **Do not reach for the margins to fix a corner artifact while any corner is
> rounded.** `@panel` is 94% opaque and is what *hides* the window border, so
> moving the panel off it restores the border to full strength. With `18px`
> corners, `margin-right` 14/16/20 took the top-right corner from 11 cream pixels
> to **60**, and `margin-bottom: 20` took the bottom-right from 3 to **60**.
> Square the corner instead; then the margins are free for layout.

### The window border itself: `hide_edge_borders --i3 smart`

Squaring the corners stopped the border *leaking through* the panel, but the
border was still there — a bright `#89b4fa` frame around the whole screen,
**2876 pixels along the top edge alone**, and against a panel whose outer edge is
deliberately square and quiet it was the loudest thing on screen. It kept reading
as an outline attached to the panel. `hide_edge_borders --i3 smart` in
`config/sway/config` takes it to **0**, verified with the panel both open and
closed, and `sway --validate` exits 0.

> [!IMPORTANT]
> **This did NOT need a patched sway, and that was the first instinct.** Before
> reaching for a source build, check whether a config directive already does it —
> measured live with `swaymsg` (which applies at runtime and is reverted by
> `swaymsg reload`, so it costs nothing to try):
>
> | setting | border px, top / bottom / left |
> |---|---|
> | baseline | 2876 / 2876 / 2260 |
> | `hide_edge_borders --i3 smart` | **0 / 0 / 0** |
> | `default_border none` | 0 / 0 / 0 |
>
> A fork would have meant carrying a fourth local build and re-patching it on
> every upstream release, for what one directive does. **The one thing here that
> genuinely cannot be done in config is rounding the window's own corners or
> blurring behind it** — plain sway has neither at any setting, and that needs
> **swayfx** (a fork, not a patch to sway). That is the only reason to build.

⚠ **`--i3` matters**, and `smart_no_gaps` is the wrong value: it additionally
requires gaps to be zero, and `gaps inner 8` / `outer 4` mean it would never
trigger. What it costs is recorded at the directive itself — with one window per
output on multi-monitor there is now no border anywhere, so it can no longer tell
you which *screen* has focus. Chosen knowingly over the two alternatives below.

Levers **not** taken, in case the call is ever revisited:

- **Dim the border instead of hiding it** — recolour `client.focused`'s border
  and indicator to a dark surface tone (`#313244`). Quiets the frame without
  hiding anything, so the multi-monitor focus cue survives. This is the
  non-destructive fallback if losing that cue turns out to matter.
- **Match the indicator to the border** — `client.focused` indicator
  `#f5e0dc` → `#89b4fa`. Was the minimal fix for the *corner tick* specifically,
  back when the corners were still rounded; superseded by squaring them.
- `default_border none` — no borders in any layout, including while tiling.

> [!CAUTION]
> **Control the cursor position deliberately when screenshotting the bar — and
> take shots BOTH ways.** Parking it away isolates the compositing question, but
> it also removes the tooltip, i.e. the thing the user is actually complaining
> about. Every diagnostic shot in this investigation was taken cursor-parked, so
> the tooltip never appeared in any of them and got written off as a measurement
> artifact rather than the defect. A shot that does not reproduce the user's view
> is not evidence about the user's view. Park with
> `swaymsg "seat - cursor set 960 600"`; reproduce with the cursor on the button.

> [!CAUTION]
> **`cursor set` does not reliably produce GTK `:hover` or a tooltip — walk the
> pointer in with `cursor move`.** After waybar reloads on `SIGUSR2` its new
> widgets do not pick up a pointer that has not moved since, so an absolute warp
> lands the cursor on a button that never enters the hover state. This produced a
> **false pass** on the tooltip fix: zero tooltip pixels, because no enter event
> ever fired. The reliable recipe, and the proof it worked:
>
> ```sh
> swaymsg "seat - cursor set 1884 60"                       # below the target
> for i in $(seq 7); do swaymsg "seat - cursor move 0 -5"; sleep 0.12; done
> sleep 2.5                                                 # past the tooltip delay
> ```
>
> ⚠ **`cursor set` and `grim -g` DO NOT USE THE SAME COORDINATE SPACE, and the
> numbers above are right — do not "correct" them.** `grim -g` takes GLOBAL
> coordinates, so capturing the bar needs the output's offset (`0,120` here);
> the cursor commands do NOT take that offset, so the bar is at y≈26 for them.
> Adding the offset to a `cursor set` puts the pointer 120 px below the bar,
> where it hovers nothing — and the failure is silent, because `cursor set`
> still returns `success: true`. Cost real time here: it looked exactly like
> "hover is broken", including on modules whose `:hover` had worked for months,
> and briefly implicated `seat -` (which is fine; `seat -` and `seat seat0`
> behave identically). Sweep y and watch for the reaction if in doubt.
>
> **Always assert the hover state itself**, or you cannot tell "tooltip
> suppressed" from "pointer never arrived": count waybar's blue `:hover`
> underline inside the pill (y=34-41, `B > 120 and B > R + 40`). It must be
> non-zero. Hovering a module with a *known-good* tooltip — the clock — is the
> control that proves tooltips work at all in that session.

> [!CAUTION]
> **Do not measure this with an exact RGB match.** That is what produced a wrong
> "artifact is gone" conclusion during this work. The panel's `box-shadow`
> alpha-blends whatever is behind it, so counting pixels equal to exactly
> `#89b4fa` reads **0** while the line is plainly on screen. Compare crops, or
> match on hue/ratio, not equality. The check that finally worked was "warm
> pixels in the corner box": `R > 150 and R > B` over x=1898-1913, y=58-95 —
> it separates the cream indicator from both the blue border and the panel.

> [!TIP]
> **`grim -g` coordinates are NOT screenshot coordinates.** `eDP-1` is at
> **y=120** in the global space, so a region captured with `grim -g` is the
> screenshot's y **+120**. Getting this wrong silently captures a different row
> and looks like the panel moved between shots — it cost real time here. Take a
> full-screen `grim` and crop in Python instead; check with
> `swaymsg -t get_outputs`.
