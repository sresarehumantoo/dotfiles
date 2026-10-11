# 3. Coding

<!-- markdownlint-disable MD013 -->

Back to the [index](README.md). This chapter uses two practice files with deliberate problems in them:

- [examples/python/demo.py](examples/python/demo.py)
- [examples/go/main.go](examples/go/main.go)

Open one with `gf` on its path above, and give the language server a few seconds to start the first time (the bottom-right corner shows its progress). Each has its own `pyproject.toml` / `go.mod`, so the servers treat them as small projects of their own.

## Who does what

| Language | Language server(s)                                | On save                        | Extra linter         |
| -------- | ------------------------------------------------- | ------------------------------ | -------------------- |
| Python   | pyright (types, hover, go-to), ruff (lint, fixes) | ruff: organize imports, format | (ruff is the linter) |
| Go       | gopls, with staticcheck and inlay hints           | goimports, gofumpt             | golangci-lint        |
| Lua      | lua_ls                                            | stylua                         |                      |
| Rust, C  | rust-analyzer, clangd                             | LSP formatting (not for C/C++) |                      |

pyright uses the project's `.venv` automatically when there is one, so imports from packages installed there resolve without activating anything.

## Reading diagnostics

Problems show up three ways:

- **A sign in the gutter** on every line with a problem.
- **The full message under the cursor's line**, wrapped to fit the window. Move onto line 18 of `demo.py` to see pyright's type error appear underneath, and off it again to see it fold away.
- **On demand**: `<leader>e` opens the line's diagnostics in a float.

| Keys         | Does                                                                            |
| ------------ | ------------------------------------------------------------------------------- |
| `]d` / `[d`  | next / previous diagnostic                                                      |
| `<leader>e`  | the line's diagnostics in a float                                               |
| `<leader>td` | show the messages under **every** line, not just the cursor's; again to go back |
| `<leader>q`  | every diagnostic in the file, in the location list                              |
| `<leader>sd` | every diagnostic in a Telescope picker                                          |

## Fixing things: code actions

`gra` lists the fixes the servers offer for the cursor's line. In `demo.py`:

1. On `import os` (line 2), `gra` offers ruff's **Remove unused import**. Pick it and the warning disappears.
2. On line 18, pyright complains that a `float` is passed where `describe` wants an `int`. There's no automatic fix: change `count: int` to `count: float` on line 12 and the error on line 18 clears.

In `main.go`:

1. On line 31, staticcheck flags `strings.Index(...) != -1` (S1003). `gra`, then **Simplify use of strings.Index**, rewrites it to `strings.Contains`.
2. On line 21, `verbose` is never used. `gra` offers **Remove unused parameter**, which also updates the call in `main`.

## Navigating code

| Keys                | Goes to / does                               |
| ------------------- | -------------------------------------------- |
| `K`                 | hover: type and documentation for the symbol |
| `grd`               | definition                                   |
| `grr`               | references (in Telescope)                    |
| `gri`               | implementations                              |
| `grt`               | type definition                              |
| `grD`               | declaration                                  |
| `gO`                | symbols in this file                         |
| `gW`                | symbols across the workspace                 |
| `grn`               | rename the symbol everywhere                 |
| `Ctrl-o` / `Ctrl-i` | back / forward through where you've jumped   |

Try in `main.go`: `K` on `Item`, `grd` on the `total(items)` call at the bottom, then `Ctrl-o` back. `grn` on `total` and type a new name: the definition and the call both change. `gO` and type `label` to jump to that function.

## Inlay hints

`<leader>th` toggles inline type and parameter-name hints, where the server supports them. In `main.go` they show the inferred type of `sum := 0.0` and the parameter names at each call. pyright doesn't send them.

## Completion

The menu appears as you type, drawing on the language server, file paths, snippets and words in open buffers.

| Keys                | Does                                             |
| ------------------- | ------------------------------------------------ |
| `Tab` / `Shift-Tab` | next / previous item                             |
| `Ctrl-e`            | accept the selected (or first) item              |
| `Ctrl-y`            | accept                                           |
| `Ctrl-s`            | accept and start a new line                      |
| `Ctrl-Space`        | open the menu, or the docs for the selected item |

Arrow keys don't drive the menu, on purpose. When you're typing a call's arguments, a signature popup shows which parameter you're on.

Try: on a new line inside `main` in `main.go`, type `strings.Rep` and accept `Repeat`.

## Formatting

Saving formats the file with the tools in the table at the top; `<leader>f` formats without saving. Try it on a scratch file rather than the practice files, so the repo stays clean: `:e /tmp/scratch.go`, type some badly indented Go, `<leader>f`.

When you're done with the practice files, `:e!` discards your edits (or `git checkout -- docs/nvim-tutorial/examples` if you saved).

Next: [Git and debugging](04-git-and-debugging.md).
