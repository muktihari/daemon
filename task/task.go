package task

import (
	"context"
)

// Status is the task status type
type Status string

const (
	// IsRunning is the status of the running task.
	IsRunning Status = "Task is Running"

	// IsStopped is the status of the stopped task.
	IsStopped Status = "Task is Stopped"

	// IsPostponed is the status of the postponed task.
	IsPostponed Status = "Task is Postponed"
)

// Type is the task type
type Type string

const (
	// MandatorySync will be executed in sequential order before any other types and will be waited until finish.
	// Daemon will be stopped if any of this task return error
	MandatorySync Type = "Mandatory Sync Task"

	// MandatoryAsync will be executed after MandatorySync but before TaskSync and TypeAsync. It will be waited until finish.
	// Daemon will be stopped if any of this task return error
	MandatoryAsync Type = "Mandatory Async Task"

	// Sync will be executed in sequential order
	Sync Type = "Sync Task"

	// Async will be executed simultaneously after synchronous tasks.
	Async Type = "Async Task"
)

// Task is the task interface that satisfy the daemon runner.
type Task interface {
	// Run executes the task with given context and return an error once done.
	Run(ctx context.Context) error
}
