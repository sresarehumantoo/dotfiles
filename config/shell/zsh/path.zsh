# PATH — prepend ~/.local/bin so user-installed, self-updating tools (e.g. the
# native Claude Code build) win over any system copy in /usr/local/bin. It was
# an append before, which let /usr/local/bin shadow ~/.local/bin.
#
# macOS: /etc/zprofile runs path_helper in every login shell, tmux panes
# included, and it moves /usr/bin and /bin ahead of everything inherited. The
# pane does not re-run ~/.profile (DF_PROFILE_SOURCED is inherited), so without
# this Apple's git and BSD sed/ls/grep win over brew's inside tmux. Same list as
# ~/.profile.
if [[ -n "$HOMEBREW_PREFIX" ]]; then
  path=($HOMEBREW_PREFIX/opt/{coreutils,findutils,gnu-sed,grep,gawk,gnu-tar,make}/libexec/gnubin(N)
        $HOMEBREW_PREFIX/opt/rustup/bin(N)
        $HOMEBREW_PREFIX/bin $HOMEBREW_PREFIX/sbin $path)
fi
[[ -d "$HOME/.cargo/bin" ]] && path=("$HOME/.cargo/bin" $path)
export PATH="$HOME/.local/bin:$PATH"
export PATH="$PATH:/usr/local/go/bin"

# Collapse duplicate entries (login shells / .profile may also add ~/.local/bin).
typeset -U path PATH
