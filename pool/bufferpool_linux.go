// +build linux

// Package pool
// Author: momentics <momentics@gmail.com>
//
// Linux-specific NUMA-aware, zero-copy buffer pool implementation.

package pool

import (
	"sync"
	"github.com/momentics/hioload-ws/api"
)

// linuxBuffer implements api.Buffer interface for Linux.
type linuxBuffer struct {
	data   []byte
	pool   *linuxBufferPool
	numaId int
	used   bool
	mu     sync.Mutex
}

// Bytes returns the data slice.
func (b *linuxBuffer) Bytes() []byte { return b.data }

// Slice creates a sub-buffer.
func (b *linuxBuffer) Slice(start, end int) api.Buffer {
	if start < 0 || end > len(b.data) || start > end {
		panic("slice bounds out of range")
	}
	return &linuxBuffer{
		data:   b.data[start:end],
		pool:   b.pool,
		numaId: b.numaId,
		used:   true,
	}
}

// Release returns the buffer to the pool.
func (b *linuxBuffer) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.used { return }
	b.pool.putBuffer(b)
	b.used = false
}

// Copy returns a deep copy.
func (b *linuxBuffer) Copy() []byte {
	dst := make([]byte, len(b.data))
	copy(dst, b.data)
	return dst
}
func (b *linuxBuffer) NUMANode() int { return b.numaId }

// linuxBufferPool implements a lock-free NUMA-aware buffer pool for Linux.
type linuxBufferPool struct {
	pool    sync.Pool
	numaId  int
	bufSize int
	stats   api.BufferPoolStats
}

func (bp *linuxBufferPool) getBuffer(size int) *linuxBuffer {
	b := bp.pool.Get()
	if b == nil {
		bb := make([]byte, size)
		return &linuxBuffer{
			data:   bb,
			pool:   bp,
			numaId: bp.numaId,
			used:   true,
		}
	}
	buf := b.(*linuxBuffer)
	if cap(buf.data) < size {
		buf.data = make([]byte, size)
	}
	buf.data = buf.data[:size]
	buf.used = true
	return buf
}

func (bp *linuxBufferPool) putBuffer(b *linuxBuffer) {
	bp.pool.Put(b)
}

func (bp *linuxBufferPool) Get(size int, numaPreferred int) api.Buffer {
	return bp.getBuffer(size)
}

func (bp *linuxBufferPool) Put(b api.Buffer) {
	if lb, ok := b.(*linuxBuffer); ok {
		bp.putBuffer(lb)
	}
}

func (bp *linuxBufferPool) Stats() api.BufferPoolStats {
	return bp.stats
}

// newBufferPool (Linux) creates a buffer pool for the specified NUMA node.
// TODO: Advanced hugepage, mmap, or memfd usage for ultra-low-latency buffer blocks.
func newBufferPool(numaNode int) api.BufferPool {
	return &linuxBufferPool{
		numaId:  numaNode,
		bufSize: 65536, // default buffer size
	}
}
