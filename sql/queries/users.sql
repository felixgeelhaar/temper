-- name: CreateUser :one
INSERT INTO users (email, name, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg(name), name),
    email = COALESCE(sqlc.narg(email), email)
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSessionByToken :one
SELECT * FROM sessions
WHERE token = $1 AND expires_at > NOW();

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW();

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE user_id = $1;
