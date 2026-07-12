-- Prose-friendly defaults for Markdown buffers. Native ftplugin — sourced by
-- Neovim for every markdown buffer, no plugin required. In-buffer rendering is
-- handled by render-markdown.nvim (lua/custom/plugins/markdown.lua).

-- Soft-wrap long prose at word boundaries instead of mid-word, and keep wrapped
-- lines visually indented under their start.
vim.opt_local.wrap = true
vim.opt_local.linebreak = true
vim.opt_local.breakindent = true

-- Spell-check prose. Misspellings are only highlighted — nothing is corrected
-- automatically. Suggestions come from `z=` (or the blink completion menu,
-- which gains a spell source in spell buffers; see init.lua). Toggle per-buffer
-- with `:setlocal nospell` if it's noisy.
vim.opt_local.spell = true
vim.opt_local.spelllang = 'en_us'
-- Treat CamelCase segments as separate words rather than flagging the whole run.
vim.opt_local.spelloptions = 'camel'

-- Suggestion-based spelling helpers (never auto-fix):
--   z=  open the full suggestion list for the word under the cursor (native)
--   zg  add the word to the dictionary   |  zw  mark it as wrong
vim.keymap.set('n', '<leader>zs', function()
  -- Jump to the previous misspelling, then open suggestions for it.
  vim.cmd 'normal! [s'
  vim.cmd 'normal! z='
end, { buffer = true, desc = '[Z] [S]pell: suggest for previous misspelling' })

-- Wrapped prose spans several screen rows per logical line; move by what you see.
-- A count (e.g. 5j) still jumps by logical lines so relativenumber stays useful.
vim.keymap.set('n', 'j', "v:count == 0 ? 'gj' : 'j'", { buffer = true, expr = true, desc = 'Down by display line' })
vim.keymap.set('n', 'k', "v:count == 0 ? 'gk' : 'k'", { buffer = true, expr = true, desc = 'Up by display line' })
