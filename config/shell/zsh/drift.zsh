# Say so, once, when something changed a dotfiles-managed file behind
# dfinstall's back: a link replaced by a file or pointing elsewhere, or managed
# settings edited (Claude Code's /config rewriting ~/.claude/settings.json).
#
# `dfinstall diff --brief` takes tens of milliseconds and p10k's instant prompt
# forbids output during startup, so the check runs in the background and its
# answer is printed by precmd at a later prompt. It runs at most every ten
# minutes across all shells, and each finding is printed once, not per pane.
# DF_NO_DRIFT_CHECK=1 turns it off.
[[ -o interactive && -z $DF_NO_DRIFT_CHECK ]] || return
(( $+commands[dfinstall] )) || return
zmodload -F zsh/stat b:zstat 2>/dev/null || return
zmodload zsh/datetime

typeset -g _df_drift=${XDG_STATE_HOME:-$HOME/.local/state}/dfinstall/drift

_df_drift_refresh() {
  local -a mtime
  if zstat -A mtime +mtime -- $_df_drift.checked 2>/dev/null && (( EPOCHSECONDS - mtime[1] < 600 )); then
    return
  fi
  [[ -d ${_df_drift:h} ]] || mkdir -p ${_df_drift:h}
  : >| $_df_drift.checked
  { dfinstall diff --brief >| $_df_drift.$$ 2>/dev/null && mv -f $_df_drift.$$ $_df_drift || rm -f $_df_drift.$$ } &!
}

_df_drift_show() {
  _df_drift_refresh
  local found seen
  [[ -r $_df_drift ]] && found=$(<$_df_drift)
  [[ -r $_df_drift.shown ]] && seen=$(<$_df_drift.shown)
  [[ $found == $seen ]] && return
  print -r -- $found >| $_df_drift.shown
  [[ -n $found ]] || return
  print -P '%F{yellow}⚠ dfinstall:%f managed files changed outside dfinstall'
  print -rl -- ${${(f)found}/#/  }
  print -P '  %F{8}review with: dfinstall diff   relink or re-merge with: dfinstall install all --upgrade%f'
}

autoload -Uz add-zsh-hook
add-zsh-hook precmd _df_drift_show
