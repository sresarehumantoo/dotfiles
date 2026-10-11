-- Nicer Markdown editing: in-buffer rendering of headings, code blocks, lists,
-- checkboxes, tables and callouts. Prose editing defaults (wrap, spell,
-- display-line motions) live in after/ftplugin/markdown.lua; formatting and
-- linting are wired up in init.lua (conform → prettier) and
-- kickstart/plugins/lint.lua (nvim-lint → markdownlint-cli2).
return {
  {
    'MeanderingProgrammer/render-markdown.nvim',
    ft = { 'markdown', 'markdown_inline' },
    dependencies = {
      'nvim-treesitter/nvim-treesitter', -- markdown/markdown_inline parsers (ensured in init.lua)
      'nvim-tree/nvim-web-devicons',
    },
    opts = {
      -- Keep the raw markup visible on the line the cursor is on so editing feels
      -- normal, while every other line renders.
      anti_conceal = { enabled = true },
      completions = { lsp = { enabled = true } },
    },
    config = function(_, opts)
      require('render-markdown').setup(opts)
      -- Flip between the rendered view and raw markup.
      vim.keymap.set('n', '<leader>tm', '<cmd>RenderMarkdown toggle<CR>', { desc = '[T]oggle [M]arkdown render' })
    end,
  },

  -- <CR>/o continue a list or checkbox and renumber ordered lists; <CR> on an
  -- empty item ends the list. <leader>x toggles the checkbox under the cursor.
  {
    'bullets-vim/bullets.vim',
    ft = 'markdown',
    init = function()
      vim.g.bullets_enabled_file_types = { 'markdown' }
      -- Obsidian only knows [ ] and [x]; the default ' .oOX' writes partial
      -- states like [o] into the parent of a half-done sublist.
      vim.g.bullets_checkbox_markers = ' x'
    end,
  },
}
