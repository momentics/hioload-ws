// File: pool/slab_pool.go
// Package pool implements lock-free slab allocation with size class support.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/core/concurrency"
)

// slabPool: fixed-size buffer allocation per size class/NUMA node.
type slabPool struct {
	size    int
	newBuf  func(size, numaNode int) api.Buffer
	release func(api.Buffer)

	// Queue takes the place of head/stack.
	// We use a fixed capacity queue.
	queue *concurrency.LockFreeQueue[api.Buffer]

	totalAlloc atomic.Uint64
	totalFree  atomic.Uint64
	numaStats  atomic.Pointer[numaMap]
}

const defaultPoolCapacity = 4096

// nodeBuf removed - no longer needed.

// numaMap: allocation counters by NUMA node.
type numaMap struct {
	mu     sync.Mutex
	counts map[int]uint64
}

func newNumamap() *numaMap { return &numaMap{counts: make(map[int]uint64)} }
func (m *numaMap) record(n int) {
	m.mu.Lock()
	m.counts[n]++
	m.mu.Unlock()
}
func (m *numaMap) Get() map[int]uint64 {
	m.mu.Lock()
	out := make(map[int]uint64, len(m.counts))
	for k, v := range m.counts {
		out[k] = v
	}
	m.mu.Unlock()
	return out
}

func (sp *slabPool) Get(_ int, numaNode int) api.Buffer {
	// Try to dequeue from pool
	if buf, ok := sp.queue.Dequeue(); ok {
		return buf
	}

	// Pool empty, allocate new
	buf := sp.newBuf(sp.size, numaNode)
	// Direct struct field assignment (no type assertion needed)
	buf.Pool = sp
	buf.Class = sp.size

	sp.totalAlloc.Add(1)
	mPtr := sp.numaStats.Load()
	if mPtr == nil {
		newMap := newNumamap()
		sp.numaStats.Store(newMap)
		mPtr = newMap
	}
	mPtr.record(numaNode)
	return buf
}

func (sp *slabPool) Put(buf api.Buffer) {
	// Try to enqueue to pool
	if sp.queue.Enqueue(buf) {
		sp.totalFree.Add(1)
		return
	}

	// Pool full, release
	if sp.release != nil {
		sp.release(buf)
	}
}

func (sp *slabPool) Stats() api.BufferPoolStats {
	totalAlloc := int64(sp.totalAlloc.Load())
	totalFree := int64(sp.totalFree.Load())
	inUse := totalAlloc - totalFree

	nm := sp.numaStats.Load()
	numaStats := make(map[int]int64)
	if nm != nil {
		raw := nm.Get()
		for node, cnt := range raw {
			numaStats[node] = int64(cnt)
		}
	}
	return api.BufferPoolStats{
		TotalAlloc: totalAlloc,
		TotalFree:  totalFree,
		InUse:      inUse,
		NUMAStats:  numaStats,
	}
}

var _ api.BufferPool = (*slabPool)(nil)
