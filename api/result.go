// Package api
// Author: momentics@gmail.com
//
// Generic result, error propagation and cancellation.

package api

// Result wraps any payload or error.
type Result[T any] struct {
    Value T
    Err   error
}

// Cancelable is any operation that may be canceled.
type Cancelable interface {
    // Cancel attempts to abort the operation.
    Cancel() error
    // Done signals completion/cancellation.
    Done() <-chan struct{}
    // Err returns cancellation reason.
    Err() error
}
