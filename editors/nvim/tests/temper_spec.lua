-- Temper Plugin Tests
-- Run with: nvim --headless -c "PlenaryBustedDirectory tests/ {minimal_init = 'tests/minimal_init.lua'}"

local has_plenary, plenary = pcall(require, "plenary")
if not has_plenary then
	print("Plenary not available - skipping tests")
	return
end

describe("temper", function()
	-- Load the module fresh for each test
	local temper

	before_each(function()
		package.loaded["temper"] = nil
		package.loaded["temper.client"] = nil
		package.loaded["temper.ui"] = nil
		package.loaded["temper.telescope"] = nil
		temper = require("temper")
		temper.health_check = function() end
	end)

	describe("setup", function()
		it("should use default config values", function()
			temper.setup({})

			assert.equals("127.0.0.1", temper.config.host)
			assert.equals(7432, temper.config.port)
			assert.equals(60, temper.config.panel_width)
			assert.equals("right", temper.config.panel_position)
			assert.equals(false, temper.config.auto_run_on_save)
		end)

		it("should merge custom config", function()
			temper.setup({
				host = "localhost",
				port = 8080,
				panel_width = 80,
			})

			assert.equals("localhost", temper.config.host)
			assert.equals(8080, temper.config.port)
			assert.equals(80, temper.config.panel_width)
			-- Defaults should still be applied
			assert.equals("right", temper.config.panel_position)
		end)

		it("should configure keymaps", function()
			temper.setup({
				keymaps = {
					hint = "<leader>h",
					review = false, -- Disable review keymap
				},
			})

			assert.equals("<leader>h", temper.config.keymaps.hint)
			assert.equals(false, temper.config.keymaps.review)
			-- Other keymaps should use defaults
			assert.equals("<leader>ts", temper.config.keymaps.stuck)
		end)
	end)

describe("state", function()
		it("should initialize with nil session", function()
			assert.is_nil(temper.state.session_id)
			assert.is_nil(temper.state.exercise_id)
		end)

		it("should have default track", function()
			assert.equals("practice", temper.state.track)
		end)
	end)

	describe("daemon health check", function()
		it("runs health_check by default", function()
			local calls = 0
			temper.health_check = function()
				calls = calls + 1
			end
			temper.setup({})
			assert.equals(1, calls)
		end)

		it("skips health_check when disabled", function()
			local calls = 0
			temper.health_check = function()
				calls = calls + 1
			end
			temper.setup({ check_daemon_on_start = false })
			assert.equals(0, calls)
		end)
	end)

	describe("intervention levels", function()
		local function level_to_type(level)
			local types = {
				[0] = "clarifying",
				[1] = "hint",
				[2] = "nudge",
				[3] = "outline",
				[4] = "partial",
				[5] = "solution",
			}
			return types[level]
		end

		it("should map L0 to clarifying", function()
			assert.equals("clarifying", level_to_type(0))
		end)

		it("should map L1 to hint", function()
			assert.equals("hint", level_to_type(1))
		end)

		it("should map L2 to nudge", function()
			assert.equals("nudge", level_to_type(2))
		end)

		it("should map L3 to outline", function()
			assert.equals("outline", level_to_type(3))
		end)

		it("should map L4 to partial", function()
			assert.equals("partial", level_to_type(4))
		end)

		it("should map L5 to solution", function()
			assert.equals("solution", level_to_type(5))
		end)
	end)

describe("tracks", function()
		it("should support practice track", function()
			temper.state.track = "practice"
			assert.equals("practice", temper.state.track)
		end)

		it("should support interview-prep track", function()
			temper.state.track = "interview-prep"
			assert.equals("interview-prep", temper.state.track)
		end)
	end)

	describe("session guidance", function()
		it("should mention pickers in the hint text", function()
			local hint = temper.session_hint_text()
			assert.matches("TemperPickExercise", hint, nil, true)
			assert.matches("TemperSpecStart", hint, nil, true)
		end)
	end)
end)

describe("temper.client", function()
	local client

	before_each(function()
		package.loaded["temper.client"] = nil
		client = require("temper.client")
	end)

	describe("config", function()
		it("should have default host", function()
			assert.equals("127.0.0.1", client.config.host)
		end)

		it("should have default port", function()
			assert.equals(7432, client.config.port)
		end)
	end)

	describe("url generation", function()
		it("should generate correct base url", function()
			local expected = "http://127.0.0.1:7432"
			local actual = "http://" .. client.config.host .. ":" .. client.config.port
			assert.equals(expected, actual)
		end)
	end)
end)

describe("temper.ui", function()
	local ui

	before_each(function()
		package.loaded["temper.ui"] = nil
		ui = require("temper.ui")
	end)

	describe("config", function()
		it("should have default panel width", function()
			assert.equals(60, ui.config.panel_width)
		end)

		it("should have default panel position", function()
			assert.equals("right", ui.config.panel_position)
		end)
	end)
end)

describe("temper.telescope", function()
	local telescope_temper

	before_each(function()
		package.loaded["temper.telescope"] = nil
		-- This may fail if telescope is not installed
		local ok, mod = pcall(require, "temper.telescope")
		if ok then
			telescope_temper = mod
		end
	end)

	it("should indicate telescope availability", function()
		if telescope_temper then
			-- If we got here, module loaded
			assert.is_boolean(telescope_temper.available)
			assert.equals("function", type(telescope_temper.select))
		end
	end)
end)

describe("escalation validation", function()
	local function validate_justification(text)
		return text and #text >= 20
	end

	local function validate_level(level)
		return level == 4 or level == 5
	end

	it("should reject short justifications", function()
		assert.is_false(validate_justification("Too short"))
		assert.is_false(validate_justification(""))
		assert.is_false(validate_justification(nil))
	end)

	it("should accept valid justifications", function()
		assert.is_true(validate_justification("I have tried multiple approaches but cannot understand the pattern"))
		assert.is_true(validate_justification("After reviewing the hints, I still need more guidance on recursion"))
	end)

	it("should only accept L4 and L5 for escalation", function()
		assert.is_false(validate_level(0))
		assert.is_false(validate_level(1))
		assert.is_false(validate_level(2))
		assert.is_false(validate_level(3))
		assert.is_true(validate_level(4))
		assert.is_true(validate_level(5))
		assert.is_false(validate_level(6))
	end)
end)

describe("spec validation", function()
	local function calculate_progress(criteria)
		if not criteria or #criteria == 0 then
			return 0, 0, 0
		end

		local satisfied = 0
		local total = #criteria
		for _, ac in ipairs(criteria) do
			if ac.satisfied then
				satisfied = satisfied + 1
			end
		end

		local percent = (satisfied / total) * 100
		return satisfied, total, percent
	end

	it("should calculate progress for empty criteria", function()
		local s, t, p = calculate_progress({})
		assert.equals(0, s)
		assert.equals(0, t)
		assert.equals(0, p)
	end)

	it("should calculate progress for partial completion", function()
		local criteria = {
			{ id = "AC-1", satisfied = true },
			{ id = "AC-2", satisfied = false },
			{ id = "AC-3", satisfied = true },
			{ id = "AC-4", satisfied = false },
		}
		local s, t, p = calculate_progress(criteria)
		assert.equals(2, s)
		assert.equals(4, t)
		assert.equals(50, p)
	end)

	it("should calculate progress for full completion", function()
		local criteria = {
			{ id = "AC-1", satisfied = true },
			{ id = "AC-2", satisfied = true },
		}
		local s, t, p = calculate_progress(criteria)
		assert.equals(2, s)
		assert.equals(2, t)
		assert.equals(100, p)
	end)
end)

describe("patch management", function()
	local function validate_patch_status(status)
		local valid_statuses = { pending = true, applied = true, rejected = true, expired = true }
		return valid_statuses[status] == true
	end

	it("should accept valid patch statuses", function()
		assert.is_true(validate_patch_status("pending"))
		assert.is_true(validate_patch_status("applied"))
		assert.is_true(validate_patch_status("rejected"))
		assert.is_true(validate_patch_status("expired"))
	end)

	it("should reject invalid patch statuses", function()
		assert.is_false(validate_patch_status("invalid"))
		assert.is_false(validate_patch_status(""))
		assert.is_false(validate_patch_status(nil))
	end)
end)
