return {
  'stevearc/oil.nvim',
  dependencies = { 'nvim-tree/nvim-web-devicons' },
  config = function()
    require('oil').setup {
      view_options = {
        show_hidden = true,
      },
    }
    vim.keymap.set('n', '-', '<cmd>Oil<CR>', { desc = 'Open parent directory (Oil)' })
    vim.keymap.set('n', '<leader>o', '<cmd>Oil<CR>', { desc = '[O]pen parent directory (Oil)' })
  end,
}
