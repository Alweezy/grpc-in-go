-- name: CreateTask :one
INSERT INTO tasks (type, value, state)
VALUES ($1, $2, 'received')
RETURNING id;

-- name: UpdateTaskState :exec
UPDATE tasks SET state = $2, last_update_time = CURRENT_TIMESTAMP WHERE id = $1;

-- name: GetTaskByID :one
SELECT * FROM tasks WHERE id = $1;

-- name: GetTasksByState :many
SELECT * FROM tasks WHERE state = $1;