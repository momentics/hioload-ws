// Package pool provides high-performance, zero-alloc batching over api.Buffer.
//
// BufferBatch accumulates Buffer references for batch I/O.
// Designed for single-goroutine use; no locks for minimal overhead.
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import "github.com/momentics/hioload-ws/api"

// BufferBatch holds a slice of buffers for batch transmission.
type BufferBatch struct {
	buffers []api.Buffer
}

// NewBufferBatch creates a batch with initial capacity `cap`.
func NewBufferBatch(cap int) *BufferBatch {
	return &BufferBatch{buffers: make([]api.Buffer, 0, cap)}
}

// Append adds `buf` to the batch.
func (bb *BufferBatch) Append(buf api.Buffer) {
	bb.buffers = append(bb.buffers, buf)
}

// Len reports current batch size.
func (bb *BufferBatch) Len() int {
	return len(bb.buffers)
}

// Get returns the i-th buffer.
func (bb *BufferBatch) Get(i int) api.Buffer {
	return bb.buffers[i]
}

// Underlying returns the raw slice for transport.Send.
func (bb *BufferBatch) Underlying() []api.Buffer {
	return bb.buffers
}

// Reset clears the batch but retains capacity.
func (bb *BufferBatch) Reset() {
	bb.buffers = bb.buffers[:0]
}
