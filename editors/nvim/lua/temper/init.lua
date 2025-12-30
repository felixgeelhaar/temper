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

	vim.api.nvim_create_user_command("TemperEscalate", function(opts)
		M.escalate(opts.args)
	end, {
		nargs = "+",
		desc = "Request explicit escalation (L4/L5) - requires level and justification",
	})

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

	-- Spec commands (Specular format)
	vim.api.nvim_create_user_command("TemperSpecCreate", function(opts)
		M.spec_create(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Create a new spec scaffold" })

	vim.api.nvim_create_user_command("TemperSpecList", function()
		M.spec_list()
	end, { desc = "List specs in workspace" })

	vim.api.nvim_create_user_command("TemperSpecValidate", function(opts)
		M.spec_validate(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Validate spec completeness" })

	vim.api.nvim_create_user_command("TemperSpecStatus", function(opts)
		M.spec_status(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Show spec progress" })

	vim.api.nvim_create_user_command("TemperSpecLock", function(opts)
		M.spec_lock(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Generate SpecLock for drift detection" })

	vim.api.nvim_create_user_command("TemperSpecDrift", function(opts)
		M.spec_drift(opts.args ~= "" and opts.args or nil)
	end, { nargs = "?", desc = "Show drift from locked spec" })

	-- Stats/Analytics commands
	vim.api.nvim_create_user_command("TemperStats", function()
		M.stats_overview()
	end, { desc = "Show learning statistics overview" })

	vim.api.nvim_create_user_command("TemperStatsSkills", function()
		M.stats_skills()
	end, { desc = "Show skill progression by topic" })

	vim.api.nvim_create_user_command("TemperStatsErrors", function()
		M.stats_errors()
	end, { desc = "Show common error patterns" })

	vim.api.nvim_create_user_command("TemperStatsTrend", function()
		M.stats_trend()
	end, { desc = "Show hint dependency over time" })

	-- Patch commands
	vim.api.nvim_create_user_command("TemperPatchPreview", function()
		M.patch_preview()
	end, { desc = "Preview pending patch" })

	vim.api.nvim_create_user_command("TemperPatchApply", function()
		M.patch_apply()
	end, { desc = "Apply pending patch" })

	vim.api.nvim_create_user_command("TemperPatchReject", function()
		M.patch_reject()
	end, { desc = "Reject pending patch" })

	vim.api.nvim_create_user_command("TemperPatches", function()
		M.list_patches()
	end, { desc = "List all patches in session" })

	-- Patch log/audit commands
	vim.api.nvim_create_user_command("TemperPatchLog", function(opts)
		local limit = nil
		if opts.args and opts.args ~= "" then
			limit = tonumber(opts.args)
		end
		M.patch_log(limit)
	end, { nargs = "?", desc = "Show patch audit log (optional: limit)" })

	vim.api.nvim_create_user_command("TemperPatchLogStats", function()
		M.patch_log_stats()
	end, { desc = "Show patch statistics" })
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

-- Request explicit escalation (L4/L5)
function M.escalate(args)
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	-- Parse args: first word is level (4 or 5), rest is justification
	local parts = vim.split(args, " ", { trimempty = true })
	if #parts < 2 then
		ui.notify("Usage: :TemperEscalate <level> <justification>", vim.log.levels.ERROR)
		ui.notify("Example: :TemperEscalate 4 I've tried multiple hints but still can't understand the recursion pattern", vim.log.levels.INFO)
		return
	end

	local level = tonumber(parts[1])
	if level ~= 4 and level ~= 5 then
		ui.notify("Level must be 4 (partial solution) or 5 (full solution)", vim.log.levels.ERROR)
		return
	end

	local justification = table.concat(vim.list_slice(parts, 2), " ")
	if #justification < 20 then
		ui.notify("Please provide a more detailed justification (at least 20 characters)", vim.log.levels.ERROR)
		return
	end

	local code = get_buffer_code()
	ui.show_loading("Requesting escalation to L" .. level .. "...")

	client.escalate(M.state.session_id, level, justification, code, function(err, result)
		if err then
			ui.notify("Escalation failed: " .. err, vim.log.levels.ERROR)
			return
		end

		if result.content then
			ui.show_response(result.content, "L" .. result.level .. " (escalated)")
		end
	end)
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

-- Spec commands (Specular format)

-- Create a new spec
function M.spec_create(name)
	if not name then
		ui.notify("Usage: :TemperSpecCreate <name>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Creating spec...")

	client.create_spec(name, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		ui.notify("Spec created: " .. (result.file_path or name))
		M.spec_list()
	end)
end

-- List specs
function M.spec_list()
	ui.show_loading("Loading specs...")

	client.list_specs(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if result.specs and #result.specs > 0 then
			local lines = { "## Specs in Workspace", "" }
			for _, spec in ipairs(result.specs) do
				table.insert(lines, string.format("### %s (v%s)", spec.name or "Unnamed", spec.version or "1.0.0"))
				table.insert(lines, string.format("- Path: `%s`", spec.file_path or "unknown"))
				if spec.goals and #spec.goals > 0 then
					table.insert(lines, "- Goals: " .. #spec.goals)
				end
				if spec.acceptance_criteria then
					local satisfied = 0
					for _, ac in ipairs(spec.acceptance_criteria) do
						if ac.satisfied then
							satisfied = satisfied + 1
						end
					end
					table.insert(lines, string.format("- Progress: %d/%d criteria", satisfied, #spec.acceptance_criteria))
				end
				table.insert(lines, "")
			end
			ui.set_panel_content(lines, "Temper - Specs")
		else
			ui.set_panel_content({ "", "  No specs found in .specs/ directory", "" }, "Temper - Specs")
		end
	end)
end

-- Validate a spec
function M.spec_validate(path)
	if not path then
		ui.notify("Usage: :TemperSpecValidate <path>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Validating spec...")

	client.validate_spec(path, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Spec Validation", "" }
		if result.valid then
			table.insert(lines, "✓ Spec is valid")
		else
			table.insert(lines, "✗ Spec has issues")
		end
		table.insert(lines, "")

		if result.errors and #result.errors > 0 then
			table.insert(lines, "### Errors")
			for _, e in ipairs(result.errors) do
				table.insert(lines, "- " .. e)
			end
			table.insert(lines, "")
		end

		if result.warnings and #result.warnings > 0 then
			table.insert(lines, "### Warnings")
			for _, w in ipairs(result.warnings) do
				table.insert(lines, "- " .. w)
			end
		end

		ui.set_panel_content(lines, "Temper - Validate")
	end)
end

-- Show spec progress
function M.spec_status(path)
	if not path then
		ui.notify("Usage: :TemperSpecStatus <path>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Loading spec progress...")

	client.get_spec_progress(path, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Spec Progress", "" }
		table.insert(lines, string.format("**Progress:** %d/%d criteria (%.0f%%)",
			result.satisfied or 0, result.total or 0, result.percent or 0))
		table.insert(lines, "")

		if result.criteria then
			table.insert(lines, "### Acceptance Criteria")
			for _, ac in ipairs(result.criteria) do
				local status = ac.satisfied and "✓" or "○"
				table.insert(lines, string.format("- %s [%s] %s", status, ac.id, ac.description or ""))
			end
		end

		ui.set_panel_content(lines, "Temper - Progress")
	end)
end

-- Lock a spec
function M.spec_lock(path)
	if not path then
		ui.notify("Usage: :TemperSpecLock <path>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Generating SpecLock...")

	client.lock_spec(path, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## SpecLock Generated", "" }
		table.insert(lines, string.format("**Version:** %s", result.version or "unknown"))
		table.insert(lines, "")

		if result.features then
			table.insert(lines, "### Locked Features")
			for id, feat in pairs(result.features) do
				table.insert(lines, string.format("- **%s**: `%s`", id, (feat.hash or ""):sub(1, 12) .. "..."))
			end
		end

		ui.set_panel_content(lines, "Temper - Lock")
		ui.notify("SpecLock generated")
	end)
end

-- Show spec drift
function M.spec_drift(path)
	if not path then
		ui.notify("Usage: :TemperSpecDrift <path>", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Checking for drift...")

	client.get_spec_drift(path, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Spec Drift Detection", "" }

		if result.has_drift then
			table.insert(lines, "⚠ Drift detected!")
			table.insert(lines, "")
			if result.drifted_features and #result.drifted_features > 0 then
				table.insert(lines, "### Changed Features")
				for _, feat in ipairs(result.drifted_features) do
					table.insert(lines, "- " .. feat)
				end
			end
		else
			table.insert(lines, "✓ No drift - spec matches lock")
		end

		ui.set_panel_content(lines, "Temper - Drift")
	end)
end

-- Stats/Analytics commands

-- Show stats overview
function M.stats_overview()
	ui.show_loading("Loading statistics...")

	client.get_stats_overview(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Learning Statistics", "" }

		if result.total_sessions then
			table.insert(lines, string.format("**Total Sessions:** %d", result.total_sessions))
		end
		if result.total_runs then
			table.insert(lines, string.format("**Total Runs:** %d", result.total_runs))
		end
		if result.total_hints then
			table.insert(lines, string.format("**Total Hints:** %d", result.total_hints))
		end
		if result.hint_dependency then
			table.insert(lines, string.format("**Hint Dependency:** %.1f%%", result.hint_dependency * 100))
		end

		table.insert(lines, "")
		table.insert(lines, "Use `:TemperStatsSkills`, `:TemperStatsErrors`, `:TemperStatsTrend` for details")

		ui.set_panel_content(lines, "Temper - Stats")
	end)
end

-- Show skills breakdown
function M.stats_skills()
	ui.show_loading("Loading skills...")

	client.get_stats_skills(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Skill Progression", "" }

		if result.skills and #result.skills > 0 then
			for _, skill in ipairs(result.skills) do
				local level = skill.level or 0
				local bar = string.rep("█", level) .. string.rep("░", 5 - level)
				table.insert(lines, string.format("**%s** [%s] L%d", skill.topic or "Unknown", bar, level))
				if skill.exercises_completed then
					table.insert(lines, string.format("  - Completed: %d exercises", skill.exercises_completed))
				end
				table.insert(lines, "")
			end
		else
			table.insert(lines, "No skill data yet. Complete some exercises!")
		end

		ui.set_panel_content(lines, "Temper - Skills")
	end)
end

-- Show error patterns
function M.stats_errors()
	ui.show_loading("Loading error patterns...")

	client.get_stats_errors(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Common Error Patterns", "" }

		if result.patterns and #result.patterns > 0 then
			for _, pattern in ipairs(result.patterns) do
				table.insert(lines, string.format("### %s", pattern.category or "Unknown"))
				table.insert(lines, string.format("- Count: %d occurrences", pattern.count or 0))
				if pattern.last_seen then
					table.insert(lines, string.format("- Last seen: %s", pattern.last_seen))
				end
				if pattern.suggestion then
					table.insert(lines, string.format("- Suggestion: %s", pattern.suggestion))
				end
				table.insert(lines, "")
			end
		else
			table.insert(lines, "No error patterns recorded yet.")
		end

		ui.set_panel_content(lines, "Temper - Errors")
	end)
end

-- Show hint dependency trend
function M.stats_trend()
	ui.show_loading("Loading trend data...")

	client.get_stats_trend(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Hint Dependency Trend", "" }

		if result.data_points and #result.data_points > 0 then
			table.insert(lines, "### Recent Sessions")
			for _, point in ipairs(result.data_points) do
				local pct = (point.dependency or 0) * 100
				local bar_len = math.floor(pct / 5)
				local bar = string.rep("▓", bar_len) .. string.rep("░", 20 - bar_len)
				table.insert(lines, string.format("%s [%s] %.0f%%", point.date or "?", bar, pct))
			end
			table.insert(lines, "")
			if result.trend then
				table.insert(lines, string.format("**Trend:** %s", result.trend))
			end
		else
			table.insert(lines, "Not enough data for trend analysis.")
			table.insert(lines, "Complete more sessions to see your progress!")
		end

		ui.set_panel_content(lines, "Temper - Trend")
	end)
end

-- Patch commands

-- Preview pending patch
function M.patch_preview()
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Loading patch preview...")

	client.patch_preview(M.state.session_id, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if not result.has_patch then
			ui.notify(result.message or "No pending patches", vim.log.levels.INFO)
			return
		end

		local preview = result.preview
		local patch = preview.patch

		local lines = { "## Patch Preview", "" }
		table.insert(lines, string.format("**File:** `%s`", patch.file or "unknown"))
		table.insert(lines, string.format("**Description:** %s", patch.description or "Code change"))
		table.insert(lines, "")

		if preview.additions or preview.deletions then
			table.insert(lines, string.format("**Changes:** +%d / -%d lines", preview.additions or 0, preview.deletions or 0))
		end

		if preview.warnings and #preview.warnings > 0 then
			table.insert(lines, "")
			table.insert(lines, "### Warnings")
			for _, w in ipairs(preview.warnings) do
				table.insert(lines, "- " .. w)
			end
		end

		table.insert(lines, "")
		table.insert(lines, "### Diff")
		table.insert(lines, "```diff")
		for line in (patch.diff or ""):gmatch("[^\n]+") do
			table.insert(lines, line)
		end
		table.insert(lines, "```")

		table.insert(lines, "")
		table.insert(lines, "Use `:TemperPatchApply` to apply or `:TemperPatchReject` to reject")

		ui.set_panel_content(lines, "Temper - Patch")
	end)
end

-- Apply pending patch
function M.patch_apply()
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Applying patch...")

	client.patch_apply(M.state.session_id, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if result.applied then
			-- Apply the patch content to the buffer
			local filename = vim.fn.expand("%:t")
			if result.file == filename and result.content then
				local lines = vim.split(result.content, "\n")
				vim.api.nvim_buf_set_lines(0, 0, -1, false, lines)
				ui.notify("Patch applied to " .. result.file, vim.log.levels.INFO)
			else
				-- Patch is for a different file, just notify
				ui.notify("Patch applied to " .. result.file .. " - open the file to see changes", vim.log.levels.INFO)
			end
		else
			ui.notify("Failed to apply patch", vim.log.levels.ERROR)
		end
	end)
end

-- Reject pending patch
function M.patch_reject()
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	client.patch_reject(M.state.session_id, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		if result.rejected then
			ui.notify("Patch rejected", vim.log.levels.INFO)
		end
	end)
end

-- List all patches in session
function M.list_patches()
	if not M.state.session_id then
		ui.notify("No active session. Use :TemperStart first", vim.log.levels.WARN)
		return
	end

	ui.show_loading("Loading patches...")

	client.list_patches(M.state.session_id, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Session Patches", "" }

		if result.count == 0 then
			table.insert(lines, "No patches in this session.")
			table.insert(lines, "")
			table.insert(lines, "Patches are created when you request L4/L5 escalation.")
		else
			table.insert(lines, string.format("**Total Patches:** %d", result.count))
			table.insert(lines, "")

			for i, p in ipairs(result.patches or {}) do
				local status_icon = "○"
				if p.status == "applied" then status_icon = "✓"
				elseif p.status == "rejected" then status_icon = "✗"
				elseif p.status == "expired" then status_icon = "⏱"
				end

				table.insert(lines, string.format("### %d. %s `%s`", i, status_icon, p.file or "unknown"))
				table.insert(lines, string.format("- Status: %s", p.status or "pending"))
				table.insert(lines, string.format("- Description: %s", p.description or "Code change"))
				table.insert(lines, "")
			end
		end

		ui.set_panel_content(lines, "Temper - Patches")
	end)
end

-- View patch audit log
function M.patch_log(limit)
	ui.show_loading("Loading patch log...")

	client.get_patch_log(limit, function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Patch Audit Log", "" }

		if result.count == 0 then
			table.insert(lines, "No patch activity recorded yet.")
		else
			table.insert(lines, string.format("**Showing:** %d entries", result.count))
			table.insert(lines, "")

			for _, entry in ipairs(result.entries or {}) do
				local action_icon = "○"
				if entry.action == "applied" then action_icon = "✓"
				elseif entry.action == "rejected" then action_icon = "✗"
				elseif entry.action == "expired" then action_icon = "⏱"
				elseif entry.action == "created" then action_icon = "+"
				elseif entry.action == "previewed" then action_icon = "👁"
				end

				table.insert(lines, string.format("### %s %s", action_icon, entry.action))
				table.insert(lines, string.format("- File: `%s`", entry.file or "unknown"))
				table.insert(lines, string.format("- Description: %s", entry.description or "-"))
				table.insert(lines, string.format("- Lines: +%d / -%d", entry.lines_added or 0, entry.lines_removed or 0))
				table.insert(lines, string.format("- Time: %s", entry.timestamp or "-"))
				table.insert(lines, "")
			end
		end

		ui.set_panel_content(lines, "Temper - Patch Log")
	end)
end

-- View patch statistics
function M.patch_log_stats()
	ui.show_loading("Loading patch stats...")

	client.get_patch_stats(function(err, result)
		if err then
			ui.show_error(err)
			return
		end

		local lines = { "## Patch Statistics", "" }

		table.insert(lines, string.format("**Total Patches:** %d", result.total_patches or 0))
		table.insert(lines, "")
		table.insert(lines, "### By Status")
		table.insert(lines, string.format("- ✓ Applied: %d", result.applied or 0))
		table.insert(lines, string.format("- ✗ Rejected: %d", result.rejected or 0))
		table.insert(lines, string.format("- ⏱ Expired: %d", result.expired or 0))
		table.insert(lines, "")
		table.insert(lines, "### Code Changes (Applied)")
		table.insert(lines, string.format("- Lines Added: +%d", result.total_lines_added or 0))
		table.insert(lines, string.format("- Lines Removed: -%d", result.total_lines_removed or 0))
		table.insert(lines, "")

		-- Calculate acceptance rate if there are patches
		local total = result.total_patches or 0
		local applied = result.applied or 0
		if total > 0 then
			local rate = math.floor((applied / total) * 100)
			table.insert(lines, string.format("**Acceptance Rate:** %d%%", rate))
		end

		ui.set_panel_content(lines, "Temper - Patch Stats")
	end)
end

return M
