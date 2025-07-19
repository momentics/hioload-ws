// Package api
// Author: momentics <momentics@gmail.com>
//
// Scheduler contract for high-resolution timer/event scheduling.

package api

// Scheduler defines the interface for scheduling timed async jobs.
type Scheduler interface {
    // Schedule registers a function to be executed after a delay in nanoseconds.
    Schedule(delayNanos int64, fn func()) (Cancelable, error)

    // Cancel removes a previously scheduled task.
    Cancel(c Cancelable) error

    // Now returns the current monotonic nanosecond time.
    Now() int64
}
