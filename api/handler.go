// Package api
// Author: momentics
//
// Strongly typed, modular handler contract for events, batches, or messages.

package api

// Handler processes events or data batches.
type Handler interface {
    // Handle processes provided event/data; error policy configurable by middleware.
    Handle(data any) error
}
