// Package api
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Poller represents a batched event-reactor.

package api

// Poller represents a batched event-reactor.
type Poller interface {
	// Poll handles up to maxEvents; returns number processed and error.
	Poll(maxEvents int) (handled int, err error)
	// Register adds a handler to this poller.
	Register(h Handler) error
	// Unregister removes a handler.
	Unregister(h Handler) error
	// Stop gracefully stops the poller, releasing resources.
	Stop()
}
