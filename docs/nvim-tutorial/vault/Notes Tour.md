# Notes Tour

This note lives in the tutorial's sample vault. If `Enter` on the first link below does nothing, nvim isn't using this vault: reopen it with the command in `Getting Here.md`, next to this file.

## Links

- Put the cursor on [[Example Linked Note]] and press `Enter` to follow it. `Ctrl-o` comes back.
- `]o` and `[o` jump between links on this page.
- In the linked note, `<leader>nb` lists backlinks, and this note is one of them.
- On a new line below, type `[[Ex` and pick from the menu. A name that doesn't exist yet is offered as a new note.
- Plain URLs open in the browser with `gx`: https://github.com/obsidian-nvim/obsidian.nvim

## Tags

#tutorial #neovim

`Enter` on a tag lists every note carrying it; `<leader>nt` browses all tags.

## Tasks

`Enter` in normal mode on a checkbox toggles it:

- [ ] Follow a link
- [ ] Find this note again with `<leader>nf`
- [ ] Open today's daily note with `<leader>nd`
- [x] Open the tour

## Headings

`Enter` on a heading line folds its section. Try it on "Tasks" above.

## Creating and renaming

- `<leader>nn` asks for a title and creates the note with that file name.
- `<leader>nd` opens today's daily note, creating it in `daily/` if needed.
- `<leader>nr` on [[Example Linked Note]]'s own buffer renames it, and the links in this note follow.

## What linting still catches

Only markup that renders wrong is flagged in a vault. This note is otherwise full of things the strict rules would complain about (it has long lines, bare URLs and tags at the start of lines) and none of them are marked. The table below is missing a cell, so its last row gets a sign in the gutter. Put the cursor on it and press `<leader>e` to read the finding. Adding `| 4 |` to the row clears it once you save.

| a | b |
| - | - |
| 1 | 2 |
| 3 |
