# 1. Moving and Editing

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md). Practice on the lines in each section; `:e!` resets the file whenever you like.

## Jumping with flash

`s` starts a flash jump: type the first one or two characters of where you want to go, and every match gets a highlighted label. Type the label to land there. It works across the whole visible window, and in visual and operator-pending mode too, so `d` `s` (label) deletes up to the target.

`S` selects by syntax tree instead: each label covers a node (a word, a call, a whole block), so `S` then a label visually selects that node.

Try it: press `s`, type `zq`, then the label that appears on the word below.

> The quick brown fox jumps over the lazy dog, and somewhere in this line hides zqx.

## Text objects (mini.ai)

Operators like `d`, `c`, `y` and `v` take text objects: `i` for inside, `a` for around. On top of Vim's built-ins, mini.ai adds:

| Object      | Means                                           |
| ----------- | ----------------------------------------------- |
| `b`         | any bracket: `()`, `[]` or `{}`                 |
| `q`         | any quote: `'`, `"` or `` ` ``                  |
| `f`         | a function call, `ia` / `aa` an argument        |
| `t`         | an HTML or XML tag                              |
| `in` / `il` | the **next** / **last** such object on the line |

Try these on the line below, starting with the cursor at the very beginning of the line:

- `ciq` changes inside the next quotes without moving there first.
- `dina` (`d` + `in` + `a`) deletes the next argument.
- `vab` selects the parentheses and everything in them, `yif` yanks the call's arguments.

```text
result = compute("first value", other(1, 2, 3), flag=True)
```

`g]` and `g[` jump to the end or start of an object: `g])` goes to the closing parenthesis.

## Surround (mini.surround)

| Keys                 | Does                                               |
| -------------------- | -------------------------------------------------- |
| `sa` + motion + char | add: `saiw"` wraps the word in quotes              |
| `sd` + char          | delete: `sd"` removes the quotes around the cursor |
| `sr` + old + new     | replace: `sr"'` turns `"x"` into `'x'`             |
| `sf` / `sF`          | find the next / previous surrounding               |
| `sh`                 | highlight the surrounding for a moment             |

In visual mode, select text and press `sa` plus a character. Try `saiw)` on `wrap`, then `sr)]`, then `sd]`:

```text
please wrap this word, then "change these quotes"
```

## Comments

| Keys                  | Does                                                 |
| --------------------- | ---------------------------------------------------- |
| `gcc`                 | toggle a comment on this line                        |
| `gc` + motion         | toggle over a motion: `gcip` comments the paragraph  |
| `gc` (visual)         | toggle the selected lines                            |
| `gco` / `gcO` / `gcA` | add a comment below / above / at the end of the line |
| `gbc`, `gb` + motion  | block comments, where the language has them          |

The comment marker follows the language under the cursor, including inside a fenced code block in Markdown. Try `gcc` on each line, and `gcj` on the first Lua line to toggle both:

```python
print("python comments with #")
```

```lua
print("lua comments with --")
print("and gcj takes this line too")
```

## Moving lines and indenting

| Keys               | Does                                                      |
| ------------------ | --------------------------------------------------------- |
| `Alt-j` / `Alt-k`  | move the line (or the selection) down / up                |
| `>` / `<` (visual) | indent / dedent and keep the selection, so you can repeat |
| `J`                | join the next line onto this one (not "move down")        |

Reorder these so they count up, using only `Alt-j` and `Alt-k`:

```text
three
one
two
```

## Small editing helpers

| Keys                | Does                                                            |
| ------------------- | --------------------------------------------------------------- |
| `<leader>d`         | delete the word under the cursor without touching the clipboard |
| `<leader>l`         | delete the line (or selection) without touching the clipboard   |
| `Ctrl-a` / `Ctrl-e` | jump to start / end of line, in normal, visual and insert mode  |
| `Ctrl-s`            | save                                                            |
| `Esc`               | clear search highlighting                                       |

Yanks go straight to the system clipboard (`clipboard=unnamedplus`), which is why the "no clipboard" deletes above exist: `<leader>l` removes a line without replacing what you copied from a browser.

`Ctrl-a` here is start of line, not Vim's "increment number"; use `g Ctrl-a` in visual mode if you need increments.

## Macros

Plain `q` is disabled so a stray keypress never starts recording. **`Q` records** instead: `Qa` starts recording into register `a`, `Q` stops, `@a` replays, `@@` repeats the last one.

Turn each line below into `- item` by recording once (`Qa`, then `I` and type a dash and a space, `Esc`, `j`, `Q`) and replaying with `2@a`:

```text
apples
pears
plums
```

## Undo history

`u` undoes, `Ctrl-r` redoes, and history survives closing the file (`undofile` is on). `<leader>u` opens **undotree**, a tree of every state the file has been in, including branches you undid away from; move through it with `j`/`k` and press `Enter` to restore one.

Next: [Files and search](02-files-and-search.md).
