// Package api
// Author: momentics@gmail.com
//
// Event loop & poll-mode interfaces (DPDK-like strategies).

package api

// Poller represents an active poll-mode event source/consumer.
type Poller interface {
    // Poll processes up to maxEvents, returns number processed.
    Poll(maxEvents int) (handled int, err error)
    // Register adds a new handler to the poll queue.
    Register(h Handler) error
    // Unregister removes a handler from the poll queue.
    Unregister(h Handler) error
}
