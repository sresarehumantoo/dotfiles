export VISUAL='nvim'
export EDITOR='nvim'
if (( $+commands[nvim] && $+commands[man] )); then
  export MANPAGER='nvim +Man!'
fi

# CA bundle exports (REQUESTS_CA_BUNDLE / SSL_CERT_FILE / CURL_CA_BUNDLE)
# moved to ~/.profile so non-zsh shells (bash subshells, ansible-playbook from
# Claude Code's Bash tool, cron, etc.) also pick them up.
#
# ⚠ "Zsh inherits via the parent login shell" is what this said, and it was not
# true: zsh never reads ~/.profile, and on a machine where no display manager
# sources it either (WSL always, and this desktop as measured 2026-09-12) those
# exports reached zsh through nothing at all. ~/.zprofile now sources it for
# login shells, which is what makes the sentence true; see config/shell/zprofile.
# NPM_CONFIG_PREFIX below is kept duplicated regardless, because a NON-login
# interactive zsh still never sees ~/.profile.

# npm global prefix -> ~/.local (see ~/.profile for the rationale). Set here too
# because zsh doesn't source ~/.profile, and `npm i -g` is typically run from an
# interactive zsh — without this it would fall back to the root-owned /usr/local
# prefix. Guarded so nvm (which owns its own prefix) is left alone.
if [[ -z "$NVM_DIR" ]] && (( $+commands[npm] )) && [[ "$commands[npm]" != "$HOME/.nvm/"* ]]; then
  export NPM_CONFIG_PREFIX="$HOME/.local"
fi

# The tmux server outlives a sway restart, so a pane's SWAYSOCK can name a dead
# socket. Attaching from the new session refreshes tmux's copy (update-environment
# in tmux.conf); this pulls it into shells that were already running.
if [[ -n $TMUX ]]; then
  _df_refresh_wayland_env() {
    [[ -n $SWAYSOCK && ! -S $SWAYSOCK ]] || return 0
    eval "$(tmux show-environment -s SWAYSOCK 2>/dev/null; tmux show-environment -s WAYLAND_DISPLAY 2>/dev/null)"
  }
  autoload -Uz add-zsh-hook
  add-zsh-hook precmd _df_refresh_wayland_env
fi
