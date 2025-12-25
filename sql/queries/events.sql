-- name: CreateEvent :one
INSERT INTO events (user_id, event_type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListEventsByUser :many
SELECT * FROM events
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListEventsByType :many
SELECT * FROM events
WHERE event_type = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountEventsByType :one
SELECT COUNT(*) FROM events
WHERE event_type = $1
AND created_at >= $2;
