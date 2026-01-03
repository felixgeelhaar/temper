-- Temper Telescope Integration
-- Provides exercise picker and spec finder using Telescope

local M = {}

local client = require("temper.client")

-- Check if Telescope is available
local has_telescope, telescope = pcall(require, "telescope")
if not has_telescope then
	M.available = false
	return M
end

M.available = true

local pickers = require("telescope.pickers")
local finders = require("telescope.finders")
local conf = require("telescope.config").values
local actions = require("telescope.actions")
local action_state = require("telescope.actions.state")
local previewers = require("telescope.previewers")

-- Exercise Picker
-- Opens a Telescope picker to browse and select exercises
function M.exercises(opts)
	opts = opts or {}

	-- Fetch exercises from daemon
	client.list_exercises(function(err, data)
		if err then
			vim.notify("Failed to fetch exercises: " .. err, vim.log.levels.ERROR)
			return
		end

		if not data or not data.packs or #data.packs == 0 then
			vim.notify("No exercise packs found", vim.log.levels.WARN)
			return
		end

		-- Build flat list of exercises with pack info
		local entries = {}
		for _, pack in ipairs(data.packs) do
			table.insert(entries, {
				display = string.format("📦 %s (%s)", pack.name, pack.language),
				pack_id = pack.id,
				pack_name = pack.name,
				description = pack.description,
				language = pack.language,
				exercise_count = pack.exercise_count,
				is_pack = true,
			})
		end

		pickers
			.new(opts, {
				prompt_title = "Temper Exercises",
				finder = finders.new_table({
					results = entries,
					entry_maker = function(entry)
						return {
							value = entry,
							display = entry.display,
							ordinal = entry.pack_name .. " " .. entry.language,
						}
					end,
				}),
				sorter = conf.generic_sorter(opts),
				previewer = previewers.new_buffer_previewer({
					title = "Pack Details",
					define_preview = function(self, entry)
						local pack = entry.value
						local lines = {
							"# " .. pack.pack_name,
							"",
							"Language: " .. pack.language,
							"Exercises: " .. tostring(pack.exercise_count),
							"",
							"## Description",
							"",
							pack.description or "No description available",
						}
						vim.api.nvim_buf_set_lines(self.state.bufnr, 0, -1, false, lines)
						vim.api.nvim_buf_set_option(self.state.bufnr, "filetype", "markdown")
					end,
				}),
				attach_mappings = function(prompt_bufnr, map)
					actions.select_default:replace(function()
						local selection = action_state.get_selected_entry()
						actions.close(prompt_bufnr)

						if selection and selection.value.is_pack then
							-- Open exercise selector for this pack
							M.exercises_in_pack(selection.value.pack_id, opts)
						end
					end)
					return true
				end,
			})
			:find()
	end)
end

-- Exercise picker within a specific pack
function M.exercises_in_pack(pack_id, opts)
	opts = opts or {}

	-- For now, prompt for exercise slug
	-- Future: fetch actual exercise list from pack
	vim.ui.input({
		prompt = "Exercise slug (e.g., hello-world): ",
	}, function(slug)
		if not slug or slug == "" then
			return
		end

		local temper = require("temper")
		temper.start(pack_id .. "/" .. slug)
	end)
end

-- Spec Picker
-- Opens a Telescope picker to browse and select specs
function M.specs(opts)
	opts = opts or {}

	client.list_specs(function(err, data)
		if err then
			vim.notify("Failed to fetch specs: " .. err, vim.log.levels.ERROR)
			return
		end

		if not data or not data.specs or #data.specs == 0 then
			vim.notify("No specs found. Create one with :TemperSpecCreate", vim.log.levels.WARN)
			return
		end

		local entries = {}
		for _, spec in ipairs(data.specs) do
			local satisfied = 0
			local total = 0
			if spec.acceptance_criteria then
				total = #spec.acceptance_criteria
				for _, ac in ipairs(spec.acceptance_criteria) do
					if ac.satisfied then
						satisfied = satisfied + 1
					end
				end
			end

			local progress_icon = "○"
			if total > 0 then
				local ratio = satisfied / total
				if ratio == 1 then
					progress_icon = "✓"
				elseif ratio >= 0.5 then
					progress_icon = "◐"
				elseif ratio > 0 then
					progress_icon = "◔"
				end
			end

			table.insert(entries, {
				display = string.format("%s %s (v%s) [%d/%d]", progress_icon, spec.name, spec.version, satisfied, total),
				name = spec.name,
				version = spec.version,
				file_path = spec.file_path,
				goals = spec.goals or {},
				features = spec.features or {},
				acceptance_criteria = spec.acceptance_criteria or {},
				satisfied = satisfied,
				total = total,
			})
		end

		pickers
			.new(opts, {
				prompt_title = "Temper Specs",
				finder = finders.new_table({
					results = entries,
					entry_maker = function(entry)
						return {
							value = entry,
							display = entry.display,
							ordinal = entry.name .. " " .. entry.version,
						}
					end,
				}),
				sorter = conf.generic_sorter(opts),
				previewer = previewers.new_buffer_previewer({
					title = "Spec Details",
					define_preview = function(self, entry)
						local spec = entry.value
						local lines = {
							"# " .. spec.name,
							"Version: " .. spec.version,
							"File: .specs/" .. spec.file_path,
							"Progress: " .. spec.satisfied .. "/" .. spec.total .. " criteria satisfied",
							"",
						}

						if #spec.goals > 0 then
							table.insert(lines, "## Goals")
							table.insert(lines, "")
							for _, goal in ipairs(spec.goals) do
								table.insert(lines, "- " .. goal)
							end
							table.insert(lines, "")
						end

						if #spec.acceptance_criteria > 0 then
							table.insert(lines, "## Acceptance Criteria")
							table.insert(lines, "")
							for _, ac in ipairs(spec.acceptance_criteria) do
								local status = ac.satisfied and "✓" or "○"
								table.insert(lines, string.format("- [%s] %s: %s", status, ac.id, ac.description))
							end
						end

						vim.api.nvim_buf_set_lines(self.state.bufnr, 0, -1, false, lines)
						vim.api.nvim_buf_set_option(self.state.bufnr, "filetype", "markdown")
					end,
				}),
				attach_mappings = function(prompt_bufnr, map)
					-- Open spec file on selection
					actions.select_default:replace(function()
						local selection = action_state.get_selected_entry()
						actions.close(prompt_bufnr)

						if selection then
							local spec_path = ".specs/" .. selection.value.file_path
							vim.cmd("edit " .. spec_path)
						end
					end)

					-- Validate spec with <C-v>
					map("i", "<C-v>", function()
						local selection = action_state.get_selected_entry()
						if selection then
							local temper = require("temper")
							temper.spec_validate(selection.value.file_path)
						end
					end)

					-- Lock spec with <C-l>
					map("i", "<C-l>", function()
						local selection = action_state.get_selected_entry()
						if selection then
							local temper = require("temper")
							temper.spec_lock(selection.value.file_path)
						end
					end)

					return true
				end,
			})
			:find()
	end)
end

-- Stats Picker
-- Opens a Telescope picker for different stats views
function M.stats(opts)
	opts = opts or {}

	local entries = {
		{ display = "📊 Overview", action = "overview", desc = "Learning statistics overview" },
		{ display = "🎯 Skills", action = "skills", desc = "Skill progression by topic" },
		{ display = "⚠️  Errors", action = "errors", desc = "Common error patterns" },
		{ display = "📈 Trend", action = "trend", desc = "Hint dependency over time" },
	}

	pickers
		.new(opts, {
			prompt_title = "Learning Statistics",
			finder = finders.new_table({
				results = entries,
				entry_maker = function(entry)
					return {
						value = entry,
						display = entry.display,
						ordinal = entry.action,
					}
				end,
			}),
			sorter = conf.generic_sorter(opts),
			attach_mappings = function(prompt_bufnr, map)
				actions.select_default:replace(function()
					local selection = action_state.get_selected_entry()
					actions.close(prompt_bufnr)

					if selection then
						local temper = require("temper")
						local action = selection.value.action
						if action == "overview" then
							temper.stats_overview()
						elseif action == "skills" then
							temper.stats_skills()
						elseif action == "errors" then
							temper.stats_errors()
						elseif action == "trend" then
							temper.stats_trend()
						end
					end
				end)
				return true
			end,
		})
		:find()
end

-- Patch Picker
-- Opens a Telescope picker for patch management
function M.patches(opts)
	opts = opts or {}

	local temper = require("temper")
	if not temper.state.session_id then
		vim.notify("No active session", vim.log.levels.WARN)
		return
	end

	client.list_patches(temper.state.session_id, function(err, data)
		if err then
			vim.notify("Failed to fetch patches: " .. err, vim.log.levels.ERROR)
			return
		end

		if not data or data.count == 0 then
			vim.notify("No patches in this session", vim.log.levels.INFO)
			return
		end

		local entries = {}
		for i, patch in ipairs(data.patches) do
			local status_icon = "○"
			if patch.status == "applied" then
				status_icon = "✓"
			elseif patch.status == "rejected" then
				status_icon = "✗"
			elseif patch.status == "expired" then
				status_icon = "⏱"
			end

			table.insert(entries, {
				display = string.format("%s %s: %s", status_icon, patch.file, patch.description),
				index = i,
				file = patch.file,
				description = patch.description,
				status = patch.status or "pending",
				diff = patch.diff,
			})
		end

		pickers
			.new(opts, {
				prompt_title = "Session Patches",
				finder = finders.new_table({
					results = entries,
					entry_maker = function(entry)
						return {
							value = entry,
							display = entry.display,
							ordinal = entry.file .. " " .. entry.description,
						}
					end,
				}),
				sorter = conf.generic_sorter(opts),
				previewer = previewers.new_buffer_previewer({
					title = "Patch Diff",
					define_preview = function(self, entry)
						local patch = entry.value
						local lines = {
							"File: " .. patch.file,
							"Status: " .. patch.status,
							"",
							"--- Diff ---",
							"",
						}
						if patch.diff then
							for line in patch.diff:gmatch("[^\n]+") do
								table.insert(lines, line)
							end
						end
						vim.api.nvim_buf_set_lines(self.state.bufnr, 0, -1, false, lines)
						vim.api.nvim_buf_set_option(self.state.bufnr, "filetype", "diff")
					end,
				}),
				attach_mappings = function(prompt_bufnr, map)
					actions.select_default:replace(function()
						local selection = action_state.get_selected_entry()
						actions.close(prompt_bufnr)

						if selection and selection.value.status == "pending" then
							temper.patch_preview()
						end
					end)
					return true
				end,
			})
			:find()
	end)
end

-- Register Telescope extension
function M.register()
	if not M.available then
		return
	end

	telescope.register_extension({
		setup = function(ext_config, config) end,
		exports = {
			exercises = M.exercises,
			specs = M.specs,
			stats = M.stats,
			patches = M.patches,
		},
	})
end

return M
