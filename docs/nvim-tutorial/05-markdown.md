# 5. Markdown

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md). Everything in this chapter works in any Markdown file. The [next chapter](06-notes.md) adds the notes-vault features on top.

## Rendering

Headings, tables, code blocks, checkboxes and callouts are drawn in the buffer, while the line under the cursor always shows its raw markup so editing feels normal. Move the cursor through the table below to watch each row switch. `<leader>tm` flips the whole buffer between rendered and raw.

| Feature    | Rendered as                    |
| ---------- | ------------------------------ |
| `# Title`  | a full-width heading band      |
| `- [ ]`    | a checkbox icon                |
| `> [!tip]` | a colored callout with an icon |

> [!tip]
> Callouts use the Obsidian and GitHub syntax: `> [!note]`, `> [!tip]`, `> [!warning]` and friends.

## Prose editing

Long lines soft-wrap at word boundaries, and `j` / `k` move by **screen** line, so you can walk through a wrapped paragraph one visual row at a time. With a count (`5j`) they still jump by real lines, so relative line numbers keep working. This paragraph is a single long line on purpose so you can try exactly that: move through it with `j` and `k`, then compare with `gj` / `gk` (always screen lines) and `+` / `-` (always real lines).

## Spelling

Spell checking is on in Markdown. Misspellings are underlined and never changed for you:

> This sentance has a definately misspelled word or two.

| Keys         | Does                                                           |
| ------------ | -------------------------------------------------------------- |
| `]s` / `[s`  | next / previous misspelling                                    |
| `z=`         | suggestions for the word under the cursor                      |
| `<leader>zs` | jump back to the previous misspelling and open its suggestions |
| `zg` / `zw`  | mark the word as good / wrong in your dictionary               |

While typing, the completion menu also offers spelling suggestions for the word you're on.

## Lists and checkboxes

Lists continue themselves:

| Keys                          | Does                                                    |
| ----------------------------- | ------------------------------------------------------- |
| `Enter` (insert), `o`         | a new item of the same kind: bullet, number or checkbox |
| `Enter` on an empty item      | ends the list                                           |
| `Ctrl-Enter` (insert)         | a plain new line, no bullet                             |
| `Ctrl-t` / `Ctrl-d` (insert)  | indent / outdent the item                               |
| `>>` / `<<`, visual `>` / `<` | indent / outdent items                                  |
| `<leader>x`                   | toggle the checkbox under the cursor                    |
| `gN` (normal or visual)       | renumber a numbered list                                |

Try: `A` then `Enter` at the end of the last checkbox and type a new task. Press `Enter` again on the empty item to end the list. Then `<leader>x` on "Two" to tick it.

- [ ] One
- [ ] Two
- [x] Three

Numbered lists renumber as you insert: `A`, `Enter` on "Second" adds an item and shifts the rest down. After deleting one with `dd`, select the list and press `gN`.

1. First
2. Second
3. Third

## Folding

Headings fold their sections, nested headings included. On a heading: `zc` closes, `zo` opens, `za` toggles; `zM` closes everything and `zR` opens it all again. Try `zM` now to see this file as an outline.

### A nested heading

Closing "Folding" above hides this section too.

## Code blocks

Fenced code is highlighted in its own language, and `gcc` inside a fence uses that language's comment marker:

```go
func main() {
    fmt.Println("gcc here gives //")
}
```

```sh
echo "and here gives #"
```

## Linting and formatting

Outside the notes vault, Markdown is checked with markdownlint's full default rules, the strictness you want for READMEs and docs like this one. Findings show only as signs in the gutter rather than text under the line, so they don't push your writing around: `]d` jumps to the next one and `<leader>e` reads it. Saving runs prettier, which normalizes list markers, spacing and table alignment.

Inside the vault (next chapter) both relax: only markup that renders wrong is flagged, and nothing is reformatted on save.

Next: [Notes](06-notes.md).
