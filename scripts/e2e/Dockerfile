# A plain trixie box for scripts/e2e. "tester" can sudo but must type a
# password, so a scenario can count every prompt dfinstall causes.
# systemd is only PID 1 for scenarios marked "# e2e: systemd"; the others run
# bash directly, so it is inert there.
FROM debian:trixie

RUN apt-get update \
 && apt-get install -y --no-install-recommends sudo expect git curl ca-certificates gnupg zsh \
      systemd systemd-sysv dbus dbus-user-session libpam-systemd iproute2 procps \
 && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash -G sudo tester \
 && echo 'tester:tester' | chpasswd
