// Package api
// Author: momentics
//
// High-performance, DPDK-style poll event loop abstraction able to batch and source events from multiple providers.

package api

// Poller represents a poll-mode reactor for high-rate event processing.
type Poller interface {
    // Poll handles up to maxEvents; returns number processed and error.
    Poll(maxEvents int) (handled int, err error)

    // Register adds a handler to this poller (atomic when in batch).
    Register(h Handler) error

    // Unregister removes a handler.
    Unregister(h Handler) error
}
