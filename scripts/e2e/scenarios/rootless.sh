#!/usr/bin/env bash
# e2e: systemd
# install extras on a box with a systemd user session: Docker runs rootless for
# the user, the system daemon stays behind sudo, and nobody joins the docker
# group. Runs as root in the e2e container, with systemd as PID 1.
set -euo pipefail
# shellcheck source=../lib.sh
source "$(dirname "$0")/../lib.sh"

# Someone already in the docker group keeps the daemon they use, and gets
# nothing rootless installed. Runs first, while uidmap is still absent.
groupadd docker
useradd -m -s /bin/bash -G sudo,docker member
echo 'member:member' | chpasswd
stage_repo member
steps="$(mktemp)"
cat > "$steps" <<EOF
member extras|DOTFILES=/home/member/dotfiles /e2e/dfinstall install extras
EOF
chmod 644 "$steps"
results="$(mktemp)"
su - member -c "expect /src/scripts/e2e/session.exp $steps member" | tee "$results"
expect_step "$results" "member extras" 1 0
check "member: no rootless unit" test ! -e /home/member/.config/systemd/user/docker.service
check "member: uidmap not installed" bash -c '! dpkg -s uidmap >/dev/null 2>&1'
echo

stage_repo tester
H=/home/tester
DF="DOTFILES=$H/dotfiles /e2e/dfinstall"

steps="$(mktemp)"
cat > "$steps" <<EOF
extras|$DF install extras
context is rootless|docker context show | grep -qx rootless
daemon is rootless|docker info --format '{{.SecurityOptions}}' | grep -q rootless
container runs|docker run --rm alpine true
extras again|$DF install extras
system daemon via sudo|sudo docker info --format '{{.SecurityOptions}}' | grep -v rootless
EOF
chmod 644 "$steps"

results="$(mktemp)"
su - tester -c "expect /src/scripts/e2e/session.exp $steps tester" | tee "$results"
echo

expect_step "$results" "extras" 1 0
expect_step "$results" "context is rootless" 0 0
expect_step "$results" "daemon is rootless" 0 0
expect_step "$results" "container runs" 0 0
expect_step "$results" "extras again" 0 0
expect_step "$results" "system daemon via sudo" 1 0

check "tester is not in the docker group" bash -c '! id -nG tester | grep -qw docker'
check "rootless unit installed" test -f "$H/.config/systemd/user/docker.service"
check "Docker repo pinned to 100" bash -c "apt-cache policy | grep -q '^ *100 https://download.docker.com'"

finish
