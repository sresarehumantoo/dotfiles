# Keybindings Reference

Quick reference for all custom keybindings across interactive menus, macOS, tmux, neovim, zsh, and shell aliases.

---

## Interactive Menus (Toolkit, OMZ Extended)

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

## macOS (mackeys, alttab, Ghostty)

The built-in keyboard's bottom row is remapped to Ctrl, Fn, Cmd, Opt, matching a Windows laptop's Ctrl, Fn, Win, Alt. Keys below are named by what they send after that swap, except `Win`, which is the key in the Windows key's position. In browsers, Electron apps and Office the Ctrl key sends Cmd and the Win key sends Ctrl, so web apps and Office get the Cmd shortcuts they expect while the letters stay where Windows has them.

| Key | Action |
|-----|--------|
| `Ctrl+C` / `Ctrl+V` / `Ctrl+X` | Copy / Paste / Cut in GUI and web apps |
| `Ctrl+Shift+V` | Paste and Match Style (paste as plain text) |
| `Ctrl+A` / `Ctrl+Z` / `Ctrl+Y` | Select All / Undo / Redo |
| `Ctrl+F` / `Ctrl+S` / `Ctrl+O` / `Ctrl+P` | Find / Save / Open / Print |
| `Ctrl+T` / `Ctrl+W` / `Ctrl+Shift+T` | New tab / Close tab / Reopen closed tab |
| `Ctrl+N` / `Ctrl+Shift+N` / `Ctrl+Shift+W` | New window / Private window / Close window |
| `Ctrl+R` / `Ctrl+L` | Reload / Address bar |
| `Ctrl+=` / `Ctrl+-` / `Ctrl+0` | Zoom in / out / reset |
| `Ctrl+Left` / `Ctrl+Right` | Word left / right in text fields (Shift selects) |
| `Ctrl+Backspace` / `Ctrl+Delete` | Delete word back / forward |
| `Home` / `End` (`Fn+Left` / `Fn+Right`) | Start / end of line |
| `Alt+Tab` / `Alt+Shift+Tab` | Switch apps (alttab); release Alt to switch, `Alt+Esc` cancels |
| `Cmd+Space` | Spotlight (the Win+Space position) |
| `Win+L` | Lock the screen (alttab) |
| `Win+Tab` | Next browser tab (Ctrl+Tab is the macOS app switcher in browsers, which get the command layout) |
| `Cmd+Shift+Q` | Log out (macOS default) |

### Ghostty on macOS

Ghostty's Linux defaults, on top of the macOS ones (`config/ghostty/macos.conf`).

| Key | Action |
|-----|--------|
| `Ctrl+C` / `Ctrl+Z` / `Ctrl+W` | Go to the shell, as on Linux |
| `Ctrl+Shift+C` / `Ctrl+Shift+V` / `Ctrl+Shift+A` | Copy / Paste / Select all |
| `Ctrl+Shift+T` / `Ctrl+Shift+W` / `Ctrl+Shift+N` | New tab / Close tab / New window |
| `Ctrl+Tab` / `Ctrl+Shift+Tab` | Next / previous tab (also `Ctrl+PageDown`/`PageUp`, `Ctrl+Shift+Right`/`Left`) |
| `Ctrl+Shift+O` / `Ctrl+Shift+E` | Split right / split down |
| `Ctrl+Shift+Enter` | Zoom the split |
| `Cmd+Alt+arrows` | Move between splits (`Ctrl+Alt+arrows` belongs to AeroSpace) |
| `Cmd+Ctrl+[` / `Cmd+Ctrl+]` | Previous / next split |
| `Cmd+Ctrl+Shift+arrows` | Resize the split |
| `Shift+PageUp` / `Shift+PageDown` (`Shift+Fn+Up` / `Shift+Fn+Down`) | Scroll back / forward |
| `Shift+Home` / `Shift+End` | Top / bottom of scrollback |
| `Ctrl+Shift+PageUp` / `Ctrl+Shift+PageDown` | Previous / next prompt |
| `Ctrl+=` / `Ctrl+-` / `Ctrl+0` | Font size bigger / smaller / reset |
| `Ctrl+Shift+P` | Command palette |
| `Ctrl+,` / `Ctrl+Shift+,` | Open / reload config |

---

## Tmux

Prefix: **Alt+a**

### General

| Key | Action |
|-----|--------|
| `prefix r` | Reload config |
| `M-n` | New window (current path) |
| `M-1`–`M-9` | Switch to window 1–9 |
| `C-q` | Kill window |
| `M-Enter` | Split vertical (current path) |
| `M--` | Split horizontal (current path) |
| `M-q` | Kill pane |
| `M-m` | Zoom pane toggle |
| `M-s` | Break pane into new window |
| `M-c` | Enter copy mode |
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
| `prefix M-←↓↑→` | Resize 5 (tmux default) |
| `prefix C-←↓↑→` | Resize 1 (tmux default) |
| Drag pane border | Resize with the mouse (mouse on by default) |

All of these repeat: after the prefix, keep pressing `H`/`J`/`K`/`L` (or the arrows) within 600ms (`repeat-time`) without pressing the prefix again. Holding `Shift` and tapping `H` grows a pane in steps of 5.

### Window Management

| Key | Action |
|-----|--------|
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
| `prefix C-s` | Save session (also resumes a paused autosave) |
| `prefix C-r` | Restore session (also resumes a paused autosave) |

### Logging Aliases

| Alias | Action |
|-------|--------|
| `tlog` | Toggle pane logging |
| `tcap` | Screen capture |
| `thist` | Save complete history |
| `tclear` | Clear pane history |

---

## Newsboat

vim motions on top of the defaults (`config/newsboat/config`). `h` backs out of every view except the feed list, so it can never quit the app.

| Key | Action |
|-----|--------|
| `j` / `k` | Down / up |
| `l` / `h` | Open / back |
| `g` / `G` | Top / bottom |
| `Ctrl+d` / `Ctrl+u` | Half page down / up |
| `Ctrl+f` / `Ctrl+b` | Page down / up |
| `J` / `K` | Next / previous feed (article list), article (article view) |
| `n` / `N` | Next / previous unread |
| `a` | Toggle read |
| `u` | List the article's links |
| `U` | Toggle showing read feeds |
| `t` / `T` | Filter by tag / clear the filter |
| `s` / `S` | Sort / reverse sort |
| `f` | Jump to a feed by title |
| `Ctrl+r` | Reload all feeds |
| `o` | Open the link in the terminal: inline if it is an image, w3m otherwise |
| `,i` | Page through the article's images (`n`/space next, `p` prev, `o` browser, `q` quit) |
| `,o` | Open in the desktop browser |
| `,y` | Copy the link |

## Neovim

Leader: **Space**

For a hands-on walkthrough of these, opened in nvim and practiced as you read, see the [Neovim tutorial](nvim-tutorial/README.md).

### Core

| Key | Action |
|-----|--------|
| `Esc` | Clear search highlight |
| `C-h/j/k/l` | Move focus between windows |
| `C-a` | Home (normal/visual/insert) |
| `C-e` | End (normal/visual/insert) |
| `C-s` | Save file, in normal, insert or visual mode, staying in that mode. In insert mode with the completion menu open it accepts the item instead (see Completion); close the menu to save |

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
| `leader T\|` | Terminal in a vertical split: the right 40% of the editor |
| `leader T-` | Terminal in a horizontal split: the bottom 30% of the editor |

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
| `leader hv` | Diff view of all changes (diffview; `q` closes) |
| `leader hf` | History of the current file (diffview) |
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
| `leader m` | Toggle harpoon menu |
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
| `leader e` | Show the full diagnostic for the line in a float |
| `leader td` | Toggle inline diagnostics: cursor line / every line |
| `q` | Disabled (mapped to `<Nop>`) — stops accidental macro recording |
| `Q` | Record macro (register) — the relocated `q`, replacing Ex mode |

### Terminal Mode

| Key | Action |
|-----|--------|
| `Esc Esc` | Exit terminal mode |
| `C-h` / `C-j` / `C-k` / `C-l` | Leave terminal mode and move focus to the left/lower/upper/right window |

A new terminal opens in insert mode with no line numbers or sign column. `C-h/j/k/l` mirror the Normal-mode window-nav maps, dropping to Normal via `<C-\><C-n>` first so no `Esc Esc` is needed. Caveat: this shadows the shell's `C-l` clear-screen inside terminal buffers — use `clear` or `reset` instead.

### Completion (blink.cmp)

| Key | Action |
|-----|--------|
| `Tab` / `S-Tab` | Next/previous completion item |
| `C-e` | Accept completion |
| `C-s` | Accept and enter |

### Markdown

| Key | Action |
|-----|--------|
| `leader tm` | Toggle render-markdown (rendered ↔ raw) |
| `j` / `k` | Move by display line (soft-wrapped prose) |
| `leader zs` | Jump to previous misspelling + open `z=` suggestions |
| `z=` | Spell suggestions for word under cursor |
| `zg` | Add word to dictionary |
| `Enter` / `o` (on a list item) | Continue the list or checkbox; `Enter` on an empty item ends it |
| `leader x` | Toggle the checkbox under the cursor |

Markdown buffers enable spell-check (`en_us`, CamelCase-aware); misspellings are only highlighted, never auto-corrected. markdownlint findings show as gutter signs only; read one with `leader e`. Headings fold with `za`/`zc`/`zo` (treesitter folds, all open by default).

### Notes (obsidian.nvim)

The vault is `$NOTES_DIR`, default `~/notes`; the [tutorial's](nvim-tutorial/06-notes.md) sample vault is registered as a second one. The keys act on whichever vault the current note is in, and on your own vault from anywhere else.

| Key | Action |
|-----|--------|
| `leader nn` | New note (filename is the title, as in the Obsidian app) |
| `leader nd` / `leader nD` | Today's daily note / pick a daily note |
| `leader nf` | Find a note by name |
| `leader ns` | Search note text |
| `leader nt` | Notes by tag |
| `leader nb` | Backlinks to this note |
| `leader nl` | Links in this note |
| `leader nr` | Rename the note, updating links to it |
| `leader np` | Paste the clipboard image into `attachments/` |
| `Enter` (normal) | Follow the link, open the tag, or toggle the checkbox under the cursor |
| `]o` / `[o` | Next/previous link |

Type `[[` for note-name completion. Inside the vault, Markdown is linted only for markup that renders wrong (`config/nvim/notes.markdownlint.yaml`) and is not reformatted on save; `leader f` still formats on demand.

### Debugging (nvim-dap)

| Key | Action |
|-----|--------|
| `F5` | Start/continue (Go via delve, Python via debugpy using the project's `.venv`) |
| `F1` / `F2` / `F3` | Step into / over / out |
| `leader b` / `leader B` | Toggle breakpoint / conditional breakpoint |
| `F7` | Toggle the DAP UI |

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

Cursor changes to steady block in normal mode, blinking block in insert mode.

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
