-- name: CreateArtifact :one
INSERT INTO artifacts (user_id, exercise_id, name, content)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetArtifactByID :one
SELECT * FROM artifacts
WHERE id = $1;

-- name: GetArtifactByIDAndUser :one
SELECT * FROM artifacts
WHERE id = $1 AND user_id = $2;

-- name: ListArtifactsByUser :many
SELECT * FROM artifacts
WHERE user_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArtifactsByExercise :many
SELECT * FROM artifacts
WHERE user_id = $1 AND exercise_id = $2
ORDER BY updated_at DESC;

-- name: UpdateArtifact :one
UPDATE artifacts
SET name = COALESCE(sqlc.narg(name), name),
    content = COALESCE(sqlc.narg(content), content)
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UpdateArtifactContent :one
UPDATE artifacts
SET content = $2
WHERE id = $1
RETURNING *;

-- name: DeleteArtifact :exec
DELETE FROM artifacts
WHERE id = $1 AND user_id = $2;

-- name: CreateArtifactVersion :one
INSERT INTO artifact_versions (artifact_id, version, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetArtifactVersion :one
SELECT * FROM artifact_versions
WHERE artifact_id = $1 AND version = $2;

-- name: ListArtifactVersions :many
SELECT * FROM artifact_versions
WHERE artifact_id = $1
ORDER BY version DESC
LIMIT $2;

-- name: GetLatestArtifactVersion :one
SELECT * FROM artifact_versions
WHERE artifact_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: CountArtifactVersions :one
SELECT COUNT(*) FROM artifact_versions
WHERE artifact_id = $1;
