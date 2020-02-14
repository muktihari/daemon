package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/muktihari/daemon/task"

	log "github.com/sirupsen/logrus"
)

// Status is daemon status type
type Status string

const (
	// IsRunning is the status of running daemon.
	IsRunning Status = "Daemon is Running"

	// IsStopped is the status of stopped daemon.
	IsStopped Status = "Daemon is Stopped"
)

// Daemon is the daemon interface.
type Daemon interface {
	// AddTask adds a number of tasks to be run by the daemon.
	AddTask(name string, taskType task.Type, theTask task.Task) Daemon

	// Workers set number of workers (concurrent process). default is 0, means workers is as many as the number of the tasks.
	Workers(int) Daemon

	// RetryIn set duration to wait before retriable running the tasks.
	RetryIn(time.Duration) Daemon

	// DoNotRetry disables retriable after fail
	DoNotRetry() Daemon

	// Run executes the daemon once then exit when done or when the context is returning context.DeadlineExceed or context.Canceled.
	// It will run MandatorySync tasks before others, return immediately when error.
	// Then run MandatoryAsync before Sync and Async, return immediately when error.
	// Finally run both Sync and Async tasks simultaneously by remaining available workers.
	// While Sync will be run in sequential order, the Async will be run concurrently.
	// Once daemon is set to IsRunning, all running tasks from RunATask will be waited until it's done before continue running the daemon.
	// All new triggered tasks from RunATask will be waiting until the daemon is done, then will be run after that.
	Run(context.Context) error

	// RunWithInterval will call the Run function periodically based on the given time interval.
	RunWithInterval(context.Context, time.Duration) error

	// RunATask executes a single task once. If daemon status is still running, it will be waiting until it finish.
	RunATask(context.Context, string) error

	// Stop terminates the running daemon by giving the cancel signal for the given context, all task will be canceled.
	Stop()

	// StopATask terminates a single running task by the given name.
	StopATask(string)

	// PostponeATask prevent task from being executed the next time Run is triggered
	PostponeATask(string)

	// PostponeATask resumes postponed task to be executed the next time Run is triggered
	ResumeATask(string)

	// GetStatus gets the status of the daemon.
	GetStatus() Status
}

type daemon struct {
	mu            sync.RWMutex
	log           *log.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	nWorker       int
	retriable     bool
	retryDuration time.Duration
	status        Status
	taskRunners   []taskRunner
}

type taskRunner struct {
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	name     string
	taskType task.Type
	status   task.Status
	task     task.Task
}

func (t *taskRunner) setContextCancellation(ctx context.Context) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.ctx, t.cancel = context.WithCancel(ctx)
}

func (t *taskRunner) setStatus(status task.Status) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status = status
}

type result struct {
	Task taskRunner
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
		log: loggers[0],
	}
}

func (d *daemon) Workers(n int) Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nWorker = 0
	if n > 0 {
		d.nWorker = n
	}
	return d
}

func (d *daemon) RetryIn(duration time.Duration) Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.retriable = true
	d.retryDuration = duration
	return d
}

func (d *daemon) DoNotRetry() Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.retriable = false
	return d
}

func (d *daemon) AddTask(name string, taskType task.Type, theTask task.Task) Daemon {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.taskRunners = append(d.taskRunners, taskRunner{
		name:     name,
		taskType: taskType,
		task:     theTask,
	})

	return d
}

func (d *daemon) setContextCancellation(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ctx, d.cancel = context.WithCancel(ctx)
}

func (d *daemon) Run(ctx context.Context) error {
	d.setContextCancellation(ctx)
	d.setStatus(IsRunning)
	defer d.Stop()

	for i := range d.taskRunners {
		if d.taskRunners[i].status == task.IsRunning {
			<-d.taskRunners[i].ctx.Done()
		}
	}

	var mandatorySyncTasks, mandatoryAsyncTasks, syncTasks, asyncTasks []taskRunner
	for i := range d.taskRunners {
		d.taskRunners[i].setContextCancellation(d.ctx)
		if d.taskRunners[i].status == task.IsPostponed {
			continue
		} else if d.taskRunners[i].taskType == task.MandatorySync {
			mandatorySyncTasks = append(mandatorySyncTasks, d.taskRunners[i])
		} else if d.taskRunners[i].taskType == task.MandatoryAsync {
			mandatoryAsyncTasks = append(mandatoryAsyncTasks, d.taskRunners[i])
		} else if d.taskRunners[i].taskType == task.Sync {
			syncTasks = append(syncTasks, d.taskRunners[i])
		} else {
			asyncTasks = append(asyncTasks, d.taskRunners[i])
		}
	}

	for _, mandatorySyncTask := range mandatorySyncTasks {
		if err := d.runner([]taskRunner{mandatorySyncTask}); err != nil {
			return err
		}
	}

	if err := d.runner(mandatoryAsyncTasks); err != nil {
		return err
	}

	errCh := make(chan error, 1+len(syncTasks))
	go func(errsCh chan<- error) {
		for i := range syncTasks {
			errsCh <- d.runner([]taskRunner{syncTasks[i]})
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

func (d *daemon) runner(tasks []taskRunner) error {
	if len(tasks) == 0 {
		return nil
	}
	taskCh := make(chan taskRunner, len(tasks))
	resultCh := make(chan result, len(tasks))

	nWorker := d.nWorker
	if nWorker > len(tasks) || nWorker == 0 {
		nWorker = len(tasks)
	}

	for i := 0; i < nWorker; i++ {
		go d.worker(taskCh, resultCh)
	}

	for i := range tasks {
		tasks[i].setStatus(task.IsRunning)
		taskCh <- tasks[i]
	}
	close(taskCh)

	var retryTasks []taskRunner
	var errs []error
	for i := 0; i < len(tasks); i++ {
		result := <-resultCh
		result.Task.setStatus(task.IsStopped)
		if result.Err != nil {
			retryTasks = append(retryTasks, result.Task)
			errs = append(errs, result.Err)
		}
	}
	close(resultCh)

	if len(retryTasks) != 0 {
		if d.retriable == false {
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

		taskType := retryTasks[0].taskType
		if taskType == "" {
			taskType = task.Async
		}
		d.log.Infof("[%s] fail %d of %d tasks, retrying in %s...\n", taskType, len(retryTasks), len(tasks), d.retryDuration)
		select {
		case <-d.ctx.Done():
			return &ErrDaemon{Parent: d.ctx.Err(), Childs: errs}
		case <-time.After(d.retryDuration):
			return d.runner(retryTasks)
		}
	}

	return nil
}

func (d *daemon) worker(taskRunners <-chan taskRunner, results chan<- result) {
	for tr := range taskRunners {
		results <- result{
			Task: tr,
			Err:  tr.task.Run(tr.ctx),
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

		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

func (d *daemon) RunATask(ctx context.Context, name string) error {
	select {
	case <-d.ctx.Done():
		for _, tr := range d.taskRunners {
			if tr.name == name {
				tr.setContextCancellation(ctx)
				return tr.task.Run(tr.ctx)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *daemon) Stop() {
	if d.GetStatus() == IsRunning {
		d.cancel()
	}
	d.setStatus(IsStopped)
}

func (d *daemon) StopATask(name string) {
	select {
	case <-d.ctx.Done():
		for i := range d.taskRunners {
			if d.taskRunners[i].name == name {
				d.taskRunners[i].cancel()
				return
			}
		}
	}
}

func (d *daemon) PostponeATask(name string) {
	for i := range d.taskRunners {
		if d.taskRunners[i].name == name {
			d.taskRunners[i].setStatus(task.IsPostponed)
			return
		}
	}
}

func (d *daemon) ResumeATask(name string) {
	for i := range d.taskRunners {
		if d.taskRunners[i].name == name {
			d.taskRunners[i].setStatus(task.IsStopped)
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
