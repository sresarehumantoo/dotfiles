-- Auto-loaded by kickstart's `{ import = 'custom.plugins' }` (lazy.nvim) when
-- the windev module (`dfinstall install windev`) symlinks this file into
-- ~/.config/nvim/lua/custom/plugins/. It wires up Windows cross-development
-- language support: C# LSP (OmniSharp) plus formatters, linters, and DAP
-- adapters for C/C++, C#, Go, and Rust.
--
-- clangd, gopls, and rust_analyzer are already enabled in init.lua's `servers`
-- table and attach automatically once their binaries (installed by the windev
-- module) are on PATH. This file deliberately mutates the live runtime tables
-- of conform / nvim-lint / nvim-dap after they load instead of re-running their
-- setup — that's the documented non-clobbering way to extend them.

local home = vim.fn.expand('$HOME')
local windev_dir = home .. '/.local/share/windev'

return {
  {
    -- Real plugin we genuinely want for C# (decompile-to-source go-to-def for
    -- external assemblies) — also serves as the anchor for the config block.
    'Hoffs/omnisharp-extended-lsp.nvim',

    -- Load conform/nvim-lint/nvim-dap/lspconfig before our config runs.
    dependencies = {
      'neovim/nvim-lspconfig',
      'stevearc/conform.nvim',
      'mfussenegger/nvim-lint',
      'mfussenegger/nvim-dap',
    },

    -- Defer load until a relevant filetype is opened — keeps startup fast.
    ft = { 'cs', 'c', 'cpp', 'go', 'rust' },

    config = function()
      -- ─── C# LSP (OmniSharp) ──────────────────────────────────────────
      local omnisharp_bin = windev_dir .. '/omnisharp/OmniSharp'
      local lsp_ok, lspconfig = pcall(require, 'lspconfig')
      if lsp_ok and vim.fn.executable(omnisharp_bin) == 1 then
        lspconfig.omnisharp.setup({
          cmd = { omnisharp_bin, '--languageserver', '--hostPID', tostring(vim.fn.getpid()) },
          handlers = {
            ['textDocument/definition'] = require('omnisharp_extended').handler,
          },
        })
      end

      -- ─── Formatters (conform reads formatters_by_ft at format time) ──
      local conform_ok, conform = pcall(require, 'conform')
      if conform_ok then
        conform.formatters_by_ft = vim.tbl_extend('force', conform.formatters_by_ft or {}, {
          c    = { 'clang-format' },
          cpp  = { 'clang-format' },
          cs   = { 'csharpier' },
          go   = { 'goimports', 'gofmt' },
          rust = { 'rustfmt' },
        })
      end

      -- ─── Linters (nvim-lint reads linters_by_ft at lint time) ────────
      local lint_ok, lint = pcall(require, 'lint')
      if lint_ok then
        lint.linters_by_ft = vim.tbl_extend('force', lint.linters_by_ft or {}, {
          c    = { 'cpplint' },
          cpp  = { 'cpplint' },
          go   = { 'golangci-lint' },
          rust = { 'clippy' },
        })
      end

      -- ─── DAP adapters + default launch configs ───────────────────────
      local dap_ok, dap = pcall(require, 'dap')
      if not dap_ok then
        return
      end

      -- codelldb (C/C++/Rust). Expected on PATH (apt lldb / mason / standalone
      -- download). Silently skipped if absent so a missing debugger never
      -- breaks LSP/format setup above.
      if vim.fn.executable('codelldb') == 1 then
        dap.adapters.codelldb = {
          type = 'server',
          port = '${port}',
          executable = { command = 'codelldb', args = { '--port', '${port}' } },
        }
        local pick_program = function()
          return vim.fn.input('Path to executable: ', vim.fn.getcwd() .. '/', 'file')
        end
        for _, lang in ipairs({ 'c', 'cpp', 'rust' }) do
          dap.configurations[lang] = dap.configurations[lang] or {}
          table.insert(dap.configurations[lang], {
            name = 'Launch (codelldb)',
            type = 'codelldb',
            request = 'launch',
            program = pick_program,
            cwd = '${workspaceFolder}',
            stopOnEntry = false,
          })
        end
      end

      -- netcoredbg (C#) — installed by the windev module into windev_dir.
      local netcoredbg_bin = windev_dir .. '/netcoredbg/netcoredbg'
      if vim.fn.executable(netcoredbg_bin) == 1 then
        dap.adapters.coreclr = {
          type = 'executable',
          command = netcoredbg_bin,
          args = { '--interpreter=vscode' },
        }
        dap.configurations.cs = {
          {
            type = 'coreclr',
            name = 'Launch .NET (netcoredbg)',
            request = 'launch',
            program = function()
              return vim.fn.input('Path to dll: ', vim.fn.getcwd() .. '/bin/Debug/', 'file')
            end,
          },
        }
      end
    end,
  },
}
