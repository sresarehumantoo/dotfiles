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

# Vi-mode cursor shape — only on terminals that support DECSCUSR
if [[ "$TERM" != "linux" && "$TERM" != "dumb" ]]; then
  function zle-keymap-select {
    if [[ $KEYMAP == vicmd ]] || [[ $1 == 'block' ]]; then
      echo -ne '\e[2 q'  # steady block
    elif [[ $KEYMAP == main ]] || [[ $KEYMAP == viins ]] || [[ $1 == 'beam' ]]; then
      echo -ne '\e[1 q'  # blinking block
    fi
  }
  zle -N zle-keymap-select

  # Start with blinking block cursor
  function _set_block_cursor { echo -ne '\e[1 q' }
  _set_block_cursor
  precmd_functions+=(_set_block_cursor)
fi
