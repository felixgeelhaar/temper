-- Temper Plugin Entry Point
-- This file is loaded automatically by Neovim

-- Don't load if already loaded or vim is too old
if vim.g.loaded_temper or vim.fn.has("nvim-0.8") ~= 1 then
	return
end
vim.g.loaded_temper = true

-- The plugin is configured via require("temper").setup()
-- This file just ensures the autoload path is set up
