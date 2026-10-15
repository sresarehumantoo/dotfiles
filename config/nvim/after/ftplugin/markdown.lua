-- Prose-friendly defaults for Markdown buffers. Native ftplugin — sourced by
-- Neovim for every markdown buffer, no plugin required. In-buffer rendering is
-- handled by render-markdown.nvim (lua/custom/plugins/markdown.lua).

-- Soft-wrap long prose at word boundaries instead of mid-word, and keep wrapped
-- lines visually indented under their start.
vim.opt_local.wrap = true
vim.opt_local.linebreak = true
vim.opt_local.breakindent = true

-- Spell-check prose. Misspellings are only highlighted — nothing is corrected
-- automatically. Suggestions come from `z=` (or the blink completion menu,
-- which gains a spell source in spell buffers; see init.lua). Toggle per-buffer
-- with `:setlocal nospell` if it's noisy.
vim.opt_local.spell = true
vim.opt_local.spelllang = 'en_us'
-- Treat CamelCase segments as separate words rather than flagging the whole run.
vim.opt_local.spelloptions = 'camel'

-- Suggestion-based spelling helpers (never auto-fix):
--   z=  open the full suggestion list for the word under the cursor (native)
--   zg  add the word to the dictionary   |  zw  mark it as wrong
vim.keymap.set('n', '<leader>zs', function()
  -- Jump to the previous misspelling, then open suggestions for it.
  vim.cmd 'normal! [s'
  vim.cmd 'normal! z='
end, { buffer = true, desc = '[Z] [S]pell: suggest for previous misspelling' })

-- Wrapped prose spans several screen rows per logical line; move by what you see.
-- A count (e.g. 5j) still jumps by logical lines so relativenumber stays useful.
vim.keymap.set('n', 'j', "v:count == 0 ? 'gj' : 'j'", { buffer = true, expr = true, desc = 'Down by display line' })
vim.keymap.set('n', 'k', "v:count == 0 ? 'gk' : 'k'", { buffer = true, expr = true, desc = 'Up by display line' })

-- <CR> follows the [text](target) link under the cursor: a file relative to
-- this one, a #heading in it, or a URL. gf alone takes "file.md#heading" for a
-- file name. Inside the notes vaults obsidian.nvim's <CR> does this (and
-- [[wikilinks]]), so it is left to that.
local file = vim.api.nvim_buf_get_name(0)
for _, dir in ipairs(vim.g.notes_dirs or {}) do
  if vim.startswith(file, dir .. '/') then
    return
  end
end

local function link_under_cursor()
  local line = vim.api.nvim_get_current_line()
  local col = vim.fn.col '.'
  local init = 1
  while true do
    local s, e, target = line:find('%[[^%]]*%]%(([^)]*)%)', init)
    if not s then
      return nil
    end
    if col >= s and col <= e then
      return (vim.trim(target):gsub('^<(.*)>$', '%1'):gsub('%s+".*"$', ''))
    end
    init = e + 1
  end
end

-- GitHub's heading anchors: lowercase, punctuation dropped, spaces to dashes.
local function slug(heading)
  return (heading:lower():gsub('[^%w%s_-]', ''):gsub('%s', '-'))
end

local function jump_to_heading(anchor)
  for lnum, l in ipairs(vim.api.nvim_buf_get_lines(0, 0, -1, false)) do
    local heading = l:match '^#+%s+(.-)%s*#*%s*$'
    if heading and slug(heading) == anchor:lower() then
      vim.api.nvim_win_set_cursor(0, { lnum, 0 })
      return
    end
  end
  vim.notify('No heading #' .. anchor, vim.log.levels.WARN)
end

vim.keymap.set('n', '<CR>', function()
  local target = link_under_cursor()
  if not target or target == '' then
    return vim.api.nvim_feedkeys(vim.keycode '<CR>', 'n', false)
  end
  if target:match '^%a[%w+.-]*:' then
    return vim.ui.open(target)
  end
  local path, anchor = target:match '^([^#]*)#?(.*)$'
  path = path:gsub('%%(%x%x)', function(h)
    return string.char(tonumber(h, 16))
  end)
  if path ~= '' then
    if not vim.startswith(path, '/') then
      path = vim.fs.joinpath(vim.fs.dirname(vim.api.nvim_buf_get_name(0)), path)
    end
    path = vim.fs.normalize(path)
    if vim.fn.filereadable(path) == 0 and vim.fn.isdirectory(path) == 0 then
      return vim.notify('No such file: ' .. path, vim.log.levels.WARN)
    end
    vim.cmd.edit(vim.fn.fnameescape(path))
  end
  if anchor ~= '' then
    jump_to_heading(anchor)
  end
end, { buffer = true, desc = 'Follow markdown link' })
