package daemon_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/muktihari/daemon"
	"github.com/muktihari/daemon/dummy"
	"github.com/muktihari/daemon/task"
)

func TestDaemon_Run_Once(t *testing.T) {
	d := daemon.New()
	defer d.Stop()

	d.AddTask("Task #1-A", task.MandatorySync, dummy.NewTask())
	d.AddTask("Task #1-B", task.MandatorySync, dummy.NewTask())
	d.AddTask("Task #2-A", task.MandatoryAsync, dummy.NewTask())
	d.AddTask("Task #2-B", task.MandatoryAsync, dummy.NewTask())
	d.AddTask("Task #3-A", task.Sync, dummy.NewTask())
	d.AddTask("Task #3-B", task.Sync, dummy.NewTask())
	d.AddTask("Task #4-A", task.Async, dummy.NewTask())
	d.AddTask("Task #4-B", task.Async, dummy.NewTask())

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("should be nil, got %v", err)
	}
}

func TestDaemon_Run_Canceled(t *testing.T) {
	d := daemon.New()
	defer d.Stop()
	d.Workers(4)
	d.AddTask("Task #1", task.Sync, dummy.NewTask())
	d.AddTask("Task #2", task.Sync, dummy.NewTask())
	d.AddTask("Task #3", task.Async, dummy.NewTask())
	d.AddTask("Task #4", task.Async, dummy.NewTask())

	go func() {
		<-time.After(15 * time.Second)
		d.Stop()
	}()

	if err := d.Run(context.Background()); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		t.Fatalf("should be %v, got %v", context.Canceled, err)
	}
}

func TestDaemon_Run_DeadlineExceeded(t *testing.T) {
	d := daemon.New()
	defer d.Stop()
	d.Workers(4)
	d.AddTask("Task #1", task.Sync, dummy.NewTask())
	d.AddTask("Task #2", task.Sync, dummy.NewTask())
	d.AddTask("Task #3", task.Async, dummy.NewTask())
	d.AddTask("Task #4", task.Async, dummy.NewTask())

	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	if err := d.Run(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		t.Fatalf("should be %v, got %v", context.DeadlineExceeded, err)
	}
}
