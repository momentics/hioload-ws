// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package reactor

import (
    "context"
    "github.com/momentics/hioload-ws/api"
)

// Reactor implements poller/event loop. Faked here, real version can use io_uring/epoll.
type Reactor struct {
    conns []api.NetConn
}

// NewReactor initializes the reactor.
func NewReactor() *Reactor {
    return &Reactor{
        conns: make([]api.NetConn, 0, 1024),
    }
}

// Run starts dummy poll loop. Real logic should replace with epoll/io_uring.
func (r *Reactor) Run(ctx context.Context) error {
    // Main loop sample (faked)
    <-ctx.Done()
    return nil
}

// Register adds connection to dummy poller.
func (r *Reactor) Register(conn api.NetConn) error {
    r.conns = append(r.conns, conn)
    return nil
}
