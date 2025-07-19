// File: pool/batch.go
// Author: momentics <momentics@gmail.com>
//
// High-performance, zero-alloc batch of api.Buffer (or generic batches).
// Thread-safety: not safe for concurrent mutation.

package pool

import "github.com/momentics/hioload-ws/api"

// BufferBatch implements zero-copy batch of api.Buffer objects.
type BufferBatch struct {
	buffers []api.Buffer
}

// NewBufferBatch creates a new batch with given capacity.
func NewBufferBatch(capacity int) *BufferBatch {
	return &BufferBatch{
		buffers: make([]api.Buffer, 0, capacity),
	}
}

// Append adds a buffer to the batch.
func (b *BufferBatch) Append(buf api.Buffer) {
	b.buffers = append(b.buffers, buf)
}

// Len returns number of items in the batch.
func (b *BufferBatch) Len() int {
	return len(b.buffers)
}

// Get retrieves item at index.
func (b *BufferBatch) Get(idx int) api.Buffer {
	return b.buffers[idx]
}

// Slice returns a zero-copy sub-batch [start:end).
func (b *BufferBatch) Slice(start, end int) *BufferBatch {
	sub := b.buffers[start:end]
	return &BufferBatch{buffers: sub}
}

// Split divides the batch at idx into two sub-batches.
func (b *BufferBatch) Split(idx int) (first, second *BufferBatch) {
	return &BufferBatch{buffers: b.buffers[:idx]}, &BufferBatch{buffers: b.buffers[idx:]}
}

// Underlying returns the underlying buffer slice.
func (b *BufferBatch) Underlying() []api.Buffer {
	return b.buffers
}

// Reset clears the batch retaining the underlying memory.
func (b *BufferBatch) Reset() {
	b.buffers = b.buffers[:0]
}
