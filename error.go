package daemon

import (
	"errors"
	"fmt"
)

// ErrSomeTasksFailed indicates that some tasks in a runner return error
var ErrSomeTasksFailed = errors.New("some tasks failed")

// ErrSomeRunnersFailed indicates that some runners return an error because some tasks are return error
var ErrSomeRunnersFailed = errors.New("some runners failed")

// ErrDaemon is an error that hold multiples error from runners -> tasks
type ErrDaemon struct {
	Parent error
	Childs []error
}

func (e *ErrDaemon) Error() string {
	if len(e.Childs) != 0 {
		return fmt.Sprintf("%s: %s", e.Parent, e.Childs)
	}
	return e.Parent.Error()
}

func (e *ErrDaemon) Unwrap() error {
	return e.Parent
}