# Neovim Tutorial

<!-- markdownlint-disable MD013 -->

A hands-on tour of the Neovim setup in this repo (`config/nvim/`). Each chapter is meant to be opened **in nvim** and worked through: it tells you what to press and what you should see. For a plain lookup table of every binding, see [Keybindings](../keybindings.md#neovim).

Start here, from the repo root:

```sh
nvim docs/nvim-tutorial/README.md
```

## Before you start

- **Leader is `Space`.** `<leader>sf` means `Space`, then `s`, then `f`.
- **Moving between chapters:** put the cursor on a file name in the list below (the part in parentheses) and press `gf`. `Ctrl-o` jumps back.
- **Nothing here needs saving.** Experiment freely; `:e!` throws away your edits and reloads the file, and `u` undoes one step at a time. Saving a Markdown file outside the notes vault runs prettier on it, so if you do save by accident, `git checkout -- docs/nvim-tutorial` puts everything back.
- **Keys that start longer keys wait for you.** `s` is flash jump, but `sa` adds surrounding characters; `gc` comments a motion, `gcc` comments the line. Neovim waits up to half a second (`timeoutlen`) after the shorter key to see whether you meant the longer one.

## Chapters

1. [Moving and editing](01-moving-and-editing.md): flash jumps, text objects, surround, comments, moving lines, undotree.
2. [Files and search](02-files-and-search.md): Telescope, Oil, Neo-tree, Harpoon, buffers, splits, the terminal.
3. [Coding](03-coding.md): LSP navigation, diagnostics, completion, formatting, Python and Go specifics.
4. [Git and debugging](04-git-and-debugging.md): gitsigns hunks and blame, nvim-dap for Go and Python.
5. [Markdown](05-markdown.md): rendering, spelling, lists and checkboxes, folding, linting.
6. [Notes](06-notes.md): the Obsidian-style vault: links, backlinks, tags, daily notes.

The coding and debugging chapters use the practice files in [examples/](examples/), and the notes chapter uses the sample vault in [vault/](vault/).
