-- name: CreatePairingSession :one
INSERT INTO pairing_sessions (user_id, artifact_id, exercise_id, policy)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPairingSession :one
SELECT * FROM pairing_sessions
WHERE id = $1;

-- name: GetActivePairingSession :one
SELECT * FROM pairing_sessions
WHERE user_id = $1 AND ended_at IS NULL
ORDER BY started_at DESC
LIMIT 1;

-- name: EndPairingSession :one
UPDATE pairing_sessions
SET ended_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateIntervention :one
INSERT INTO interventions (
    session_id, user_id, run_id, intent, level, type,
    content, targets, rationale, requested_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetInterventionByID :one
SELECT * FROM interventions
WHERE id = $1;

-- name: ListInterventionsBySession :many
SELECT * FROM interventions
WHERE session_id = $1
ORDER BY delivered_at DESC
LIMIT $2;

-- name: ListInterventionsByUser :many
SELECT * FROM interventions
WHERE user_id = $1
ORDER BY delivered_at DESC
LIMIT $2 OFFSET $3;

-- name: CountInterventionsBySession :one
SELECT COUNT(*) FROM interventions
WHERE session_id = $1;

-- name: GetLastInterventionBySession :one
SELECT * FROM interventions
WHERE session_id = $1
ORDER BY delivered_at DESC
LIMIT 1;
