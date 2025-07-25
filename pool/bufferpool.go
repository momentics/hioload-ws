// File: pool/bufferpool.go
// Package pool implements NUMA-aware, zero-copy buffer pooling with size class subpooling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/normalize"
)

// Predefined (power-of-two) buffer size classes (bytes)
// This table can be tuned for deployment needs.
var sizeClasses = [...]int{
	2 * 1024,        // 2K
	4 * 1024,        // 4K
	8 * 1024,        // 8K
	16 * 1024,       // 16K
	32 * 1024,       // 32K
	64 * 1024,       // 64K
	128 * 1024,      // 128K
	256 * 1024,      // 256K
	512 * 1024,      // 512K
	1 * 1024 * 1024, // 1M
}

// sizeClassUpperBound returns the smallest class >= requested size.
func sizeClassUpperBound(size int) int {
	for _, c := range sizeClasses {
		if size <= c {
			return c
		}
	}
	return sizeClasses[len(sizeClasses)-1] // fallback: biggest class
}

// Buffer implements api.Buffer and carries info for zero-copy management.
type Buffer struct {
	data     []byte
	numaNode int
	slab     *slabPool
	class    int // size class used by source pool
}

// Bytes returns the full byte slice backing this Buffer.
func (b *Buffer) Bytes() []byte { return b.data }
func (b *Buffer) NUMANode() int { return b.numaNode }
func (b *Buffer) Copy() []byte  { dup := make([]byte, len(b.data)); copy(dup, b.data); return dup }
func (b *Buffer) Slice(from, to int) api.Buffer {
	return &Buffer{
		data:     b.data[from:to],
		numaNode: b.numaNode,
		class:    b.class,
		slab:     b.slab,
	}
}
func (b *Buffer) Release() {
	if b.slab != nil {
		b.slab.Put(b)
	}
}

// BufferPoolManager manages all size-classed pools for all NUMA nodes.
type BufferPoolManager struct {
	nodeCnt int
	nodes   []*nodeClassPools // per NUMA node
}

// nodeClassPools manages all size-class subpools for a given node.
type nodeClassPools struct {
	mu    sync.RWMutex
	class map[int]*slabPool // maps size class -> slab pool
}

// NewBufferPoolManager initializes the global manager.
// nodeCnt: number of NUMA nodes (from OS topology, >=1).
func NewBufferPoolManager(nodeCnt int) *BufferPoolManager {
	nodes := make([]*nodeClassPools, nodeCnt)
	for i := 0; i < nodeCnt; i++ {
		nodes[i] = &nodeClassPools{class: make(map[int]*slabPool)}
	}
	return &BufferPoolManager{
		nodeCnt: nodeCnt,
		nodes:   nodes,
	}
}

// getPreferredNUMANode unified with normalize.NUMANodeAuto for all BufferPool allocations.
func getPreferredNUMANode(numaPreferred int) int {
	return normalize.NUMANodeAuto(numaPreferred)
}

// GetPool returns a NUMA-aware BufferPool for the requested buffer size,
// routing all requests for sizes within a given class to the corresponding pool.
func (m *BufferPoolManager) GetPool(size, numaPreferred int) api.BufferPool {
	node := getPreferredNUMANode(numaPreferred)
	clz := sizeClassUpperBound(size)
	return m.nodes[node].getOrCreatePool(clz)
}

// getOrCreatePool returns the subpool for a class, lazily allocating on first use.
func (n *nodeClassPools) getOrCreatePool(class int) api.BufferPool {
	n.mu.RLock()
	pool, ok := n.class[class]
	n.mu.RUnlock()
	if ok {
		return pool
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if pool, ok = n.class[class]; ok {
		return pool
	}
	npool := newSlabPool(class)
	n.class[class] = npool
	return npool
}
