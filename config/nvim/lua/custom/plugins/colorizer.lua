-- catgoose's maintained fork; norcalli's original calls vim.tbl_flatten,
-- deprecated in Neovim 0.12. Highlights every filetype by default.
return {
  'catgoose/nvim-colorizer.lua',
  event = 'BufReadPre',
  config = function()
    -- The fork errors without truecolor, which nvim enables only on a capable
    -- terminal; over ssh that can arrive after startup, so check at load time.
    if vim.o.termguicolors then
      require('colorizer').setup()
    end
  end,
}
