-- Neo-tree is a Neovim plugin to browse the file system
-- https://github.com/nvim-neo-tree/neo-tree.nvim

return {
  'nvim-neo-tree/neo-tree.nvim',
  version = '3.*',
  dependencies = {
    'nvim-lua/plenary.nvim',
    'nvim-tree/nvim-web-devicons',
    'MunifTanjim/nui.nvim',
  },
  lazy = false,
  init = function()
    -- When nvim is opened with a directory, wipe the dir buffer and open neo-tree
    vim.api.nvim_create_autocmd('VimEnter', {
      callback = function(data)
        if vim.fn.isdirectory(data.file) ~= 1 then
          return
        end
        vim.cmd.cd(data.file)
        vim.cmd.enew()
        vim.cmd.bw(data.buf)
        local ok, cmd = pcall(require, 'neo-tree.command')
        if ok then
          cmd.execute { toggle = false, dir = vim.uv.cwd() }
        end
      end,
    })
  end,
  keys = {
    { '\\', ':Neotree toggle<CR>', desc = 'NeoTree toggle', silent = true },
  },
  opts = {
    open_files_do_not_replace_types = { 'terminal', 'Trouble', 'qf' },
    enable_git_status = true,
    enable_diagnostics = true,
    window = {
      position = 'left',
      width = 30,
    },
    filesystem = {
      filtered_items = {
        hide_dotfiles = false,
        hide_gitignored = false,
      },
      follow_current_file = {
        enabled = true,
      },
      window = {
        mappings = {
          ['\\'] = 'close_window',
        },
      },
    },
  },
}
