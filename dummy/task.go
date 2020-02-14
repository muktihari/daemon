package dummy

import (
	"context"
	"fmt"
	"time"

	"github.com/muktihari/daemon/task"
)

type dummy struct{}

// NewTask create new dummy task
func NewTask() task.Task {
	return &dummy{}
}

func (t *dummy) Run(ctx context.Context) error {
	// do the job: printing text in 1 second interval
	start := time.Now()
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			fmt.Printf("dummy task is running in %s\n", time.Now().Sub(start))
		}
	}
	return nil
}
