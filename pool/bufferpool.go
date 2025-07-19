// pool/bufferpool.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform NUMA-aware BufferPool manager with transparent selection and trace.

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

type BufferPoolManager struct {
	pools map[int]api.BufferPool
	lock  sync.RWMutex
}

func NewBufferPoolManager() *BufferPoolManager {
	return &BufferPoolManager{
		pools: make(map[int]api.BufferPool),
	}
}

func (m *BufferPoolManager) GetPool(numaNode int) api.BufferPool {
	m.lock.RLock()
	pool, ok := m.pools[numaNode]
	m.lock.RUnlock()
	if ok {
		return pool
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	pool, ok = m.pools[numaNode]
	if ok {
		return pool
	}
	pool = newBufferPool(numaNode)
	m.pools[numaNode] = pool
	return pool
}

// Platform-specific implementations of newBufferPool are provided in bufferpool_linux.go and bufferpool_windows.go
