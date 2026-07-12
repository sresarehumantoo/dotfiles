export VISUAL='nvim'
export EDITOR='nvim'

# CA bundle exports (REQUESTS_CA_BUNDLE / SSL_CERT_FILE / CURL_CA_BUNDLE)
# moved to ~/.profile so non-zsh shells (bash subshells, ansible-playbook from
# Claude Code's Bash tool, cron, etc.) also pick them up. Zsh inherits via the
# parent login shell.

# npm global prefix -> ~/.local (see ~/.profile for the rationale). Set here too
# because zsh doesn't source ~/.profile, and `npm i -g` is typically run from an
# interactive zsh — without this it would fall back to the root-owned /usr/local
# prefix. Guarded so nvm (which owns its own prefix) is left alone.
if [[ -z "$NVM_DIR" ]] && (( $+commands[npm] )) && [[ "$commands[npm]" != "$HOME/.nvm/"* ]]; then
  export NPM_CONFIG_PREFIX="$HOME/.local"
fi
