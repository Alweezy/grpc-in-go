package persistence

import (
	"context"
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestCreateTask ensures that tasks are properly created in the database using sqlmock
func TestCreateTask(t *testing.T) {
	// Create a mock DB connection
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Initialize Queries object with the mock DB
	queries := New(db)

	// Define input parameters for CreateTask
	taskType := sql.NullInt32{Int32: 2, Valid: true}
	taskValue := sql.NullInt32{Int32: 50, Valid: true}
	ctx := context.Background()

	// Set up the expected SQL execution and return the generated ID (e.g., 1)
	mock.ExpectQuery("INSERT INTO tasks").
		WithArgs(taskType, taskValue).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1)) // Returning the generated ID

	// Call the CreateTask method
	taskID, err := queries.CreateTask(ctx, CreateTaskParams{Type: taskType, Value: taskValue})
	assert.NoError(t, err)
	assert.Equal(t, int32(1), taskID) // Ensure the returned ID is correct

	// Ensure all mock expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateTaskState ensures that the task state is properly updated in the database using sqlmock
func TestUpdateTaskState(t *testing.T) {
	// Create a mock DB connection
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Initialize Queries object with the mock DB
	queries := New(db)

	// Define input parameters for UpdateTaskState
	taskID := int32(1)
	taskState := sql.NullString{String: "done", Valid: true}
	ctx := context.Background()

	// Set up the expected SQL execution
	mock.ExpectExec("UPDATE tasks SET state").
		WithArgs(taskID, taskState).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the UpdateTaskState method
	err = queries.UpdateTaskState(ctx, UpdateTaskStateParams{ID: taskID, State: taskState})
	assert.NoError(t, err)

	// Ensure all mock expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTaskByID ensures that the task can be retrieved from the database using sqlmock
func TestGetTaskByID(t *testing.T) {
	// Create a mock DB connection
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Initialize Queries object with the mock DB
	queries := New(db)

	// Define expected task data
	taskID := int32(1)
	taskType := sql.NullInt32{Int32: 2, Valid: true}
	taskValue := sql.NullInt32{Int32: 50, Valid: true}
	taskState := sql.NullString{String: "received", Valid: true}

	// Define the expected SQL query and result
	mock.ExpectQuery("SELECT id, type, value, state, creation_time, last_update_time FROM tasks WHERE id").
		WithArgs(taskID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "value", "state", "creation_time", "last_update_time"}).
			AddRow(taskID, taskType.Int32, taskValue.Int32, taskState.String, nil, nil))

	// Call the GetTaskByID method
	ctx := context.Background()
	task, err := queries.GetTaskByID(ctx, taskID)
	assert.NoError(t, err)

	// Validate the returned task
	assert.Equal(t, taskID, task.ID)
	assert.Equal(t, taskType.Int32, task.Type.Int32)
	assert.Equal(t, taskValue.Int32, task.Value.Int32)

	// Compare the string value inside taskState, not the struct
	assert.True(t, task.State.Valid)                     // Ensure the state is valid
	assert.Equal(t, taskState.String, task.State.String) // Compare the string values

	// Ensure all mock expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetTasksByState ensures that tasks can be retrieved by state using sqlmock
func TestGetTasksByState(t *testing.T) {
	// Create a mock DB connection
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Initialize Queries object with the mock DB
	queries := New(db)

	// Define expected task data
	taskState := sql.NullString{String: "received", Valid: true}
	taskID := int32(1)
	taskType := sql.NullInt32{Int32: 2, Valid: true}
	taskValue := sql.NullInt32{Int32: 50, Valid: true}

	// Set up the expected SQL query and result
	mock.ExpectQuery("SELECT id, type, value, state, creation_time, last_update_time FROM tasks WHERE state").
		WithArgs(taskState.String). // Pass the actual string value, not sql.NullString
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "value", "state", "creation_time", "last_update_time"}).
			AddRow(taskID, taskType.Int32, taskValue.Int32, taskState.String, nil, nil))

	// Call the GetTasksByState method
	ctx := context.Background()
	tasks, err := queries.GetTasksByState(ctx, taskState)
	assert.NoError(t, err)

	// Validate the returned tasks
	assert.Len(t, tasks, 1)
	assert.Equal(t, taskID, tasks[0].ID)
	assert.Equal(t, taskType.Int32, tasks[0].Type.Int32)
	assert.Equal(t, taskValue.Int32, tasks[0].Value.Int32)

	// Compare the string value inside taskState, not the struct
	assert.True(t, tasks[0].State.Valid)                     // Ensure the state is valid
	assert.Equal(t, taskState.String, tasks[0].State.String) // Compare the string values

	// Ensure all mock expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
