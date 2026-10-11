-- Obsidian-style vault in nvim: [[wikilink]] completion and following, backlinks,
-- tags, daily notes, image paste. The vaults are vim.g.notes_dirs ($NOTES_DIR,
-- default ~/notes, plus the tutorial's sample vault); one that doesn't exist is
-- skipped, and the plugin loads only once one does. Lint and format-on-save
-- treat them differently (init.lua, kickstart/plugins/lint.lua).
-- render-markdown draws the buffer, so this plugin's own UI layer is off.
local dirs = vim.tbl_filter(function(dir)
  return vim.fn.isdirectory(dir) == 1
end, vim.g.notes_dirs)

local function cmd(sub)
  return '<cmd>Obsidian ' .. sub .. '<CR>'
end

return {
  'obsidian-nvim/obsidian.nvim',
  version = '*',
  -- With no vault, setup aborts and every <leader>n key is a stack trace.
  cond = #dirs > 0,
  cmd = 'Obsidian',
  event = {
    {
      event = { 'BufReadPre', 'BufNewFile' },
      pattern = vim.tbl_map(function(dir)
        return dir .. '/*.md'
      end, dirs),
    },
  },
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
    workspaces = vim.tbl_map(function(dir)
      return { name = vim.fs.basename(dir), path = dir }
    end, dirs),
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
  config = function(_, opts)
    require('obsidian').setup(opts)
    -- The plugin switches workspace only on entering a note in another vault,
    -- so after a visit to the tutorial vault, <leader>nd from a code file would
    -- still write today's note there. Leaving the vaults restores the default.
    vim.api.nvim_create_autocmd('BufEnter', {
      group = vim.api.nvim_create_augroup('custom-notes-default-workspace', { clear = true }),
      callback = function(ev)
        if vim.bo[ev.buf].buftype ~= '' or ev.file == '' or ev.file:match '^%a+://' then
          return
        end
        local default = Obsidian.workspaces[1]
        if Obsidian.workspace ~= default and not require('obsidian.api').find_workspace(ev.file) then
          require('obsidian.workspace').set(default)
        end
      end,
    })
  end,
}
