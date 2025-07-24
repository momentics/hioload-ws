// Package client: exposes BufferPool for zero-copy buffer management.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
package client

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
)

// ClientBufferPool returns the shared NUMA-aware pool used by clients.
func ClientBufferPool(numaNode int) api.BufferPool {
	// Delegates to pool.NewBufferPoolManager().GetPool
	mgr := pool.NewBufferPoolManager()
	return mgr.GetPool(numaNode)
}
