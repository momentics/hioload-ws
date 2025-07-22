// File: pool/bufferpool.go
// Package pool
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Cross-platform NUMA-aware BufferPool manager with transparent backend selection.

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// BufferPoolManager provides NUMA-segmented pools for each NUMA node.
type BufferPoolManager struct {
	mu    sync.RWMutex
	pools map[int]api.BufferPool // Key: NUMA node (-1 for default)
}

func NewBufferPoolManager() *BufferPoolManager {
	return &BufferPoolManager{
		pools: make(map[int]api.BufferPool),
	}
}

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

// newBufferPool реализована в файлах bufferpool_linux.go и bufferpool_windows.go
