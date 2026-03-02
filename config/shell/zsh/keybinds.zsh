# Vi mode
set -o vi

bindkey -M viins '^A' beginning-of-line
bindkey -M viins '^E' end-of-line
bindkey -M vicmd '^A' beginning-of-line
bindkey -M vicmd '^E' end-of-line

# Vi-mode keybinds: j/k history nav, gg/G history start/end, / incremental search
bindkey -M vicmd 'j' down-line-or-history
bindkey -M vicmd 'k' up-line-or-history
bindkey -M vicmd 'G' end-of-buffer-or-history
bindkey -M vicmd 'gg' beginning-of-buffer-or-history
bindkey -M vicmd '/' history-incremental-search-backward

# Vi-mode cursor shape: block in normal, beam in insert
function zle-keymap-select {
  if [[ $KEYMAP == vicmd ]] || [[ $1 == 'block' ]]; then
    echo -ne '\e[2 q'  # block cursor
  elif [[ $KEYMAP == main ]] || [[ $KEYMAP == viins ]] || [[ $1 == 'beam' ]]; then
    echo -ne '\e[6 q'  # beam cursor
  fi
}
zle -N zle-keymap-select

# Start with beam cursor
function _set_beam_cursor { echo -ne '\e[6 q' }
_set_beam_cursor
precmd_functions+=(_set_beam_cursor)
