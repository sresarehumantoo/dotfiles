#!/usr/bin/env bash
# A deb install with no clone: init --packaged, install all, then a package
# upgrade that brings new content. Runs as root; the user steps go through one
# pty so sudo prompts are counted.
# e2e: packages
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

H=/home/tester
store=$H/.local/share/dfinstall/content
build_deb 1 /e2e/pkg/1/content /tmp/dfinstall-1.deb
content_v2 /tmp/content-2
build_deb 2 /tmp/content-2 /tmp/dfinstall-2.deb

# Modules that apt-install or chsh need root; the sudo scenario covers those.
mkdir -p $H/.config/dfinstall
printf 'skip_backup: true\nskip_modules: [packages, extras, newsboat, docs, defaultshell, locale]\n' > $H/.config/dfinstall/config.yaml
chown -R tester: $H/.config

echo "== v1"
check "v1 package installs" dpkg -i /tmp/dfinstall-1.deb
shared1="$(tree_sum /usr/share/dfinstall)"

steps="$(mktemp)"
cat > "$steps" <<'EOF'
init|cd /tmp && dfinstall init --packaged
install all|cd /tmp && dfinstall install all
EOF
chmod 644 "$steps"
results="$(mktemp)"
su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
expect_step "$results" "init" 0 0
expect_step "$results" "install all" 0 0

check "canonical pointer names the store" test "$(cat $H/.config/dfinstall/dotfiles-dir)" = "$store/current"
check "store current is v1" test "$(readlink "$store/current")" = v3.0.0-e2e.1
check ".zshrc links into the store" test "$(readlink $H/.zshrc)" = "$store/current/config/shell/zshrc"
check "htoprc is a copy" test -f $H/.config/htop/htoprc -a ! -L $H/.config/htop/htoprc
check "p10k.zsh is a copy" test -f $H/.p10k.zsh -a ! -L $H/.p10k.zsh
check "no link points at /usr/share/dfinstall" test -z "$(links_under $H | grep /usr/share/dfinstall)"
check "no dangling links" test -z "$(find $H -xtype l)"
check "/usr/share/dfinstall unchanged" test "$(tree_sum /usr/share/dfinstall)" = "$shared1"
check "the store is read-only" test -z "$(find "$store/v3.0.0-e2e.1" -perm /222)"
check "doctor passes the content version" bash -c "su - tester -c 'dfinstall doctor' | grep -q 'content version.*matches dfinstall'"

echo "== v2"
su - tester -c 'echo "# mine" >> ~/.p10k.zsh'
check "v2 package installs" dpkg -i /tmp/dfinstall-2.deb
shared2="$(tree_sum /usr/share/dfinstall)"
doctor="$(su - tester -c 'dfinstall doctor' 2>&1 || true)"
check "doctor flags the content version" grep -q 'packaged content is v3.0.0-e2e.1 but dfinstall is v3.0.0-e2e.2' <<<"$doctor"

cat > "$steps" <<'EOF'
update|cd /tmp && dfinstall update all
EOF
su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
expect_step "$results" "update" 0 0

check "store current is v2" test "$(readlink "$store/current")" = v3.0.0-e2e.2
check "v1 is kept for rollback" test -d "$store/v3.0.0-e2e.1"
check "a linked file shows v2" bash -c "tail -1 $H/.config/tmux/tmux.conf | grep -qx '# v2'"
check "an untouched seeded file took v2" bash -c "tail -1 $H/.config/htop/htoprc | grep -qx '# v2'"
check "an edited seeded file is kept" bash -c "tail -1 $H/.p10k.zsh | grep -qx '# mine'"
check "its v2 waits under pending" bash -c "tail -1 $H/.local/share/dfinstall/pending/.p10k.zsh | grep -qx '# v2'"
check "no dangling links" test -z "$(find $H -xtype l)"
check "/usr/share/dfinstall unchanged" test "$(tree_sum /usr/share/dfinstall)" = "$shared2"
doctor="$(su - tester -c 'dfinstall doctor' 2>&1 || true)"
check "doctor passes the content version" grep -q 'content version.*matches dfinstall' <<<"$doctor"
check "doctor lists the pending update" grep -q 'p10k.zsh' <<<"$doctor"

finish
