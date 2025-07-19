// Package fake
// Author: momentics <momentics@gmail.com>
//
// Fake buffer and buffer pool implementations for testing.

package fake

import (
	"github.com/momentics/hioload-ws/api"
	"sync"
)

// Buffer is a fake implementation of api.Buffer.
type Buffer struct {
	data     []byte
	released bool
	numaNode int
	mu       sync.Mutex
}

// NewBuffer creates a new fake buffer.
func NewBuffer(data []byte, numaNode int) *Buffer {
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	return &Buffer{
		data:     dataCopy,
		numaNode: numaNode,
	}
}

// Bytes returns an immutable view of the current buffer data.
func (b *Buffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		return nil
	}
	return b.data
}

// Slice produces a sub-buffer in O(1), memory-safe fashion.
func (b *Buffer) Slice(from, to int) api.Buffer {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		return nil
	}
	if from < 0 || to > len(b.data) || from > to {
		return nil
	}
	return NewBuffer(b.data[from:to], b.numaNode)
}

// Release returns the buffer to its pool.
func (b *Buffer) Release() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.released = true
	b.data = nil
}

// Copy returns a deep copy of buffer contents.
func (b *Buffer) Copy() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.released {
		return nil
	}
	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

// NUMANode returns the NUMA node this buffer was allocated from.
func (b *Buffer) NUMANode() int {
	return b.numaNode
}

// BufferPool is a fake implementation of api.BufferPool.
type BufferPool struct {
	mu        sync.Mutex
	allocated int64
	freed     int64
	inUse     int64
	numaStats map[int]int64
}

// NewBufferPool creates a new fake buffer pool.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		numaStats: make(map[int]int64),
	}
}

// Get returns a buffer sized at least 'size' bytes.
func (p *BufferPool) Get(size int, numaPreferred int) api.Buffer {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	data := make([]byte, size)
	p.allocated++
	p.inUse++
	p.numaStats[numaPreferred]++
	
	return NewBuffer(data, numaPreferred)
}

// Put returns buffer to pool.
func (p *BufferPool) Put(b api.Buffer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.freed++
	if p.inUse > 0 {
		p.inUse--
	}
	
	numaNode := b.NUMANode()
	if p.numaStats[numaNode] > 0 {
		p.numaStats[numaNode]--
	}
	
	b.Release()
}

// Stats exposes resource/accounting metrics.
func (p *BufferPool) Stats() api.BufferPoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	numaStatsCopy := make(map[int]int64)
	for k, v := range p.numaStats {
		numaStatsCopy[k] = v
	}
	
	return api.BufferPoolStats{
		TotalAlloc: p.allocated,
		TotalFree:  p.freed,
		InUse:      p.inUse,
		NUMAStats:  numaStatsCopy,
	}
}
