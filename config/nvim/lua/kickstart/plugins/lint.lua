return {

  { -- Linting
    'mfussenegger/nvim-lint',
    event = { 'BufReadPre', 'BufNewFile' },
    config = function()
      local lint = require 'lint'
      lint.linters_by_ft = {
        markdown = { 'markdownlint-cli2' },
        go = { 'golangcilint' },
      }

      -- Notes in the vault are prose, not published docs: lint them only for
      -- markup that renders wrong. markdownlint-cli2 reads stdin here, so it
      -- would otherwise only find a config in Neovim's cwd.
      lint.linters['markdownlint-notes'] = vim.tbl_extend('force', lint.linters['markdownlint-cli2'], {
        args = { '--config', vim.fn.stdpath 'config' .. '/notes.markdownlint.yaml', '-' },
      })

      -- Style lint is reference material, not something to read while typing:
      -- a sign in the gutter, details on demand with <leader>e. The global
      -- virtual_lines would otherwise unfold every finding under the cursor
      -- line and shove the text down on each j/k.
      for _, name in ipairs { 'markdownlint-cli2', 'markdownlint-notes' } do
        vim.diagnostic.config({ virtual_lines = false, virtual_text = false, underline = false }, lint.get_namespace(name))
      end

      -- To allow other plugins to add linters to require('lint').linters_by_ft,
      -- instead set linters_by_ft like this:
      -- lint.linters_by_ft = lint.linters_by_ft or {}
      -- lint.linters_by_ft['markdown'] = { 'markdownlint' }
      --
      -- However, note that this will enable a set of default linters,
      -- which will cause errors unless these tools are available:
      -- {
      --   clojure = { "clj-kondo" },
      --   dockerfile = { "hadolint" },
      --   inko = { "inko" },
      --   janet = { "janet" },
      --   json = { "jsonlint" },
      --   markdown = { "vale" },
      --   rst = { "vale" },
      --   ruby = { "ruby" },
      --   terraform = { "tflint" },
      --   text = { "vale" }
      -- }
      --
      -- You can disable the default linters by setting their filetypes to nil:
      -- lint.linters_by_ft['clojure'] = nil
      -- lint.linters_by_ft['dockerfile'] = nil
      -- lint.linters_by_ft['inko'] = nil
      -- lint.linters_by_ft['janet'] = nil
      -- lint.linters_by_ft['json'] = nil
      -- lint.linters_by_ft['markdown'] = nil
      -- lint.linters_by_ft['rst'] = nil
      -- lint.linters_by_ft['ruby'] = nil
      -- lint.linters_by_ft['terraform'] = nil
      -- lint.linters_by_ft['text'] = nil

      -- Create autocommand which carries out the actual linting
      -- on the specified events.
      local lint_augroup = vim.api.nvim_create_augroup('lint', { clear = true })
      vim.api.nvim_create_autocmd({ 'BufEnter', 'BufWritePost', 'InsertLeave' }, {
        group = lint_augroup,
        callback = function()
          -- Only run the linter in buffers that you can modify in order to
          -- avoid superfluous noise, notably within the handy LSP pop-ups that
          -- describe the hovered symbol using Markdown.
          if not vim.bo.modifiable then
            return
          end

          -- Only run linters whose executable is actually on PATH. Otherwise a
          -- missing tool (e.g. markdownlint-cli2 before Mason finishes
          -- installing it, or on a machine without Node) makes try_lint spawn a
          -- nonexistent binary and spam "ENOENT: no such file or directory" on
          -- every BufEnter.
          local names = lint.linters_by_ft[vim.bo.filetype] or {}
          local name = vim.api.nvim_buf_get_name(0)
          local in_vault = vim.iter(vim.g.notes_dirs):any(function(dir)
            return vim.startswith(name, dir .. '/')
          end)
          if vim.bo.filetype == 'markdown' and in_vault then
            names = { 'markdownlint-notes' }
          end
          local runnable = {}
          for _, name in ipairs(names) do
            local linter = lint.linters[name]
            local cmd = type(linter) == 'table' and linter.cmd or nil
            if type(cmd) == 'function' then
              cmd = cmd()
            end
            if cmd and vim.fn.executable(cmd) == 1 then
              table.insert(runnable, name)
            end
          end

          if #runnable > 0 then
            lint.try_lint(runnable)
          end
        end,
      })
    end,
  },
}
