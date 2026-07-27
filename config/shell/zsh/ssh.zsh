# Persistent SSH agent — shared across all shells
SSH_KEY="$HOME/.ssh/github_ed25519"
SSH_AGENT_SOCK="$HOME/.ssh/agent.sock"

# Bail if the key doesn't exist
[[ -f "$SSH_KEY" ]] || return

# Reuse existing agent or start a new one
export SSH_AUTH_SOCK="$SSH_AGENT_SOCK"
ssh-add -l &>/dev/null
local agent_status=$?

# 2 = can't connect — start a fresh agent
if [[ $agent_status -eq 2 ]]; then
  rm -f "$SSH_AGENT_SOCK"
  eval "$(ssh-agent -a "$SSH_AGENT_SOCK")" >/dev/null
fi

# Is the key already loaded? Compare fingerprints, not names.
#
# `ssh-add -l` prints each key's *comment*, which for this key is an email
# address — it never contains the filename. Matching on "github_ed25519"
# therefore never matched, so every new shell re-ran `ssh-add` and asked for a
# passphrase the agent had already been given: one prompt per pane, for nothing.
local key_fp
key_fp="$(ssh-keygen -lf "$SSH_KEY" 2>/dev/null | awk '{print $2}')"

if [[ -n "$key_fp" ]] && ! ssh-add -l 2>/dev/null | grep -qF -- "$key_fp"; then
  # Only ask when there is somebody to ask. A passphrase prompt during startup
  # blocks the terminal and writes to the tty, which also defeats P10k's instant
  # prompt. Non-interactive shells — scripts, tmux respawns, the MCP server —
  # skip it silently instead of hanging on a prompt nobody can answer.
  if [[ -o interactive ]] && [[ -t 0 ]]; then
    ssh-add "$SSH_KEY" 2>/dev/null
  fi
fi
