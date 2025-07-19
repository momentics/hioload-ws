// Package pool — zero-alloc batching without locks.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-performance zero-copy batch of api.Buffer objects.
// This implementation is NOT thread-safe and avoids mutex in hot-path.

package pool

import "github.com/momentics/hioload-ws/api"

// BufferBatch is a minimal zero-alloc batch of api.Buffer.
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

// Slice returns zero-copy sub-batch [start:end).
func (b *BufferBatch) Slice(start, end int) *BufferBatch {
	return &BufferBatch{buffers: b.buffers[start:end]}
}

// Underlying returns the underlying slice.
func (b *BufferBatch) Underlying() []api.Buffer {
	return b.buffers
}

// Split divides the batch at idx into two sub-batches.
func (b *BufferBatch) Split(idx int) (first, second *BufferBatch) {
	return &BufferBatch{buffers: b.buffers[:idx]}, &BufferBatch{buffers: b.buffers[idx:]}
}

// Reset clears the batch retaining underlying buffer.
func (b *BufferBatch) Reset() {
	b.buffers = b.buffers[:0]
}
