# daemon

[![daemon](https://img.shields.io/badge/version-1.0&ndash;beta-blue.svg?style=flat)](#)
[![daemon](https://img.shields.io/badge/go-1.13-yellow.svg?style=flat)](#)
[![daemon](https://img.shields.io/badge/go-doc-green.svg?style=flat)](https://godoc.org/github.com/muktihari/daemon)
[![Go Report Card](https://goreportcard.com/badge/github.com/muktihari/daemon)](https://goreportcard.com/report/github.com/muktihari/daemon)

Orchestrate your tasks with simple configuration, set the number of concurrent process and run it once or continuously by given time interval (except it's stopped manually or the program is exited).

The daemon is using context to propagate the cancellation, cancel signal will be sent to the tasks when the daemon is stopped or the context deadline has exceeded. 

#### Getting Started

Create your own task that satisfy the Task interface{}:

```go
type Task interface{
    Run(context context.Context) error
}
```

*See [dummy/task.go](https://github.com/muktihari/daemon/blob/master/dummy/task.go) for example.*

Once your task completed, create daemon instance to assign them and run the daemon, example:
```go
package main 

import(
  "context"

  "github.com/muktihari/daemon"
  "github.com/muktihari/daemon/dummy"
  "github.com/muktihari/daemon/task"
)

func main(){
    d := daemon.New()
    
    d.AddTask("dummy task", task.Sync, dummy.NewTask())
    // d.AddTask("your task name", task.Sync, YourTask)
    // d.AddTask("your task name", task.Sync, AnotherTask)
    
    if err := d.Run(context.Background()); err != nil {
        panic(err)
    }
}

```
There are 4 types of task:

1. MandatorySync
   
   It will be executed in sequential order before any other tasks and will be waited until finish. Daemon will be stopped if any of this task return error. (using 1 worker)
1. MandatoryAsync
   
   It will be executed asynchronously after MandatorySync but before Sync or Async and will be waited until finish. Daemon will be stopped if any of this task return error. (using all workers) 
1. Sync
   
   It will be executed in sequential order. Alongside with Async task simultaneously. (using 1 worker)
1. Async
   
   It will be executed concurrently (using remaining available workers)

#### Configuration Methods
- Workers(n int) [default n is 0, means workers is as many as the number of the tasks]
  > Set number of workers for concurrent process of the asynchronous tasks.
- RetryIn(t time.Duration)
  > Set to retry the task with specific delay if error occurs, so the daemon will always run except it's stopped manually by calling the Stop() func or cancelling the context. Leave it unsetted may stop the daemon once the error occurs.
- DoNotRetry()
  > Unset the task retry.
   
 
 
#### Run the daemon periodically until it returns error, manually stopped or the program is exited.
```go
d := daemon.New()
defer d.Stop()

d.Workers(3)
d.RetryIn(10 * time.Second)


// Add the tasks
d.AddTask("<task name>", task.MandatorySync, MustBeRunBeforeOthersTask)
d.AddTask("<task name>", task.Async, AsyncTask)
d.AddTask("<task name>", task.Async, AnotherTask1)
d.AddTask("<task name>", task.Async, AnotherTask2)
d.AddTask("<task name>", task.Async, AnotherTask3)
d.AddTask("<task name>", task.Async, AnotherTask4)
d.AddTask("<task name>", task.Async, AnotherOfAnotherTask)

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