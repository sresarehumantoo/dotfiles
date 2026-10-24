#!/usr/bin/env bash
# The macOS bootstrap's Ghostty step when the firewall blocks Ghostty's own
# download site. The bootstrap code runs as is; only macOS commands are stand
# ins: brew succeeds exactly when the cask's download host answers, and ditto
# and shasum map onto unzip, cp and sha256sum.
set -uo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

trust_ca
stage
stubs="$(mktemp -d)"
cat > "$stubs/brew" <<'STUB'
#!/bin/sh
for a; do case "$a" in --appdir=*) appdir="${a#--appdir=}" ;; esac; done
curl -fsSI -o /dev/null --max-time 30 https://release.files.ghostty.org/1.3.1/Ghostty.dmg || {
    echo "Error: Download failed on Cask 'ghostty'" >&2
    exit 1
}
mkdir -p "$appdir/Ghostty.app" && touch "$appdir/Ghostty.app/from-brew"
STUB
cat > "$stubs/ditto" <<'STUB'
#!/bin/sh
if [ "$1" = -x ]; then mkdir -p "$4" && exec unzip -q "$3" -d "$4"; fi
exec cp -R "$1" "$2"
STUB
printf '#!/bin/sh\nexec sha256sum -c --quiet -\n' > "$stubs/shasum"
chmod +x "$stubs"/*

app="$HOME/Applications/Ghostty.app"
# bootstrap_ghostty: the bootstrap's own install_ghostty, answered yes.
bootstrap_ghostty() {
    rm -rf "$app"
    (PATH="$stubs:$PATH"
        # shellcheck source=../../../bootstrap/macos-setup.sh
        source /src/bootstrap/macos-setup.sh
        ASSUME_YES=true
        install_ghostty) >"$HOME/ghostty.log" 2>&1
}

bootstrap_ghostty
check "site reachable: Homebrew installs it" test -f "$app/from-brew"

policy release.files.ghostty.org
bootstrap_ghostty
check "site blocked: the nightly from GitHub is installed" test -x "$app/Contents/MacOS/ghostty"
check "and the log says it is the nightly" grep -q "nightly build" "$HOME/ghostty.log"

policy release.files.ghostty.org github.com
if bootstrap_ghostty; then
    fail "site and GitHub blocked: the step fails"
else
    pass "site and GitHub blocked: the step fails"
fi
check "and says GitHub was unreachable too" grep -q "Could not reach GitHub" "$HOME/ghostty.log"
check "and installs nothing" test ! -e "$app"

finish
