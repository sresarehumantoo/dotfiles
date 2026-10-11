# 2. Files and Search

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md). These exercises search the repo you're in, so start nvim from the repo root.

## Telescope: find anything

Every search lives under `<leader>s`:

| Keys               | Searches                                                  |
| ------------------ | --------------------------------------------------------- |
| `<leader>sf`       | file names                                                |
| `<leader>sg`       | file contents, live as you type (ripgrep)                 |
| `<leader>sw`       | the word under the cursor, across the project             |
| `<leader>s.`       | recently opened files                                     |
| `<leader><leader>` | open buffers                                              |
| `<leader>/`        | lines in the current buffer, fuzzy                        |
| `<leader>s/`       | live grep, but only in open files                         |
| `<leader>sd`       | diagnostics                                               |
| `<leader>sh`       | help pages                                                |
| `<leader>sk`       | **keymaps**: the fastest way to find a binding you forgot |
| `<leader>sn`       | your Neovim config files                                  |
| `<leader>sr`       | resume the last search where you left it                  |
| `<leader>ss`       | the list of every Telescope picker                        |

Inside a picker:

| Keys                | Does                                              |
| ------------------- | ------------------------------------------------- |
| `Ctrl-n` / `Ctrl-p` | next / previous result                            |
| `Enter`             | open it                                           |
| `Ctrl-v` / `Ctrl-x` | open in a vertical / horizontal split             |
| `Ctrl-q`            | send every result to the quickfix list            |
| `Ctrl-/`            | show the picker's keys (`?` in normal mode)       |
| `Esc`               | normal mode inside the picker; `Esc` again closes |

Try:

1. `<leader>sf`, type `keybinds`, `Enter`: you're in `config/nvim/lua/custom/keybinds.lua`. `Ctrl-o` comes back.
2. `<leader>sg`, type `timeoutlen`: every file that mentions it. `Ctrl-q` drops them into the quickfix list; `:cnext` / `:cprev` step through, `:cclose` hides it.
3. `<leader>sk`, type `harpoon`: every Harpoon mapping with its description.
4. `<leader>sr` reopens that last search exactly as you left it.

## Oil: edit a directory like a buffer

`-` opens the current file's directory as an editable buffer (`<leader>o` does the same). `-` again goes up a level, `Enter` opens the entry under the cursor.

The trick is that it's text: **rename** by editing a name, **delete** with `dd`, **create** by typing a new line (end it with `/` for a directory), **move** by cutting a line and pasting it in another directory's buffer. Nothing touches disk until `:w`, which shows the planned changes and asks first. `g?` lists Oil's keys.

Try: press `-` here to see this tutorial's directory, then `-` again for `docs/`. `:q` or `Ctrl-o` gets you back without changing anything.

## Neo-tree: the sidebar

`\` toggles a file tree on the left. In it, `Enter` opens, `a` adds a file, `r` renames, `d` deletes, `?` shows the rest. It's handy for getting a picture of an unfamiliar project; Oil and Telescope are usually faster for actually getting somewhere.

## Harpoon: your working set

Harpoon pins the few files you're bouncing between, each on a fixed key:

| Keys                        | Does                                                |
| --------------------------- | --------------------------------------------------- |
| `<leader>a`                 | add the current file                                |
| `<leader>m`                 | the menu: reorder by moving lines, remove with `dd` |
| `<leader>1` ... `<leader>4` | jump straight to file 1 to 4                        |

Try: `<leader>a` here, open chapter 1 (`gf` on its name in the [index](README.md)) and `<leader>a` there too. Now `<leader>1` and `<leader>2` flip between the two, and `<leader>m` shows the list.

## Buffers, splits and windows

| Keys                                | Does                                        |
| ----------------------------------- | ------------------------------------------- |
| `<leader>bn` / `<leader>bp`         | next / previous buffer                      |
| `<leader>bw`                        | close the buffer                            |
| `<leader>\|` / `<leader>-`          | split vertically / horizontally             |
| `Ctrl-h` `Ctrl-j` `Ctrl-k` `Ctrl-l` | move to the window left / down / up / right |
| `Ctrl-w =`                          | make all windows the same size              |
| `Ctrl-w q`                          | close the window                            |

## The terminal

`:split | terminal` opens a shell under the current window. Typing goes to the shell; `Esc Esc` drops to normal mode so you can scroll and yank its output, and `i` goes back to typing. `Ctrl-h/j/k/l` work from inside the terminal too, so you can hop between code and shell without leaving terminal mode first. (That means the shell's own `Ctrl-l` clear is shadowed: type `clear`.)

Next: [Coding](03-coding.md).
