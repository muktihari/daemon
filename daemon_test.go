package daemon_test

import (
	"context"
	"errors"
	"github.com/muktihari/daemon"
	"testing"
	"time"
)

func TestDaemon_Run(t *testing.T) {
	d := daemon.New()
	defer d.Stop()
	d.SetNumberOfWorker(4)
	d.AddTask(
		daemon.NewDummyTask("Task #1", daemon.TaskTypeMandatorySync),
		daemon.NewDummyTask("Task #2", daemon.TaskTypeSync),
		daemon.NewDummyTask("Task #3", daemon.TaskTypeAsync),
		daemon.NewDummyTask("Task #4", daemon.TaskTypeAsync),
	)

	if err := d.Run(context.Background()); err != nil {
		t.Fatalf("should be nil, got %v", err)
	}
}

func TestDaemon_Run_Canceled(t *testing.T) {
	d := daemon.New()
	defer d.Stop()
	d.SetNumberOfWorker(4)
	d.AddTask(
		daemon.NewDummyTask("Task #1", daemon.TaskTypeMandatorySync),
		daemon.NewDummyTask("Task #2", daemon.TaskTypeSync),
		daemon.NewDummyTask("Task #3", daemon.TaskTypeAsync),
		daemon.NewDummyTask("Task #4", daemon.TaskTypeAsync),
	)

	go func() {
		<-time.After(15 * time.Second)
		d.Stop()
	}()

	if err := d.Run(context.Background()); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		t.Fatalf("should be %v, got %v", context.DeadlineExceeded, err)
	}
}

func TestDaemon_Run_DeadlineExceeded(t *testing.T) {
	d := daemon.New()
	defer d.Stop()
	d.SetNumberOfWorker(4)
	d.AddTask(
		daemon.NewDummyTask("Task #1", daemon.TaskTypeMandatorySync),
		daemon.NewDummyTask("Task #2", daemon.TaskTypeSync),
		daemon.NewDummyTask("Task #3", daemon.TaskTypeAsync),
		daemon.NewDummyTask("Task #4", daemon.TaskTypeAsync),
	)
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	if err := d.Run(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return
		}
		t.Fatalf("should be %v, got %v", context.DeadlineExceeded, err)
	}
}
