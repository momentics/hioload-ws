//go:build !linux && !windows
// +build !linux,!windows

// Package pool
// Author: momentics <momentics@gmail.com>
//
// Platform-independent NUMA-aware BufferPool dispatcher.

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// BufferPoolManager chooses the correct NUMA BufferPool at runtime.
type BufferPoolManager struct {
	pools map[int]api.BufferPool
	lock  sync.RWMutex
}

// NewBufferPoolManager returns a new pool manager for NUMA nodes.
func NewBufferPoolManager() *BufferPoolManager {
	return &BufferPoolManager{
		pools: make(map[int]api.BufferPool),
	}
}

// GetPool returns BufferPool for a given NUMA node, initializing it if needed.
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

// The real implementation of newBufferPool is provided in bufferpool_linux.go or bufferpool_windows.go.
