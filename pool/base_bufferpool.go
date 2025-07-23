// File: pool/base_bufferpool.go
// Package pool provides a generic, NUMA-aware buffer pool base implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file implements baseBufferPool without using Go generics, managing channels
// of api.Buffer. Concrete buffer types (linuxBuffer, windowsBuffer) satisfy api.Buffer.

package pool

import (
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// baseBufferPool implements api.BufferPool by maintaining per-NUMA-node channels
// of api.Buffer. It uses a factory function to create new buffers.
type baseBufferPool struct {
	pools      map[int]chan api.Buffer
	mu         sync.Mutex
	factory    func(size int, numaNode int) api.Buffer
	totalAlloc atomic.Int64
	totalFree  atomic.Int64
	numaStats  sync.Map // map[int]*atomic.Int64
}

// newBaseBufferPool constructs a new baseBufferPool for the specified NUMA node.
func newBaseBufferPool(numaNode int, factory func(size, numaNode int) api.Buffer) *baseBufferPool {
	return &baseBufferPool{
		pools:   map[int]chan api.Buffer{numaNode: make(chan api.Buffer, 1024)},
		factory: factory,
	}
}

// getChan returns or lazily creates the channel for a given NUMA node.
func (p *baseBufferPool) getChan(numaNode int) chan api.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch, ok := p.pools[numaNode]
	if !ok {
		ch = make(chan api.Buffer, 1024)
		p.pools[numaNode] = ch
	}
	return ch
}

// Get retrieves a buffer of at least the requested size, preferring the specified NUMA node.
// If reuse is not possible, allocates via factory.
func (p *baseBufferPool) Get(size, numaNode int) api.Buffer {
	ch := p.getChan(numaNode)
	select {
	case buf := <-ch:
		data := buf.Bytes()
		if cap(data) < size {
			p.totalAlloc.Add(1)
			return p.factory(size, numaNode)
		}
		return buf.Slice(0, size)
	default:
		p.totalAlloc.Add(1)
		return p.factory(size, numaNode)
	}
}

// Put returns a buffer to its NUMA-node channel and updates stats.
func (p *baseBufferPool) Put(b api.Buffer) {
	node := b.NUMANode()
	ch := p.getChan(node)
	select {
	case ch <- b:
		p.totalFree.Add(1)
		v, _ := p.numaStats.LoadOrStore(node, &atomic.Int64{})
		v.(*atomic.Int64).Add(1)
	default:
		// drop if full
	}
}

// recycle is an alias for Put, used by concrete buffer Release methods.
func (p *baseBufferPool) recycle(b api.Buffer) {
	p.Put(b)
}

// Stats returns current pool usage statistics.
func (p *baseBufferPool) Stats() api.BufferPoolStats {
	totalAlloc := p.totalAlloc.Load()
	totalFree := p.totalFree.Load()
	inUse := totalAlloc - totalFree

	numaMap := make(map[int]int64)
	p.numaStats.Range(func(k, v any) bool {
		numaMap[k.(int)] = v.(*atomic.Int64).Load()
		return true
	})

	return api.BufferPoolStats{
		TotalAlloc: totalAlloc,
		TotalFree:  totalFree,
		InUse:      inUse,
		NUMAStats:  numaMap,
	}
}
