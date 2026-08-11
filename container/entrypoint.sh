#!/usr/bin/env bash
# Entrypoint for dotfiles-box. Seeds the home volume on first boot, ensures
# sshd host keys and authorized_keys exist, then execs the CMD.
#
# Everything here must be idempotent: it runs on every container start, not
# just the first.
set -euo pipefail

USER_NAME="${BOX_USER:-owen}"
HOME_DIR="/home/${USER_NAME}"
SEED_STAMP="${HOME_DIR}/.box-seeded"
SKEL_DIR="/opt/skel"

log() { printf '[box] %s\n' "$*"; }

# ── Seed the home volume ─────────────────────────────────────────
# A fresh named volume mounts empty over /home/<user>, masking the home the
# image built. Copy it in once and stamp it. Docker's own "copy image content
# into an empty volume" behaviour would half-do this, but it is silent, does
# not apply to bind mounts, and gives no way to re-seed deliberately.
if [[ ! -e "$SEED_STAMP" ]]; then
    if [[ -d "$SKEL_DIR" ]]; then
        log "first boot — seeding ${HOME_DIR} from ${SKEL_DIR}"
        rsync -a "${SKEL_DIR}/" "${HOME_DIR}/"
        chown -R "${USER_NAME}:${USER_NAME}" "$HOME_DIR"
        date -Iseconds > "$SEED_STAMP"
        chown "${USER_NAME}:${USER_NAME}" "$SEED_STAMP"
        log "home seeded"
    else
        log "WARNING: ${SKEL_DIR} missing, cannot seed home"
    fi
fi

# ── sshd host keys (on the volume, so they survive recreate) ─────
mkdir -p /etc/ssh/keys /run/sshd
if ! compgen -G '/etc/ssh/keys/ssh_host_*_key' > /dev/null; then
    log "generating sshd host keys"
    ssh-keygen -q -t ed25519 -N '' -f /etc/ssh/keys/ssh_host_ed25519_key
    ssh-keygen -q -t rsa -b 4096 -N '' -f /etc/ssh/keys/ssh_host_rsa_key
    # sshd_config's stock HostKey lines (which the image rewrote to point here)
    # include ecdsa; without it sshd logs "Unable to load host key" on every boot.
    ssh-keygen -q -t ecdsa -b 521 -N '' -f /etc/ssh/keys/ssh_host_ecdsa_key
fi
chmod 600 /etc/ssh/keys/ssh_host_*_key
chmod 644 /etc/ssh/keys/ssh_host_*_key.pub

# ── authorized_keys ──────────────────────────────────────────────
# BOX_AUTHORIZED_KEYS is the public key(s) to admit, passed via compose. This is
# how the Windows side (Windows Terminal ssh profile, SSHFS-Win) gets in.
if [[ -n "${BOX_AUTHORIZED_KEYS:-}" ]]; then
    install -d -m 0700 -o "$USER_NAME" -g "$USER_NAME" "${HOME_DIR}/.ssh"
    printf '%s\n' "$BOX_AUTHORIZED_KEYS" > "${HOME_DIR}/.ssh/authorized_keys"
    chown "${USER_NAME}:${USER_NAME}" "${HOME_DIR}/.ssh/authorized_keys"
    chmod 0600 "${HOME_DIR}/.ssh/authorized_keys"
    log "authorized_keys installed from BOX_AUTHORIZED_KEYS"
fi

# ── Workspace ownership ──────────────────────────────────────────
# /work is a named volume; a fresh one is root-owned and the user cannot write.
if [[ -d /work ]]; then
    if [[ "$(stat -c '%U' /work)" != "$USER_NAME" ]]; then
        log "claiming /work for ${USER_NAME}"
        chown "${USER_NAME}:${USER_NAME}" /work
    fi
fi

log "ready — entering ${*}"
exec "$@"
