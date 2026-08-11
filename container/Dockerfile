# syntax=docker/dockerfile:1
#
# dotfiles-box — a persistent Linux "OS" container used as a WSL replacement on
# Windows hosts that have Docker Desktop but no WSL2.
#
# Deliberately NOT systemd-based. See container/README.md ("Why no systemd").
# The image is a *seed*: /home/<user> is built here, then moved to /opt/skel and
# copied into a named volume on first boot, so the running container is a pet
# whose home survives image rebuilds.

FROM debian:trixie

ARG USERNAME=owen
ARG BRANCH=develop
# Building Neovim from source takes several minutes. Set to 0 for a fast image
# that uses Debian's packaged nvim instead (kickstart may warn on older versions).
ARG BUILD_NEOVIM=1

ENV DEBIAN_FRONTEND=noninteractive \
    PERL_BADLANG=0 \
    LC_ALL=C

# Prerequisites wsl-setup.sh itself needs, plus the container-specific extras
# (sshd for entry + SSHFS-Win, tini for reaping, rsync for home seeding).
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl git sudo procps \
      openssh-server tini rsync xz-utils \
 && rm -rf /var/lib/apt/lists/*

# The whole repo is the build context so bootstrap/ and config/ come along.
# .dockerignore keeps bin/, .git/ and the host's own .config.yaml out.
COPY . /src

# Container-specific dfinstall config. install_dotfiles copies $source_dir to
# ~/dotfiles wholesale, so placing it here is what gets it to <dotfiles>/.config.yaml
# BEFORE `dfinstall install all` runs and reads skip_modules.
COPY container/config.container.yaml /src/.config.yaml

# System packages, user creation, sudo group, locale. wsl.conf is skipped
# automatically — wsl-setup.sh's is_wsl() is false in a container.
RUN /src/bootstrap/wsl-setup.sh setup-root "${USERNAME}"

# Passwordless sudo: there is no TTY during `docker build`, and a pet container
# with a single interactive user gains nothing from the password prompt.
# wsl-setup.sh sets the password to 'root'; this makes that irrelevant.
RUN printf '%s ALL=(ALL) NOPASSWD:ALL\n' "${USERNAME}" > /etc/sudoers.d/90-${USERNAME} \
 && chmod 0440 /etc/sudoers.d/90-${USERNAME}

RUN if [ "${BUILD_NEOVIM}" = "1" ]; then /src/bootstrap/wsl-setup.sh build-neovim; fi

# cp -a inside install_dotfiles would otherwise try to preserve root ownership
# while running as an unprivileged user.
RUN chown -R "${USERNAME}:${USERNAME}" /src

USER ${USERNAME}
# Copies /src -> ~/dotfiles, runs `make build`, then `dfinstall install all`.
RUN /src/bootstrap/wsl-setup.sh install-dotfiles "${BRANCH}" /src

# Put dfinstall + dfinstall-mcp on PATH. Without this the binaries exist only at
# ~/dotfiles/bin and `dfinstall status` is a "command not found" in a fresh shell,
# which is not what a WSL-equivalent should feel like.
RUN cd "/home/${USERNAME}/dotfiles" && make install-bin

# Sync Neovim plugins at build time so first launch in a fresh container is not a
# multi-minute download. dfinstall's nvim module attempts this already but reports
# failure under a TTY-less build; run it explicitly and tolerate a partial sync
# rather than failing the image.
RUN timeout 600 nvim --headless "+Lazy! sync" +qa 2>&1 | tail -5 || true

USER root

# The defaultshell module's chsh cannot authenticate during `docker build` (no
# TTY, and PAM refuses). Set the login shell directly — otherwise sshd and tmux
# both spawn bash, and the whole zsh/p10k/omz stack silently never loads.
RUN chsh -s /usr/bin/zsh "${USERNAME}"
# Move the fully-built home aside. The entrypoint seeds it into the (initially
# empty) named volume on first boot; thereafter the volume wins and image
# rebuilds no longer clobber the user's state.
RUN mv "/home/${USERNAME}" /opt/skel \
 && mkdir -p "/home/${USERNAME}" \
 && chown "${USERNAME}:${USERNAME}" "/home/${USERNAME}"

# Host keys live on a volume so recreating the container doesn't trip
# StrictHostKeyChecking on the Windows side. Generated at runtime, never baked.
RUN mkdir -p /etc/ssh/keys /run/sshd \
 && sed -i \
      -e 's@^#\?HostKey /etc/ssh/ssh_host@HostKey /etc/ssh/keys/ssh_host@' \
      -e 's@^#\?PasswordAuthentication.*@PasswordAuthentication no@' \
      -e 's@^#\?PermitRootLogin.*@PermitRootLogin no@' \
      -e 's@^#\?X11Forwarding.*@X11Forwarding no@' \
      /etc/ssh/sshd_config \
 && printf 'AcceptEnv COLORTERM TERM_PROGRAM WINDOWS_HOST\n' >> /etc/ssh/sshd_config

COPY container/entrypoint.sh /usr/local/sbin/entrypoint.sh
RUN chmod 0755 /usr/local/sbin/entrypoint.sh

ENV BOX_USER=${USERNAME}
EXPOSE 22
ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/sbin/entrypoint.sh"]
CMD ["/usr/sbin/sshd", "-D", "-e"]
