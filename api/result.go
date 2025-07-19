// Package api
// Author: momentics
//
// Composable, strongly typed async result and cancellation primitives.

package api

// Result wraps any value or error for composable/async calls.
type Result[T any] struct {
    Value T
    Err   error
}

// Cancelable defines contract for cancelable async operations.
type Cancelable interface {
    // Cancel aborts the operation if still pending.
    Cancel() error

    // Done returns a channel closed when operation completes or is canceled.
    Done() <-chan struct{}

    // Err returns cancellation or completion reason.
    Err() error
}
