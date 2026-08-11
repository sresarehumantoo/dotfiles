# dotfiles-box — a persistent container as a WSL replacement

A long-lived ("pet") Linux container that stands in for WSL on a Windows host
that has Docker Desktop but no WSL2. CLI only — no GUI, no WSLg.

```
docker compose -f container/compose.yaml up -d --build   # from the repo root
docker exec -it -u owen box zsh -l                       # enter
```

On Windows, `container/windows/box.ps1` wraps that as a `wsl`-like command.

---

## Status

Built and **verified end to end on Linux Docker 29.7.1 (cgroup v2, overlay2)**:
zsh 5.9 login shell, oh-my-zsh, powerlevel10k rendering in truecolor, tmux 3.5a
auto-attach, Neovim 0.12.4 with Lazy plugins synced, `dfinstall` on `PATH`, SSH
entry with `COLORTERM=truecolor` propagated, OSC 52 clipboard, home persisting on
a named volume across container recreate.

**Partially verified on a real Windows host** (Server 2025 Core): `probe.ps1`
runs, and SSHFS-Win mounts and reads at usable speed — but **writes through the
mount are broken**, which is a live problem for the inverse-mount plan. See
*SSHFS-Win: measured*.

**Not verified at all:** bind-mount performance on the Hyper-V backend (needs
bare-metal client Windows with Docker Desktop), the Windows Terminal profile, and
`box.ps1`'s container subcommands (`enter`/`up`/`down`/`status`/`run`), which need
a local Docker daemon.

---

## Design decisions

### Why no systemd

Deliberate. Research into systemd-in-container under Docker Desktop's LinuxKit
VM produced no verifiable evidence either way, and nothing in this CLI stack
needs it: ssh-agent is handled by `config/shell/zsh/ssh.zsh`, and there are no
timers, no logind sessions, no dbus consumers. PID 1 is `tini`; the long-lived
process is `sshd`.

One thing that *is* established: a systemd-less host is not itself a blocker —
runc's `cgroupfs` driver is a fully supported cgroup v2 path, and the systemd
cgroup driver is explicitly optional ("highly recommended… though not
mandatory"). So if a future need for systemd appears, it is worth revisiting
rather than assumed impossible.

### Why the source tree is a named volume, not a bind mount

This is the single most important performance decision.

Under the **Hyper-V/LinuxKit backend**, Docker Desktop shares host bind mounts
over **gRPC-FUSE** — a userspace FUSE server over gRPC on Hypervisor sockets,
which replaced Samba/CIFS in Docker Desktop 2.1.7.0 (Dec 2019). Every file
operation crosses the host/VM boundary. Docker and Microsoft both document the
boundary itself as the cause; there is no tunable that fixes it.

Two corrections to advice you will find elsewhere:

- **virtiofs is not available on Windows.** It is a Mac / Docker-Desktop-for-Linux
  mechanism; Docker's settings reference still marks the relevant flags "Mac only".
  Hyper-V Gen2 exposes VMBus, not a virtio device model, so there is no transport.
- **`:cached` / `:delegated` are dead.** Legacy osxfs hints, no-ops today.

So `/work` is a **named volume** — native ext4 inside the VM, never crossing the
boundary. Windows-side editors reach *into* it over SSHFS-Win rather than the
tree being bind-mounted *out*. This is the same topology as `\\wsl$`.

> **No trustworthy benchmark numbers exist for any of this.** Every quantified
> figure surfaced during research (npm 47min→4min, yarn 62min, gRPC-FUSE "order
> of magnitude", Docker's own "2–10x") failed verification — they trace to a
> single blog post or to marketing with no methodology. The ranking below is
> architectural inference. **Measure on the real host before trusting it.**

Ranked options, fastest first:

| # | Option | Cost |
|---|--------|------|
| 1 | Source on a named volume (**what this does**) | Invisible to Windows tools without SSHFS-Win |
| 2 | Docker Desktop Synchronized File Shares | Paid Pro/Team/Business; every file stored twice; eventually consistent |
| 3 | Named volume + SSHFS-Win | Windows client unmaintained since Feb 2021 |
| 4 | Bind mount + named-volume overlay on hot dirs | Still crosses the boundary for the rest |
| 5 | Plain bind mount | Worst |

Synchronized File Shares **does** work on the Hyper-V backend — in fact until
Docker Desktop 4.78.0 (2026-06-15) Hyper-V was the *only* Windows backend where
it worked. It is Mutagen keeping an ext4 copy inside the VM in sync with the
host, i.e. architecturally the same trick as option 1, packaged and bidirectional.

### Why these modules are skipped

`container/config.container.yaml` sets `skip_modules`. The notable one is
**`fonts`**: in container-as-OS the font is rendered by the *Windows* terminal,
so the module would download ~28 MB of IosevkaTerm into a container with no
renderer. **Install the Nerd Font on Windows instead.** `ghostty`, `sway`,
`konsole` are GUI; `wsl` configures the thing being replaced; `vmguest` would
self-skip anyway (`DetectVirt()` reports `docker`).

### Why the image is a seed, not the home

`/home/owen` is built during `docker build`, then moved to `/opt/skel`. The
entrypoint copies it into the (empty) named volume on first boot and stamps
`~/.box-seeded`. Docker's native "copy image content into an empty volume"
behaviour would half-do this, but it is silent, does not apply to bind mounts,
and offers no deliberate re-seed.

**Consequence:** anything the image installs *into `$HOME`* — including
`~/.local/bin/dfinstall` — will **not** reach an already-seeded pet on rebuild.
Update it the way you would a real machine:

```
cd ~/dotfiles && git pull && make install-bin && dfinstall install all
```

To start over: `docker compose -f container/compose.yaml down -v` (destroys the
home and work volumes).

---

## Traps found while building this

- **`docker exec -t` without `-i` hangs.** The zshrc auto-execs
  `tmux new-session -A -s main` on an interactive shell. With a tty but no stdin,
  tmux blocks and leaves orphaned servers (PPID 0). Always pass `-it`.
- **`chsh` cannot work during `docker build`** — no TTY, and PAM refuses. The
  `defaultshell` module reports "Could not change shell" and the login shell
  stays bash, so sshd *and* tmux silently spawn bash and the entire zsh stack
  never loads. The Dockerfile sets it directly with `chsh -s /usr/bin/zsh`.
- **`.dockerignore` must not exclude `container/`** wholesale — the Dockerfile
  COPYs two files from it. Exclude the *contents* (`container/*`) and re-include
  those two; re-includes under a directory exclusion are unreachable.
- **`.git` must stay in the build context.** `bootstrap/wsl-setup.sh`'s
  `install_dotfiles()` only uses a local source dir when it finds
  `$source_dir/.git`; without it, it silently falls through to cloning from
  GitHub and you build `origin` instead of your working tree.
- **sshd needs an ecdsa host key** if the stock `sshd_config` HostKey lines are
  repointed, or it logs `Unable to load host key` every boot.
- **`AcceptEnv COLORTERM`** (image) + **`SendEnv COLORTERM`** (client) is what
  makes truecolor survive SSH. Verified: p10k renders `38;2;R;G;B` sequences.

- **A `curl: (22) 404` during the image build** turned out to be a real bug in
  `src/modules/delta.go`, not a container quirk: it built
  `/releases/latest/download/git-delta_<arch>.deb`, but delta embeds the version
  in the filename, so that URL had never resolved on any machine and every
  install silently fell back to the distro package. Fixed to resolve the asset
  from the release listing via the shared `latestAssets`/`pickAsset` helpers.

---

## What is irrecoverably lost vs WSL

- **Interop.** No calling `.exe` from Linux, no `clip.exe`, no `explorer.exe .`,
  no Windows PATH appended. There is no equivalent layer. The closest substitute
  is an SSH listener on the Windows side that the container dials via
  `host.docker.internal`. Not built here. <!-- decoupling-ok: Docker-provided DNS name, not site infrastructure -->
- **Clipboard** goes through the terminal with **OSC 52** rather than a Windows
  binary. **Already working** — see *Clipboard* below.
- **`\\wsl$` drive mapping** is replaced by SSHFS-Win, a genuinely weaker
  substitute: unmaintained since Feb 2021, and file *creation* is broken from
  elevated sessions by a known upstream bug. Reads are fast and work well. See
  *SSHFS-Win: measured*.
- **WSLg** — out of scope by choice.

---

## Windows bring-up (not yet done)

1. **Probe the host first** — read-only, changes nothing:
   ```powershell
   powershell -ExecutionPolicy Bypass -File container\windows\probe.ps1
   ```
   It reports which virtualization features are enabled (Hyper-V,
   VirtualMachinePlatform and WSL are *separate* features), whether this account
   can create Hyper-V VMs versus merely use Docker, the active backend, and
   whether Docker Desktop is new enough for Synchronized File Shares.

2. **Install a Nerd Font on Windows** (IosevkaTerm, to match this repo) and point
   the Windows Terminal profile at it.

3. **Build and start**, then set `BOX_AUTHORIZED_KEYS` in `container/.env` to the
   Windows public key so SSH and SSHFS-Win can get in.

4. **Windows Terminal profile** — `commandline` set to
   `powershell.exe -NoLogo -File <repo>\container\windows\box.ps1`.

### Still to build

- A Windows-side listener to substitute for interop, if it turns out to matter.
- Actual measurement of the file-sharing options on the real host.
- Confirmation that Windows Terminal accepts the OSC 52 write (it supports the
  sequence, but this has not been tested against it).

---

## SSHFS-Win: measured (2026-08-02)

Tested from a real Windows host (Server 2025 Core, WinFsp 2.1.25156 + SSHFS-Win
3.5.20357 — note that sshfs build is from Dec 2020) against the box running on a
Linux workstation over the LAN. **Reads are fine. Writes are broken.**

### What worked

Mounting on the **first** attempt, contrary to the open bug reports:

```
net use X: \\sshfs.kr\owen@<linux-host-ip>!2222\work
```

The `!PORT` form resolved correctly — no workaround needed. Read performance,
against a 1107-file / 99 MB git tree:

| operation | over SSHFS-Win | in-container (native ext4) |
|---|---|---|
| `git status` (cold) | 0.36 s | 0.008 s |
| `git status` (warm) | 0.07 s | 0.008 s |
| recursive enumerate | 2.50 s | — |
| read one file | 0.05 s | — |

These are a **pessimistic upper bound**: the traffic crossed a LAN (over Wi-Fi)
rather than the loopback interface it would use in the real topology. Warm
`git status` at 0.07 s is comfortably usable.

### Creating files failed — an upstream bug specific to ELEVATED sessions

Every file-*creation* API (`New-Item`, `Set-Content`, `Out-File`,
`[IO.File]::WriteAllText`, `[IO.File]::Create`) returned
`UnauthorizedAccessException` **while creating the file anyway** — 0 bytes, mode
`0700`, correct owner. `Copy-Item` succeeded fully. Writes to *existing* files
always succeeded.

This is [winfsp/sshfs-win#322](https://github.com/winfsp/sshfs-win/issues/322)
and [#186](https://github.com/winfsp/sshfs-win/issues/186), both open. #322
describes the exact signature — "a zero-byte file is created after the first
failure, then populated on retry" — and reports:

> Normal user sessions do not experience this issue; file creation succeeds on
> the first attempt for non-elevated users.

**The test above ran as `Administrator` on Server Core, i.e. always elevated.**
So the read-only behaviour is most likely an artifact of the test account rather
than a property of this design.

Systematically ruled out along the way:

- **Not Linux permissions.** `/work` is owned by `owen` (uid 1000), and that user
  writes fine over plain SSH.
- **Not mode-derived ACLs.** Files pre-created server-side at `0600`, `0644`,
  `0666` and `0777` were **all writable** through the mount, and `Get-Acl` maps
  the remote owner to the local user (`Write, Delete, ChangePermissions,
  TakeOwnership`). This disproved the initial "0700 grants Everyone nothing"
  theory.
- **Not subdirectory-specific.** The volume root fails identically.

### UNRESOLVED: the non-elevated case was never proven

Running the identical test under a purpose-made non-admin account did not reach a
verdict. Three obstacles, none related to the actual question:

1. `SeBatchLogonRight` is not granted to `Users`, so a scheduled task under that
   account never starts (`0x41303`).
2. A fresh account has no `known_hosts`, and a service-context mount cannot
   accept a host key interactively (error 67).
3. Cygwin's `ssh` refuses a private key whose Windows ACLs map to
   group/other-readable — `Load key: Permission denied` (error 64).

**Do not read this section as "SSHFS-Win is read-only."** Read it as: writing
from an *elevated* session is broken by a known upstream bug, and the ordinary
non-elevated case is untested here. Retest on client Windows under a normal user
account — which is the intended deployment anyway.

### Consequence for the design

The inverse mount is **not** disqualified. If the upstream reports hold, a normal
user session writes fine and option 3 stands. Confirm before relying on it; if it
does not hold, the fallbacks are editing from inside the box (nvim over SSH), or
Synchronized File Shares.

### Prefix trap

`\\sshfs.k\...` is relative to the remote **home**; an absolute path like `/work`
fails with "The network name cannot be found". `\\sshfs.kr\...` is relative to
the remote **root** and is the correct prefix. The drive argument to
`sshfs-win.exe` also requires its colon (`X:`, not `X`). `box.ps1` originally had
both of these wrong.

---

## Clipboard (OSC 52) — verified working

No new configuration was needed: the repo's `tmux.conf` already sets
`set-clipboard on`, and `config/nvim/init.lua` already falls through to
`vim.ui.clipboard.osc52` when neither Wayland nor X11 is present — which is
exactly the container's situation.

Verified in the container by decoding the emitted bytes, not by inspection:

```
nvim yank ->  ESC ] 52 ; c ; aGVsbG8gb3NjNTIK   ->  base64 -d  ->  "hello osc52"
```

and again with nvim running *inside* tmux, confirming tmux relays it outward.

### Trap: the clipboard dies silently unless TERM is `xterm*`

tmux decides whether to forward OSC 52 from its **built-in terminal-features
table keyed on TERM**, not from terminfo's `Ms` capability. Measured in this
container:

| outer TERM | OSC 52 relayed |
|---|---|
| `xterm-256color` | yes |
| `screen-256color` | no |
| `vt100` | no |

`infocmp` shows **no `Ms=` for any of the three**, so terminfo is not what is
driving this. There is no error when it fails — yanks simply never reach the
Windows clipboard.

Consequences:

- `box.ps1` forces `TERM=xterm-256color` when `$env:TERM` is unset, which is the
  normal state in Windows PowerShell. Without that it would pass an empty TERM
  and lose the clipboard.
- Over SSH the client's TERM is what counts; make sure the Windows Terminal
  profile is not sending something non-`xterm*`.
- If you want this to be robust regardless of TERM, adding
  `set -as terminal-features ',*:clipboard'` to `config/tmux/tmux.conf` would
  tell tmux every terminal supports OSC 52. That is a **shared** config change
  affecting the Linux/Ghostty setup too, so it is deliberately not made here.

### Testbed note

Reproducing the target scenario needs **bare-metal client Windows** — which is
harder to come by than it sounds:

- **Windows Server is out.** Docker Desktop is not supported there at all, so a
  Server box can only exercise the Windows-side scripts (which is how the
  SSHFS-Win measurements above were taken).
- **A nested VM is out for performance work.** Nesting does not apply a uniform
  multiplier; it disproportionately penalises VM-exit-heavy paths, and gRPC-FUSE
  over hypervisor sockets is exactly that. A nested VM would exaggerate the
  bind-mount penalty — useful for confirming the sign, useless for magnitude.
  It is fine for functional testing.
- **An existing workstation is not free.** Enabling Hyper-V is not a
  test-only cost: Windows then runs permanently as a Hyper-V root partition,
  which is persistent overhead on GPU/compute workloads and adds friction with
  kernel anti-cheat.

So: functional testing on a throwaway VM (or a Server box, for the parts that do
not need Docker), and a short, deliberate window on real hardware for the
performance numbers — with the benchmark harness written in advance so that
window stays short.
