#!/usr/bin/env bash
# Exercise the macOS bootstrap and dfinstall on a Mac that is already set up and
# in daily use, without changing it.
#
# Usage: scripts/macos-harness/run.sh [fresh|upgrade|all] [--interactive] [--keep] [--strict]
#
#   fresh    The bootstrap and `dfinstall install all` as a brand-new account
#            would run them: HOME, XDG and TMPDIR under /tmp, a Homebrew that
#            installs nothing (each formula and cask it is asked for is checked
#            against the real Homebrew, read-only, and recorded), and shims in
#            place of every command that acts on the whole machine or login
#            session (defaults writes, launchctl, hidutil, chsh, sudo,
#            activateSettings, ...), which record what they were asked to do.
#   upgrade  This checkout's dfinstall against your real setup, read-only:
#            status, doctor, diff, `install all --dry-run`, plus the bootstrap's
#            own preflight and TLS-inspection probe. The same shims are on PATH,
#            and they never read a sandboxed app's preferences (that needs Full
#            Disk Access, and can raise a privacy prompt).
#   all      Both (the default).
#
#   --interactive  Leave the terminal attached, so prompts can be answered.
#                  Without it the run has no terminal, as in CI.
#   --keep         Keep the sandbox (it is deleted otherwise; the report is not).
#   --strict       Also fail when a formula or cask does not exist, or when the
#                  real Homebrew could not be asked (CI).
#
# Before and after, the real state dfinstall manages is snapshotted (dotfile
# links, ~/.config/dfinstall, the dotfiles clone, the defaults domains it
# writes, key remapping, launch agents, login shell, Homebrew's package list).
# Any difference is reported and the run exits 1. The report lands in
# /tmp/dfinstall-macos-harness-<time>/; review it before sharing it.
set -euo pipefail

usage() { sed -n '2,33p' "$0" | sed 's/^# \{0,1\}//'; }

scenario=all interactive=false keep=false strict=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        fresh | upgrade | all) scenario="$1" ;;
        --interactive) interactive=true ;;
        --keep) keep=true ;;
        --strict) strict=true ;;
        -h | --help) usage; exit 0 ;;
        *) usage >&2; exit 2 ;;
    esac
    shift
done

[[ "$(uname -s)" == Darwin ]] || { echo "macos-harness: macOS only" >&2; exit 2; }
[[ "$(id -u)" -ne 0 ]] || { echo "macos-harness: run as your normal user" >&2; exit 2; }

repo="$(cd "$(dirname "$0")/../.." && pwd -P)"
REAL_HOME="$HOME"
REAL_PATH="$PATH"
REAL_BREW="$(command -v brew || true)"
REAL_GO="$(command -v go || true)"
[[ -n "$REAL_GO" ]] || { echo "macos-harness: needs Go on PATH to build dfinstall" >&2; exit 2; }

out="/tmp/dfinstall-macos-harness-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$out"
out="$(cd "$out" && pwd -P)"
sb="$out/sandbox"
shims="$out/shims"
fakebrew="$sb/brew"
calls="$out/shim-calls.log"
report="$out/report.md"
mkdir -p "$sb/home" "$sb/tmp" "$sb/go" "$sb/brew-cache" "$shims" "$fakebrew/bin" "$out/bin"
: >"$calls"

# shellcheck disable=SC2329  # run by the EXIT trap
cleanup() {
    if [[ "$keep" != true ]]; then
        chmod -R u+w "$sb" 2>/dev/null || true
        rm -rf "$sb"
    fi
}
# Keep the run's exit status; cleanup's own would otherwise replace it.
trap 'rc=$?; cleanup; exit "$rc"' EXIT

section() { printf '\n## %s\n\n' "$1" >>"$report"; }
fence() { { printf '```\n'; cat; printf '```\n'; } >>"$report"; }

# --- shims ---------------------------------------------------------------

# Writes a shim that records every call. Reads given by the case patterns in
# $2 are passed to the real binary; anything else gets exit status $3.
shim() {
    local name="$1" passthrough="$2" status="$3" real
    real="$(PATH="$REAL_PATH" command -v "$name" || true)"
    cat >"$shims/$name" <<SHIM
#!/bin/bash
printf '%s\t%s\t%s\n' "\$(date +%H:%M:%S)" "$name" "\$*" >>"$calls"
case " \$* " in
    $passthrough) [ -n "$real" ] && exec "$real" "\$@" ;;
esac
exit $status
SHIM
    chmod +x "$shims/$name"
}

# Session- and machine-wide commands: recorded, never run. Reads that only
# report state go through, so the bootstrap sees the real machine.
shim defaults '*" read "* | *" read-type "* | *" domains "* | *" export "*' 0
shim launchctl '*" list "* | *" print "*' 0
shim hidutil '*" --get "*' 0
shim chsh '__never__' 0
shim osascript '__never__' 0
shim killall '__never__' 0
shim open '__never__' 0
shim softwareupdate '*" --list "* | *" -l "*' 0
# Applies keyboard settings to the running session; dfinstall prefers one on PATH.
shim activateSettings '__never__' 0
# The bootstrap's Ghostty nightly fallback copies the app into /Applications.
shim ditto '__never__' 0
shim pmset '*" -g "*' 0
shim sudo '__never__' 1
shim doas '__never__' 1
shim xcode-select '*" -p "* | *" --print-path "* | *" -v "* | *" --version "*' 0
shim security '*" find-certificate "* | *" find-identity "* | *" list-keychains "* | *" verify-cert "*' 1
shim dscl '*" -read "* | *" -list "*' 1
shim profiles '*" status "* | *" show "* | *" list "*' 0
shim santactl '*" status "* | *" version "*' 0
ln -s "$REAL_GO" "$shims/go"

# In the fresh sandbox defaults reads find nothing, as on a new account.
cat >"$shims/defaults-fresh" <<SHIM
#!/bin/bash
printf '%s\t%s\t%s\n' "\$(date +%H:%M:%S)" defaults "\$*" >>"$calls"
case " \$* " in
    *" read "* | *" read-type "* | *" export "*) echo "The domain/default pair does not exist" >&2; exit 1 ;;
esac
exit 0
SHIM
chmod +x "$shims/defaults-fresh"

# A Homebrew that has nothing installed and installs nothing. Each formula or
# cask it is asked for is looked up in the real Homebrew (read-only), so a name
# that does not exist fails the same way it would for real.
cat >"$fakebrew/bin/brew" <<FAKE
#!/bin/bash
printf '%s\t%s\t%s\n' "\$(date +%H:%M:%S)" brew "\$*" >>"$calls"
real() { [ -n "$REAL_BREW" ] && HOME="$REAL_HOME" HOMEBREW_CACHE="$sb/brew-cache" HOMEBREW_LOGS="$sb/brew-cache/logs" HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_INSTALL_CLEANUP=1 "$REAL_BREW" "\$@"; }
verify=\$(cat "$out/brew-verify" 2>/dev/null)
cmd="\$1"; shift || true
case "\$cmd" in
    --prefix) if [ \$# -gt 0 ]; then echo "$fakebrew/opt/\$1"; else echo "$fakebrew"; fi ;;
    --repository|--repo) echo "$fakebrew" ;;
    --cellar) echo "$fakebrew/Cellar" ;;
    --version|-v) echo "Homebrew 4.6.0 (dfinstall macos-harness, installs nothing)" ;;
    shellenv)
        echo "export HOMEBREW_PREFIX=\"$fakebrew\";"
        echo "export HOMEBREW_CELLAR=\"$fakebrew/Cellar\";"
        echo "export HOMEBREW_REPOSITORY=\"$fakebrew\";"
        echo "export PATH=\"$fakebrew/bin:$fakebrew/sbin\\\${PATH+:\\\$PATH}\";" ;;
    list|ls) [ \$# -eq 0 ] && exit 0; exit 1 ;;
    info|desc|search) real "\$cmd" "\$@" ;;
    install|reinstall|upgrade)
        cask=""; rc=0
        for a in "\$@"; do
            case "\$a" in
                --cask|--casks) cask="--cask" ;;
                -*) ;;
                *)
                    if [ "\$verify" != yes ]; then
                        printf 'harness: would %s %s %s (UNVERIFIED: the real Homebrew could not be asked)\n' "\$cmd" "\$cask" "\$a" >>"$out/brew-requests.log"
                    elif real info \$cask "\$a" >/dev/null 2>&1; then
                        printf 'harness: would %s %s %s\n' "\$cmd" "\$cask" "\$a" >>"$out/brew-requests.log"
                    else
                        printf 'harness: NO SUCH %s %s\n' "\${cask:-formula}" "\$a" >>"$out/brew-requests.log"
                        echo "Error: No available \${cask:+cask}\${cask:-formula} with the name \"\$a\"." >&2
                        rc=1
                    fi ;;
            esac
        done
        exit \$rc ;;
    outdated|deps|leaves|uses|missing) exit 0 ;;
    *) exit 0 ;;
esac
FAKE
chmod +x "$fakebrew/bin/brew"
mkdir -p "$fakebrew/Cellar" "$fakebrew/sbin" "$fakebrew/opt"

# Formula names are only checked when the real Homebrew can answer. Behind a
# proxy that blocks its API it cannot, and every name would read as missing.
if [[ -n "$REAL_BREW" ]] && HOME="$REAL_HOME" HOMEBREW_CACHE="$sb/brew-cache" HOMEBREW_NO_AUTO_UPDATE=1 \
    "$REAL_BREW" info --json=v2 git >/dev/null 2>&1; then
    echo yes >"$out/brew-verify"
else
    echo no >"$out/brew-verify"
fi

# --- real-state snapshot -------------------------------------------------

# The defaults domains dfinstall writes, read from its own source: those named
# in a write or delete, and the system ones its settings tables list.
defaults_domains() {
    {
        echo NSGlobalDomain
        grep -rhoE '"(write|delete)", "[A-Za-z0-9.-]+"' "$repo/internal" | sed -E 's/.*", "//; s/"$//'
        grep -rhoE '\{"com\.apple\.[A-Za-z0-9.]+", "' "$repo/internal" | sed -E 's/^\{"//; s/", "$//'
    } | sort -u
}

# Every probe here is informational: one that fails (no launch agents of
# your own, say, so grep matches nothing) must not end the run. The report
# lists the sections that came back empty.
snapshot() {
    (
        set +e +o pipefail
        snapshot_state
    ) >"$1"
}

snapshot_state() {
    local canon
    (
        cd "$REAL_HOME"
        echo "== dotfile links"
        find . -maxdepth 1 -name '.*' -type l -exec sh -c 'printf "%s -> %s\n" "$1" "$(readlink "$1")"' _ {} \; | sort
        find .config -maxdepth 2 -type l -exec sh -c 'printf "%s -> %s\n" "$1" "$(readlink "$1")"' _ {} \; 2>/dev/null | sort
        echo "== dfinstall state"
        find .config/dfinstall .local/share/dfinstall -maxdepth 1 -type f -exec shasum {} + 2>/dev/null | sort -k2
        echo "== ~/.local/bin"
        for f in .local/bin/*; do
            if [ -L "$f" ]; then
                printf '%s -> %s\n' "$f" "$(readlink "$f")"
            elif [ -f "$f" ]; then
                printf '%s %s\n' "$f" "$(shasum <"$f" | cut -c1-12)"
            fi
        done
        echo "== launch agents"
        ls Library/LaunchAgents 2>/dev/null
    )
    canon="$(cat "$REAL_HOME/.config/dfinstall/dotfiles-dir" 2>/dev/null || true)"
    if [[ -n "$canon" && -d "$canon/.git" ]]; then
        echo "== clone $canon"
        git -C "$canon" rev-parse HEAD
        git -C "$canon" status --porcelain | shasum
    fi
    echo "== login shell"
    /usr/bin/dscl . -read "/Users/$(id -un)" UserShell 2>/dev/null
    echo "== key remapping"
    /usr/bin/hidutil property --get UserKeyMapping 2>/dev/null
    # macOS starts and stops its own jobs all the time; only the rest is ours to compare.
    echo "== launchd labels"
    /bin/launchctl list 2>/dev/null | awk 'NR>1 {print $3}' | grep -vE '^(com\.apple\.|application\.)' | sort
    echo "== defaults"
    defaults_domains | while read -r d; do
        printf -- '-- %s\n' "$d"
        /usr/bin/defaults export "$d" - 2>/dev/null | /usr/bin/plutil -p - 2>/dev/null
        /usr/bin/defaults -currentHost export "$d" - 2>/dev/null | /usr/bin/plutil -p - 2>/dev/null
    done
    if [[ -n "$REAL_BREW" ]]; then
        echo "== brew"
        HOMEBREW_NO_AUTO_UPDATE=1 "$REAL_BREW" list --versions 2>/dev/null
    fi
}

# --- build ---------------------------------------------------------------

{
    printf '# dfinstall macOS harness\n\n'
    printf -- '- run: %s\n- checkout: %s (%s)\n' "$(date)" "$repo" "$(git -C "$repo" describe --tags --always --dirty 2>/dev/null)"
    printf -- '- macOS %s on %s\n' "$(sw_vers -productVersion)" "$(uname -m)"
    printf -- '- installed dfinstall: %s\n' "$(PATH="$REAL_PATH" dfinstall --version 2>/dev/null || echo none)"
    printf -- '- real Homebrew: %s\n' "${REAL_BREW:-none}"
    printf -- '- Go: %s\n' "$("$REAL_GO" version)"
} >"$report"

echo "macos-harness: building dfinstall from $repo"
(cd "$repo" && GOPATH="$sb/go" GOMODCACHE="$sb/go/pkg/mod" GOCACHE="$sb/go/cache" \
    "$REAL_GO" build -o "$out/bin/dfinstall" ./cmd/dfinstall)

echo "macos-harness: snapshotting your real setup"
snapshot "$out/real-before.txt"

# --- scenarios -----------------------------------------------------------

# Runs a command with no controlling terminal unless --interactive, so a
# prompt reads end-of-input instead of waiting on the terminal.
detached() {
    if [[ "$interactive" == true ]]; then
        "$@"
    else
        perl -e 'use POSIX qw(setsid); defined(setsid()) or die "setsid: $!"; exec @ARGV or die "exec: $!"' "$@" </dev/null
    fi
}

sandbox_env=(
    env -i
    "HOME=$sb/home"
    "USER=$(id -un)" "LOGNAME=$(id -un)" "SHELL=/bin/zsh"
    "TERM=${TERM:-xterm-256color}" "LANG=${LANG:-en_US.UTF-8}"
    "TMPDIR=$sb/tmp"
    "XDG_CONFIG_HOME=$sb/home/.config" "XDG_DATA_HOME=$sb/home/.local/share"
    "XDG_STATE_HOME=$sb/home/.local/state" "XDG_CACHE_HOME=$sb/home/.cache"
    "PATH=$shims:$fakebrew/bin:$sb/home/.local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
    "HOMEBREW_NO_AUTO_UPDATE=1" "HOMEBREW_NO_ANALYTICS=1"
)
# The Go module proxy is the same for every account on a managed Mac, and
# without it the source build fails behind a corporate firewall for reasons
# unrelated to dfinstall.
for v in GOPROXY GOSUMDB GONOSUMDB GOPRIVATE GONOPROXY GOINSECURE; do
    val="$("$REAL_GO" env "$v")"
    [[ -n "$val" ]] && sandbox_env+=("$v=$val")
done
# A proxy set in your shell is part of the network, not of your dotfiles.
for v in http_proxy https_proxy HTTP_PROXY HTTPS_PROXY no_proxy NO_PROXY all_proxy ALL_PROXY; do
    val="$(printenv "$v" || true)"
    [[ -n "$val" ]] && sandbox_env+=("$v=$val")
done

run_fresh() {
    echo "macos-harness: fresh install in $sb/home"
    cp "$shims/defaults-fresh" "$shims/defaults"
    git clone --quiet --no-local "$repo" "$sb/home/dotfiles"
    git -C "$sb/home/dotfiles" checkout --quiet "$(git -C "$repo" rev-parse HEAD)"
    local rc=0
    detached "${sandbox_env[@]}" /bin/bash "$sb/home/dotfiles/bootstrap/macos-setup.sh" \
        --yes --from-source --dir "$sb/home/dotfiles" --brew-prefix "$fakebrew" \
        >"$out/fresh-bootstrap.log" 2>&1 || rc=$?

    local df="$sb/home/.local/bin/dfinstall"
    [[ -x "$df" ]] || df="$sb/home/dotfiles/bin/dfinstall"
    for c in "--version" "status" "doctor" "diff"; do
        echo "\$ dfinstall $c"
        # shellcheck disable=SC2086
        detached "${sandbox_env[@]}" "$df" $c 2>&1 || true
        echo
    done >"$out/fresh-dfinstall.log"

    section "Fresh install (sandbox)"
    printf "Bootstrap exit status: %s. Full log: \`fresh-bootstrap.log\`; dfinstall afterwards: \`fresh-dfinstall.log\`.\n\n" "$rc" >>"$report"
    printf '### Bootstrap, last 60 lines\n\n' >>"$report"
    sed 's/\x1b\[[0-9;]*m//g' "$out/fresh-bootstrap.log" | tail -60 | fence
    printf '\n### What Homebrew was asked for\n\n' >>"$report"
    if [[ "$(cat "$out/brew-verify")" != yes ]]; then
        printf '**The real Homebrew could not be asked, so these names are UNVERIFIED** (offline, or its API is blocked).\n\n' >>"$report"
    fi
    if [[ -s "$out/brew-requests.log" ]]; then
        sort -u "$out/brew-requests.log" | fence
    else
        echo "(nothing)" | fence
    fi
    printf '\n### Files the install created (sandbox HOME, depth 3)\n\n' >>"$report"
    (cd "$sb/home" && find . -maxdepth 3 \( -name .git -o -name dotfiles -o -name .cache -o -name go \) -prune -o -print | sort) | fence
}

run_upgrade() {
    echo "macos-harness: read-only checks against your real setup"
    # Reads pass through, except a sandboxed app's preferences: those live in
    # its container, and reading another app's container needs Full Disk
    # Access and can raise a privacy prompt.
    cat >"$shims/defaults" <<SHIM
#!/bin/bash
printf '%s\t%s\t%s\n' "\$(date +%H:%M:%S)" defaults "\$*" >>"$calls"
case " \$* " in
    *" read "* | *" read-type "* | *" export "* | *" domains "*)
        prev=""; dom=""
        for a in "\$@"; do
            case "\$prev" in read | read-type | export) dom="\$a"; break ;; esac
            prev="\$a"
        done
        if [ -n "\$dom" ] && [ -d "$REAL_HOME/Library/Containers/\$dom" ]; then
            printf '%s\t%s\t%s\n' "\$(date +%H:%M:%S)" harness "did not read \$dom: a sandboxed app's preferences" >>"$calls"
            echo "Domain \$dom does not exist" >&2
            exit 1
        fi
        exec /usr/bin/defaults "\$@" ;;
esac
exit 0
SHIM
    chmod +x "$shims/defaults"
    local real_env=(env "PATH=$shims:$REAL_PATH")
    {
        for c in "--version" "status" "doctor" "diff" "install all --dry-run"; do
            echo "\$ dfinstall $c   (this checkout's build)"
            # shellcheck disable=SC2086
            detached "${real_env[@]}" "$out/bin/dfinstall" $c 2>&1 || true
            echo
        done
    } >"$out/upgrade-dfinstall.log"
    {
        echo "\$ bootstrap preflight and TLS probe (sourced, read-only)"
        # shellcheck disable=SC2016  # expanded by the inner bash
        detached "${real_env[@]}" /bin/bash -c '
            set +e
            source "$1"
            trap - EXIT
            preflight
            echo "brew prefix: ${BREW_PREFIX:-none}, read-only brew: ${READONLY_BREW:-none}"
            echo "TLS-intercepted hosts: $(intercepted_hosts || echo "(could not check: no probe host answered)")"
        ' _ "$repo/bootstrap/macos-setup.sh" 2>&1 || true
    } >"$out/upgrade-preflight.log"

    section "This checkout against your real setup (read-only)"
    printf "Full output: \`upgrade-dfinstall.log\`, \`upgrade-preflight.log\`.\n\n" >>"$report"
    sed 's/\x1b\[[0-9;]*m//g' "$out/upgrade-preflight.log" | fence
    printf '\n### doctor\n\n' >>"$report"
    sed 's/\x1b\[[0-9;]*m//g' "$out/upgrade-dfinstall.log" | awk '/^\$ dfinstall doctor/{p=1} /^\$ dfinstall diff/{p=0} p' | fence
}

case "$scenario" in
    fresh) run_fresh ;;
    upgrade) run_upgrade ;;
    all) run_fresh; run_upgrade ;;
esac

# --- verdict -------------------------------------------------------------

section "What the shims stopped"
printf "Every call to a machine- or session-wide command, most frequent first. Full list: \`shim-calls.log\`.\n\n" >>"$report"
# awk rather than grep and head: under pipefail an empty match, or head closing
# the pipe early, would end the run here.
awk -F'\t' '$2" "$3 !~ /^(xcode-select -p|security find-certificate|profiles status|santactl status)/ {print $2, $3}' "$calls" |
    sort | uniq -c | sort -rn | awk 'NR <= 80' | fence

snapshot "$out/real-after.txt"
section "Your real setup"
# A read that fails before and after looks unchanged; say which came back empty.
empty="$(awk '/^== /{if (h && !n) print h; h=$0; n=0; next} {n++} END{if (h && !n) print h}' "$out/real-before.txt")"
if [[ -n "$empty" ]]; then
    printf 'These read nothing, so they cannot show a change:\n\n' >>"$report"
    printf '%s\n' "$empty" | fence
    printf '\n' >>"$report"
fi
if diff -u "$out/real-before.txt" "$out/real-after.txt" >"$out/real-changes.diff"; then
    printf "Unchanged: every item in \`real-before.txt\` reads the same afterwards.\n" >>"$report"
    verdict=0
else
    printf 'CHANGED during the run (by the harness, or by you or the system meanwhile):\n\n' >>"$report"
    fence <"$out/real-changes.diff"
    verdict=1
fi

if [[ "$strict" == true && "$scenario" != upgrade ]]; then
    if [[ "$(cat "$out/brew-verify")" != yes ]]; then
        echo "macos-harness: --strict: the real Homebrew could not be asked, so no formula name was checked" >&2
        verdict=1
    fi
    if grep -q 'NO SUCH' "$out/brew-requests.log" 2>/dev/null; then
        echo "macos-harness: --strict: dfinstall asked Homebrew for something that does not exist:" >&2
        grep 'NO SUCH' "$out/brew-requests.log" | sort -u >&2
        verdict=1
    fi
fi

echo
echo "macos-harness: report in $report"
[[ "$verdict" -eq 0 ]] || echo "macos-harness: your real setup CHANGED during the run; see the end of the report" >&2
echo "macos-harness: review it before sharing (it holds paths, hostnames and package lists)"
exit "$verdict"
