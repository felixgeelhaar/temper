-- name: CreateLearningProfile :one
INSERT INTO learning_profiles (user_id)
VALUES ($1)
RETURNING *;

-- name: GetLearningProfile :one
SELECT * FROM learning_profiles
WHERE user_id = $1;

-- name: UpsertLearningProfile :one
INSERT INTO learning_profiles (user_id, topic_skills, total_exercises, total_runs, hint_requests)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE
SET topic_skills = EXCLUDED.topic_skills,
    total_exercises = EXCLUDED.total_exercises,
    total_runs = EXCLUDED.total_runs,
    hint_requests = EXCLUDED.hint_requests
RETURNING *;

-- name: UpdateLearningProfileStats :one
UPDATE learning_profiles
SET total_exercises = COALESCE(sqlc.narg(total_exercises), total_exercises),
    total_runs = COALESCE(sqlc.narg(total_runs), total_runs),
    hint_requests = COALESCE(sqlc.narg(hint_requests), hint_requests),
    avg_time_to_green_ms = COALESCE(sqlc.narg(avg_time_to_green_ms), avg_time_to_green_ms),
    common_errors = COALESCE(sqlc.narg(common_errors), common_errors)
WHERE user_id = $1
RETURNING *;

-- name: IncrementProfileRuns :one
UPDATE learning_profiles
SET total_runs = total_runs + 1
WHERE user_id = $1
RETURNING *;

-- name: IncrementProfileHints :one
UPDATE learning_profiles
SET hint_requests = hint_requests + 1
WHERE user_id = $1
RETURNING *;

-- name: UpdateTopicSkills :one
UPDATE learning_profiles
SET topic_skills = $2
WHERE user_id = $1
RETURNING *;
