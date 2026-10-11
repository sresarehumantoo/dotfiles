# 4. Git and Debugging

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md).

## Git hunks with gitsigns

In any tracked file, the gutter marks lines that differ from the index: `+` added, `~` changed, `_` deleted below, `‾` deleted above. Make some changes here to see them: add a line under this paragraph, edit a word in the next one, delete a line from the list.

| Keys                        | Does                                                      |
| --------------------------- | --------------------------------------------------------- |
| `]c` / `[c`                 | next / previous change                                    |
| `<leader>hp`                | preview the hunk in a float                               |
| `<leader>tD`                | show the deleted lines inline                             |
| `<leader>hr`                | reset (throw away) the hunk                               |
| `<leader>hs`                | stage the hunk; in visual mode, only the selected lines   |
| `<leader>hu`                | unstage it again                                          |
| `<leader>hS` / `<leader>hR` | stage / reset the whole buffer                            |
| `<leader>hd` / `<leader>hD` | diff against the index / the last commit                  |
| `<leader>hb`                | blame the line in a float                                 |
| `<leader>tb`                | toggle blame for the cursor's line at the end of the line |

- One
- Two
- Three

Try: `]c` to each of your changes, `<leader>hp` to read one, `<leader>hr` to undo it. When you're done, `:e!` or `<leader>hR` puts the file back. Staging is real `git add`, so if you try `<leader>hs` here, undo it with `<leader>hu`.

## Debugging with nvim-dap

The debugger works for Go (delve) and Python (debugpy). Both are installed by Mason; Python runs the program with the project's `.venv` (or `$VIRTUAL_ENV`) when there is one.

| Keys               | Does                                                  |
| ------------------ | ----------------------------------------------------- |
| `<leader>b`        | toggle a breakpoint on the line                       |
| `<leader>B`        | a conditional breakpoint: it asks for the condition   |
| `F5`               | start, or continue to the next breakpoint             |
| `F2` / `F1` / `F3` | step over / into / out                                |
| `F7`               | toggle the debugger UI (to review a finished session) |

When a session starts, the UI opens around the code: variables in scope, the call stack, breakpoints, watches, and a console for the program's output. It closes again when the program ends.

### Go

Open [examples/go/main.go](examples/go/main.go) (`gf` on the path).

1. Put the cursor on line 16 (`sum += it.Price`) and press `<leader>b`. A breakpoint sign appears.
2. `F5`, and pick **Debug** from the list.
3. It stops in the loop on the first item. The scopes pane shows `it` (the tea, `2.5`) and `sum` (still `0`).
4. `F5` continues to the next stop: now `it` is the bread and `sum` is `2.5`.
5. `F5` once more finishes the program (`total: 5.75`) and the UI closes. `F7` reopens it to look back at the session.

Now try a condition: `<leader>b` to clear the plain breakpoint, then `<leader>B` on the same line and enter `it.Name == "bread"`. `F5`, **Debug**: it skips the tea and stops only once, on the bread, with `sum` already at `2.5`.

### Python

Open [examples/python/demo.py](examples/python/demo.py), put a breakpoint on line 8 (`result += price`) with `<leader>b`, then `F5` and pick **file**. Step through with `F2` and watch `result` change in the scopes pane.

The other entries in the list: **file:args** asks for command-line arguments, **attach** connects to a running debugpy, and **file:doctest** runs the file's doctests under the debugger.

Next: [Markdown](05-markdown.md).
