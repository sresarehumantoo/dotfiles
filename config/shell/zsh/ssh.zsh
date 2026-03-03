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

# Add key if not already loaded (suppress askpass stderr for P10k instant prompt)
if ! ssh-add -l 2>/dev/null | grep -q "github_ed25519"; then
  ssh-add "$SSH_KEY" 2>/dev/null
fi
