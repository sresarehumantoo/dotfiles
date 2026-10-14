# A plain trixie box for scripts/e2e. "tester" can sudo but must type a
# password, so a scenario can count every prompt dfinstall causes.
FROM debian:trixie

RUN apt-get update \
 && apt-get install -y --no-install-recommends sudo expect git curl ca-certificates gnupg zsh \
 && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash -G sudo tester \
 && echo 'tester:tester' | chpasswd
