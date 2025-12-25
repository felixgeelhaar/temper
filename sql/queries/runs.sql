-- name: CreateRun :one
INSERT INTO runs (artifact_id, user_id, exercise_id, status, recipe)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRunByID :one
SELECT * FROM runs
WHERE id = $1;

-- name: GetRunByIDAndUser :one
SELECT * FROM runs
WHERE id = $1 AND user_id = $2;

-- name: ListRunsByArtifact :many
SELECT * FROM runs
WHERE artifact_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListRunsByUser :many
SELECT * FROM runs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListPendingRuns :many
SELECT * FROM runs
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateRunStatus :one
UPDATE runs
SET status = $2,
    started_at = COALESCE(sqlc.narg(started_at), started_at),
    finished_at = COALESCE(sqlc.narg(finished_at), finished_at)
WHERE id = $1
RETURNING *;

-- name: UpdateRunOutput :one
UPDATE runs
SET status = $2,
    output = $3,
    finished_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CountRunsByUser :one
SELECT COUNT(*) FROM runs
WHERE user_id = $1;

-- name: CountSuccessfulRunsByUser :one
SELECT COUNT(*) FROM runs
WHERE user_id = $1
AND status = 'completed'
AND output->>'testsFailed' = '0';
