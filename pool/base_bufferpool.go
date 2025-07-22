// File: pool/base_bufferpool.go
// Package pool provides a generic, NUMA-aware buffer pool base implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// baseBufferPool is the common core for all platform buffer pools.
// It manages per-NUMA-node channels of buffers and tracks basic pool operations.

package pool

import (
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// bufferFactory defines how to create a new buffer of type T.
type bufferFactory[T api.Buffer] func(size int, numaNode int) T

// baseBufferPool implements api.BufferPool.
// T is a concrete Buffer type (e.g. *linuxBuffer or *windowsBuffer).
type baseBufferPool[T api.Buffer] struct {
	// pools maps NUMA node => buffered channel of buffers.
	pools   map[int]chan T
	mu      sync.Mutex
	factory bufferFactory[T]

	// Statistics counters.
	totalAlloc atomic.Int64
	totalFree  atomic.Int64
	// per-NUMA free counts: map[node]*atomic.Int64
	numaStats sync.Map
}

// newBaseBufferPool constructs a new baseBufferPool for the given initial NUMA node.
func newBaseBufferPool[T api.Buffer](initialNUMA int, factory bufferFactory[T]) *baseBufferPool[T] {
	return &baseBufferPool[T]{
		pools:   map[int]chan T{initialNUMA: make(chan T, 1024)},
		factory: factory,
	}
}

// getChan returns or creates the channel for the specified numaNode.
func (p *baseBufferPool[T]) getChan(numaNode int) chan T {
	p.mu.Lock()
	ch, ok := p.pools[numaNode]
	if !ok {
		ch = make(chan T, 1024)
		p.pools[numaNode] = ch
	}
	p.mu.Unlock()
	return ch
}

// Get retrieves a buffer of at least size bytes, preferring numaNode.
// It reuses an existing buffer if available, else allocates a new one.
func (p *baseBufferPool[T]) Get(size, numaNode int) api.Buffer {
	ch := p.getChan(numaNode)
	select {
	case buf := <-ch:
		// Reuse only if capacity suffices.
		if cap(buf.Bytes()) < size {
			p.totalAlloc.Add(1)
			return p.factory(size, numaNode)
		}
		return buf.Slice(0, size)
	default:
		p.totalAlloc.Add(1)
		return p.factory(size, numaNode)
	}
}

// Put returns a buffer to the pool. Updates statistics counters.
func (p *baseBufferPool[T]) Put(b api.Buffer) {
	if tb, ok := b.(T); ok {
		node := tb.NUMANode()
		ch := p.getChan(node)
		select {
		case ch <- tb:
			p.totalFree.Add(1)
			// update per-NUMA stats
			v, _ := p.numaStats.LoadOrStore(node, &atomic.Int64{})
			counter := v.(*atomic.Int64)
			counter.Add(1)
		default:
			// pool channel full; drop buffer
		}
	}
}

// recycle is an alias for Put, used by platform-specific Release() implementations.
func (p *baseBufferPool[T]) recycle(b T) {
	p.Put(b)
}

// Stats returns current pool usage statistics per api.BufferPoolStats.
func (p *baseBufferPool[T]) Stats() api.BufferPoolStats {
	totalAlloc := p.totalAlloc.Load()
	totalFree := p.totalFree.Load()
	inUse := totalAlloc - totalFree

	numaMap := make(map[int]int64)
	p.numaStats.Range(func(key, value any) bool {
		node := key.(int)
		count := value.(*atomic.Int64).Load()
		numaMap[node] = count
		return true
	})

	return api.BufferPoolStats{
		TotalAlloc: totalAlloc,
		TotalFree:  totalFree,
		InUse:      inUse,
		NUMAStats:  numaMap,
	}
}
