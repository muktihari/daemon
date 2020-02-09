package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type dummyTask struct {
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	name     string
	taskType TaskType
	status   TaskStatus
}

func NewDummyTask(name string, taskType TaskType) Task {
	return &dummyTask{
		name:     name,
		taskType: taskType,
		status:   TaskStatusIsStopped,
	}
}

func (t *dummyTask) setContextPropagation(ctx context.Context) {
	t.mu.Lock()
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.mu.Unlock()
}

func (t *dummyTask) Run(ctx context.Context) error {
	t.setContextPropagation(ctx)
	t.setStatus(TaskStatusIsRunning)
	defer t.Stop()

	// do the job
	start := time.Now()
	for i := 0; i < 10; i++ {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case <-time.After(1 * time.Second):
			fmt.Printf("%s is running in %s\n", t.name, time.Now().Sub(start))
		}
	}
	return nil
}

func (t *dummyTask) Stop() {
	if t.GetStatus() == TaskStatusIsRunning {
		t.cancel()
	}
	t.setStatus(TaskStatusIsStopped)

	fmt.Printf("%s is stopped\n", t.name)
}

func (t *dummyTask) Postpone() {
	t.mu.Lock()
	t.status = TaskStatusIsPostponed
	t.mu.Unlock()
}

func (t *dummyTask) Resume() {
	t.mu.Lock()
	t.status = TaskStatusIsStopped
	t.mu.Unlock()
}

func (t *dummyTask) GetName() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.name
}

func (t *dummyTask) setStatus(status TaskStatus) {
	t.mu.Lock()
	t.status = status
	t.mu.Unlock()
}

func (t *dummyTask) GetStatus() TaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.status
}

func (t *dummyTask) GetType() TaskType {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.taskType
}
