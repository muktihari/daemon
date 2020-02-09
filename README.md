# daemon

![daemon](https://img.shields.io/badge/version-1.0-blue.svg?style=flat)
![daemon](https://img.shields.io/badge/go-1.13-yellow.svg?style=flat)
[![daemon](https://img.shields.io/badge/godoc-reference-green.svg?style=flat)](https://godoc.org/github.com/muktihari/daemon)

Orchestrate your Tasks in a running daemon you can configure how the task will be run.

The daemon is using context to propagate the cancellation, cancel signal will be sent to the tasks when the daemon is stopped or the context deadline has exceeded. 

#### Getting Started

There are 3 types of Task:

1. TaskTypeMandatorySync
> TaskTypeMandatorySync will be executed in sequential order before TaskSync or TypeAsync and will be waited until finish. Daemon will be stopped if any of this task return error. (using 1 worker) 
2. TaskSync
> TaskTypeSync will be executed in sequential order (using 1 worker)
3. TaskAsync
> TaskTypeASync will be executed concurrently (using remaining available workers)

Create your own task that satisfy the Task interface{}. Your task should be responsible to listen to the context propagation so when the daemon send the cancel signal, the task will also be cancelled.

See the dummy task for as an example how the task can be composed.

##### Run the daemon and exit:
```go
d := daemon.New()
defer d.Stop()

// set number of worker to run the tasks simultaneously. (default = 1)
d.SetNumberOfWorker(4)
 
d.AddTask(
    daemon.NewDummyTask("Task #1", daemon.TaskTypeSync)
    daemon.NewDummyTask("Task #2", daemon.TaskTypeAsync)
    //YourMandatorySyncTask(),
    //YourSyncTask(),
    //YourAsyncTask(),        
)
if err := d.Run(context.Background()); err != nil {
    panic(err)
}
``` 
##### Run the daemon periodically until it's stopped.
```go
...
// You may want to retry the task with specific delay if error occurs, so the daemon will always run except you stop it manually by calling the Stop() func or cancelling the context.
d.SetTaskRetryAfterFail(10 * time.Second)

quit := make(chan os.Signal, 1)
signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

go func(){
    if err := d.RunWithInterval(context.Background(), 1 * time.Hour); err != nil {
        panic(err)
    }
}

<-quit
```

---

&copy; 2020