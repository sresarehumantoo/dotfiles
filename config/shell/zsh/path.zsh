# PATH — prepend ~/.local/bin so user-installed, self-updating tools (e.g. the
# native Claude Code build) win over any system copy in /usr/local/bin. It was
# an append before, which let /usr/local/bin shadow ~/.local/bin.
export PATH="$HOME/.local/bin:$PATH"
export PATH="$PATH:/usr/local/go/bin"

# Collapse duplicate entries (login shells / .profile may also add ~/.local/bin).
typeset -U path PATH
