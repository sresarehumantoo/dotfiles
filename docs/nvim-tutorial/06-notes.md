# 6. Notes

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md).

Neovim doubles as an Obsidian replacement through [obsidian.nvim](https://github.com/obsidian-nvim/obsidian.nvim). It works on a **vault**: a plain directory of Markdown files, compatible with the Obsidian app, so the same notes open in either.

## The vault

The vault is `$NOTES_DIR`, or `~/notes` when that's unset. Everything in this chapter only exists once that directory does: create it with `mkdir ~/notes`, or point `NOTES_DIR` at an existing vault (an Obsidian vault, a synced or mounted share) in your shell config.

Inside the vault, Markdown is treated as notes rather than published docs:

- **Linting is relaxed** to the rules that catch markup that renders wrong: broken tables, reversed or empty links, links to headings that don't exist. Style rules like line length or blank lines around headings are off. The rule set is `config/nvim/notes.markdownlint.yaml`.
- **Nothing is reformatted on save.** `<leader>f` still formats on demand.
- **New notes are named by their title**, as in the Obsidian app, and no frontmatter is added to your notes.

## Keys

| Keys                        | Does                                                                                     |
| --------------------------- | ---------------------------------------------------------------------------------------- |
| `<leader>nn`                | new note                                                                                 |
| `<leader>nd` / `<leader>nD` | today's daily note / pick a daily note                                                   |
| `<leader>nf`                | find a note by name                                                                      |
| `<leader>ns`                | search the text of every note                                                            |
| `<leader>nt`                | browse notes by tag                                                                      |
| `<leader>nb`                | backlinks: notes that link to this one                                                   |
| `<leader>nl`                | links in this note                                                                       |
| `<leader>nr`                | rename this note and update every link to it                                             |
| `<leader>np`                | paste the clipboard image into `attachments/` and link it                                |
| `Enter`                     | follow the link, open the tag, toggle the checkbox, or fold the heading under the cursor |
| `]o` / `[o`                 | next / previous link                                                                     |
| `[[`                        | start a link: completes note names as you type                                           |

The `<leader>n` keys work from anywhere, so you can jump into today's note from the middle of a coding session.

## Try it on the sample vault

This tutorial ships a tiny vault in [vault/](vault/). Point a session at it with `NOTES_DIR`, from the repo root:

```sh
NOTES_DIR=docs/nvim-tutorial/vault nvim "docs/nvim-tutorial/vault/Notes Tour.md"
```

The tour inside walks through links, backlinks, tags, daily notes and the relaxed linting. Daily notes and pasted images are gitignored there. A note you create with `<leader>nn` shows up as an untracked file: `git clean -n docs/nvim-tutorial/vault` lists what you've added, and `git checkout -- docs/nvim-tutorial/vault` restores the tour notes if you've edited them.

That's the end of the tutorial. The full binding reference is [Keybindings](../keybindings.md#neovim), and `<leader>sk` searches every mapping from inside nvim.
