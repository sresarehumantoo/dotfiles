#!/usr/bin/env bash
# A real fresh install as a new, standard (non-admin) macOS account: the
# bootstrap from nothing, then dfinstall, for real, on a throwaway runner.
#
# Usage: sudo-capable runner: scripts/macos-ci/fresh-user.sh [log-dir]
#
# The account shape of a managed work Mac: not an admin, and the admin-owned
# /opt/homebrew is not writable by it. It gets a Homebrew prefix of its own,
# /opt/hb (no longer than /opt/homebrew, so bottles still pour), the way IT
# might hand one out. Checks the install, that a second run changes nothing,
# doctor, and that uninstall removes every link.
set -euo pipefail
# shellcheck source=lib.sh
source "$(dirname "$0")/lib.sh"

case "${1:-}" in -h | --help) sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;; esac
logs="$(mkdir -p "${1:-/tmp/dfinstall-fresh-user}" && cd "${1:-/tmp/dfinstall-fresh-user}" && pwd -P)"

user=dftest
home="/Users/$user"
prefix=/opt/hb

echo "== account"
sudo sysadminctl -addUser "$user" -fullName "dfinstall CI" -password "$(openssl rand -hex 16)" -home "$home" >/dev/null 2>&1
sudo createhomedir -c -u "$user" >/dev/null
check "the account exists" id "$user"
check "the account is not an admin" sh -c "! id -Gn $user | tr ' ' '\n' | grep -qx admin"
check "/opt/homebrew is not writable by it" sudo -u "$user" test ! -w /opt/homebrew
sudo mkdir -p "$prefix"
sudo chown "$user:staff" "$prefix"
sudo cp -R "$repo" "$home/dotfiles"
sudo chown -R "$user:staff" "$home/dotfiles"

# A login-free environment with the account's own HOME and nothing on PATH but
# the system: no runner tools, no admin Homebrew.
as_user() {
    sudo -u "$user" -H env -i HOME="$home" USER="$user" LOGNAME="$user" SHELL=/bin/zsh \
        TERM=dumb LANG=en_US.UTF-8 PATH=/usr/bin:/bin:/usr/sbin:/sbin "$@"
}

echo "== bootstrap"
rc=0
as_user /bin/bash "$home/dotfiles/bootstrap/macos-setup.sh" --yes --from-source \
    --dir "$home/dotfiles" --brew-prefix "$prefix" </dev/null >"$logs/bootstrap.log" 2>&1 || rc=$?
check "the bootstrap succeeds (exit $rc)" test "$rc" -eq 0
tail -40 "$logs/bootstrap.log" | sed 's/\x1b\[[0-9;]*m//g' | sed 's/^/      /'

df="$home/dotfiles/bin/dfinstall"
check "dfinstall was built" test -x "$df"
check "Homebrew is at $prefix" test -x "$prefix/bin/brew"
check_output "the recorded Homebrew prefix is $prefix" "^$prefix\$" sudo cat "$home/.config/dfinstall/brew-prefix"
check "config lives in ~/.config/dfinstall/config.yaml" sudo test -f "$home/.config/dfinstall/config.yaml"
check ".zshrc links into the checkout" sh -c "sudo readlink '$home/.zshrc' | grep -q '^$home/dotfiles/config/'"
check "Ghostty went to ~/Applications (no admin)" sudo test -d "$home/Applications/Ghostty.app"
check "nothing new in /Applications" test ! -d /Applications/Ghostty.app

echo "== dfinstall"
check_output "--version reports a source build" '\(source\)$' as_user "$df" --version
as_user "$df" doctor >"$logs/doctor.log" 2>&1 || true
check_output "doctor names $prefix as the Homebrew" "homebrew.*$prefix" cat "$logs/doctor.log"
check_output "doctor sees a standard account" 'macOS admin: no \(standard account' cat "$logs/doctor.log"
sed 's/\x1b\[[0-9;]*m//g' "$logs/doctor.log" | sed 's/^/      /'

as_user "$df" install all </dev/null >"$logs/second-install.log" 2>&1 || true
check_output "a second install all changes nothing" 'Unchanged' cat "$logs/second-install.log"
check "a second install all reports no changes" sh -c "! grep -q 'Changed:' '$logs/second-install.log'"

echo "== uninstall"
as_user "$df" uninstall all </dev/null >"$logs/uninstall.log" 2>&1 || true
left="$(sudo find "$home" -maxdepth 4 -type l -lname "$home/dotfiles/*" 2>/dev/null | wc -l | tr -d ' ')"
check "uninstall all leaves no link into the checkout ($left left)" test "$left" -eq 0

finish
