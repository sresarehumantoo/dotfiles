#!/usr/bin/env bash
# bootstrap/wsl-setup.sh in the order wsl-bootstrap.ps1 runs it: root setup,
# dotfiles as the new user, then finish-bootstrap. Runs as root, non-WSL path.
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

U=boot
setup=/tmp/wsl-setup.sh
cp /src/bootstrap/wsl-setup.sh "$setup"
chmod 755 "$setup"

echo "== setup-root"
check "setup-root succeeds" "$setup" setup-root "$U" </dev/null
check "temporary NOPASSWD drop-in present" grep -qx "$U ALL=(ALL:ALL) NOPASSWD: ALL" /etc/sudoers.d/90-dfinstall-bootstrap
check "no password set during setup" test "$(passwd -S "$U" | awk '{print $2}')" != P

echo "== install-dotfiles as $U"
stage_repo "$U"
check "install-dotfiles succeeds without a password" \
    su - "$U" -c "$setup install-dotfiles \"\$(git -C ~/dotfiles branch --show-current)\" ~/dotfiles" </dev/null
# install all turns a failed sudo step into a warning, so the exit code alone
# would not notice a drop-in that did not work. These needed root.
check "extras' apt packages were installed via sudo" dpkg -s ripgrep

echo "== finish-bootstrap"
check "finish-bootstrap succeeds" "$setup" finish-bootstrap "$U" </dev/null
check "drop-in removed" test ! -e /etc/sudoers.d/90-dfinstall-bootstrap
# No terminal here, so the fallback applies: a known password, already expired.
check "fallback password is expired" test "$(passwd -S "$U" | awk '{print $3}')" = 1970-01-01
check "sudo now demands a password change" bash -c "su - $U -c 'sudo -n true' 2>&1 | grep -q 'change your password'"
check "login shell is zsh" test "$(getent passwd "$U" | cut -d: -f7)" = /usr/bin/zsh

finish
