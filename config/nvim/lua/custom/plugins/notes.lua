-- Obsidian-style vault in nvim: [[wikilink]] completion and following, backlinks,
-- tags, daily notes, image paste. The vault is vim.g.notes_dir ($NOTES_DIR,
-- default ~/notes), and the plugin loads only once that directory exists. Lint
-- and format-on-save treat it differently (init.lua, kickstart/plugins/lint.lua).
-- render-markdown draws the buffer, so this plugin's own UI layer is off.
local dir = vim.g.notes_dir

local function cmd(sub)
  return '<cmd>Obsidian ' .. sub .. '<CR>'
end

return {
  'obsidian-nvim/obsidian.nvim',
  version = '*',
  -- Without the vault, setup aborts and every <leader>n key is a stack trace.
  cond = vim.fn.isdirectory(dir) == 1,
  cmd = 'Obsidian',
  event = { 'BufReadPre ' .. dir .. '/*.md', 'BufNewFile ' .. dir .. '/*.md' },
  keys = {
    { '<leader>nn', cmd 'new', desc = '[N]otes: [N]ew' },
    { '<leader>nd', cmd 'today', desc = '[N]otes: to[D]ay' },
    { '<leader>nD', cmd 'dailies', desc = '[N]otes: [D]ailies' },
    { '<leader>nf', cmd 'quick_switch', desc = '[N]otes: [F]ind' },
    { '<leader>ns', cmd 'search', desc = '[N]otes: [S]earch text' },
    { '<leader>nt', cmd 'tags', desc = '[N]otes: [T]ags' },
    { '<leader>nb', cmd 'backlinks', desc = '[N]otes: [B]acklinks' },
    { '<leader>nl', cmd 'links', desc = '[N]otes: [L]inks in note' },
    { '<leader>nr', cmd 'rename', desc = '[N]otes: [R]ename, updating links' },
    { '<leader>np', cmd 'paste_img', desc = '[N]otes: [P]aste image' },
  },
  ---@module 'obsidian'
  ---@type obsidian.config
  opts = {
    legacy_commands = false,
    workspaces = { { name = 'notes', path = dir } },
    -- Name the file after the title, as the Obsidian app does, rather than the
    -- plugin's default random zettel ID.
    note_id_func = function(title)
      title = title and vim.trim((title:gsub('[\\/:*?"<>|#^%[%]]', ''))) or ''
      return title ~= '' and title or os.date '%Y-%m-%d %H%M%S'
    end,
    -- Otherwise every save writes id/aliases/tags frontmatter into the note,
    -- including notes that never had any.
    frontmatter = { enabled = false },
    picker = { name = 'telescope.nvim' },
    ui = { enable = false },
    daily_notes = { folder = 'daily', workdays_only = false },
    checkbox = { order = { ' ', 'x' } },
  },
}
