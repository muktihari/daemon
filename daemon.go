package daemon

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// Status is daemon status type
type Status string

const (
	//IsRunning is the status of running daemon.
	IsRunning Status = "Daemon is Running"

	//IsStopped is the status of stopped daemon.
	IsStopped Status = "Daemon is Stopped"
)

// Daemon is the daemon interface.
type Daemon interface {
	// AddTask adds a number of tasks to be run by the daemon.
	AddTask(...Task) Daemon

	// SetNumberOfWorker set number of workers (concurrent process).
	SetNumberOfWorker(int) Daemon

	// SetTaskRetryAfterFail set duration to wait before retry running the tasks.
	SetTaskRetryAfterFail(time.Duration) Daemon

	// UnsetTaskRetryAfterFail disables retry after fail
	UnsetTaskRetryAfterFail() Daemon

	// Run runs the daemon once then exit when done or when the context is returning context.DeadlineExceed or context.Canceled.
	// It will run TaskTypeMandatorySync tasks before others, return immediately when error.
	// Then run both TaskTypeSync and TaskTypeAsync tasks simultaneously by remaining available workers.
	// While TaskTypeSync will be run in sequential order, the TaskTypeAsync will be run concurrently.
	Run(context.Context) error

	// RunWithInterval will call the Run function periodically based on the given time interval.
	RunWithInterval(context.Context, time.Duration) error

	// RunATask runs a single task once.
	RunATask(context.Context, string) error

	// Stop stops the running daemon by giving the cancel signal for the given context.
	// A task should listen to the context propagation to determine task's cancellation.
	Stop()

	// StopATask stops a single running task by the given name.
	StopATask(string)

	// PostponeATask prevent task from being executed the next time Run is triggered
	PostponeATask(string)

	// PostponeATask resume postponed task to be executed the next time Run is triggered
	ResumeATask(string)

	// GetStatus gets the status of the daemon.
	GetStatus() Status
}

type daemon struct {
	mu                 sync.RWMutex
	log                *log.Logger
	ctx                context.Context
	cancel             context.CancelFunc
	nWorker            int
	taskRetryAfterFail bool
	taskRetryDuration  time.Duration
	status             Status
	tasks              []Task
}

// Result is result structure of the executed task by the worker
type Result struct {
	Task Task
	Err  error
}

// New creates new Daemon instance, retrieve optional logrus.Logger for custom logging process (only first logger will be used).
func New(loggers ...*log.Logger) Daemon {
	if len(loggers) == 0 {
		loggers = append(loggers, &log.Logger{
			Formatter: &log.TextFormatter{
				ForceColors:     true,
				FullTimestamp:   true,
				TimestampFormat: "2006/01/02 15:04:05",
			},
		})
	}
	return &daemon{
		log:     loggers[0],
		nWorker: 1,
	}
}

func (d *daemon) SetNumberOfWorker(n int) Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nWorker = 1
	if n > 1 {
		d.nWorker = n
	}
	return d
}

func (d *daemon) SetTaskRetryAfterFail(duration time.Duration) Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.taskRetryAfterFail = true
	d.taskRetryDuration = duration
	return d
}

func (d *daemon) UnsetTaskRetryAfterFail() Daemon {
	d.mu.Lock()
	d.taskRetryAfterFail = false
	d.mu.Unlock()

	return d
}

func (d *daemon) AddTask(tasks ...Task) Daemon {
	d.mu.Lock()
	d.tasks = append(d.tasks, tasks...)
	d.mu.Unlock()

	return d
}

func (d *daemon) setContextCancellation(ctx context.Context) {
	d.mu.Lock()
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.mu.Unlock()
}

func (d *daemon) Run(ctx context.Context) error {
	d.setContextCancellation(ctx)
	d.setStatus(IsRunning)
	defer d.Stop()

	var mandatorySyncTasks, syncTasks, asyncTasks []Task
	for _, task := range d.tasks {
		if task.GetStatus() == TaskStatusIsPostponed {
			continue
		} else if task.GetType() == TaskTypeMandatorySync {
			mandatorySyncTasks = append(mandatorySyncTasks, task)
		} else if task.GetType() == TaskTypeSync {
			syncTasks = append(syncTasks, task)
		} else {
			asyncTasks = append(asyncTasks, task)
		}
	}

	if err := d.runner(mandatorySyncTasks); err != nil {
		return err
	}

	errCh := make(chan error, 1+len(syncTasks))
	go func(errsCh chan<- error) {
		for _, task := range syncTasks {
			errsCh <- d.runner([]Task{task})
		}
	}(errCh)

	go func(errsCh chan<- error) {
		errsCh <- d.runner(asyncTasks)
	}(errCh)

	var errs []error
	for i := 0; i < 1+len(syncTasks); i++ {
		if err := <-errCh; err != nil {
			errs = append(errs, err)
		}
	}
	close(errCh)

	if len(errs) != 0 {
		select {
		case <-d.ctx.Done():
			return &ErrDaemon{Parent: d.ctx.Err(), Childs: errs}
		default:
		}
		return &ErrDaemon{Parent: ErrSomeRunnersFailed, Childs: errs}
	}

	return nil
}

func (d *daemon) runner(tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}
	taskCh := make(chan Task, len(tasks))
	resultCh := make(chan Result, len(tasks))

	nWorker := d.nWorker
	if nWorker > len(tasks) {
		nWorker = len(tasks)
	}

	for i := 0; i < nWorker; i++ {
		go d.worker(taskCh, resultCh)
	}

	for _, task := range tasks {
		taskCh <- task
	}
	close(taskCh)

	var retryTasks []Task
	var errs []error
	for i := 0; i < len(tasks); i++ {
		result := <-resultCh
		if result.Err != nil {
			retryTasks = append(retryTasks, result.Task)
			errs = append(errs, result.Err)
		}
	}
	close(resultCh)

	if len(retryTasks) != 0 {
		if d.taskRetryAfterFail == false {
			select {
			case <-d.ctx.Done():
				return &ErrDaemon{Parent: d.ctx.Err(), Childs: errs}
			default:
			}
			return &ErrDaemon{Parent: ErrSomeTasksFailed, Childs: errs}
		}
		for _, err := range errs {
			d.log.Warn(err)
		}

		taskType := retryTasks[0].GetType()
		if taskType == "" {
			taskType = TaskTypeAsync
		}
		d.log.Infof("[%s] fail %d of %d tasks, retrying...\n", taskType, len(retryTasks), len(tasks))
		select {
		case <-d.ctx.Done():
			return &ErrDaemon{Parent: d.ctx.Err(), Childs: errs}
		case <-time.After(d.taskRetryDuration):
			return d.runner(retryTasks)
		}
	}

	return nil
}

func (d *daemon) worker(tasks <-chan Task, results chan<- Result) {
	for task := range tasks {
		results <- Result{
			Task: task,
			Err:  task.Run(d.ctx),
		}
	}
}

func (d *daemon) RunWithInterval(ctx context.Context, interval time.Duration) error {
	d.log.WithField("daemon", "RunWithInterval")
	ticker := time.NewTicker(interval)

	for {
		startTime := time.Now()
		d.log.Info("Started")
		if err := d.Run(ctx); err != nil {
			return err
		}
		d.log.Infof("Completed in %v", time.Now().Sub(startTime))
		d.log.Infof("Waiting")

		<-ticker.C
	}
}

func (d *daemon) RunATask(ctx context.Context, name string) error {
	for _, task := range d.tasks {
		if task.GetName() == name {
			return task.Run(ctx)
		}
	}
	return nil
}

func (d *daemon) Stop() {
	if d.GetStatus() == IsRunning {
		d.cancel()
	}

	d.setStatus(IsStopped)
}

func (d *daemon) StopATask(name string) {
	for i := range d.tasks {
		if d.tasks[i].GetName() == name {
			d.tasks[i].Stop()
			return
		}
	}
}

func (d *daemon) PostponeATask(name string) {
	for i := range d.tasks {
		if d.tasks[i].GetName() == name {
			d.tasks[i].Postpone()
			return
		}
	}
}

func (d *daemon) ResumeATask(name string) {
	for i := range d.tasks {
		if d.tasks[i].GetName() == name {
			d.tasks[i].Resume()
			return
		}
	}
}

func (d *daemon) setStatus(status Status) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.status = status
}

func (d *daemon) GetStatus() Status {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.status
}
