export VISUAL='nvim'
export EDITOR='nvim'

# CA bundle exports (REQUESTS_CA_BUNDLE / SSL_CERT_FILE / CURL_CA_BUNDLE)
# moved to ~/.profile so non-zsh shells (bash subshells, ansible-playbook from
# Claude Code's Bash tool, cron, etc.) also pick them up. Zsh inherits via the
# parent login shell.
