// Package api
// Author: momentics
//
// Scheduler contract for high-precision timed and event-driven job execution.

package api

// Scheduler abstracts event/timer scheduling for async/highload loops.
type Scheduler interface {
    // Schedule schedules a callback to be executed after delayNanos.
    Schedule(delayNanos int64, fn func()) (Cancelable, error)

    // Cancel cancels a previously scheduled callback.
    Cancel(c Cancelable) error

    // Now returns monotonic time in nanoseconds.
    Now() int64
}
