# History
unsetopt HIST_VERIFY
setopt HIST_IGNORE_ALL_DUPS
setopt HIST_SAVE_NO_DUPS
setopt SHARE_HISTORY

# SHARE_HISTORY makes every prompt touch the history file, and this setup runs
# a lot of shells at once (zshrc execs into tmux, so each pane is another one).
# Zsh's default locking creates a <histfile>.LOCK and spins with retries when it
# is contended; HIST_FCNTL_LOCK uses a single fcntl() call instead, which is
# both faster and safe against a stale lockfile left by a killed shell.
# Requires a filesystem with working fcntl locking — true for ~ on ext4,
# including WSL's, which is why this is safe to set unconditionally here.
setopt HIST_FCNTL_LOCK

# ⚠ HISTSIZE/SAVEHIST are deliberately unbounded and are NOT to be "optimized"
# down. The startup cost of reading a large history is real but it is a chosen
# trade; the thing that actually made it hurt on WSL was `autoMemoryReclaim`
# dropping the page cache out from under the file on every reclaim, which is
# fixed in config/wsl/wslconfig by using `gradual` instead of `dropCache`.
HISTFILE=~/.zsh_history
HISTSIZE=999999999
SAVEHIST=999999999

# Zsh options
setopt AUTO_CD
setopt EXTENDED_GLOB
