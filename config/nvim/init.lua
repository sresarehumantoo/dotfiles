vim.g.mapleader = ' '
vim.g.maplocalleader = ' '

vim.g.have_nerd_font = true

-- Disable netrw so neo-tree owns directory browsing (must be set before plugins load)
vim.g.loaded_netrw = 1
vim.g.loaded_netrwPlugin = 1

-- [[ Setting options ]]
vim.o.number = true
vim.o.relativenumber = true

vim.o.showmode = false

-- Pin clipboard provider to unix tools; nvim's auto-detect picks
-- clip.exe / win32yank.exe on WSL which forks a Windows process per yank
-- (~1–2s of latency). WSLg ships X11 + Wayland so xclip / wl-copy work.
if vim.env.WAYLAND_DISPLAY and vim.fn.executable('wl-copy') == 1 then
  vim.g.clipboard = {
    name = 'wl-clipboard',
    copy = {
      ['+'] = { 'wl-copy', '--type', 'text/plain' },
      ['*'] = { 'wl-copy', '--primary', '--type', 'text/plain' },
    },
    paste = {
      ['+'] = { 'wl-paste', '--no-newline' },
      ['*'] = { 'wl-paste', '--no-newline', '--primary' },
    },
    cache_enabled = 0,
  }
elseif vim.env.DISPLAY and vim.fn.executable('xclip') == 1 then
  vim.g.clipboard = {
    name = 'xclip',
    copy = {
      ['+'] = { 'xclip', '-selection', 'clipboard' },
      ['*'] = { 'xclip', '-selection', 'primary' },
    },
    paste = {
      ['+'] = { 'xclip', '-selection', 'clipboard', '-o' },
      ['*'] = { 'xclip', '-selection', 'primary', '-o' },
    },
    cache_enabled = 0,
  }
elseif vim.env.DISPLAY and vim.fn.executable('xsel') == 1 then
  vim.g.clipboard = {
    name = 'xsel',
    copy = {
      ['+'] = { 'xsel', '--clipboard', '--input' },
      ['*'] = { 'xsel', '--primary', '--input' },
    },
    paste = {
      ['+'] = { 'xsel', '--clipboard', '--output' },
      ['*'] = { 'xsel', '--primary', '--output' },
    },
    cache_enabled = 0,
  }
else
  -- OSC 52 fallback for SSH / headless: routes yank through terminal
  -- escape sequences. Tmux relays via 'set-clipboard on'. Paste is
  -- best-effort — most terminals refuse OSC 52 read for security.
  local osc52 = require('vim.ui.clipboard.osc52')
  vim.g.clipboard = {
    name = 'OSC 52',
    copy = { ['+'] = osc52.copy('+'), ['*'] = osc52.copy('*') },
    paste = { ['+'] = osc52.paste('+'), ['*'] = osc52.paste('*') },
  }
end

vim.schedule(function()
  vim.o.clipboard = 'unnamedplus'
end)

vim.o.breakindent = true

vim.o.undofile = true

vim.o.ignorecase = true
vim.o.smartcase = true

vim.o.signcolumn = 'yes'

vim.o.updatetime = 250

vim.o.timeoutlen = 300

vim.o.splitright = true
vim.o.splitbelow = true

vim.o.list = true
vim.opt.listchars = { tab = '» ', trail = '·', nbsp = '␣' }

vim.o.inccommand = 'split'

vim.o.cursorline = true

vim.o.scrolloff = 10

vim.o.confirm = true

-- [[ Basic Keymaps ]]
vim.keymap.set('n', '<Esc>', '<cmd>nohlsearch<CR>')

vim.keymap.set('n', '<leader>q', vim.diagnostic.setloclist, { desc = 'Open diagnostic [Q]uickfix list' })

vim.keymap.set('t', '<Esc><Esc>', '<C-\\><C-n>', { desc = 'Exit terminal mode' })

vim.keymap.set('n', '<C-h>', '<C-w><C-h>', { desc = 'Move focus to the left window' })
vim.keymap.set('n', '<C-l>', '<C-w><C-l>', { desc = 'Move focus to the right window' })
vim.keymap.set('n', '<C-j>', '<C-w><C-j>', { desc = 'Move focus to the lower window' })
vim.keymap.set('n', '<C-k>', '<C-w><C-k>', { desc = 'Move focus to the upper window' })

-- [[ Basic Autocommands ]]
vim.api.nvim_create_autocmd('TextYankPost', {
  desc = 'Highlight when yanking (copying) text',
  group = vim.api.nvim_create_augroup('kickstart-highlight-yank', { clear = true }),
  callback = function()
    vim.hl.on_yank()
  end,
})

-- [[ Tmux status bar: collapse fancy gap while in nvim ]]
if vim.env.TMUX then
  local tmux_group = vim.api.nvim_create_augroup('tmux-status-toggle', { clear = true })

  local saved_bar = nil

  local function tmux_single_bar()
    saved_bar = vim.fn.system({ 'tmux', 'show', '-gv', 'status-format[1]' }):gsub('\n$', '')
    vim.fn.system({ 'tmux', 'set', 'status-format[0]', saved_bar })
    vim.fn.system({ 'tmux', 'set', 'status', 'on' })
  end

  local function tmux_double_bar()
    local bar = saved_bar or vim.fn.system({ 'tmux', 'show', '-gv', 'status-format[1]' }):gsub('\n$', '')
    vim.fn.system({ 'tmux', 'set', 'status-format[0]', '#[bg=terminal,fill=terminal] ' })
    vim.fn.system({ 'tmux', 'set', 'status-format[1]', bar })
    vim.fn.system({ 'tmux', 'set', 'status', '2' })
  end

  vim.api.nvim_create_autocmd({ 'VimEnter', 'VimResume' }, {
    group = tmux_group,
    callback = tmux_single_bar,
  })
  vim.api.nvim_create_autocmd({ 'VimLeave', 'VimSuspend' }, {
    group = tmux_group,
    callback = tmux_double_bar,
  })
end

-- [[ Install `lazy.nvim` plugin manager ]]
local lazypath = vim.fn.stdpath 'data' .. '/lazy/lazy.nvim'
if not (vim.uv or vim.loop).fs_stat(lazypath) then
  local lazyrepo = 'https://github.com/folke/lazy.nvim.git'
  local out = vim.fn.system { 'git', 'clone', '--filter=blob:none', '--branch=stable', lazyrepo, lazypath }
  if vim.v.shell_error ~= 0 then
    error('Error cloning lazy.nvim:\n' .. out)
  end
end

---@type vim.Option
local rtp = vim.opt.rtp
rtp:prepend(lazypath)

-- [[ Configure and install plugins ]]
require('lazy').setup({
  'NMAC427/guess-indent.nvim',

  { -- Gitsigns
    'lewis6991/gitsigns.nvim',
    opts = {
      signs = {
        add = { text = '+' },
        change = { text = '~' },
        delete = { text = '_' },
        topdelete = { text = '‾' },
        changedelete = { text = '~' },
      },
    },
  },

  { -- Telescope
    'nvim-telescope/telescope.nvim',
    event = 'VimEnter',
    dependencies = {
      'nvim-lua/plenary.nvim',
      {
        'nvim-telescope/telescope-fzf-native.nvim',
        build = 'make',
        cond = function()
          return vim.fn.executable 'make' == 1
        end,
      },
      { 'nvim-telescope/telescope-ui-select.nvim' },
      { 'nvim-tree/nvim-web-devicons', enabled = vim.g.have_nerd_font },
    },
    config = function()
      require('telescope').setup {
        extensions = {
          ['ui-select'] = {
            require('telescope.themes').get_dropdown(),
          },
        },
      }

      pcall(require('telescope').load_extension, 'fzf')
      pcall(require('telescope').load_extension, 'ui-select')

      local builtin = require 'telescope.builtin'
      vim.keymap.set('n', '<leader>sh', builtin.help_tags, { desc = '[S]earch [H]elp' })
      vim.keymap.set('n', '<leader>sk', builtin.keymaps, { desc = '[S]earch [K]eymaps' })
      vim.keymap.set('n', '<leader>sf', builtin.find_files, { desc = '[S]earch [F]iles' })
      vim.keymap.set('n', '<leader>ss', builtin.builtin, { desc = '[S]earch [S]elect Telescope' })
      vim.keymap.set('n', '<leader>sw', builtin.grep_string, { desc = '[S]earch current [W]ord' })
      vim.keymap.set('n', '<leader>sg', builtin.live_grep, { desc = '[S]earch by [G]rep' })
      vim.keymap.set('n', '<leader>sd', builtin.diagnostics, { desc = '[S]earch [D]iagnostics' })
      vim.keymap.set('n', '<leader>sr', builtin.resume, { desc = '[S]earch [R]esume' })
      vim.keymap.set('n', '<leader>s.', builtin.oldfiles, { desc = '[S]earch Recent Files ("." for repeat)' })
      vim.keymap.set('n', '<leader><leader>', builtin.buffers, { desc = '[ ] Find existing buffers' })

      vim.keymap.set('n', '<leader>/', function()
        builtin.current_buffer_fuzzy_find(require('telescope.themes').get_dropdown {
          winblend = 10,
          previewer = false,
        })
      end, { desc = '[/] Fuzzily search in current buffer' })

      vim.keymap.set('n', '<leader>s/', function()
        builtin.live_grep {
          grep_open_files = true,
          prompt_title = 'Live Grep in Open Files',
        }
      end, { desc = '[S]earch [/] in Open Files' })

      vim.keymap.set('n', '<leader>sn', function()
        builtin.find_files { cwd = vim.fn.stdpath 'config' }
      end, { desc = '[S]earch [N]eovim files' })
    end,
  },

  -- LSP Plugins
  {
    'folke/lazydev.nvim',
    ft = 'lua',
    opts = {
      library = {
        { path = '${3rd}/luv/library', words = { 'vim%.uv' } },
      },
    },
  },
  {
    'neovim/nvim-lspconfig',
    dependencies = {
      { 'mason-org/mason.nvim', opts = {} },
      'mason-org/mason-lspconfig.nvim',
      'WhoIsSethDaniel/mason-tool-installer.nvim',
      { 'j-hui/fidget.nvim', opts = {} },
      'saghen/blink.cmp',
    },
    config = function()
      vim.api.nvim_create_autocmd('LspAttach', {
        group = vim.api.nvim_create_augroup('kickstart-lsp-attach', { clear = true }),
        callback = function(event)
          local map = function(keys, func, desc, mode)
            mode = mode or 'n'
            vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = 'LSP: ' .. desc })
          end

          map('grn', vim.lsp.buf.rename, '[R]e[n]ame')
          map('gra', vim.lsp.buf.code_action, '[G]oto Code [A]ction', { 'n', 'x' })
          map('grr', require('telescope.builtin').lsp_references, '[G]oto [R]eferences')
          map('gri', require('telescope.builtin').lsp_implementations, '[G]oto [I]mplementation')
          map('grd', require('telescope.builtin').lsp_definitions, '[G]oto [D]efinition')
          map('grD', vim.lsp.buf.declaration, '[G]oto [D]eclaration')
          map('gO', require('telescope.builtin').lsp_document_symbols, 'Open Document Symbols')
          map('gW', require('telescope.builtin').lsp_dynamic_workspace_symbols, 'Open Workspace Symbols')
          map('grt', require('telescope.builtin').lsp_type_definitions, '[G]oto [T]ype Definition')

          ---@param client vim.lsp.Client
          ---@param method vim.lsp.protocol.Method
          ---@param bufnr? integer some lsp support methods only in specific files
          ---@return boolean
          local function client_supports_method(client, method, bufnr)
            if vim.fn.has 'nvim-0.11' == 1 then
              return client:supports_method(method, bufnr)
            else
              return client.supports_method(method, { bufnr = bufnr })
            end
          end

          local client = vim.lsp.get_client_by_id(event.data.client_id)
          if client and client_supports_method(client, vim.lsp.protocol.Methods.textDocument_documentHighlight, event.buf) then
            local highlight_augroup = vim.api.nvim_create_augroup('kickstart-lsp-highlight', { clear = false })
            vim.api.nvim_create_autocmd({ 'CursorHold', 'CursorHoldI' }, {
              buffer = event.buf,
              group = highlight_augroup,
              callback = vim.lsp.buf.document_highlight,
            })

            vim.api.nvim_create_autocmd({ 'CursorMoved', 'CursorMovedI' }, {
              buffer = event.buf,
              group = highlight_augroup,
              callback = vim.lsp.buf.clear_references,
            })

            vim.api.nvim_create_autocmd('LspDetach', {
              group = vim.api.nvim_create_augroup('kickstart-lsp-detach', { clear = true }),
              callback = function(event2)
                vim.lsp.buf.clear_references()
                vim.api.nvim_clear_autocmds { group = 'kickstart-lsp-highlight', buffer = event2.buf }
              end,
            })
          end

          if client and client_supports_method(client, vim.lsp.protocol.Methods.textDocument_inlayHint, event.buf) then
            map('<leader>th', function()
              vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled { bufnr = event.buf })
            end, '[T]oggle Inlay [H]ints')
          end
        end,
      })

      -- Diagnostic Config

      -- Native `virtual_lines` renders with virt_lines_overflow = 'scroll', so a
      -- long message (e.g. a pyright/ruff error) is CLIPPED at the window's right
      -- edge — painful in a narrow split. The renderer does, however, split the
      -- formatted message on '\n' into one virtual line each. So we word-wrap the
      -- message to the usable width here; each wrapped row becomes its own
      -- (fitting) virtual line. Budget for the gutter + the '└──── ' connector
      -- indent, which grows with the diagnostic's column.
      local function wrap_diagnostic(diagnostic)
        local msg = diagnostic.code and string.format('%s: %s', diagnostic.code, diagnostic.message) or diagnostic.message
        local win_width = vim.api.nvim_win_get_width(0)
        local indent = (diagnostic.col or 0)
        local width = math.max(30, win_width - indent - 12)

        local rows, line = {}, ''
        local function push(word)
          if line == '' then
            line = word
          elseif #line + 1 + #word <= width then
            line = line .. ' ' .. word
          else
            rows[#rows + 1] = line
            line = word
          end
        end
        for word in msg:gmatch '%S+' do
          -- Hard-break a single token longer than the wrap width (e.g. a path).
          while #word > width do
            push(word:sub(1, width))
            word = word:sub(width + 1)
          end
          push(word)
        end
        if line ~= '' then
          rows[#rows + 1] = line
        end
        return table.concat(rows, '\n')
      end

      vim.diagnostic.config {
        severity_sort = true,
        -- Wrap and cap the hover/jump float so long messages stay on screen.
        float = { border = 'rounded', source = 'if_many', max_width = 80, wrap = true },
        underline = { severity = vim.diagnostic.severity.ERROR },
        signs = vim.g.have_nerd_font and {
          text = {
            [vim.diagnostic.severity.ERROR] = '󰅚 ',
            [vim.diagnostic.severity.WARN] = '󰀪 ',
            [vim.diagnostic.severity.INFO] = '󰋽 ',
            [vim.diagnostic.severity.HINT] = '󰌶 ',
          },
        } or {},
        -- Full message, wrapped on its own line(s) under the cursor's line — never
        -- clipped. Every offending line still gets a gutter sign for scanning.
        virtual_text = false,
        virtual_lines = { current_line = true, format = wrap_diagnostic },
      }

      -- Toggle: expand diagnostics inline under *every* line (virtual_lines for
      -- all) vs. just the cursor's line. Handy when scanning a whole file.
      vim.keymap.set('n', '<leader>td', function()
        local cfg = vim.diagnostic.config()
        local all_lines = type(cfg.virtual_lines) == 'table' and cfg.virtual_lines.current_line == nil
        vim.diagnostic.config {
          virtual_lines = all_lines and { current_line = true, format = wrap_diagnostic } or { format = wrap_diagnostic },
        }
      end, { desc = '[T]oggle [D]iagnostic virtual lines (all/current)' })

      -- Open the full diagnostic for the current line in a wrapped float.
      vim.keymap.set('n', '<leader>e', vim.diagnostic.open_float, { desc = 'Show diagnostic [E]rror float' })

      local capabilities = require('blink.cmp').get_lsp_capabilities()

      local servers = {
        -- --query-driver lets clangd run the mingw-w64 cross compilers to
        -- harvest their target + system includes, so <windows.h> and the Win32
        -- API resolve when cross-developing Windows C/C++ from Linux (see
        -- projects with a compile_commands.json / .clangd naming the driver).
        clangd = {
          cmd = { 'clangd', '--query-driver=/usr/bin/*-w64-mingw32-*' },
        },
        gopls = {},
        -- Python: pyright owns types/hover/completion; ruff owns
        -- linting, import sorting and code actions (one fast binary).
        pyright = {
          settings = {
            pyright = {
              -- ruff organizes imports, so silence pyright's overlapping action.
              disableOrganizeImports = true,
            },
          },
        },
        ruff = {
          -- Defer hover to pyright so a single source answers `K`.
          on_attach = function(client)
            client.server_capabilities.hoverProvider = false
          end,
        },
        rust_analyzer = {},
        lua_ls = {
          settings = {
            Lua = {
              completion = {
                callSnippet = 'Replace',
              },
            },
          },
        },
      }

      local ensure_installed = vim.tbl_keys(servers or {})
      vim.list_extend(ensure_installed, {
        'stylua',
        'markdownlint-cli2', -- markdown linter (nvim-lint, see kickstart/plugins/lint.lua)
        'prettierd', -- markdown formatter daemon (conform) — avoids node cold-start
        'prettier', -- fallback if the prettierd daemon isn't available
      })
      require('mason-tool-installer').setup { ensure_installed = ensure_installed }

      require('mason-lspconfig').setup {
        ensure_installed = {},
        automatic_installation = false,
        handlers = {
          function(server_name)
            local server = servers[server_name] or {}
            server.capabilities = vim.tbl_deep_extend('force', {}, capabilities, server.capabilities or {})
            require('lspconfig')[server_name].setup(server)
          end,
        },
      }
    end,
  },

  { -- Autoformat
    'stevearc/conform.nvim',
    event = { 'BufWritePre' },
    cmd = { 'ConformInfo' },
    keys = {
      {
        '<leader>f',
        function()
          require('conform').format { async = true, lsp_format = 'fallback' }
        end,
        mode = '',
        desc = '[F]ormat buffer',
      },
    },
    opts = {
      notify_on_error = false,
      format_on_save = function(bufnr)
        local disable_filetypes = { c = true, cpp = true }
        if disable_filetypes[vim.bo[bufnr].filetype] then
          return nil
        else
          -- Generous timeout: a bare `prettier` is a cold Node process (~600ms
          -- here, already over the old 500ms) and the first `prettierd` call
          -- pays a one-time daemon spawn. 500ms made every markdown save fail
          -- with "Formatter 'prettier' timeout".
          return {
            timeout_ms = 3000,
            lsp_format = 'fallback',
          }
        end
      end,
      formatters_by_ft = {
        lua = { 'stylua' },
        -- Organize imports first, then format — both run by ruff on save.
        python = { 'ruff_organize_imports', 'ruff_format' },
        -- Prefer the persistent daemon; fall back to one-shot prettier.
        markdown = { 'prettierd', 'prettier', stop_after_first = true },
      },
    },
  },

  { -- Autocompletion
    'saghen/blink.cmp',
    event = 'VimEnter',
    version = '1.*',
    dependencies = {
      {
        'L3MON4D3/LuaSnip',
        version = '2.*',
        build = (function()
          if vim.fn.has 'win32' == 1 or vim.fn.executable 'make' == 0 then
            return
          end
          return 'make install_jsregexp'
        end)(),
        dependencies = {},
        opts = {},
      },
      'folke/lazydev.nvim',
      -- Spelling suggestions + dictionary word completion as a completion
      -- source. Only active where 'spell' is set (markdown, see
      -- after/ftplugin/markdown.lua), so it stays out of the way in code.
      'ribru17/blink-cmp-spell',
    },
    --- @module 'blink.cmp'
    --- @type blink.cmp.Config
    opts = {
      keymap = {
        preset = 'default',
        ['<Tab>'] = { 'select_next', 'fallback' },
        ['<S-Tab>'] = { 'select_prev', 'fallback' },
        ['<C-e>'] = { 'select_and_accept', 'fallback' },
        ['<C-s>'] = { 'select_accept_and_enter', 'fallback' },
        ['<Up>'] = {},
        ['<Down>'] = {},
      },

      appearance = {
        nerd_font_variant = 'mono',
      },

      completion = {
        documentation = { auto_show = false, auto_show_delay_ms = 500 },
      },

      sources = {
        default = { 'lsp', 'path', 'snippets', 'lazydev', 'buffer' },
        -- Markdown (and any 'spell' buffer) additionally gets word/spelling
        -- suggestions. The 'spell' provider self-disables where spell is off,
        -- so this only fires for prose. 'buffer' gives plain word completion
        -- from open buffers.
        per_filetype = {
          markdown = { 'spell', 'lsp', 'path', 'snippets', 'buffer' },
        },
        providers = {
          lazydev = { module = 'lazydev.integrations.blink', score_offset = 100 },
          spell = {
            name = 'Spell',
            module = 'blink-cmp-spell',
            -- Suggestion-based, never auto-correcting: entries only appear in
            -- the completion menu for you to pick. Native `z=` still works too.
            enabled = function()
              return vim.opt_local.spell:get()
            end,
            opts = {
              max_entries = 5,
              preselect_current_word = true,
            },
          },
        },
      },

      snippets = { preset = 'luasnip' },

      -- Sort spelling suggestions alphabetically among themselves (they have no
      -- meaningful fuzzy score), everything else by score as usual.
      fuzzy = {
        implementation = 'lua',
        sorts = {
          function(a, b)
            local sort = require 'blink.cmp.fuzzy.sort'
            if a.source_id == 'spell' and b.source_id == 'spell' then
              return sort.label(a, b)
            end
          end,
          'score',
          'sort_text',
          'kind',
          'label',
        },
      },

      signature = { enabled = true },
    },
  },

  { -- Colorscheme
    'folke/tokyonight.nvim',
    priority = 1000,
    config = function()
      ---@diagnostic disable-next-line: missing-fields
      require('tokyonight').setup {
        styles = {
          comments = { italic = false },
        },
      }
      vim.cmd.colorscheme 'tokyonight-night'
    end,
  },

  { 'folke/todo-comments.nvim', event = 'VimEnter', dependencies = { 'nvim-lua/plenary.nvim' }, opts = { signs = false } },

  { -- Mini.nvim
    'echasnovski/mini.nvim',
    config = function()
      require('mini.ai').setup { n_lines = 500 }
      require('mini.surround').setup()

      local statusline = require 'mini.statusline'
      statusline.setup { use_icons = vim.g.have_nerd_font }

      ---@diagnostic disable-next-line: duplicate-set-field
      statusline.section_location = function()
        return '%2l:%-2v'
      end
    end,
  },
  { -- Treesitter
    'nvim-treesitter/nvim-treesitter',
    build = ':TSUpdate',
    config = function()
      require('nvim-treesitter').setup()
      vim.treesitter.language.register('markdown', 'markdown_inline')

      local ensure = { 'bash', 'c', 'diff', 'html', 'lua', 'luadoc', 'markdown', 'markdown_inline', 'python', 'query', 'vim', 'vimdoc' }
      for _, lang in ipairs(ensure) do
        pcall(function() vim.treesitter.language.add(lang) end)
      end

      vim.api.nvim_create_autocmd('FileType', {
        callback = function(args)
          pcall(vim.treesitter.start, args.buf)
        end,
      })
    end,
  },

  require 'kickstart.plugins.indent_line',
  require 'kickstart.plugins.lint',
  require 'kickstart.plugins.autopairs',
  require 'kickstart.plugins.neo-tree',
  require 'kickstart.plugins.gitsigns',

  { import = 'custom.plugins' },
}, {
  ui = {
    icons = vim.g.have_nerd_font and {} or {
      cmd = '⌘',
      config = '🛠',
      event = '📅',
      ft = '📂',
      init = '⚙',
      keys = '🗝',
      plugin = '🔌',
      runtime = '💻',
      require = '🌙',
      source = '📄',
      start = '🚀',
      task = '📌',
      lazy = '💤 ',
    },
  },
})

-- vim: ts=2 sts=2 sw=2 et

require 'custom.keybinds'
