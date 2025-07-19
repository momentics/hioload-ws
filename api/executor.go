// Package api
// Author: momentics
//
// Executor contract for parallel task dispatch and custom eventloop integration.

package api

// Executor abstracts parallel task and custom eventloop execution.
type Executor interface {
    // Submit schedules task for execution.
    Submit(task func()) error

    // NumWorkers returns current number of active worker routines.
    NumWorkers() int

    // Resize adjusts the concurrency at runtime.
    Resize(newCount int)
}
