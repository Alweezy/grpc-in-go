// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: queries.sql

package persistence

import (
	"context"
	"database/sql"
)

const createTask = `-- name: CreateTask :one
INSERT INTO tasks (type, value, state)
VALUES ($1, $2, 'received')
RETURNING id
`

type CreateTaskParams struct {
	Type  sql.NullInt32 `json:"type"`
	Value sql.NullInt32 `json:"value"`
}

func (q *Queries) CreateTask(ctx context.Context, arg CreateTaskParams) (int32, error) {
	row := q.db.QueryRowContext(ctx, createTask, arg.Type, arg.Value)
	var id int32
	err := row.Scan(&id)
	return id, err
}

const getTaskByID = `-- name: GetTaskByID :one
SELECT id, type, value, state, creation_time, last_update_time FROM tasks WHERE id = $1
`

func (q *Queries) GetTaskByID(ctx context.Context, id int32) (Task, error) {
	row := q.db.QueryRowContext(ctx, getTaskByID, id)
	var i Task
	err := row.Scan(
		&i.ID,
		&i.Type,
		&i.Value,
		&i.State,
		&i.CreationTime,
		&i.LastUpdateTime,
	)
	return i, err
}

const getTasksByState = `-- name: GetTasksByState :many
SELECT id, type, value, state, creation_time, last_update_time FROM tasks WHERE state = $1
`

func (q *Queries) GetTasksByState(ctx context.Context, state sql.NullString) ([]Task, error) {
	rows, err := q.db.QueryContext(ctx, getTasksByState, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Task
	for rows.Next() {
		var i Task
		if err := rows.Scan(
			&i.ID,
			&i.Type,
			&i.Value,
			&i.State,
			&i.CreationTime,
			&i.LastUpdateTime,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateTaskState = `-- name: UpdateTaskState :exec
UPDATE tasks SET state = $2, last_update_time = CURRENT_TIMESTAMP WHERE id = $1
`

type UpdateTaskStateParams struct {
	ID    int32          `json:"id"`
	State sql.NullString `json:"state"`
}

func (q *Queries) UpdateTaskState(ctx context.Context, arg UpdateTaskStateParams) error {
	_, err := q.db.ExecContext(ctx, updateTaskState, arg.ID, arg.State)
	return err
}
