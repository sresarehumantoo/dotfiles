#!/usr/bin/env bash
# Fail if site-coupled content appears in tracked files.
#
# WHY THIS EXISTS
# This repo is public and configures workstations generally, including machines
# out-of-band of any particular network. Site-specific content (internal hosts,
# RFC1918 addressing, private CAs, Vault paths, forge credentials) belongs in
# the local-override files the tracked configs already opt into:
#
#     ~/.gitconfig.local     <- [include] in config/git/gitconfig
#     ~/.profile.local       <- sourced from config/shell/profile
#     ~/.config/sway/outputs.conf  <- include in config/sway/config
#
# Those are versioned in a separate internal repo, not here.
#
# This check exists because the coupling it looks for was previously committed
# and published, and stayed there for months without anyone noticing. Removing
# it from HEAD does not unpublish it -- so the value now is preventing the next
# one, which is exactly what a CI gate does and what a history rewrite does not.
#
# Escape hatch: a line carrying the marker  decoupling-ok:<reason>  is exempt.
# Use it sparingly and say why.
set -euo pipefail

cd -- "$(git rev-parse --show-toplevel)"

fail=0
report() {
    printf '\n  %s\n' "$1"
    shift
    printf '    %s\n' "$@"
    fail=1
}

# Tracked files PLUS untracked-but-not-ignored ones, so this catches a leak
# before it is committed rather than after. Excludes this script, which
# necessarily names every pattern it looks for.
mapfile -t files < <(
    git ls-files -co --exclude-standard |
        grep -vE '^scripts/check-decoupling\.sh$' || true
)
[ "${#files[@]}" -gt 0 ] || {
    echo "no tracked files found -- refusing to pass vacuously" >&2
    exit 2
}

scan() {
    local label="$1" pattern="$2"
    local hits
    hits=$(grep -nIE --with-filename "$pattern" -- "${files[@]}" 2>/dev/null |
        grep -v 'decoupling-ok:' || true)
    if [ -n "$hits" ]; then
        report "$label" "$hits"
    fi
}

echo "Checking ${#files[@]} tracked files for site coupling..."

# A full dotted quad, so version strings like "sway 1.10.1" do not match.
scan "RFC1918 address" \
    '\b(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[01])\.[0-9]{1,3}\.[0-9]{1,3})\b'

# Deliberately NOT a generic ".local/.lan" sweep: those collide with the
# established dotfile convention (.gitconfig.local, .tmux.conf.local, .env.local)
# and produced nothing but false positives. This targets the identifiers that
# actually couple this repo to one site.
scan "internal hostname / private domain" \
    '\b[A-Za-z0-9-]+\.(internal|homelab|intranet)\b|\baboutowenpierce\b|\blabcore\b'

scan "Vault path or secret reference" \
    '(vault (kv|read|write) |VAULT_ADDR|\bhomelab/[a-z-]+|hvs\.[A-Za-z0-9])'

scan "credential material" \
    '(BEGIN (OPENSSH|RSA|EC|DSA|PGP) PRIVATE KEY|glpat-[A-Za-z0-9_-]{20}|ghp_[A-Za-z0-9]{36}|AKIA[0-9A-Z]{16})'

if [ "$fail" -ne 0 ]; then
    cat <<'EOF'

FAIL: site-coupled content found in tracked files.

Move it to the matching local-override file (see the header of this script),
and version it in the internal companion repo instead. If a hit is a false
positive -- documentation of the convention, an example, an upstream URL --
append a marker to that line:

    # decoupling-ok: <why>

EOF
    exit 1
fi

echo "OK: no site coupling in tracked files."
