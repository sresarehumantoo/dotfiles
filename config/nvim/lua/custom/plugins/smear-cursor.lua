-- Cursor trail *inside* nvim.
--
-- Ghostty draws its own trail via a `custom-shader` (config/ghostty/shaders/), and that
-- one also covers the shell prompt and tmux copy-mode. Running both stacks two trails on
-- top of each other inside nvim, so under Ghostty this plugin stays unloaded and the
-- terminal wins. On any other terminal it loads as before.
--
-- Force it either way with DF_SMEAR_CURSOR=1 (always on) or DF_SMEAR_CURSOR=0 (always off).
local function want_smear()
  local override = vim.env.DF_SMEAR_CURSOR
  if override == '1' then
    return true
  elseif override == '0' then
    return false
  end

  -- TERM_PROGRAM is rewritten to 'tmux' inside a session; the GHOSTTY_* vars are
  -- inherited by the tmux server, so they survive. Check both.
  local in_ghostty = vim.env.GHOSTTY_RESOURCES_DIR ~= nil or vim.env.GHOSTTY_BIN_DIR ~= nil or vim.env.TERM_PROGRAM == 'ghostty'
  return not in_ghostty
end

return {
  'sphamba/smear-cursor.nvim',
  event = 'VeryLazy',
  cond = want_smear,
  opts = {},
}
