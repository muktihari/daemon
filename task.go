package daemon

import (
	"context"
)

// TaskStatus is the task status type
type TaskStatus string

const (
	// TaskStatusIsRunning is the status of the running task.
	TaskStatusIsRunning TaskStatus = "Task is Running"

	// TaskStatusIsStopped is the status of the stopped task.
	TaskStatusIsStopped TaskStatus = "Task is Stopped"

	// TaskStatusIsRunning is the status of the postponed task.
	TaskStatusIsPostponed TaskStatus = "Task is Postponed"
)

type TaskType string

const (
	// TaskTypeMandatorySync will be executed in sequential order before TaskSync or TypeAsync and will be waited until finish.
	// Daemon will be stopped if any of this task return error
	TaskTypeMandatorySync TaskType = "Mandatory Sync Task"

	// TaskTypeSync will be executed in sequential order
	TaskTypeSync TaskType = "Sync Task"

	// TaskTypeASync will be executed simultaneously after synchronous tasks.
	TaskTypeAsync TaskType = "Async Task"
)

type Task interface {
	// Run executes the task.
	Run(ctx context.Context) error

	// Stop terminates the running task. Give the context.Cancelled signal.
	Stop()

	// Postpone set status to TaskStatusIsPostponed. The task executor should notice that the task should not be run.
	Postpone()

	// Postpone set status to TaskStatusIsStopped. The task executor should notice that the task is now runnable.
	Resume()

	// GetName gets the name of the task.
	GetName() string

	// GetStatus gets the status of the task.
	GetStatus() TaskStatus

	// GetType gets the type of the task.
	GetType() TaskType
}
