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

-- Buffer navigation
map('n', '<leader>bn', '<cmd>bnext<CR>', { desc = '[B]uffer [N]ext' })
map('n', '<leader>bp', '<cmd>bprevious<CR>', { desc = '[B]uffer [P]revious' })
map('n', '<leader>bw', '<cmd>bdelete<CR>', { desc = '[B]uffer Close' })

-- Splits
map('n', '<leader>|', '<cmd>vsplit<CR>', { desc = 'Vertical split' })
map('n', '<leader>-', '<cmd>split<CR>', { desc = 'Horizontal split' })

-- Visual reindent (stay in visual mode)
map('v', '>', '>gv', { desc = 'Indent and reselect' })
map('v', '<', '<gv', { desc = 'Dedent and reselect' })

-- Line movement (Alt-j/k)
map('n', '<A-j>', '<cmd>m .+1<CR>==', { desc = 'Move line down' })
map('n', '<A-k>', '<cmd>m .-2<CR>==', { desc = 'Move line up' })
map('v', '<A-j>', ":m '>+1<CR>gv=gv", { desc = 'Move selection down' })
map('v', '<A-k>', ":m '<-2<CR>gv=gv", { desc = 'Move selection up' })

-- Quick save
map('n', '<C-s>', '<cmd>w<CR>', { desc = 'Save file' })
