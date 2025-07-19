// Package pool
// Author: momentics <momentics@gmail.com>
//
// High-performance zero-allocation batching for Buffers and generic objects.

package pool

import (
	"sync"
	"github.com/momentics/hioload-ws/api"
)

// BufferBatch is a lock-free batch of api.Buffer objects.
type BufferBatch struct {
	buffers []api.Buffer
	size    int
	mu      sync.Mutex
}

// NewBufferBatch creates a new batch with given capacity.
func NewBufferBatch(capacity int) *BufferBatch {
	return &BufferBatch{
		buffers: make([]api.Buffer, 0, capacity),
	}
}
func (b *BufferBatch) Append(buf api.Buffer) {
	b.mu.Lock()
	b.buffers = append(b.buffers, buf)
	b.size++
	b.mu.Unlock()
}
func (b *BufferBatch) Len() int {
	return b.size
}
func (b *BufferBatch) Get(idx int) api.Buffer {
	return b.buffers[idx]
}
func (b *BufferBatch) Slice(start, end int) *BufferBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	sub := make([]api.Buffer, end-start)
	copy(sub, b.buffers[start:end])
	return &BufferBatch{buffers: sub, size: end - start}
}
func (b *BufferBatch) Underlying() []api.Buffer {
	return b.buffers
}
func (b *BufferBatch) Split(idx int) (*BufferBatch, *BufferBatch) {
	return b.Slice(0, idx), b.Slice(idx, b.size)
}
func (b *BufferBatch) Reset() {
	b.mu.Lock()
	b.buffers = b.buffers[:0]
	b.size = 0
	b.mu.Unlock()
}
