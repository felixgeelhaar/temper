-- Temper UI Components
-- Display panels and floating windows for intervention content

local M = {}

-- Buffer and window for the side panel
local panel_buf = nil
local panel_win = nil

-- Configuration
M.config = {
	panel_width = 60,
	panel_position = "right", -- "left" or "right"
	border = "rounded",
}

-- Create or get the panel buffer
local function get_panel_buf()
	if panel_buf and vim.api.nvim_buf_is_valid(panel_buf) then
		return panel_buf
	end

	panel_buf = vim.api.nvim_create_buf(false, true)
	vim.api.nvim_buf_set_option(panel_buf, "buftype", "nofile")
	vim.api.nvim_buf_set_option(panel_buf, "bufhidden", "hide")
	vim.api.nvim_buf_set_option(panel_buf, "swapfile", false)
	vim.api.nvim_buf_set_option(panel_buf, "filetype", "markdown")
	vim.api.nvim_buf_set_name(panel_buf, "temper://panel")

	return panel_buf
end

-- Open the side panel
function M.open_panel()
	if panel_win and vim.api.nvim_win_is_valid(panel_win) then
		return panel_win
	end

	local buf = get_panel_buf()
	local width = M.config.panel_width
	local height = vim.o.lines - 4

	-- Calculate position
	local col
	if M.config.panel_position == "right" then
		col = vim.o.columns - width - 2
	else
		col = 2
	end

	-- Create floating window
	panel_win = vim.api.nvim_open_win(buf, false, {
		relative = "editor",
		width = width,
		height = height,
		row = 2,
		col = col,
		style = "minimal",
		border = M.config.border,
		title = " Temper ",
		title_pos = "center",
	})

	-- Set window options
	vim.api.nvim_win_set_option(panel_win, "wrap", true)
	vim.api.nvim_win_set_option(panel_win, "linebreak", true)

	return panel_win
end

-- Close the panel
function M.close_panel()
	if panel_win and vim.api.nvim_win_is_valid(panel_win) then
		vim.api.nvim_win_close(panel_win, true)
		panel_win = nil
	end
end

-- Toggle the panel
function M.toggle_panel()
	if panel_win and vim.api.nvim_win_is_valid(panel_win) then
		M.close_panel()
	else
		M.open_panel()
	end
end

-- Set panel content
function M.set_panel_content(lines, title)
	local buf = get_panel_buf()

	-- Ensure window is open
	M.open_panel()

	-- Update title if provided
	if title and panel_win and vim.api.nvim_win_is_valid(panel_win) then
		vim.api.nvim_win_set_config(panel_win, {
			title = " " .. title .. " ",
			title_pos = "center",
		})
	end

	-- Convert to table if string
	if type(lines) == "string" then
		lines = vim.split(lines, "\n")
	end

	-- Set content
	vim.api.nvim_buf_set_option(buf, "modifiable", true)
	vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
	vim.api.nvim_buf_set_option(buf, "modifiable", false)
end

-- Show intervention result
function M.show_intervention(result)
	local lines = {}

	-- Header with level info
	table.insert(lines, "## Intervention")
	table.insert(lines, "")
	table.insert(lines, string.format("**Level:** L%d (%s)", result.level or 0, result.type or "unknown"))
	table.insert(lines, string.format("**Intent:** %s", result.intent or "unknown"))
	table.insert(lines, "")
	table.insert(lines, "---")
	table.insert(lines, "")

	-- Content
	if result.content then
		for _, line in ipairs(vim.split(result.content, "\n")) do
			table.insert(lines, line)
		end
	end

	M.set_panel_content(lines, "Temper - " .. string.upper(result.intent or "Help"))
end

-- Show run results
function M.show_run_result(result)
	local lines = {}

	table.insert(lines, "## Run Results")
	table.insert(lines, "")

	-- Format check
	if result.result and result.result.format_ok ~= nil then
		local status = result.result.format_ok and "✓" or "✗"
		table.insert(lines, string.format("**Format:** %s", status))
		if not result.result.format_ok and result.result.format_diff then
			table.insert(lines, "```diff")
			for _, line in ipairs(vim.split(result.result.format_diff, "\n")) do
				table.insert(lines, line)
			end
			table.insert(lines, "```")
		end
	end

	-- Build check
	if result.result and result.result.build_ok ~= nil then
		local status = result.result.build_ok and "✓" or "✗"
		table.insert(lines, string.format("**Build:** %s", status))
		if not result.result.build_ok and result.result.build_output then
			table.insert(lines, "```")
			for _, line in ipairs(vim.split(result.result.build_output, "\n")) do
				table.insert(lines, line)
			end
			table.insert(lines, "```")
		end
	end

	-- Test results
	if result.result and result.result.test_ok ~= nil then
		local status = result.result.test_ok and "✓" or "✗"
		table.insert(lines, string.format("**Tests:** %s", status))
		if result.result.test_output then
			table.insert(lines, "```")
			for _, line in ipairs(vim.split(result.result.test_output, "\n")) do
				table.insert(lines, line)
			end
			table.insert(lines, "```")
		end
	end

	M.set_panel_content(lines, "Temper - Run")
end

-- Show error
function M.show_error(message)
	local lines = {
		"## Error",
		"",
		message,
	}
	M.set_panel_content(lines, "Temper - Error")
end

-- Show notification
function M.notify(message, level)
	level = level or vim.log.levels.INFO
	vim.notify("[Temper] " .. message, level)
end

-- Show loading indicator
function M.show_loading(message)
	message = message or "Loading..."
	M.set_panel_content({ "", "  " .. message, "" }, "Temper")
end

-- Show session info
function M.show_session(session)
	local lines = {}

	table.insert(lines, "## Session")
	table.insert(lines, "")
	table.insert(lines, string.format("**ID:** `%s`", session.id or "unknown"))
	table.insert(lines, string.format("**Exercise:** %s", session.exercise_id or "unknown"))
	table.insert(lines, string.format("**Status:** %s", session.status or "unknown"))
	table.insert(lines, "")
	table.insert(lines, "### Stats")
	table.insert(lines, string.format("- Runs: %d", session.run_count or 0))
	table.insert(lines, string.format("- Hints: %d", session.hint_count or 0))
	table.insert(lines, "")
	table.insert(lines, "### Policy")
	if session.policy then
		table.insert(lines, string.format("- Track: %s", session.policy.track or "default"))
		table.insert(lines, string.format("- Max Level: L%d", session.policy.max_level or 3))
		table.insert(lines, string.format("- Cooldown: %ds", session.policy.cooldown_seconds or 60))
	end

	M.set_panel_content(lines, "Temper - Session")
end

-- Show exercise list
function M.show_exercises(packs)
	local lines = {}

	table.insert(lines, "## Available Exercises")
	table.insert(lines, "")

	for _, pack in ipairs(packs) do
		table.insert(lines, string.format("### %s", pack.name or pack.id))
		if pack.description then
			table.insert(lines, pack.description)
		end
		table.insert(lines, string.format("- Language: %s", pack.language or "unknown"))
		table.insert(lines, string.format("- Exercises: %d", pack.exercise_count or 0))
		table.insert(lines, "")
	end

	M.set_panel_content(lines, "Temper - Exercises")
end

return M
