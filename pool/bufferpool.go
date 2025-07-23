// File: pool/bufferpool.go
// Package pool
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Cross-platform, NUMA-aware BufferPool manager for hioload-ws.
// This file provides the BufferPoolManager type, allowing multiple buffer pools
// segmented per NUMA node. Buffer pool channel capacity can be configured via 
// explicit constructors, providing memory locality and scaling for high-load systems.
//
// The BufferPoolManager is thread-safe, optimally reuses memory between connections,
// and relies on lower-level newBufferPool/newBufferPoolWithCap helpers to instantiate
// actual pools with backend-specific (Linux/Windows) logic.

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// BufferPoolManager manages a set of BufferPool instances.
// Each NUMA node (represented by an integer ID; -1 for default/fallback) is assigned
// a dedicated BufferPool to reduce cross-node memory traffic.
//
// All operations are concurrent-safe; initialization and access to pool map use an RWMutex.
// For maximal performance in NUMA environments, buffer allocation and reuse are kept as
// local as possible. This reduces cache-misses and improves throughput under stress.
type BufferPoolManager struct {
	mu       sync.RWMutex          // Protects concurrent access to pools map.
	pools    map[int]api.BufferPool // Map NUMA node ID -> BufferPool instance.
	capacity int                   // Used when creating new pools (channel cap per pool).
}

// NewBufferPoolManager constructs a BufferPoolManager with the default capacity (1024).
// Default capacity is usually sufficient for most workloads, but may be tuned when
// using hugepages, large buffers, or known specific memory locality patterns.
func NewBufferPoolManager() *BufferPoolManager {
	return &BufferPoolManager{
		pools:    make(map[int]api.BufferPool),
		capacity: 1024,
	}
}

// NewBufferPoolManagerWithCap constructs a BufferPoolManager with a custom channel capacity.
// Allows advanced users to tune per-NUMA buffer pooling vs. memory pressure tradeoffs.
//   cap - desired channel capacity for each pooled NUMA node.
func NewBufferPoolManagerWithCap(cap int) *BufferPoolManager {
	if cap < 1 {
		cap = 1024
	}
	return &BufferPoolManager{
		pools:    make(map[int]api.BufferPool),
		capacity: cap,
	}
}

// GetPool returns the BufferPool for the specified NUMA node.
// If a pool for the node does not exist, it is created using the supplied channel capacity.
//
// NUMA node selection: node ID -1 is always reserved for the default pool;
// otherwise, users can explicitly pick a node for optimal locality.
//
// Thread-safe for concurrent GetPool calls from many goroutines/threads.
// Fast path: uses RLock if the pool already exists.
// Slow path: upgrades to Lock and constructs new pool.
func (m *BufferPoolManager) GetPool(numaNode int) api.BufferPool {
	// Fast path: read lock
	m.mu.RLock()
	pool, ok := m.pools[numaNode]
	m.mu.RUnlock()
	if ok {
		return pool
	}
	// Slow path: acquire write lock and double-check
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool, ok := m.pools[numaNode]; ok {
		return pool
	}
	// Create a new NUMA-aware buffer pool with specified capacity
	pool = newBufferPoolWithCap(numaNode, m.capacity)
	m.pools[numaNode] = pool
	return pool
}

// newBufferPoolWithCap is a backend-specific helper (see bufferpool_linux.go or bufferpool_windows.go).
// It constructs an actual BufferPool implementation (per OS/platform) with appropriate channel capacity.
//
// For Linux: allocates buffers using mmap/hugepages when available.
// For Windows: allocates buffers using VirtualAllocExNuma with large pages.
//
// Signature: func newBufferPoolWithCap(numaNode int, cap int) api.BufferPool
// The platform-specific files must provide this implementation.
