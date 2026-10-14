#!/usr/bin/env bash
# When does dfinstall ask for sudo, and what does it leave cached afterwards?
# Runs as root in the e2e container; drives the tester account through one pty.
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

stage_repo tester
H=/home/tester
DF="DOTFILES=$H/dotfiles /e2e/dfinstall"

steps="$(mktemp)"
cat > "$steps" <<EOF
links only|$DF install git
locale|$DF install locale
nvim|$DF install nvim
delta|$DF install delta
extras|$DF install extras
cached after extras|sudo -n true
extras again|$DF install extras
user sudo -v|sudo -v
shell with sudo cached|$DF install defaultshell
shell again|$DF install defaultshell
root with user's credentials|$DF root
cached after root|sudo -n true
user sudo -k|sudo -k
root cold|$DF root
cached after cold root|sudo -n true
EOF
chmod 644 "$steps"

results="$(mktemp)"
su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
echo

# Nothing that installs into the user's home may escalate.
expect_step "$results" "links only" 0 0
# C.UTF-8 is built in; nothing is generated as root.
expect_step "$results" "locale" 0 0
expect_step "$results" "nvim" 0 0
expect_step "$results" "delta" 0 0
# apt needs root: one prompt for the whole run, and dfinstall drops the
# credentials it obtained.
expect_step "$results" "extras" 1 0
expect_step "$results" "cached after extras" 0 1
# Everything is installed now, so a re-run has no reason to ask.
expect_step "$results" "extras again" 0 0
# Credentials the user already had are used, and left alone.
expect_step "$results" "user sudo -v" 1 0
# chsh asks for the user's own password rather than borrowing sudo, even with
# sudo cached; once the account's shell is zsh, a re-run asks nothing.
expect_step "$results" "shell with sudo cached" 0 0 1
expect_step "$results" "shell again" 0 0
expect_step "$results" "root with user's credentials" 0 0
expect_step "$results" "cached after root" 0 0
expect_step "$results" "user sudo -k" 0 0
expect_step "$results" "root cold" 1 0
expect_step "$results" "cached after cold root" 0 1

for bin in nvim tree-sitter delta terraform; do
    check "$bin in ~/.local/bin, owned by tester" owned_by tester "$H/.local/bin/$bin"
    check "$bin runs" su - tester -c "$H/.local/bin/$bin --version >/dev/null"
done
check "nvim links into ~/.local/opt" test "$(readlink "$H/.local/bin/nvim")" = "$H/.local/opt/nvim-linux-$(uname -m | sed s/aarch64/arm64/)/bin/nvim"
check "no file under ~/.local/opt is owned by anyone but tester" test -z "$(find "$H/.local/opt" ! -user tester)"
check "nothing installed into /usr/local/bin" test -z "$(ls -A /usr/local/bin)"
check "no git-delta package" bash -c '! dpkg -s git-delta >/dev/null 2>&1'
check "no HashiCorp apt repo" test ! -e /etc/apt/sources.list.d/hashicorp.list
check "login shell changed to zsh" test "$(getent passwd tester | cut -d: -f7)" = /usr/bin/zsh
check "en_US.UTF-8 was not generated" bash -c '! locale -a | grep -qi "^en_us"'
check "shells fall back to C.UTF-8" test "$(env -i PATH=/usr/bin:/bin zsh -fc "source /src/config/shell/zsh/locale.zsh; print -r -- \$LANG")" = C.UTF-8
check "/root/.zshrc is a root-owned copy" test "$(stat -c '%F %U:%G %a' /root/.zshrc)" = "regular file root:root 644"

finish
