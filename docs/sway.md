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
| Applets | `network-manager-gnome` `blueman` | **right**-clicking the network module (left-click opens the control center); Bluetooth pairing. nm-applet is also NetworkManager's secret agent, so without it wifi password prompts never appear |
| Calendar | `gir1.2-gtklayershell-0.1` | `sway-calendar` dies on import — and since waybar launches it from a click, the clock just silently does nothing |
| Python helpers | `python3-gi` | `sway-calendar` and `sway-tray-filter` both die on import. The tray one takes the whole tray with it, so every app that hides itself on close becomes unreachable |

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
> **`gir1.2-gtklayershell-0.1` and `python3-gi` are the entries that are not probed
> on `PATH`.** They ship a GObject-introspection typelib and a Python package
> respectively, and no binary, so `exec.LookPath` would report them missing forever
> and `install sway` would reinstall them on every run. `swayPkgGlobs` in
> `src/modules/sway.go` checks for a marker file instead — for the typelib, globbing
> both Debian's multiarch directory and the plain one; for PyGObject, both Debian's
> `dist-packages` and everyone else's `site-packages`. Adding another binary-less
> package means adding a glob there.

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
| `$mod+n` | Toggle the swaync control center |
| click the clock | Open the month calendar (`sway-calendar`) — see *The clock is the center module* |
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
  are changing it, which is when swayosd puts it center-screen. Set it
  deliberately from the swaync panel's slider.
- **Volume is icon-only.** Same reasoning; the glyph still carries muted-or-not,
  which is the part you read at a glance.
- **Network is icon-only.** The glyph is a signal-strength ramp; SSID, address
  and dBm are one hover away.
- **Battery keeps its percentage**, and the clock keeps the date. These are read
  at rest rather than adjusted, so a number earns its place.

Color is reserved to *mean* something: yellow and red are left free for warning
and critical states, and nothing at rest competes with them — the focused
workspace and the hover ring are a dark pigment plus a lit edge rather than a
hue (see *Selection and hover*), and the clock is plain `@text`. An earlier scheme gave every module its own hue, and a red
battery warning had to compete with five other colors.

Modules that hide themselves entirely when idle: `privacy` (mic/camera/
screenshare), `mpris`, `network#vpn`, and `tray`. An indicator that is always lit
is one you stop seeing.

## The tray, and the broker that makes it possible

For a long time this bar had **no tray at all**, and the write-up in
`config/waybar/config` defended that as a taste decision. It was not: it was one
unfilterable icon, and the cost was much larger than the icon.

### What the missing tray actually cost

An app that hides itself on close is **stranded** without a tray. Measured on
Discord: its close button (`swaymsg '[app_id="discord"] kill'`, which is what the
titlebar X sends) leaves **zero windows and a live process**. sway cannot help —
the client unmapped its own toplevel, so there is nothing left in the tree for
any `swaymsg` criteria to address, and no scratchpad trick reaches it. Calling
`Activate` on its `StatusNotifierItem` — exactly what a tray left-click does —
brought the window straight back. So the tray is not decoration here; it is the
**entire recovery path** for Discord, Steam and KeePassXC.

### Why the icon could not just be filtered

`nm-applet` publishes an item that is always `Active`:

```
Id 'nm-applet'   Status 'Active'   Category 'SystemServices'   IconName 'nm-signal-75'
```

and it does so with `--indicator` dropped **and** with
`gsettings set org.gnome.nm-applet show-applet false` — both retested 2026-08-12,
neither suppresses it. It also cannot simply be stopped: nm-applet is
NetworkManager's **secret agent** on this box, the thing that prompts for a wifi
password on a new network, and it does that over D-Bus whether or not an icon is
on screen. And waybar cannot drop it either — `man waybar-tray` (0.12.0) lists
only `icon-size`, `spacing`, `show-passive-items`, `reverse-direction`, `expand`
and `on-update`. The `ignore-list` people remember belongs to `wlr/taskbar`.
`show-passive-items` is no help against an item that is `Active`.

Which left it duplicating the `network` module's own essid + strength readout,
permanently, in a bar that had every other number deliberately removed.

### `sway-tray-filter`: own the watcher, not the bar

The tray protocol has three roles: **items** (apps), a **host** (waybar, which
draws them) and a **watcher**, a single session-wide broker owning the bus name
`org.kde.StatusNotifierWatcher`. `config/sway/sway-tray-filter` is that watcher,
minus nm-applet. waybar is stock.

> [!IMPORTANT]
> **waybar does not insist on being the watcher, and this was measured, not
> assumed.** With the script holding the name, waybar came up and registered
> against it as a plain host (`RegisterStatusNotifierHost` → `/StatusNotifierHost/0`)
> and drew whatever list it was handed.

**The name flags are load-bearing.** sway forks its `exec` lines asynchronously,
so there is no way to make the script reach the bus before waybar. That race is
dissolved rather than won: waybar requests the name **with `ALLOW_REPLACEMENT`**
(measured — requesting it with `REPLACE_EXISTING` against a running waybar
returned `PRIMARY_OWNER`, i.e. the steal succeeded), so the script can take it
back at any point and waybar re-syncs from the new owner.

> [!WARNING]
> **Do not add `ALLOW_REPLACEMENT` to `sway-tray-filter`.** It is what would let a
> restarted waybar take the name back and silently restore nm-applet's icon. Not
> requesting it also makes a second copy harmless: the bus puts it `IN_QUEUE`, it
> sees it is not the owner and exits 0 — which is what makes `exec_always` safe
> with no pidfile, the same outcome as the pidfile guards in `sway-calendar` and
> `sway-workspaces`, for free. The failure mode is deliberately soft: if the
> process dies, waybar's own queued request makes it owner again and the tray
> comes back **unfiltered** rather than not at all.

### ⚠ The watcher must exist before the app starts

Measured in both directions:

- An app **already registered** re-registers happily when the watcher changes
  hands. Restarting the filter is safe; icons come back within a second.
- An app **launched while no watcher owns the name** never gets an icon at all,
  however long it runs. Chromium — so every Electron app, so Discord — decides
  once at tray-icon creation and falls back to X11 XEmbed, which does not exist
  under sway. libayatana clients (nm-applet) do not have this problem and retry.

Nothing reports this; the icon is simply absent. Hence the filter is autostarted
from `config/sway/config` rather than started on demand, and hence
`IsStatusNotifierHostRegistered` is answered **`true` unconditionally** — a
truthful `false` during the gap before waybar registers would cost the icon
entirely, and nothing consults that property to decide whether to *draw*, only
whether publishing is worth it.

To see what is registered and what was dropped:

```
sway-tray-filter -v                     # logs every item's Id and the decision
busctl --user get-property org.kde.StatusNotifierWatcher \
    /StatusNotifierWatcher org.kde.StatusNotifierWatcher RegisteredStatusNotifierItems
```

`--ignore ID` (repeatable) replaces the default list, which is just `nm-applet`.
`blueman-applet` registers no item at all — also measured — so it was never in
the tray and nothing changed for it.

### Where it sits, and what CSS can and cannot do to it

The tray is **last in `group/indicators`**, the self-hiding pill. It qualifies
for that group only because the filter removes the one item that would otherwise
always be there: measured with an empty registry, waybar's tray widget takes zero
width and the whole pill paints **0 pixels**. Last, because `modules-right` is
right-aligned — the further right a widget sits the less it moves, and app icons
are aim targets that should not shuffle sideways every time an `mpris` track
title changes.

> [!WARNING]
> **Padding goes on the item's *child*, not on the item — and the difference is
> the whole shape of the hover ring.** waybar wraps each item in a `GtkEventBox`,
> and an EventBox ignores the CSS box model when it asks for its size: measured,
> `padding` **and** `min-width` on `#tray > *` move nothing at all (`min-width:
> 26px` against an 18px logo left the pitch at icon-size + spacing, unchanged).
> The obvious conclusion — that the box is therefore stuck at the icon's size —
> is **wrong**: padding on the child inside it (`#tray > * > *`) *is* honored and
> the EventBox grows with it. That is the only way to get air between the logo
> and the ring. A non-inset shadow is not an alternative: waybar gives each item
> its own `GdkWindow`, so anything drawn outside the allocation is clipped.

The symptom of getting that wrong is specific and worth recognizing: **the ring
collapses to a narrow tall oval that clips the logo's sides**, against the
generous rounded rect every module chip lights. It reads as a radius problem and
is not one.

Settled geometry, all measured: `icon-size 18` + `padding: 0 7px` on the child
puts each item at **32px**, one pixel off the status chips' 31, so the two rings
read as the same object; `spacing: 1` then gives a **33.0px pitch**, matching the
status cluster's 32.5–33.5 exactly. `#tray` itself takes `padding: 0`, overriding
the shared chip rule's `0 9px` — once each item carries its own 7px that is pure
surplus at the ends of the row, and it had pushed the gap between the last module
chip and the first icon to 39px against everything else's 33 (0 brings it to
34.0). Radius is **12**, matching the chips; at 9 it read as a tighter, different
shape beside them.

Hovering lights a hairline ring and moves nothing — **ring only, no dwell
bloom**. Now that padding here is live that is a rule rather than a limitation:
the icons are a row, and growing one would shove its neighbours, which is the
bar-glitching effect the status chips were rebuilt to avoid (their bloom is
affordable only because it comes out of a margin they already reserve).
`needs-attention` gets a red ring borrowing the workspace chips' urgent language;
`-gtk-icon-effect: highlight` was the old treatment and is far too quiet to
interrupt, since it brightens a logo that is already bright.

These are the only widgets in the bar whose artwork is not ours — app-supplied
logos, usually a raw pixmap rather than a themed icon name (Discord's `IconName`
property errors outright; only `IconPixmap` answers). There is no lever to
desaturate or symbolize them: GTK3 offers `-gtk-icon-effect: dim|highlight` and
nothing else.

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
  This is not an optimization; deriving it caused a real bug where **brightness
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
| `config/sway/swaync-system-stats.patch` | swaync 0.11.0 | the `system-stats` widget (cpu / memory / temperature / **battery**) |
| `config/sway/swaync-cc-fade.patch` | swaync 0.11.0 | fade in/out for the control center |

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

**The control-center fade** reuses swaync's own `Animation` class (frame-clock
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
> readable frosted gray.
>
> That is **compensation for a dark backdrop, not the intended look.** Proven by
> temporarily swapping in a colorful wallpaper: at 1.0 the same settings make
> each pill tint to the color behind it, which is the whole point of the
> effect. **If the wallpaper changes, re-judge `blur_brightness`/`blur_saturation`
> first and expect to drop them.**
>
> The waybar pill alpha is the coupled half — it went 0.88 → 0.62, because blur
> is only visible through whatever the alpha leaves transparent. Raising it back
> towards opaque removes the glass *while still paying the GPU cost for it*.
> Change both together or neither.

Two settings deliberately **not** taken: `blur_xray` (it blurs only the
wallpaper, so a bar over a maximized terminal would blur something it is not in
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
the boundary is invisible. Push it above 1.0 and the rectangle lights up: a gray
block with **square corners sitting behind a rounded pill**, which is the
straight-edge-against-a-curve mismatch this desktop has been burned by before.

Luminance of the strip just outside the pill, minus bare wallpaper:

| `blur_brightness` | delta |
|---|---|
| 1.0 | **−2.8** (invisible) |
| 1.1 | +16.3 |
| 1.25 | **+45.0** (a gray block on every pill) |

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
Saturation *is* safe to spend: it scales color rather than lifting the region.

> [!NOTE]
> **The wallpaper was the fix, and it was applied.** The old one measured
> **13/255** in the strip under the bar, so the glass had nothing to render at
> `blur_brightness 1.0` and the pills were nearly invisible. It was replaced
> with an ESA galaxy image measuring well above that, and the pills now
> genuinely tint to what is behind them.
>
> ⚠ **Which image is in use is deliberately NOT recorded here** — it changes,
> and four files hard-coding its name and luminance went stale the first time
> it did. `~/Pictures/wallpapers/README.md` is the single place that tracks the
> current choice, the shortlist, and each candidate's measured bar-strip
> luminance.
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

## Monocle: `$mod+m`, and why it is not `fullscreen`

`config/sway/sway-monocle`. One window takes over the workspace's **tiling
area** — bar and gaps untouched — the rest sit hidden behind it, and the focus
keys swap which one is showing. Per workspace, so ws1 can be in monocle while
ws2 is tiled.

sway has no monocle layout, and the obvious build is wrong twice over:

> [!CAUTION]
> **`fullscreen` takes the whole output and ignores both the bar and the gaps.**
> Measured: a fullscreen window's rect is `0,120 1920x1200` — the entire output
> — against `10,172 1900x1138` for the tiling area. It covers waybar (the bar is
> `layer: top` and sway renders fullscreen above that layer) and discards the
> 10px gaps. `$mod+f` already does that; a monocle built on it adds nothing.

> [!CAUTION]
> **Every focus command is a no-op while a window is fullscreen.** Measured with
> three tiled windows and the first fullscreened — `focus left`, `focus right`,
> `focus next`, `focus prev`, `focus next sibling`, `focus parent` and even
> `move left` all left the focused container unchanged, and the same
> `focus right` moved focus the instant fullscreen was disabled.

So the window is **floated onto the workspace's own rect**, which sway publishes
with the bar's exclusive zone and the gaps already subtracted — there is no
geometry to compute and nothing to keep in step with waybar.

### Every window is floated, not just the one on show

Floating removes a window from the tiling tree, and `floating disable` reinserts
it next to whatever tiled container is focused — not where it came from. So
cycling by float-one/unfloat-one mangles the layout on every step:

```
flat    [43, 44, 45]  ->  [45, 43, 44]                      (rotated)
nested  [63,[splitv,[64,[splith,[66]]]]]
        ->  [[splith,[[splitv,[[splith,[[splitv, ...]]]]]]]] (wrappers accumulate)
```

Anchoring the reinsertion to the window's recorded left neighbour fixes the flat
case exactly and does **not** fix the nested one — and this box runs
`autotiling`, so every layout here is nested.

Monocle therefore floats **all** of the workspace's windows at once, stacked on
the same rect. The tiling tree is empty for the duration, so cycling is one
`focus` with zero tree operations, and the restore is a single deterministic
pass. ⚠ The trade, taken knowingly: **the restore is flat** — windows return as
siblings in their original left-to-right order, so a 2x2 grid comes back as four
columns.

### The geometry races, and where it had to be applied

Sizing the whole stack up front does not converge. Three increasingly careful
versions, floating four windows:

```
one call per window          1900x1138, 1004x680,  946x565, 1004x680
float-all then place-all     1900x1138, 1900x565, 1900x1138, 1004x680
all placements one message   still two wrong after four retries
```

Each `floating enable` makes sway re-tile the windows still tiled, and that
re-tile is evaluated against geometry that is still moving. Only one window is
visible at a time, so the others' sizes are not observable — geometry is applied
**lazily, to the window being shown**, against a settled empty tiling, with a
bounded read-back check because the very first show still races the float pass.
Measured after: every cycle step exact at `10,172 1900x1138`, order preserved,
nothing left floating.

#### ⚠ Lazy geometry goes stale when the area moves, and that reads as a bar bug

Lazy is right, but it has one failure mode: if the tiling area changes **while
monocle is already on**, nothing re-measures until the next `show()`, so the
window keeps a size that no longer fits. Seen for real — a workspace whose
windows sat 80px short and 80px low, long after the surface that had reserved
that space was gone. It presents as *"the bar is pushing windows down"*, and the
bar is not involved at all: the workspace rect was correct throughout.

`reconcile()` closes it, from two triggers that are **not** interchangeable:

- **An output change** — dock plugged or unplugged, mode changed — does emit an
  IPC event (`output`, always `change: "unspecified"`, which no window or
  workspace event uses). The server reconciles **every workspace in state** on
  it, not just the focused one: a dock plug moves the area on every output at
  once, and a workspace you are not looking at would otherwise keep its stale
  geometry until you switched to it and cycled.
- **A layer surface changing its exclusive zone** — waybar restarting taller, a
  panel appearing — emits **nothing**. There is no event to subscribe to, so
  this case is caught opportunistically on the next event of any kind.
  ⚠ "Any kind" means a real one: re-focusing the window that already has focus
  emits nothing, so `swaymsg [con_id=…] focus` at the shown window is not a way
  to trigger it. Anything that actually changes something is.

Two properties that keep it safe:

- **Only the shown window is corrected**, deliberately — the rest are behind it
  and unobservable, exactly as the lazy argument says, and each is placed
  against the current area when next shown (verified: a stale window cycled to
  comes back exact).
- **One attempt per distinct target.** `reconcile()` runs off events a `place()`
  can itself provoke, so a window that *cannot* reach the area — an XWayland
  client with size hints, a dialog with a max size, and monocle floats every
  window on the workspace including those — would otherwise be re-placed on
  every event forever. A memo keyed by con_id holds the last target asked for
  and is cleared the moment the rects agree.

Measured, one window event after a 40px zone change: old code `h=1138` against
an `h=1098` area, forever; new code `1138 -> 1098`, and `1098 -> 1138` when the
zone went away. The output-event path was confirmed on its own, with no window
event in play at all.

Two smaller facts worth keeping:

- **`floating enable` + `resize` + `move` must be ONE chained message.** Issued
  as separate messages the resize races the float and silently loses:
  `10,172 804x604` (the client's default) against `10,172 1900x1138` chained —
  with every individual command returning success.
- **`hide` must NOT be chained**, for the mirror-image reason: sway settles focus
  between messages, and the reinsertion point is decided from the focus that has
  already landed. Chained, the `floating disable` ran against the pre-focus state
  and left the window floating.

### ⚠ Float the window being shown LAST — focus and paint order are separate

`floating enable` **raises** the window it acts on. Floating the row in tree
order therefore leaves the *last* window on top, and that raise happens after
the correct window has already been focused — so focus and paint order
disagree: the right window holds input focus while a different one covers the
screen. Typing goes where you expect and the screen shows something else.

Reported as "focused the terminal, pressed `$mod+m`, and firefox went monocle".
The fix is ordering: float `shown` last so it is topmost by construction, before
any placement or focus is attempted.

> [!CAUTION]
> **A test that asserts focus and geometry cannot see this.** The original suite
> checked which window was focused and whether its rect filled the tiling area —
> both were already correct — and never checked which window was on **top**,
> which is the only thing visible. Assert the stacking order.

> [!CAUTION]
> **Two terminals do not reproduce it.** The symptom depends on the second
> window being slower to settle, so it needs a real terminal+browser pair.

Two things that will waste time when checking this by hand:

- **sway sometimes wraps a floated window in a NEW floating container**, so a
  workspace's `floating_nodes` holds wrapper ids rather than window ids.
  Comparing those to a window id silently never matches — it reported a
  correct case as broken. Walk into each floating node for the `pid`-bearing
  child instead.
- **Luminance will not tell a dark terminal from a dark browser.** Measured 38.1
  against 39.8 mean on the two states, i.e. indistinguishable. Identify the
  visible window from the tree, not from a screenshot.

### The keys are shadowed at runtime, not routed through the script

sway accepts `bindsym` and `unbindsym` over IPC (both return success), so
monocle installs its own focus/move bindings when it turns on and removes them
when it turns off. Monocle costs **nothing** when off — the config's own
bindings are in force, handled inside sway with no process spawned.

⚠ The shadowed bindings are read from sway, not hardcoded: `swaymsg -t
get_config` returns the raw config text, and the script shadows exactly the
`bindsym` lines whose command is `focus <dir>` or `move <dir>`, expanding
`set $var` first. Rebinding those keys in the config needs no change here.

### `$mod+t` is the bar-preserving alternative

`layout toggle split tabbed` — note the list; with only `split` it flips
splith ↔ splitv, which is not what the key looks like it should do. One key
flips the focused container between the tiling you built and one-window-fills-it.
It keeps waybar and costs a **25px tab bar row** (sway cannot hide it — neither
`default_border none` nor `hide_edge_borders` touch it). It is not a substitute
for monocle on this box: measured at workspace level with a nested tree it showed
*two* windows at once, because a tab holding a split shows the whole split.

## The workspace row is nine custom modules, not `sway/workspaces`

`config/sway/sway-workspaces` + `custom/ws1..ws9`. Added when the row needed a
**traveling highlight**: jumping 9 → 1 sweeps back through 8,7,6…2 before
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
center / right, with the right one also sitting 3 px lower. `.modules-left` and
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

### `smart_gaps off`: a lone window is framed like a tiled one

**Reverted to `off` on 2026-08-09, together with `smart_corner_radius disable`.**
A lone window now gets the same 10px gaps and the same 12px rounded corners as a
tiled one, so splitting a workspace no longer restyles the window you were
already looking at. Measured live: one window → `x=10 y=172 w=1900 h=1138`, i.e.
10px on all four sides, top unchanged at y=52.

This setting has now been written up both ways — treat any earlier revision as
history, not authority.

**The old "alone = full bleed" trio is dissolved, and only two of it moved.**
`smart_corner_radius` existed for exactly one reason: a window flush to the
screen edge cannot have rounded corners without cutting visible notches out of
the display corners. Nothing is ever flush now, so the reason is gone — and
re-enabling it alone would just square a window that has room to be round. The
two must stay in step.

`hide_edge_borders --i3 smart` **stays on and is no longer part of this set.**
It drops the *border* on a lone window, which is an independent aesthetic call
(see its own comment in `config/sway/config`); gaps do not re-justify a bright
`#89b4fa` frame. Verified it still hides the border once gaps exist: the columns
just outside the lone window read wallpaper, not border.

Verified after the change: no notch at the display corners (nothing is flush),
and the swaync panel's new 16px corners still measure **0 bright leaks** on all
four (peaks 76/76/56/94) under this geometry — the lone window's right edge now
sits at x=1910, the same place the two-window case put it when that rounding was
measured.

> [!CAUTION]
> **The `swaymsg reload` that applied this killed the compositor.** Sway died
> silently at 23:47:42 (every client lost its Wayland connection at once; no
> segfault, no assert, no coredump) and GDM started a fresh session ~10s later.
> The `sway-fx: rejected: layer_effects …` lines in the journal are the
> *consequence* — `swaymsg` could no longer connect — not the cause.
>
> The settings themselves are not implicated: the replacement session read this
> exact config at startup and came up clean and stable. What is suspect is the
> **reload path** under SwayFX 0.6 on DisplayLink, where a reload also restarts
> the `exec_always` children (swaybg had just re-run when it died). Unproven —
> reproducing it costs another session. If you reload often, save your work
> first.

## Hover: what each surface does, and what GTK3 makes impossible

Two different behaviors, chosen by how many things share the pill:

| pill | on hover |
|---|---|
| clock (alone in its pill) | the **pill itself** grows, 4px a side. No oval inside it, no shadow |
| workspaces, status (many items) | the hovered item's **outline ring lights immediately** at the size the chip already is; nothing moves. **Stay on it ~200ms and it blooms** +2px wide, +4px tall — still moving no neighbour |

The split is the point. Growing a chip grows its pill, because a pill hugs its
contents — legible when the chip *is* the pill's only content, and meaningless
in a nine-item row where the chip is a fraction of what you see. Measured:
workspace and status pills do not move a pixel in any hover state, clock pill
`168 → 176` (`876..1043` → `872..1047`, symmetric about the center).

### ⚠ The expand is DWELL-GATED, and the `transition-delay` is the whole mechanism

The chips used to inflate the moment the pointer touched them: a workspace chip
went 15 → 29px painted, a status chip 23 → 31, each paying for the growth out of
its own margin so the row did not reflow. Every measurement behind that scheme
was right and it still looked wrong in use, for a reason no single-chip
measurement can show: **sweeping the pointer along the row is the ordinary way
to use it**, and a row of boxes each part-way through its own inflate/deflate
reads as the bar glitching rather than as a highlight traveling. Landing on one
chip looked good; passing over nine did not.

CSS cannot tell arriving at a chip from passing over it — which is why the
obvious fixes (shorter durations, monotonic easings, removing the bounce) only
ever changed how violent the thrash looked. **A `transition-delay` can tell them
apart**, because a transition that has not started has nothing to animate back:
leave inside the delay window and the property never moves at all. So the delay
goes on `padding`/`margin`/`min-width` and *not* on the ring:

- **ring** — instant, both directions (120ms in / 150ms out). It is the
  acknowledgement, and delaying it would make the bar feel unresponsive.
- **size** — 100ms, after a **100ms** delay. It is the reward for stopping, and
  it is deliberately quicker than the wait in front of it: the dwell is what
  makes the bloom deliberate, so the bloom itself does not also need to be slow.
  The shrink on the way out stays at 150ms with no delay — see the asymmetry
  note below.

The two numbers were walked down from `160ms / 200ms`, each verified against
both cases that matter — a pass-through must not bloom, a settle must:

```
gate     brief pass   settled     panning, lit regions (pan speed)
200ms    29 × 24      31 × 28     all 29px  (~30ms/chip)
150ms    29 × 24      31 × 28     all 29px  (~30ms/chip)
100ms    29 × 24      31 × 28     all 29px  at 8ms, 59ms and 109ms/chip
```

⚠ **The gate must outlast the pointer's dwell per chip in a normal sweep, and
that is a property of the hand rather than of the CSS.** The only evidence a
value is still safe is a measured pan in which every lit region is still the
resting width — so re-run the sweep before shortening it again, at more than one
speed. At 100ms there is real margin: a deliberate 109ms/chip pan still bloomed
nothing, because the pointer has to hold ONE chip for the whole window, not
merely cross the row slowly.

Pan across the row and nothing changes size anywhere. Settle, and the chip under
the pointer blooms.

| workspace state | padding | margin | painted |
|---|---|---|---|
| rest, `.focused`, `.urgent` | 11 | 1 | **29** |
| `:hover`, settled | 12 | 0 | **31** |
| `.trail` | 8 | 4 | 23 |

`padding + margin = 12` throughout, so the allocation is 31 for all nine chips
in every state and no bloom can move a neighbour. The status cluster does the
same at allocation 33 (`min-width 13 + padding sum 18 + margin 1` at rest,
`min-width 15 + margin 0` settled).

> [!CAUTION]
> **⚠ TWO PROPERTIES THAT COMPENSATE EACH OTHER MUST CHANGE BY THE SAME AMOUNT
> PER SIDE, OR THE PILL SHAKES.** The status bloom was first written as
> `min-width: 13 → 15` against `margin: 1 → 0`: algebraically exact
> (`13 + 18 + 1 + 1 = 15 + 18 + 0 + 0 = 33`) and wrong in motion. **GTK rounds
> each animated length to an integer independently**, and those two sweep at
> different rates — +2 across one box against −1 on each of two margins — so for
> most of the animation the rounded pair does not cancel and the allocation
> transiently gains a pixel. Because `.modules-right` is packed at the END of the
> bar, that pixel moves everything to its left: the cluster visibly shook as the
> pointer passed through it.
>
> Measured, glyph centers in the right cluster while one chip bloomed:
> ```
> rest / settled   1711.5  1756.5  1787.0  1821.5
> mid-bloom        1710.5  1755.5  1786.0  1820.5     (−1)
> mid-bloom        1709.5  1754.5  1785.0             (−2)
> ```
> Chips to the **right** of the hovered one never moved — the tell for a
> right-aligned container, not a mis-set width.
>
> The fix is the pairing the workspace row already used and which measures
> stable: **padding against margin, +1 / −1 per side.** That does mean the
> per-glyph splits are duplicated on `:hover` — accepted, under one rule: **each
> hover row is its resting row + 1px on each side**, nothing else, so the sum is
> 20 and the offset is unchanged. Re-measured at ten sample times across the
> animation and on two pan speeds: **drift 0.0px on every chip**.
>
> ⚠ The resting `min-width: 13px` stays as it is, and `(min-width − 7)` must stay
> **even** — 12 made GTK's integer label centring floor, dropping every glyph
> half a pixel left.

⚠ **The resting box is no longer invisible.** It is the box the ring traces the
moment you hover, so a chip whose resting padding differs from its neighbours'
has a differently-sized oval — the "One box, one oval" discipline now applies to
the *resting* values, not just the hover ones.

Measured live, pointer walked in (an absolute warp fires no motion event and
therefore no `:hover`), grabbed at 50ms and at 2s:

```
                     brief pass                 settled 2s
ws5 hovered          x 140..168  29 x 24        x 139..169  31 x 28
battery hovered      x 1840..1870 31 x 24       x 1839..1871 33 x 28
ws1 focused, resting x  16..44   29 x 24        (same object as the ws ring)
ws1 focused + hovered                           NO CHANGED PIXELS AT ALL
clock hovered        pill 168 -> 176, no ring, no shadow
```

Both blooms are symmetric (+1px each side, +2px each end) and the whole-bar diff
touches only those columns — hovering `ws5` changes `x 139..169` and nothing
else in the bar, settled or not. Mid-sweep across all nine chips, the lit
regions measure **29px**, i.e. the gate holds: no chip in a sweep ever grows.

⚠ The unlit ring is 4px shorter than the old always-expanded one (`31 × 28` →
`31 × 24`), since that height was the margin the old hover spent unconditionally.
It is back at 28 once you settle.

> [!CAUTION]
> **`#id:hover` and `#id.focused` are the same specificity, so hovering the
> focused workspace was silently REMOVING its fill.** Both are (1 id, 1
> class-or-pseudo); GTK3 breaks the tie by source order like CSS does, and the
> generic `:hover` rule is later in the file, so its `background-color:
> transparent` beat `.focused`'s pigment: the current workspace went to
> ring-only while the pointer sat on it, i.e. hovering it made it look *less*
> selected. The same tie took the red gradient and the crust text off
> `.urgent:hover` — the one chip in the bar whose entire job is to be
> impossible to miss.
>
> This was always true; the size change masked it, since something visible
> happened either way. Removing the size change would have left it as the whole
> effect. `.focused:hover` and `.urgent:hover` now restate their own fill (1 id
> + 2, which wins outright) and set no geometry — ⚠ **do not delete them for
> "setting nothing but what the base state already sets".** Verified: hovering
> the focused chip now changes zero pixels.

> [!CAUTION]
> **Margin must never go negative, and the reason is not layout — the overflow
> is clipped on ONE SIDE ONLY.** A negative margin makes the painted box wider
> than the allocation, which lays out fine, but waybar gives each module its own
> `GdkWindow` and later siblings stack above earlier ones. So a chip paints over
> its left neighbour and is painted over by its right one: the ring comes out
> cut off down one side and off-center from its own digit. Measured at
> `padding 14 / margin -3`, which predicts a 35px box:
> ```
> ws5 hovered   ring x 142..173 = 32px   digit center 159, ring center 157.5
> ws9 hovered   ring x 258..289 = 32px   clipped at allocation end 289
> ```
> ws9 has nothing to its right and was clipped anyway, so this is the widget's
> own clip rectangle rather than a neighbour drawing over it. Both stopped
> exactly at `allocation.x + allocation.width` while overhanging freely left.

> [!CAUTION]
> **An offscreen GTK harness will not reproduce that clip.** Plain `GtkLabel`s
> have no `GdkWindow` of their own, so a probe built from them reports the full
> 35px and says the scheme works. This was measured as working offscreen and
> clipped on the real bar. The harness is still the right tool for *allocation*
> questions — it is how the padding/margin trade was found — but chip geometry
> has to be confirmed on the bar.

### ⚠ A duplicate `transition:` further down the file made the expand snap

The right-hand chips carried a second `transition` declaration, left over from
a hover underline that had since been deleted. Same specificity, later in the
file, so it won — and it listed only `background-color` and `box-shadow`,
silently dropping the `padding` and `margin` entries from the rule above. The
visible symptom was that the clock's hover expand had no motion in it at all.

Measured offscreen, painted clock width after PRELIGHT:

```
with the duplicate:   53 -> 67px between t=0 and t=20ms     (one frame; a snap)
without it:           53 -> 60 -> 64 -> 66 -> 67 over ~320ms
```

One state, one rule. This file has now been bitten three times by a later
duplicate quietly winning — this, the `#custom-notification` padding shorthand,
and a redundant second workspace `:hover` stub.

### The "slide" is asymmetric timing, because a ring cannot move between widgets

Each chip owns its own box, so there is no way to animate one outline from one
widget to another — GTK3 has no overlay to draw it in and no `transform` to
move it with. What is available is timing, and it is enough:

> GTK3 reads the transition from the style being animated **towards**, exactly
> as web CSS does. So the base rule governs hover-**out** and the `:hover` rule
> governs hover-**in**.

The ring is therefore quick to arrive (120ms) and slower to let go (150ms), so
sweeping the pointer along a row leaves the chip behind still lit as the next
one lights up, which reads as one outline traveling rather than as several
fading in and out. Verified offscreen by sampling the rendered ring's alpha over
time at in=100ms / out=400ms: entering reached full at ~150ms, leaving decayed
`255 → 191 → 126 → 62 → 0` across ~400ms.

Both halves of the *ring* are pure cross-fades — `box-shadow` and `color`, no
length — so the sweep case has no easing curve left to get wrong. The bloom is
the only length still animated on these chips, and it runs on a plain
`ease-out`: its target painted box **is** the allocation (31 on the workspaces,
33 on the status chips), so an overshoot has nowhere to go and would be clipped
on the right. The overshooting bezier survives only on the clock, which has no
neighbours and no allocation to clip against.

> [!CAUTION]
> **The exit must be monotonic, and its duration is a budget rather than a
> taste.** This shipped at 300ms using the same overshooting bezier as the
> arrival, and both were wrong: a bounce on the way *out* makes a departing chip
> spring past its resting size and settle back, and a long exit lets a fast
> pointer stack the whole row. The exit duration bounds how many chips a sweep
> can light, roughly `exit duration / time the pointer spends per chip`.
> Measured, real rings (≥15px) per frame across an identical sweep:
> ```
> 300ms bouncy exit  ->  up to 9 lit, several mid-bounce
> 150ms ease-out     ->  peak 2
> ```
> Shortening further buys nothing — 110ms measured identical on both the sweep
> and the boundary case below.

> [!CAUTION]
> **An overshooting easing on a SIZE has a hard ceiling, and it is the
> allocation.** This is why the dwell bloom uses a plain `ease-out`: it lands
> *on* the allocation, so any overshoot at all is clipped on the right — the
> same cliff the negative-margin caution describes, reached by a different
> route. When hover inflated from 15px the peak painted box was `15 + 14 * peak`
> and the budget was `peak ≤ 1.143`:
> ```
> cubic-bezier(0.34, 1.56, 0.64, 1)   peak 1.098  ->  30.4px   fits, but only just
> cubic-bezier(0.34, 1.25, 0.64, 1)   peak 1.020  ->  29.3px   <- what shipped
> ```
> 1.56 was also violent when re-triggered every few milliseconds at a chip
> boundary — which was the first hint that the *ungated* size change was the
> problem, not its tuning. The clock's pill growth keeps the 1.25 curve; it is
> one object with no neighbours and no allocation to clip against.

### ⚠ A 2px band at every chip boundary lights both neighbours, and no CSS fixes it

waybar's module event regions overlap by 2px, so a pointer in that band is
claimed by both chips at once and they trade it back and forth, each
perpetually mid-animation. Mapped from cold starts (park off the bar, walk in,
hold 1.3s), `ws2` allocation `46..76` against `ws3` allocation `77..107`:

```
x <= 74     ws2 alone, stable 29px
x = 75, 76  BOTH lit, both ~26px, oscillating
x >= 77     ws3 alone, stable 29px
```

It is deterministic rather than pointer jitter, and nothing in the stylesheet
causes it: **the event region is the full allocation regardless of the CSS
margin** — verified, `x=50` hovers `ws2` cleanly and settles at 29px even though
its resting painted box is only `54..68`. Transition duration does not touch it
either (110ms and 150ms exits measured the same spread, 21px on the thrashing
chip). GTK3 CSS has no hysteresis to add.

What the timing *does* control is how violent the trade looks, which is the
reason the exit is monotonic and short. Anything claiming to have fixed this
should be checked against the map above.

⚠ **This paragraph used to dismiss `transition-delay` here — "it would trade the
boundary case straight back for the sweep case, since it cannot tell a re-entry
from a pass-through" — and that was wrong twice over.** It does not need to tell
them apart: a delay makes *both* cases free, because neither holds the hover
state long enough to start the transition. And it costs the sweep case nothing,
because the delay is on the size only and the ring is what a sweep is for. That
one sentence is what kept the dwell gate from being tried for as long as it was.

⚠ **What the band costs is now much smaller.** The `~26px, oscillating` above is
what two chips trading the pointer looked like while an ungated size was
attached. Re-measured after the gate, walking the pointer to `x=75` and to
`x=76` and holding: **one chip lit, stable at 31px, over independent 2s and 3s
dwells** — no second ring and no oscillation in the captures. Two chips may well
still be trading the pointer underneath; what reaches the screen no longer says
so. Re-map it before believing any claim that the band itself is gone.

The lift under the status chips is a drop shadow, since `translateY` does not
exist here. ⚠ **The clock does not get one, and that is not an oversight.** A
non-inset shadow on a chip paints a dark oval just inside its pill — on a chip
that shares a pill with eight others that reads as picking one of them out, and
on the chip that *is* the pill it reads as a second pill nested 4px inside the
first. The clock's inset ring was removed for exactly that (it looked like an
inner pill, which it was); removing only the ring and keeping the shadow draws
the same nested shape in shadow instead of in light. Both or neither there. It is safe now in a way it would not have
been before `window#waybar` was tinted: the compositor's blur region is every
non-transparent pixel, the whole bar strip is already in it, and the shadow
falls well inside the strip — measured vertical extent y 12..39 against a pill
interior of y 12..40. Keep it tight anyway; a wide one would smudge across the
pill even though it can no longer leak blur.

> [!WARNING]
> **`transform` does not exist in GTK3, and a stray `transform:` invalidates
> the ENTIRE stylesheet.** Verified by feeding it to a real `CssProvider`,
> which rejects it outright. So the usual hover-scale is unavailable. What GTK3
> *does* interpolate: `padding`, `min-width`, `font-size`, `box-shadow`,
> `opacity`, `background-size`. The first three change the widget's allocation
> and therefore reflow neighbours; the last two do not.

> [!WARNING]
> **The pill CONTAINERS never receive `:hover`.** `.modules-left:hover`,
> `.modules-center:hover` and friends match nothing — they are `GtkBox`es and a
> `GtkBox` has no event window, so GTK never puts them in the prelight state.
> Measured both ways: a background color on `.modules-center:hover` produced
> **0** changed pixels, while the same property on the child painted the whole
> pill. Grow the child, never the container. (Same fact that makes a click on
> `sway-calendar`'s padding arrive at the toplevel.)

> [!CAUTION]
> **`background-color: transparent` does NOT remove the theme's hover
> highlight — `background-image: none` is the one that matters.** The GTK theme
> paints its prelight with a background *image* gradient, so setting the color
> transparent removes nothing visible; the chip still lit up by **+27**
> luminance. `config/swaync/style.css` already recorded this for Adwaita's
> buttons and it applies to waybar's chips too. This cost several rounds,
> partly because a red-background test proved our own selector *did* paint the
> chip — which made "our rule isn't reaching it" look wrong when the real
> answer was "our rule overrides the wrong property".

> [!CAUTION]
> **`cursor set` and `grim -g` do not share a coordinate space.** grim takes
> GLOBAL coordinates, so capturing the bar needs the output's offset (`0,120`
> here); the cursor commands do NOT take that offset, so the bar is at y≈26 for
> them. Adding the offset parks the pointer 120px below the bar where it hovers
> nothing — and `cursor set` still returns `success: true`, so it presents as
> "hover is broken" on modules whose `:hover` has worked for months. `seat -`
> and `seat seat0` behave identically; that is not the variable.

## The battery lives in two places, on purpose

The bar shows **only a glyph**; the percentage is in the control center. Same
rule as volume and brightness — a number you are not currently changing is
noise in the bar, and the exact value is one predictable click away. It is
also in the bar's tooltip (capacity · time-to · watts).

> [!IMPORTANT]
> **The glyph ramp has ELEVEN entries and the count is the whole point.**
> waybar buckets `format-icons` by percentage at `100/len`. With the five icons
> that used to be there the buckets were `[0-19][20-39][40-59][60-79][80-100]`,
> so **everything from 80% up drew the full battery** — at 85% the glyph
> claimed a full charge. Eleven gives ~9% buckets: 85% lands on `battery_90`,
> only 91-100% draws full, and 0-9% gets the empty outline. Check codepoints
> against the installed font's cmap before adding any; all eleven were verified
> present.

The panel-side readout is part of `config/sway/swaync-system-stats.patch`.
⚠ **swaync cannot do this natively** — its widget list has no battery, and the
`label` widget is static text (`update-command` exists in the binary but
belongs to buttons-grid *toggles*, setting their active state). Extending the
local patch was the only route, so **a swaync reinstall from the Debian package
removes the percentage entirely**; put `{capacity}%` back in the bar's format
until the patch is rebuilt.

⚠ **Auto-detection skips `scope=Device` supplies.** This box exposes two
`type=Battery` power supplies — the laptop cell and a Logitech mouse on a
Powerplay mat sitting near 80% forever — so "first battery found" can report
the mouse as the laptop. `config/waybar/config` pins `bat` explicitly for the
same reason. `battery-path` overrides the heuristic.

⚠ Both surfaces compute the glyph independently (waybar from its bucket ramp,
the widget from `(pct/10)-1`), so **they must be kept in step**; verified
agreeing at 40%.

⚠ `interval` is not optional. See *Two bar modules poll* above — battery
defaulted to 60s, which meant plugging in took up to a minute to show.

## The glass tint, and where the accent went

**Four surfaces share one material now**: the bar, the swaync control center,
`sway-calendar` and the fuzzel launcher. They are all a dark pigment
(`#11111b`) plus a lit hairline, and **none of them carries an accent hue in a
resting state**. Before this the panel was on blue-gray `@surface0` tiles, the
calendar on a flat `#1e1e2e` with a solid `#89b4fa` "today", and fuzzel on
Catppuccin surfaces with a blue match color — three different materials
sitting next to each other.

**As of 2026-08-09 they also share one corner radius: 16px.** fuzzel's
`radius=16` and the waybar pills already had it; `sway-calendar` had *no*
`border-radius` at all (square body hanging under a rounded clock pill) and the
swaync panel was deliberately squared to fix a corner artifact that has since
stopped firing. Both are now 16px, so the family is consistent in pigment, edge
*and* silhouette.

> [!CAUTION]
> **On both panels, rounding AND shadow must come from CSS — the compositor
> applies both to the layer SURFACE, and neither surface is the size of the panel
> it draws.** The calendar's spans the whole usable area so click-outside can
> dismiss it; swaync's is likewise much larger.
>
> `corner_radius` is merely inert — it rounds something invisible (proven by
> elimination: the swaync panel measured square while its namespace carried
> `corner_radius 12`).
>
> **`shadows` was actively wrong.** It drew the shadow around the *surface*, so
> it landed as a ~10px **full-width dark band under the bar**, in the gap between
> waybar and the window — very obvious on an empty workspace. The giveaway was
> that both panels darkened that band by the *identical* amount (−8.30 / −9.01 /
> −5.94 across three regions) despite being completely different sizes; a panel's
> own shadow cannot do that. `shadows disable` on both namespaces takes it to
> **+0.00**, and the panels now cast from `box-shadow` in CSS, where the shape is
> the real panel.
>
> ⚠ Those CSS shadows carry a **0-offset halo that is load-bearing**: it dims the
> few pixels outside the rounded corners where a cut-out would otherwise expose
> the focused window's border. Removing the compositor shadow without adding it
> took the bottom-right corner from 0 leaked pixels to 4 (peak 130); with it, 1
> (peak 104).
>
> ⚠ **Adding that CSS shadow is what forced the calendar's blur off.** The
> compositor blurs every not-fully-transparent pixel, so a soft `0 12px 36px`
> shadow hands it a ~36px ring of them and the blur region grows to the shadow's
> whole extent — measured, the wallpaper for ~40px around the calendar went
> visibly out of focus. Nothing is lost at 0.94 opacity; this is the same trade
> already made for swaync, and the same mechanism as the bar's pills.
>
> `swaync-notification-window` is **not** affected — its surface is sized to the
> notification, measured −0.00, so it keeps `shadows enable`. The bug is
> specifically a surface-much-larger-than-panel one.

### The layers, and why the numbers differ

| surface | value | why |
|---|---|---|
| `window#waybar` (the strip) | `#11111b` @ **0.32** | ties the three pills into one piece of glass |
| bar pills | `#11111b` @ **0.60** | eased once the strip carried part of the job |
| swaync / calendar / fuzzel | `#11111b` @ **~0.94** | **not blurred** — see below |

> [!IMPORTANT]
> **Do not copy the bar's alpha onto the other surfaces.** The bar can sit at
> 0.60 because it is genuinely blurred, so a low alpha reads as glass. The
> panel-sized surfaces are **not** blurred, so anything showing through arrives
> **sharp**: at 0.86 the terminal behind was plainly legible through the swaync
> panel. Measured bleed-through — the same panel pixels over a text backdrop vs
> over the wallpaper — **5.93 / 2.70 / 0.73** at 0.86 / 0.94 / 0.98.
>
> **Blur was re-tested for swaync rather than assumed**, since the bar's leak
> turned out to be soft shadows: with its panel shadow tightened, blur *still*
> smeared a 60px band beside the panel (backdrop texture 20.3 → 12.9 with blur
> on, unchanged with it off). A panel-sized surface spills regardless.

> [!NOTE]
> **Tinting `window#waybar` removed the blur leak at source**, which was not
> the reason for doing it. The compositor blurs every pixel that is not fully
> transparent; with the strip transparent, only the pills and their shadows
> were blurred, so the blurred area had a hard rectangular edge sitting in
> sharp wallpaper. Tinting the whole strip makes it one uniform blur region —
> high-frequency energy in the gaps between pills went **10.2 → 1.3**. The
> trade is that the wallpaper behind the *whole* bar is blurred, not just
> behind the pills.

> [!NOTE]
> **Darkening the pill is nearly free, which is not obvious.** Across
> 0.50 → 0.82 the pill's luminance moves 48.7 → 21.6 while color retention
> only moves 0.56 → 0.51, because `blur_saturation` preserves hue independently
> of luminance. So the smoked-glass look costs almost none of the tint and buys
> a lot of legibility. What matters is not the pill's absolute darkness but its
> **separation from the strip behind it** — below roughly +20 luminance the
> pills start dissolving into the bar.


The selected and hover states are a **dark pigment plus a lit edge**, not a
color fill. Apple's Liquid Glass guidance is that a tint modulates the
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
center each glyph *within its own chip*, but the gap you see is ink-to-ink and
every glyph's side bearing differs — nominally identical padding measured
**22 / 27 / 25 / 31 px**. Two rounds against the ink-span script brought it to
**29 / 29 / 29 / 31**.

⚠ **…and then that tuning had to be undone, because it was written in the wrong
property.** See *One box, one oval* below. Padding is no longer available for
spacing; `margin` is.

### One box, one oval

> [!CAUTION]
> **The hover outline is an INSET ring, so it is drawn on the chip's
> padding-box edge — which means padding stopped being invisible the moment the
> oval replaced the pill-growth cue.** Every padding value in the status cluster
> is now on screen. The scheme above had padding doing two incompatible jobs at
> once — centring the glyph *and* evening the ink-to-ink gaps — plus four
> different `min-width` floors left over from an anti-reflow pass. Measured on
> the live bar:
>
> | chip | padding | min-width | oval | ink off-center |
> |---|---|---|---|---|
> | `battery` | 9 / 2 | 14 | **25 × 28** | +3.0px |
> | `idle_inhibitor` | 9 / 13 | — | **29 × 28** | +0.0px |
> | `network` | 8 / 11 | 20 | **39 × 28** | +0.5px |
> | `pulseaudio` | 12 / 9 | 20 | **41 × 28** | +2.5px |
> | `custom/notification` | 10 / 10 | 26 | **46 × 28** | +1.0px |
>
> A **21px spread** in oval width — the battery's ring barely over half the
> hub's — with the glyph up to **3px** right of the ring it is supposed to sit
> inside.

The fix is to decouple the box from the glyph:

- **`min-width: 13px` + a padding sum of `18px` on every glyph chip** → a
  uniform **31 × 24** oval, whatever the chip prints — and **33 × 28** once the
  dwell bloom fires, which adds 1px to each side (sum 20, same offsets). ⚠ The
  bloom deliberately does **not** grow `min-width`, even though that would keep
  the splits in one place; see the shake caution in *Hover*.
- the 18px is **split** per glyph by that glyph's own ink offset. Moving *N*px
  of padding from left to right pulls the content box *N*px left without
  changing the total, so the ink lands on the ring's center line and the ring
  does not move.
- **`margin`** — outside the padding box, so it cannot touch the oval — is now
  the only lever for inter-chip spacing.

Result: oval width spread **0px**, ink centerd to within **±0.5px** (confirmed
twice, by ink bounding box and by luminance-weighted centroid, which agree). The
status pill is **24px narrower** (198 → 174).

> [!WARNING]
> **13, not 12 — and the parity is the point.** GTK centers the label in the
> content box by an **integer** offset, so with an odd gap between the box and
> the glyph's 7px natural width it floors, and the glyph lands half a pixel
> left. At `min-width: 12px` all five chips measured
> **−0.51 · −0.11 · −0.74 · −0.53 · −1.14px** — a bias in the *same direction*
> on every one, which is the signature of a rounding rather than of a bad
> correction value. `13 − 7 = 6` divides evenly and it disappears. Keep
> `(min-width − 7)` even.

> [!CAUTION]
> **A shorthand later in the file silently beat the longhand correction.**
> `#custom-notification` carried `padding: 0 10px` in its color rule, ~100
> lines below its `padding-left`/`padding-right` correction. Same specificity,
> later wins, and a shorthand overrides both longhands — so the bell's optical
> correction was dead code for as long as both existed, with nothing on screen
> to say why the glyph sat off-center. One chip, one place for its padding.

**The chip margins are deliberately all still 1px.** With uniform boxes and
centerd glyphs the ink-to-ink gaps come out at **23 / 21 / 24 / 24 px** — a 3px
spread against the 9px that justified hand-tuning them in the first place, and
not worth closing: the volume ramp is three glyphs whose ink is **4.9 / 7.3 /
9.8px** wide, so two of those four gaps swing **±2.4px** with nothing but the
volume level. Tuning them to the pixel would be tuning to one screenshot of one
volume. The same caveat is why `pulseaudio`'s own correction is 1px rather than
an exact value — its offset is 0.00 / 0.40 / 1.62px depending on level.

End clearance inside the pill is **5px** each side (`#status` padding 4 + the
chip's 1px margin) against 2px between chips, measured symmetric (pill interior
`1736..1908`, end ovals `1741..1771` and `1873..1903`). It looks tighter than
that at 6× magnification because the pill's 16px corner radius curves away
beside the end chips; it is not clipped.

⚠ `#mpris` and `#privacy` are deliberately **out** of the uniform-box rule.
mpris prints a track title, so it can never share the 31px oval — a text chip is
legitimately wider than a glyph chip — and pinning a 31px floor on a member of
`group/indicators` argues with that pill's requirement to collapse to nothing.
privacy renders its own icon row and has no hover rule at all.

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

## The clock is the center module, and the calendar is a real window

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
the same row as the control center, so the two line up when both are open.

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
# anchoring NEITHER left nor right is what centers it; there is no "center" anchor
```

`GAP` is **0** because a layer surface's margins resolve against the **usable
area**, so waybar's 52px exclusive zone is already subtracted — no bar arithmetic
— and a tiled window starts at that same y, so any margin drops the calendar
below the window beside it. (It was 6, from a comment that put the zone at 46 by
omitting waybar's `margin-bottom`; see *Margins* above.)
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
code**, so there was nothing in the calendar to optimize — a spawn-per-click design
sits permanently past the ~300ms where a click starts feeling sluggish. The window
is therefore built once by a resident server (autostarted from
`config/sway/config`, the same shape as `swayosd-server`) and a click just signals
it: **SIGUSR1 toggles visibility**.

Measured after: **~58-65ms** per click, and the server costs ~55 MB RSS resident.

> [!CAUTION]
> **The client path must not import GTK, and defining helpers above `import gi`
> does not achieve that.** Module-level imports run at exec time, before `main()`
> is entered — so the first version of this optimization put the dispatch inside
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
> center "stops opening" and the waybar bell does nothing, silently.
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

## One destination: every status readout left-clicks to the control center

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
| `pulseaudio` | control center | `sway-quickpanel` | `pavucontrol` |
| `network` | control center | — | `nm-connection-editor` |
| `battery` | control center | — | — |
| `custom/notification` | control center | — | DND toggle |
| `clock` | `sway-calendar` | — | — |

Two things not to undo by accident:

- **`nm-connection-editor` moved to right-click; it was not dropped.** It is what
  justifies `network-manager-gnome` in `swayPackages`, so check there before
  deleting that line. nm-applet's *other* job — being NetworkManager's secret
  agent, i.e. the wifi password prompt — is independent of any click and comes
  from the `exec nm-applet` line in `config/sway/config`.
- **`idle_inhibitor` is deliberately exempt.** It is not a readout; it toggles a
  Wayland object it holds on waybar's own surface (see the `modules-right`
  comment), so sending it to the control center would break the one thing it does.

`battery` had no click action at all before, so nothing was displaced there.

## The control hub (what used to be "the notification bell")

`custom/notification` is the **last** entry in `modules-right` and its glyph is
`md-tune` (U+F062E, two horizontal sliders) — the same shorthand macOS uses for
Control Center. It stopped being "notifications" when it took the corner: behind
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

> [!IMPORTANT]
> **The border half of this was superseded on 2026-08-09: the panel is rounded
> again at 16px** (`config/swaync/style.css`), matching fuzzel, the waybar pills
> and `sway-calendar`. The tooltip fix stands unchanged — that was always the
> cause you actually saw.
>
> Two changes since killed the border half. `hide_edge_borders --i3 smart` means
> a single-window workspace has **no border under the panel at all**, and the
> panel's drop shadow dims the ~5px band just outside its edge — precisely the
> band a radius exposes — so the border arrives in the cut-out pre-dimmed
> instead of undimmed. Re-measured with the worst case rebuilt deliberately (two
> windows so borders return, rightmost focused so `#89b4fa` runs the full height
> at x=1908-1909 under the panel edge): bright pixels per corner are **0/0/0/0
> at radius 16**, identical to squared, peaks 76/76/65/65 vs 76/78/56/56. The
> shadow does it: same column, panel open vs closed, y≤50 is untouched while
> y=54 goes (133,175,243) → (67,69,88).
>
> The measurement tables below are still correct **for the geometry they were
> taken in** — margins were `top 14 / right 8` then and are `top 5 / right 10`
> now, and `usable_area.y` is **52** (waybar `height 40 + margin-top 6 +
> margin-bottom 6`), not the 46 used below, which puts the panel top at y=57 and
> the window's top border at y=52-53 — i.e. *above* the panel, not under it.
> Re-derive before reusing a number from here.
>
> ⚠ **Confirm the panel is open before believing a clean corner measurement.** A
> closed panel measures zero leaks in all four corners and is indistinguishable
> from a pass; `swaync-client --reload-css` can leave it closed. Assert
> `swaync-control-center` is in `swaymsg -t get_outputs` → `layer_shell_surfaces`
> first. That produced a false pass during this very change.

**The tooltip is the one that mattered, and it hid the real story for months.**
`custom/notification` had `"tooltip": true`, commented as showing swaync's own
text ("3 Notifications"). That field is **empty when idle**, which is most of the
time, and waybar then falls back to rendering the module's *own label* — so the
tooltip was a rounded box containing nothing but a copy of the md-tune glyph you
were already pointing at. A GTK tooltip is its own surface and is **not** confined
to waybar's 40px height: this one drew at x≈1855-1910, **y≈46-93**, straight over
the control center's top-right corner and part of "Clear all". Its rounded corners
meeting the panel's edge is precisely the "two corners that don't line up" look.

And it was not an edge case — **it was the default view of the panel.** Opening
the control center by clicking that button leaves the pointer parked on it, so the
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

**The geometry, which is simple and was not.** ⚠ *The numbers in this block are
the ones in force when the border half was investigated, and both have since
changed — see the [!IMPORTANT] note at the top of this section. The zone is now
**52** (the `margin-bottom 6` term was missing), and the gaps are `outer 2` /
`top -8`, putting a window's top edge at 52.* As measured then:

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
  ⚠ **Identify the panel by its color** (`@panel` over dark content reads
  `(24,24,36)`) **and across a wide row run, not one column.** A single-column
  probe at x=1700 manufactured a *second* fake clamp later in this same work
  (margins 6 and 8 both reporting y=60) — the row-wide test
  (`>300 panel-colored px across x=1500..1900`) is what gave the clean line
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

At full height against the screen's corner the squared panel read as a docked
sidebar, and inner elements kept their own radius so it still felt soft. *(That
was the shipped look until 2026-08-09; the outer edge is now rounded at 16px
again — see the note at the top of this section.)*

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

**Superseded 2026-08-09 — the margins are now zero-ish, and that is the fix, not
a regression.** The panel is aligned to the *tiled window*, edge for edge:

| value | why |
|---|---|
| `control-center-margin-top: 0` | Panel top y=**52** = the window's top edge. `usable_area.y` is already 52 and a tiled window starts at exactly 52 (`gaps top -8` cancels `gaps inner 8`), so any margin here is pure surplus that drops the panel *below* the window beside it. |
| `control-center-margin-bottom: 10` | Panel last row y=**1189** = the window's last row; matches the 10px screen-edge gap windows get. |
| `control-center-margin-right: 10` | Right edge x=**1909** = the window's right edge, and 10 *is* waybar's `margin-right`. One shared inset. |

`panel top y = usable_area.y + margin-top`, linear, no clamp.

> [!WARNING]
> **The old derivation on this page was arithmetically stale, and that is why
> both panels sat low.** It read `panel top y = 47 + margin-top` (table 0→47,
> 2→49 … 14→61) and picked 5 to land on y=52. The real zone is **52** — waybar's
> exclusive zone is `height 40 + margin-top 6 + margin-bottom 6`, and the
> **margin-bottom was the term left out**. So 5 landed the panel on y=57, five
> pixels below the window. `sway-calendar` carried the identical error with the
> identical cause (`GAP = 6`, comment claiming "waybar's 46px (height 40 +
> margin-top 6)", landing on y=58); it is now `GAP = 0`.
>
> **Measure the usable area; do not recompute it from the bar's parts.** The
> exclusive zone includes *both* waybar margins, which is the term that keeps
> getting dropped. Verified after the fix: window top, swaync top and calendar
> top all read **y=52**, and swaync's last row is **y=1189**, the window's.

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
**2876 pixels along the top edge alone**, and against a panel whose outer edge was
deliberately quiet it was the loudest thing on screen. It kept reading
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

- **Dim the border instead of hiding it** — recolor `client.focused`'s border
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
