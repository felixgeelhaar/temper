-- Minimal init for Plenary tests
-- Run tests with: nvim --headless -c "PlenaryBustedDirectory tests/ {minimal_init = 'tests/minimal_init.lua'}"

-- Add the plugin to runtimepath
local root = vim.fn.fnamemodify(vim.fn.expand("%:p:h"), ":h")
vim.opt.runtimepath:prepend(root)

-- Load plenary
local ok, plenary = pcall(require, "plenary")
if not ok then
	print("Plenary not found - tests require plenary.nvim")
	print("Install with: git clone https://github.com/nvim-lua/plenary.nvim ~/.local/share/nvim/site/pack/test/start/plenary.nvim")
	vim.cmd("qa!")
	return
end

-- Optionally load telescope if available
pcall(require, "telescope")
