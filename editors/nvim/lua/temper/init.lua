-- Temper - Adaptive AI Pairing for Learning
-- Neovim Plugin

local M = {}

local client = require("temper.client")
local ui = require("temper.ui")

-- Current session state
M.state = {
	session_id = nil,
	exercise_id = nil,
	track = "practice",
}

-- Default configuration
M.config = {
	-- Daemon connection
	host = "127.0.0.1",
	port = 7432,

	-- UI settings
	panel_width = 60,
	panel_position = "right",

	-- Auto behaviors
	auto_run_on_save = false,
	show_hints_in_quickfix = false,

	-- Keymaps (set to false to disable)
	keymaps = {
		hint = "<leader>th",
		review = "<leader>tr",
		stuck = "<leader>ts",
		next = "<leader>tn",
		explain = "<leader>te",
		run = "<leader>tR",
		toggle = "<leader>tt",
	},
}

-- Setup function
function M.setup(opts)
	opts = opts or {}
	M.config = vim.tbl_deep_extend("force", M.config, opts)

	-- Update client config
	client.config.host = M.config.host
	client.config.port = M.config.port

	-- Update UI config
	ui.config.panel_width = M.config.panel_width
	ui.config.panel_position = M.config.panel_position

	-- Create user commands
	M.create_commands()

	-- Setup keymaps if enabled
	if M.config.keymaps then
		M.setup_keymaps()
	end

	-- Auto-run on save
	if M.config.auto_run_on_save then
		vim.api.nvim_create_autocmd("BufWritePost", {
			pattern = "*.go",
			callback = function()
				if M.state.session_id then
					M.run()
				end
			end,
		})
	end
end

-- Create user commands
function M.create_commands()
	-- Session management
	vim.api.nvim_create_user_command("TemperStart", function(opts)
		M.start(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Start a Temper session" })

	vim.api.nvim_create_user_command("TemperStop", function()
		M.stop()
	end, { desc = "Stop current Temper session" })

	vim.api.nvim_create_user_command("TemperStatus", function()
		M.status()
	end, { desc = "Show Temper session status" })

	-- Pairing commands
	vim.api.nvim_create_user_command("TemperHint", function()
		M.hint()
	end, { desc = "Request a hint" })

	vim.api.nvim_create_user_command("TemperReview", function()
		M.review()
	end, { desc = "Request code review" })

	vim.api.nvim_create_user_command("TemperStuck", function()
		M.stuck()
	end, { desc = "Signal that you are stuck" })

	vim.api.nvim_create_user_command("TemperNext", function()
		M.next_step()
	end, { desc = "Ask what to do next" })

	vim.api.nvim_create_user_command("TemperExplain", function()
		M.explain()
	end, { desc = "Request an explanation" })

	-- Code execution
	vim.api.nvim_create_user_command("TemperRun", function()
		M.run()
	end, { desc = "Run code checks" })

	vim.api.nvim_create_user_command("TemperFormat", function()
		M.format()
	end, { desc = "Format code" })

	-- Mode/track selection
	vim.api.nvim_create_user_command("TemperMode", function(opts)
		M.set_mode(opts.args)
	end, {
		nargs = 1,
		complete = function()
			return { "practice", "interview-prep" }
		end,
		desc = "Set learning track",
	})

	-- Exercise browsing
	vim.api.nvim_create_user_command("TemperExercises", function()
		M.list_exercises()
	end, { desc = "List available exercises" })

	-- UI toggle
	vim.api.nvim_create_user_command("TemperToggle", function()
		ui.toggle_panel()
	end, { desc = "Toggle Temper panel" })

	-- Health check
	vim.api.nvim_create_user_command("TemperHealth", function()
		M.health_check()
	end, { desc = "Check daemon health" })
end

-- Setup keymaps
function M.setup_keymaps()
	local km = M.config.keymaps

	if km.hint then
		vim.keymap.set("n", km.hint, M.hint, { desc = "Temper: Hint" })
	end
	if km.review then
		vim.keymap.set("n", km.review, M.review, { desc = "Temper: Review" })
	end
	if km.stuck then
		vim.keymap.set("n", km.stuck, M.stuck, { desc = "Temper: Stuck" })
	end
	if km.next then
		vim.keymap.set("n", km.next, M.next_step, { desc = "Temper: Next" })
	end
	if km.explain then
		vim.keymap.set("n", km.explain, M.explain, { desc = "Temper: Explain" })
	end
	if km.run then
		vim.keymap.set("n", km.run, M.run, { desc = "Temper: Run" })
	end
	if km.toggle then
		vim.keymap.set("n", km.toggle, ui.toggle_panel, { desc = "Temper: Toggle Panel" })
	end
end

-- Get current buffer code as a map
local function get_buffer_code()
	local bufnr = vim.api.nvim_get_current_buf()
	local filename = vim.fn.expand("%:t")
	local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)
	return { [filename] = table.concat(lines, "\n") }
end

-- Start a session
function M.start(exercise_id)
	if not exercise_id then
		ui.notify("Usage: :TemperStart <pack>/<exercise>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Starting session...")

	client.create_session(exercise_id, M.state.track, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		M.state.session_id = result.id
		M.state.exercise_id = exercise_id

		ui.show_session(result)
		ui.notify("Session started: " .. result.id:sub(1, 8))
	end)
end

-- Stop current session
function M.stop()
	if not M.state.session_id then
		ui.notify("No active session", vim.log.levels.WARN)
		return
	end

	client.delete_session(M.state.session_id, function(err)
		if err then
			ui.show_error(err)
			return
		end

		local session_id = M.state.session_id
		M.state.session_id = nil
		M.state.exercise_id = nil

		ui.close_panel()
		ui.notify("Session ended: " .. session_id:sub(1, 8))
	end)
end

-- Show session status
function M.status()
	if not M.state.session_id then
		ui.notify("No active session", vim.log.levels.INFO)
		return
	end

	client.get_session(M.state.session_id, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		ui.show_session(result)
	end)
end

-- Request hint
function M.hint()
	M.request_intervention("hint", client.hint)
end

-- Request review
function M.review()
	M.request_intervention("review", client.review)
end

-- Signal stuck
function M.stuck()
	M.request_intervention("stuck", client.stuck)
end

-- Request next steps
function M.next_step()
	M.request_intervention("next", client.next)
end

-- Request explanation
function M.explain()
	M.request_intervention("explain", client.explain)
end

-- Common intervention request handler
function M.request_intervention(intent, request_fn)
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	local code = get_buffer_code()
	ui.show_loading("Requesting " .. intent .. "...")

	request_fn(M.state.session_id, code, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		-- Check for cooldown
		if result.error == "cooldown active" then
			ui.notify(result.message, vim.log.levels.WARN)
			return
		end

		ui.show_intervention(result)
	end)
end

-- Run code checks
function M.run()
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	local code = get_buffer_code()
	ui.show_loading("Running checks...")

	client.run(M.state.session_id, code, { format = true, build = true, test = true }, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		ui.show_run_result(result)
	end)
end

-- Format code
function M.format()
	if not M.state.session_id then
		ui.notify("No active session", vim.log.levels.WARN)
		return
	end

	local code = get_buffer_code()

	client.format(M.state.session_id, code, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if result.formatted then
			-- Apply formatted code back to buffer
			local filename = vim.fn.expand("%:t")
			if result.formatted[filename] then
				local lines = vim.split(result.formatted[filename], "\n")
				vim.api.nvim_buf_set_lines(0, 0, -1, false, lines)
				ui.notify("Code formatted")
			end
		end
	end)
end

-- Set learning track/mode
function M.set_mode(mode)
	if mode ~= "practice" and mode ~= "interview-prep" then
		ui.notify("Invalid mode. Use 'practice' or 'interview-prep'", vim.log.levels.WARN)
		return
	end

	M.state.track = mode
	ui.notify("Track set to: " .. mode)

	-- If session is active, warn that it won't apply until next session
	if M.state.session_id then
		ui.notify("Note: Track change will apply to the next session", vim.log.levels.INFO)
	end
end

-- List exercises
function M.list_exercises()
	ui.show_loading("Loading exercises...")

	client.list_exercises(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if result.packs then
			ui.show_exercises(result.packs)
		else
			ui.show_error("No exercises found")
		end
	end)
end

-- Health check
function M.health_check()
	client.is_running(function(running)
		if running then
			ui.notify("Daemon is healthy", vim.log.levels.INFO)
		else
			ui.notify("Daemon is not running. Start with: temper start", vim.log.levels.ERROR)
		end
	end)
end

return M
