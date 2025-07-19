// Package api
// Author: momentics <momentics@gmail.com>
//
// Executor contract for parallel task dispatch and dynamic eventloop scaling.

package api

// Executor abstracts parallel task pools and scaling of background workers.
type Executor interface {
    // Submit dispatches a task to be executed asynchronously.
    Submit(task func()) error

    // NumWorkers returns the current number of active worker goroutines.
    NumWorkers() int

    // Resize dynamically scales the worker pool.
    Resize(newCount int)
}
