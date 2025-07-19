// Package api
// Author: momentics@gmail.com
//
// Generic event/data handler.

package api

// Handler processes events/batches from Poller/Transport.
type Handler interface {
    // Handle processes provided data.
    Handle(data any) error
}
