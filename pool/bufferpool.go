// File: pool/bufferpool.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform NUMA-aware BufferPool manager with transparent backend selection.
// All public API is OS/NUMA-agnostic; platform-specific allocators in separate files.

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// BufferPoolManager provides NUMA-segmented pools for each NUMA node.
type BufferPoolManager struct {
	mu    sync.RWMutex
	pools map[int]api.BufferPool // Key: NUMA node (-1 for system default)
}

// NewBufferPoolManager creates and initializes a new manager.
func NewBufferPoolManager() *BufferPoolManager {
	return &BufferPoolManager{
		pools: make(map[int]api.BufferPool),
	}
}

// GetPool obtains or creates a NUMA-specific BufferPool.
// NUMA node -1 means "system default"; other values refer to platform-specific ID.
func (m *BufferPoolManager) GetPool(numaNode int) api.BufferPool {
	m.mu.RLock()
	pool, ok := m.pools[numaNode]
	m.mu.RUnlock()
	if ok {
		return pool
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, ok := m.pools[numaNode]; ok {
		return pool
	}
	pool = newBufferPool(numaNode)
	m.pools[numaNode] = pool
	return pool
}

// Platform-specific implementations of newBufferPool reside in bufferpool_linux.go and bufferpool_windows.go.
