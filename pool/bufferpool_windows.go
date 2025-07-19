// +build windows

// Package pool
// Author: momentics <momentics@gmail.com>
//
// Windows-specific NUMA-aware, zero-copy buffer pool implementation.

package pool

import (
	"sync"
	"github.com/momentics/hioload-ws/api"
)

// windowsBuffer implements api.Buffer for Windows.
type windowsBuffer struct {
	data   []byte
	pool   *windowsBufferPool
	numaId int
	used   bool
	mu     sync.Mutex
}

func (b *windowsBuffer) Bytes() []byte { return b.data }
func (b *windowsBuffer) Slice(start, end int) api.Buffer {
	if start < 0 || end > len(b.data) || start > end {
		panic("slice bounds out of range")
	}
	return &windowsBuffer{
		data:   b.data[start:end],
		pool:   b.pool,
		numaId: b.numaId,
		used:   true,
	}
}
func (b *windowsBuffer) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.used { return }
	b.pool.putBuffer(b)
	b.used = false
}
func (b *windowsBuffer) Copy() []byte {
	dst := make([]byte, len(b.data))
	copy(dst, b.data)
	return dst
}
func (b *windowsBuffer) NUMANode() int { return b.numaId }

type windowsBufferPool struct {
	pool    sync.Pool
	numaId  int
	bufSize int
	stats   api.BufferPoolStats
}

func (bp *windowsBufferPool) getBuffer(size int) *windowsBuffer {
	b := bp.pool.Get()
	if b == nil {
		bb := make([]byte, size)
		return &windowsBuffer{
			data:   bb,
			pool:   bp,
			numaId: bp.numaId,
			used:   true,
		}
	}
	buf := b.(*windowsBuffer)
	if cap(buf.data) < size {
		buf.data = make([]byte, size)
	}
	buf.data = buf.data[:size]
	buf.used = true
	return buf
}

func (bp *windowsBufferPool) putBuffer(b *windowsBuffer) {
	bp.pool.Put(b)
}

func (bp *windowsBufferPool) Get(size int, numaPreferred int) api.Buffer {
	return bp.getBuffer(size)
}

func (bp *windowsBufferPool) Put(b api.Buffer) {
	if wb, ok := b.(*windowsBuffer); ok {
		bp.putBuffer(wb)
	}
}

func (bp *windowsBufferPool) Stats() api.BufferPoolStats {
	return bp.stats
}

// newBufferPool (Windows) creates buffer pool with potential NUMA affinity.
func newBufferPool(numaNode int) api.BufferPool {
	return &windowsBufferPool{
		numaId:  numaNode,
		bufSize: 65536,
	}
}
