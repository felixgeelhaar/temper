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

return M
