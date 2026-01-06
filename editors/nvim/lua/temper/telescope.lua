local M = {
	available = false,
}

local has_telescope, pickers = pcall(require, "telescope.pickers")
if has_telescope then
	local finders = require("telescope.finders")
	local sorters = require("telescope.config").values.generic_sorter
	local actions = require("telescope.actions")
	local action_state = require("telescope.actions.state")

	M.available = true

	function M.select(choices, opts, callback)
		opts = opts or {}
		if #choices == 0 then
			callback(nil)
			return
		end

		local finder = finders.new_table({
			results = choices,
			entry_maker = function(entry)
				local display = entry.label or tostring(entry.value)
				return {
					value = entry,
					display = display,
					ordinal = display,
				}
			end,
		})

		pickers.new(opts, {
			prompt_title = opts.prompt or "Temper",
			finder = finder,
			sorter = sorters(opts),
			attach_mappings = function(_, map)
				actions.select_default:replace(function(prompt_bufnr)
					actions.close(prompt_bufnr)
					local selection = action_state.get_selected_entry()
					callback(selection and selection.value or nil)
				end)
				map("i", "<C-c>", function(prompt_bufnr)
					actions.close(prompt_bufnr)
					callback(nil)
				end)
				map("n", "<Esc>", function(prompt_bufnr)
					actions.close(prompt_bufnr)
					callback(nil)
				end)
				return true
			end,
		}):find()
	end
else
	function M.select(choices, opts, callback)
		opts = opts or {}
		local format_item = function(item)
			return item.label or tostring(item.value)
		end
		vim.ui.select(choices, {
			prompt = opts.prompt or "Temper",
			format_item = format_item,
		}, function(choice)
			callback(choice or nil)
		end)
	end
end

return M
