-- A reading margin for notes: inside the vaults (vim.g.notes_dirs, same list
-- notes.lua uses) the note window is pinned to a fixed width with a blank
-- filler window to its right, so long lines soft-wrap at that margin instead of
-- running to the screen edge. The text stays LEFT-aligned (the padding is only
-- on the right) and nothing is written to the file -- the bytes stay as
-- full-length logical lines, so pasting a paragraph into Teams/Outlook keeps it
-- one line. This is deliberately NOT hard-wrap (textwidth): README and code
-- buffers, and markdown outside the vaults, are left exactly as they are.
--
-- No plugin dependency: a scratch split is all it takes, and a centering plugin
-- (no-neck-pain) can't left-align a fixed width anyway -- it always reserves
-- half the slack on each side.
local WIDTH = 80

-- A local lazy spec (dir, not a repo) just to get a one-time config hook that
-- runs after startup; it clones nothing.
return {
  dir = vim.fn.stdpath 'config',
  name = 'notes-margin',
  event = 'VeryLazy',
  cond = #(vim.tbl_filter(function(dir)
    return vim.fn.isdirectory(dir) == 1
  end, vim.g.notes_dirs or {})) > 0,
  config = function()
    local dirs = vim.g.notes_dirs or {}
    local function in_notes(file)
      for _, d in ipairs(dirs) do
        if vim.startswith(file, d .. '/') then
          return true
        end
      end
      return false
    end

    -- Count real editing windows, ignoring floats and sidebars (neo-tree, the
    -- filler) so `nvim .` -> neo-tree -> open a note still counts as one window.
    -- A genuine two-file split counts as two, and is left alone.
    local function editing_windows()
      local n = 0
      for _, w in ipairs(vim.api.nvim_tabpage_list_wins(0)) do
        local cfg = vim.api.nvim_win_get_config(w)
        if (cfg.relative == nil or cfg.relative == '') and vim.bo[vim.api.nvim_win_get_buf(w)].buftype == '' then
          n = n + 1
        end
      end
      return n
    end

    -- Global chrome saved while the margin is up. With laststatus=2 each window
    -- draws its own statusline, so the filler split the bar at the margin and
    -- left a stub to the right ("the extra bar"); a single global statusline
    -- (3) spans the full width instead. Blanking the vertical separator hides
    -- the line between the note and the filler. Both are restored in disable().
    local saved_laststatus, saved_fillchars

    -- Pin a window to WIDTH with the filler absorbing the slack. winfixwidth
    -- keeps equalalways from stealing columns back once a sidebar appears.
    local function pin(win)
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_set_option_value('winfixwidth', true, { win = win })
        vim.api.nvim_win_set_width(win, WIDTH)
      end
    end

    local function enable()
      local note = vim.api.nvim_get_current_win()
      if vim.api.nvim_win_get_width(note) <= WIDTH then
        return -- screen already narrower than the cap; nothing to pad
      end
      if saved_laststatus == nil then
        saved_laststatus = vim.o.laststatus
        saved_fillchars = vim.o.fillchars
      end
      vim.o.laststatus = 3
      vim.opt.fillchars:append { vert = ' ' }
      -- noautocmd: the split must not re-trigger the BufWinEnter handler below.
      vim.cmd 'noautocmd silent vertical rightbelow new'
      local fill = vim.api.nvim_get_current_win()
      local fb = vim.api.nvim_win_get_buf(fill)
      vim.bo[fb].buftype = 'nofile'
      vim.bo[fb].bufhidden = 'wipe'
      vim.bo[fb].swapfile = false
      vim.bo[fb].buflisted = false
      vim.bo[fb].modifiable = false
      local wo = vim.wo[fill]
      wo.number, wo.relativenumber, wo.cursorline, wo.list, wo.spell = false, false, false, false, false
      wo.signcolumn, wo.foldcolumn, wo.statuscolumn = 'no', '0', ''
      wo.fillchars = 'eob: '
      -- The filler stays flexible so it absorbs the slack; only the note window
      -- is pinned.
      vim.api.nvim_set_current_win(note)
      pin(note)
      vim.t.notes_margin_fill = fill
      vim.t.notes_margin_on = true
    end

    local function disable()
      local fill = vim.t.notes_margin_fill
      if fill and vim.api.nvim_win_is_valid(fill) then
        pcall(vim.api.nvim_win_close, fill, true)
      end
      if saved_laststatus ~= nil then
        vim.o.laststatus = saved_laststatus
        vim.o.fillchars = saved_fillchars
        saved_laststatus, saved_fillchars = nil, nil
      end
      vim.t.notes_margin_fill = nil
      vim.t.notes_margin_on = false
    end

    -- Enabling is deferred a tick: opening a file from neo-tree fires
    -- BufWinEnter in a transient window that neo-tree then closes, so the window
    -- the note actually lands in only exists on the next loop. Re-validate there.
    local pending = false
    local function schedule_enable()
      if pending then
        return
      end
      pending = true
      vim.schedule(function()
        pending = false
        local buf = vim.api.nvim_get_current_buf()
        if vim.bo[buf].buftype ~= '' or vim.t.notes_margin_on == true then
          return
        end
        local file = vim.api.nvim_buf_get_name(buf)
        if file ~= '' and in_notes(file) and editing_windows() <= 1 then
          enable()
        end
      end)
    end

    local function update(buf)
      if not vim.api.nvim_buf_is_valid(buf) or vim.bo[buf].buftype ~= '' then
        return
      end
      local file = vim.api.nvim_buf_get_name(buf)
      local want = file ~= '' and in_notes(file)
      local on = vim.t.notes_margin_on == true
      if want and not on then
        schedule_enable()
      elseif not want and on then
        disable()
      end
    end

    vim.api.nvim_create_autocmd('BufWinEnter', {
      group = vim.api.nvim_create_augroup('custom-notes-margin', { clear = true }),
      callback = function(ev)
        update(ev.buf)
      end,
    })

    -- Startup may have opened straight into a note, before the autocmd existed.
    vim.schedule(function()
      update(vim.api.nvim_get_current_buf())
    end)

    vim.keymap.set('n', '<leader>tz', function()
      if vim.t.notes_margin_on == true then
        disable()
      else
        enable()
      end
    end, { desc = '[T]oggle [Z]en reading margin' })
  end,
}
