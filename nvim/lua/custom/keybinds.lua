local map = vim.keymap.set
local opts = { noremap = true, silent = true }

vim.keymap.set('n', '<Leader>d', '"_diw', opts)
vim.keymap.set('n', '<Leader>l', '"_dd', opts)
vim.keymap.set('v', '<Leader>l', '"_dd', opts)
vim.keymap.set('n', '<C-a>', '<Home>', opts)
vim.keymap.set('v', '<C-a>', '<Home>', opts)
vim.keymap.set('n', '<C-e>', '<End>', opts)
vim.keymap.set('v', '<C-e>', '<End>', opts)
vim.keymap.set('i', '<C-a>', '<Home>', opts)
vim.keymap.set('i', '<C-e>', '<End>', opts)
