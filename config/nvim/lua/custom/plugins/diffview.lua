-- Side-by-side diffs. config/shell/zsh/git.zsh opens `git diff` and
-- `git show` here from a terminal.
local close = { 'n', 'q', '<cmd>DiffviewClose<CR>', { desc = 'Close diffview' } }

return {
  'sindrets/diffview.nvim',
  cmd = { 'DiffviewOpen', 'DiffviewFileHistory' },
  keys = {
    { '<leader>hv', '<cmd>DiffviewOpen<CR>', desc = 'git diff [v]iew' },
    { '<leader>hf', '<cmd>DiffviewFileHistory %<CR>', desc = 'git [f]ile history' },
  },
  -- git.zsh starts nvim with DF_DIFFVIEW set to diff or show; nvim then exists
  -- only for the view and quits with it.
  init = function()
    -- The index buffer (diffview:///repo/.git/:0:/file) is written through a
    -- BufWriteCmd yet left at buftype '', so vim.lsp.enable attaches servers to
    -- it and gopls/clangd reject the non-file URI. acwrite is the honest
    -- buftype for it, and LSP auto-attach skips acwrite buffers.
    vim.api.nvim_create_autocmd('BufFilePost', {
      pattern = 'diffview://*',
      callback = function(ev)
        if vim.bo[ev.buf].buftype == '' then
          vim.bo[ev.buf].buftype = 'acwrite'
        end
      end,
    })

    local kind = vim.env.DF_DIFFVIEW
    if not kind then
      return
    end
    local args = vim.split(vim.env.DF_DIFFVIEW_ARGS or '', '\n', { trimempty = true })
    vim.env.DF_DIFFVIEW, vim.env.DF_DIFFVIEW_ARGS = nil, nil
    vim.api.nvim_create_autocmd('VimEnter', {
      once = true,
      callback = function()
        vim.api.nvim_create_autocmd('User', { pattern = 'DiffviewViewClosed', once = true, command = 'qall' })
        if kind == 'show' then
          require('diffview').file_history(nil, args)
        else
          require('diffview').open(args)
        end
      end,
    })
  end,
  opts = {
    keymaps = {
      view = { close },
      file_panel = { close },
      file_history_panel = { close },
    },
  },
}
