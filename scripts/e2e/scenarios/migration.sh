#!/usr/bin/env bash
# A clone install moved to packaged content and back: a clone edit stops the
# move until update carries it to an override file, an edit to a seeded file
# goes along as the copy, and init --dir puts every link back.
# e2e: packages
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

H=/home/tester
store=$H/.local/share/dfinstall/content
build_deb 1 /e2e/pkg/1/content /tmp/dfinstall-1.deb
check "package installs" dpkg -i /tmp/dfinstall-1.deb

# A clone with the packaged version's config/ and a remote that has it all.
mkdir -p $H/.config/dfinstall $H/dotfiles
printf 'skip_backup: true\n' > $H/.config/dfinstall/config.yaml
cp -a /e2e/pkg/1/content/config $H/dotfiles/
chown -R tester: $H/.config $H/dotfiles
su - tester -c 'set -e
    git init -q --bare ~/remote.git
    cd ~/dotfiles
    git init -q -b main
    git add config
    git -c user.name=e2e -c user.email=e2e@example.invalid commit -qm v1
    git remote add origin ~/remote.git
    git push -q -u origin main'

steps="$(mktemp)"
results="$(mktemp)"
chmod 644 "$steps"
run_steps() {
    cat > "$steps"
    su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
}

echo "== clone install"
run_steps <<'EOF'
init clone|dfinstall init --dir ~/dotfiles
install|dfinstall install shell git tmux htop
EOF
expect_step "$results" "init clone" 0 0
expect_step "$results" "install" 0 0
start="$(links_under $H)"
check "the clone install made links" test "$(grep -c "$H/dotfiles/config" <<<"$start")" -gt 10
check "htoprc links into the clone" test "$(readlink $H/.config/htop/htoprc)" = $H/dotfiles/config/htop/htoprc

su - tester -c 'echo "set -g history-limit 50000" >> ~/dotfiles/config/tmux/tmux.conf
    echo "# mine" >> ~/.config/htop/htoprc'

echo "== to packaged"
run_steps <<'EOF'
refused|dfinstall init --packaged
update|dfinstall update htop
init packaged|dfinstall init --packaged
EOF
expect_step "$results" "refused" 0 1
expect_step "$results" "update" 0 0
expect_step "$results" "init packaged" 0 0

check "the edit moved to tmux.user.conf" grep -qx 'set -g history-limit 50000' $H/.config/tmux/tmux.user.conf
check "the clone's tmux.conf is clean" su - tester -c 'git -C ~/dotfiles diff --quiet -- config/tmux'
check "tmux.conf links into the store" test "$(readlink $H/.config/tmux/tmux.conf)" = "$store/current/config/tmux/tmux.conf"
check "htoprc is a copy with the edit" bash -c "test ! -L $H/.config/htop/htoprc && tail -1 $H/.config/htop/htoprc | grep -qx '# mine'"
check "the previous clone is recorded" grep -qx $H/dotfiles $H/.local/share/dfinstall/previous-clone
check "no dangling links" test -z "$(find $H -xtype l)"

echo "== back to the clone"
run_steps <<'EOF'
rollback|dfinstall init --dir ~/dotfiles
EOF
expect_step "$results" "rollback" 0 0
end="$(links_under $H)"
check "every link is as it started" test "$start" = "$end"
if [[ "$start" != "$end" ]]; then diff <(echo "$start") <(echo "$end") || true; fi
check "the clone still has the htoprc edit" bash -c "tail -1 $H/dotfiles/config/htop/htoprc | grep -qx '# mine'"

finish
