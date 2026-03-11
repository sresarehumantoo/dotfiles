# Keybindings Reference

Quick reference for all custom keybindings across interactive menus, tmux, neovim, zsh, and shell aliases.

---

## Interactive Menus (Toolkit, OMZ Extended, Shell Preserve)

### Category Navigation

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate options |
| `Enter` | Browse into category |
| `q` | Quit menu (keep existing config) |
| `Esc` / `Ctrl+C` | Quit menu (keep existing config) |

### Tool / Plugin Selection

| Key | Action |
|-----|--------|
| `↑` / `↓` or `j` / `k` | Navigate options |
| `Space` | Toggle selection |
| `Enter` | Confirm selections |
| `Esc` / `q` | Go back to categories (no changes) |

---

## Tmux

Prefix: **Alt+a**

### General

| Key | Action |
|-----|--------|
| `prefix r` | Reload config |
| `prefix c` | New window (current path) |
| `prefix "` | Split vertical (current path) |
| `prefix %` | Split horizontal (current path) |
| `prefix m` | Toggle mouse on/off |

### Pane Navigation

| Key | Action |
|-----|--------|
| `prefix h` | Select pane left |
| `prefix j` | Select pane down |
| `prefix k` | Select pane up |
| `prefix l` | Select pane right |

### Pane Resize

| Key | Action |
|-----|--------|
| `prefix H` | Resize left 5 |
| `prefix J` | Resize down 5 |
| `prefix K` | Resize up 5 |
| `prefix L` | Resize right 5 |

### Window Management

| Key | Action |
|-----|--------|
| `prefix C-h` | Previous window |
| `prefix C-l` | Next window |
| `prefix <` | Swap window left |
| `prefix >` | Swap window right |

### Copy Mode (vi)

| Key | Action |
|-----|--------|
| `v` | Begin selection |
| `y` | Copy selection |
| `C-v` | Toggle rectangle select |

### Plugins (TPM)

| Key | Action |
|-----|--------|
| `prefix I` | Install plugins |
| `prefix U` | Update plugins |

### Session (resurrect)

| Key | Action |
|-----|--------|
| `prefix C-s` | Save session |
| `prefix C-r` | Restore session |

### Logging Aliases

| Alias | Action |
|-------|--------|
| `tlog` | Toggle pane logging |
| `tcap` | Screen capture |
| `thist` | Save complete history |
| `tclear` | Clear pane history |

---

## Neovim

Leader: **Space**

### Core

| Key | Action |
|-----|--------|
| `Esc` | Clear search highlight |
| `C-h/j/k/l` | Move focus between windows |
| `C-a` | Home (normal/visual/insert) |
| `C-e` | End (normal/visual/insert) |
| `C-s` | Save file |

### Editing

| Key | Action |
|-----|--------|
| `leader d` | Delete word (no yank) |
| `leader l` | Delete line (no yank) |
| `A-j` / `A-k` | Move line(s) down/up |
| `>` / `<` (visual) | Indent/dedent and reselect |

### Buffers & Splits

| Key | Action |
|-----|--------|
| `leader bn` | Next buffer |
| `leader bp` | Previous buffer |
| `leader bw` | Close buffer |
| `leader \|` | Vertical split |
| `leader -` | Horizontal split |

### Telescope (Search)

| Key | Action |
|-----|--------|
| `leader sh` | Search help tags |
| `leader sk` | Search keymaps |
| `leader sf` | Search files |
| `leader ss` | Search select (Telescope pickers) |
| `leader sw` | Search current word |
| `leader sg` | Search by grep |
| `leader sd` | Search diagnostics |
| `leader sr` | Search resume |
| `leader s.` | Search recent files |
| `leader sn` | Search neovim config files |
| `leader s/` | Search in open files (live grep) |
| `leader leader` | Find existing buffers |
| `leader /` | Fuzzy search in current buffer |

### LSP

| Key | Action |
|-----|--------|
| `grn` | Rename symbol |
| `gra` | Code action |
| `grr` | Go to references |
| `gri` | Go to implementation |
| `grd` | Go to definition |
| `grD` | Go to declaration |
| `gO` | Document symbols |
| `gW` | Workspace symbols |
| `grt` | Go to type definition |
| `leader th` | Toggle inlay hints |

### Git (Gitsigns)

| Key | Action |
|-----|--------|
| `]c` / `[c` | Next/previous git change |
| `leader hs` | Stage hunk |
| `leader hr` | Reset hunk |
| `leader hS` | Stage buffer |
| `leader hu` | Undo stage hunk |
| `leader hR` | Reset buffer |
| `leader hp` | Preview hunk |
| `leader hb` | Blame line |
| `leader hd` | Diff against index |
| `leader hD` | Diff against last commit |
| `leader tb` | Toggle blame line |
| `leader tD` | Toggle show deleted |

### Formatting

| Key | Action |
|-----|--------|
| `leader f` | Format buffer |

### Harpoon

| Key | Action |
|-----|--------|
| `leader a` | Add file to harpoon |
| `leader e` | Toggle harpoon menu |
| `leader 1-4` | Jump to harpoon file 1-4 |

### Flash

| Key | Action |
|-----|--------|
| `s` | Flash jump |
| `S` | Flash treesitter select |

### File Navigation

| Key | Action |
|-----|--------|
| `-` or `leader o` | Open parent directory (Oil) |
| `\` | Toggle Neo-tree |

### Misc

| Key | Action |
|-----|--------|
| `leader u` | Toggle undotree |
| `leader q` | Open diagnostic quickfix list |
| `Esc Esc` (terminal) | Exit terminal mode |

### Completion (blink.cmp)

| Key | Action |
|-----|--------|
| `Tab` / `S-Tab` | Next/previous completion item |
| `C-e` | Accept completion |
| `C-s` | Accept and enter |

---

## Zsh (vi mode)

### Insert & Normal Mode

| Key | Action |
|-----|--------|
| `C-a` | Beginning of line |
| `C-e` | End of line |

### Normal Mode

| Key | Action |
|-----|--------|
| `j` / `k` | History navigation (down/up) |
| `G` | End of buffer/history |
| `gg` | Beginning of buffer/history |
| `/` | Incremental search backward |

Cursor changes to block in normal mode, beam in insert mode.

---

## Shell Aliases

### Navigation

| Alias | Command |
|-------|---------|
| `..` | `cd ..` |
| `...` | `cd ../..` |
| `ll` | `ls -lAh` |
| `la` | `ls -A` |
| `q` | `exit` |

### Editors

| Alias | Command |
|-------|---------|
| `vim` | `nvim` |
| `zshrc` | `$EDITOR ~/.zshrc` |
| `vimrc` | `$EDITOR ~/.config/nvim/init.lua` |
| `tmuxrc` | `$EDITOR ~/.config/tmux/tmux.conf` |

### Docker

| Alias | Command |
|-------|---------|
| `dps` | `docker ps` |
| `dimg` | `docker images` |
| `dlog` | `docker logs -f` |
| `dex` | `docker exec -it` |
| `dcp` | `docker compose` |
| `dcup` | `docker compose up -d` |
| `dcdn` | `docker compose down` |

### Terraform

| Alias | Command |
|-------|---------|
| `tf` | `terraform` |
| `tfi` | `terraform init` |
| `tfp` | `terraform plan` |
| `tfa` | `terraform apply` |

### Tmux Logging

| Alias | Action |
|-------|--------|
| `tlog` | Toggle pane logging |
| `tcap` | Screen capture |
| `thist` | Save complete history |
| `tclear` | Clear pane history |

### System

| Alias | Command |
|-------|---------|
| `ports` | `ss -tulnp` |
| `myip` | `curl -s ifconfig.me` |
| `clip` | Copy to clipboard (xclip or wl-copy) |

### Package Management (apt)

| Alias | Command |
|-------|---------|
| `update` | `sudo apt-get update` |
| `upgrade` | `sudo apt-get upgrade` |
| `updateDist` | `sudo apt-get dist-upgrade` |
| `updateAll` | update + upgrade + dist-upgrade |
