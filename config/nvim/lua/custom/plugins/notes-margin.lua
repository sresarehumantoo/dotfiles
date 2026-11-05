-- A reading margin for notes: inside the vaults (vim.g.notes_dirs, same list
-- notes.lua uses) the editable area is capped at a fixed width by padding the
-- window, so long lines soft-wrap at that margin instead of running to the
-- screen edge. Nothing is written to the file -- the bytes stay as full-length
-- logical lines, so pasting a paragraph into Teams/Outlook keeps it one line.
-- This is deliberately NOT hard-wrap (textwidth): README and code buffers, and
-- markdown outside the vaults, are left exactly as they are.
local WIDTH = 80

return {
  'shortcuts/no-neck-pain.nvim',
  version = '*',
  -- Loaded early so the buffer-enter autocmd below is armed for every note,
  -- but it draws nothing until enable() runs.
  event = 'VeryLazy',
  cond = #(vim.tbl_filter(function(dir)
    return vim.fn.isdirectory(dir) == 1
  end, vim.g.notes_dirs or {})) > 0,
  opts = {
    width = WIDTH,
    -- We drive enable/disable ourselves from the notes autocmd.
    autocmds = { enableOnVimEnter = false, enableOnTabEnter = false },
    mappings = { enabled = false },
  },
  config = function(_, opts)
    require('no-neck-pain').setup(opts)

    local dirs = vim.g.notes_dirs or {}
    local function in_notes(file)
      for _, d in ipairs(dirs) do
        if vim.startswith(file, d .. '/') then
          return true
        end
      end
      return false
    end

    -- Enable the margin when the window shows a note, drop it when it shows
    -- anything else. The single-window guard keeps it from hijacking a split
    -- the user set up on purpose; <leader>tz is the manual override for that.
    local function update(buf)
      if not vim.api.nvim_buf_is_valid(buf) or vim.bo[buf].buftype ~= '' then
        return
      end
      local file = vim.api.nvim_buf_get_name(buf)
      local want = file ~= '' and in_notes(file)
      local on = vim.t.notes_margin_on == true
      if want and not on and vim.fn.winnr '$' == 1 then
        pcall(function()
          require('no-neck-pain').enable()
        end)
        vim.t.notes_margin_on = true
      elseif not want and on then
        pcall(function()
          require('no-neck-pain').disable()
        end)
        vim.t.notes_margin_on = false
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
      require('no-neck-pain').toggle()
      vim.t.notes_margin_on = not (vim.t.notes_margin_on == true)
    end, { desc = '[T]oggle [Z]en reading margin' })
  end,
}
