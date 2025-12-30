-- Temper Daemon Client
-- HTTP client for communicating with the temperd daemon

local M = {}

-- Configuration
M.config = {
	host = "127.0.0.1",
	port = 7432,
	timeout = 30000, -- ms
}

-- Build base URL
local function base_url()
	return string.format("http://%s:%d", M.config.host, M.config.port)
end

-- Make HTTP request using curl (works on all platforms)
local function request(method, path, body, callback)
	local url = base_url() .. path
	local cmd = { "curl", "-s", "-X", method }

	-- Add headers
	table.insert(cmd, "-H")
	table.insert(cmd, "Content-Type: application/json")

	-- Add body if present
	if body then
		table.insert(cmd, "-d")
		table.insert(cmd, vim.fn.json_encode(body))
	end

	-- Add timeout
	table.insert(cmd, "--max-time")
	table.insert(cmd, tostring(M.config.timeout / 1000))

	table.insert(cmd, url)

	-- Execute asynchronously
	vim.fn.jobstart(cmd, {
		stdout_buffered = true,
		on_stdout = function(_, data)
			if data and #data > 0 and data[1] ~= "" then
				local ok, result = pcall(vim.fn.json_decode, table.concat(data, ""))
				if ok then
					callback(nil, result)
				else
					callback("Failed to parse response: " .. table.concat(data, ""), nil)
				end
			end
		end,
		on_stderr = function(_, data)
			if data and #data > 0 and data[1] ~= "" then
				callback(table.concat(data, "\n"), nil)
			end
		end,
		on_exit = function(_, code)
			if code ~= 0 then
				callback("Request failed with code: " .. code, nil)
			end
		end,
	})
end

-- Health check
function M.health(callback)
	request("GET", "/v1/health", nil, callback)
end

-- Get daemon status
function M.status(callback)
	request("GET", "/v1/status", nil, callback)
end

-- List exercises
function M.list_exercises(callback)
	request("GET", "/v1/exercises", nil, callback)
end

-- Get exercise details
function M.get_exercise(pack, slug, callback)
	local path = string.format("/v1/exercises/%s/%s", pack, slug)
	request("GET", path, nil, callback)
end

-- Create session
function M.create_session(exercise_id, track, callback)
	local body = { exercise_id = exercise_id }
	if track then
		body.track = track
	end
	request("POST", "/v1/sessions", body, callback)
end

-- Get session
function M.get_session(session_id, callback)
	request("GET", "/v1/sessions/" .. session_id, nil, callback)
end

-- Delete session
function M.delete_session(session_id, callback)
	request("DELETE", "/v1/sessions/" .. session_id, nil, callback)
end

-- Run code in session
function M.run(session_id, code, opts, callback)
	opts = opts or {}
	local body = {
		code = code,
		format = opts.format or true,
		build = opts.build or true,
		test = opts.test or true,
	}
	request("POST", "/v1/sessions/" .. session_id .. "/runs", body, callback)
end

-- Request hint
function M.hint(session_id, code, callback)
	local body = {}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/hint", body, callback)
end

-- Request review
function M.review(session_id, code, callback)
	local body = {}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/review", body, callback)
end

-- Signal stuck
function M.stuck(session_id, code, callback)
	local body = {}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/stuck", body, callback)
end

-- Request next steps
function M.next(session_id, code, callback)
	local body = {}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/next", body, callback)
end

-- Request explanation
function M.explain(session_id, code, callback)
	local body = {}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/explain", body, callback)
end

-- Request explicit escalation (L4/L5)
function M.escalate(session_id, level, justification, code, callback)
	local body = {
		level = level,
		justification = justification,
	}
	if code then
		body.code = code
	end
	request("POST", "/v1/sessions/" .. session_id .. "/escalate", body, callback)
end

-- Format code
function M.format(session_id, code, callback)
	request("POST", "/v1/sessions/" .. session_id .. "/format", { code = code }, callback)
end

-- Check if daemon is running
function M.is_running(callback)
	M.health(function(err, result)
		if err then
			callback(false)
		else
			callback(result and result.status == "healthy")
		end
	end)
end

-- Spec management (Specular format)

-- Create a new spec
function M.create_spec(name, callback)
	request("POST", "/v1/specs", { name = name }, callback)
end

-- List all specs
function M.list_specs(callback)
	request("GET", "/v1/specs", nil, callback)
end

-- Get a spec by path
function M.get_spec(path, callback)
	request("GET", "/v1/specs/file/" .. path, nil, callback)
end

-- Validate a spec
function M.validate_spec(path, callback)
	request("POST", "/v1/specs/validate/" .. path, nil, callback)
end

-- Get spec progress
function M.get_spec_progress(path, callback)
	request("GET", "/v1/specs/progress/" .. path, nil, callback)
end

-- Lock a spec (generate SpecLock)
function M.lock_spec(path, callback)
	request("POST", "/v1/specs/lock/" .. path, nil, callback)
end

-- Get spec drift
function M.get_spec_drift(path, callback)
	request("GET", "/v1/specs/drift/" .. path, nil, callback)
end

-- Mark criterion as satisfied
function M.mark_criterion_satisfied(path, criterion_id, evidence, callback)
	request("PUT", "/v1/specs/criteria/" .. criterion_id, {
		path = path,
		evidence = evidence or "",
	}, callback)
end

-- Analytics/Stats

-- Get analytics overview
function M.get_stats_overview(callback)
	request("GET", "/v1/analytics/overview", nil, callback)
end

-- Get skills breakdown
function M.get_stats_skills(callback)
	request("GET", "/v1/analytics/skills", nil, callback)
end

-- Get error patterns
function M.get_stats_errors(callback)
	request("GET", "/v1/analytics/errors", nil, callback)
end

-- Get hint dependency trend
function M.get_stats_trend(callback)
	request("GET", "/v1/analytics/trend", nil, callback)
end

return M
