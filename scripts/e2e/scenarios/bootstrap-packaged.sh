#!/usr/bin/env bash
# wsl-setup.sh install-dotfiles on main with dfinstall already installed from
# its package: no clone, the packaged configs, and no sudo prompt.
# e2e: packages
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

H=/home/tester
store=$H/.local/share/dfinstall/content
build_deb 1 /e2e/pkg/1/content /tmp/dfinstall-1.deb
check "package installs" dpkg -i /tmp/dfinstall-1.deb

# Modules that apt-install or chsh need root; the sudo scenario covers those.
mkdir -p $H/.config/dfinstall
printf 'skip_backup: true\nskip_modules: [packages, extras, newsboat, docs, defaultshell, locale]\n' > $H/.config/dfinstall/config.yaml
chown -R tester: $H/.config

steps="$(mktemp)"
cat > "$steps" <<'EOF'
install-dotfiles|bash /src/bootstrap/wsl-setup.sh install-dotfiles main
EOF
chmod 644 "$steps"
results="$(mktemp)"
su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
expect_step "$results" "install-dotfiles" 0 0

check "no clone was made" test ! -e $H/dotfiles -a ! -e $H/.local/share/dfinstall/dotfiles
check "canonical pointer names the store" test "$(cat $H/.config/dfinstall/dotfiles-dir)" = "$store/current"
check ".zshrc links into the store" test "$(readlink $H/.zshrc)" = "$store/current/config/shell/zshrc"
check "no dangling links" test -z "$(find $H -xtype l)"

finish
