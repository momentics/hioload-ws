// Package client implements lock-free batch accumulation for outgoing Buffers.
package client

import "github.com/momentics/hioload-ws/api"

// Batch holds zero-copy Buffers for batch sends.
type Batch struct {
	mu      chan struct{} // semaphore lock-free guard
	buffers []api.Buffer
	cap     int
}

// NewBatch initializes a batch with fixed capacity.
func NewBatch(capacity int) *Batch {
	b := &Batch{buffers: make([]api.Buffer, 0, capacity), cap: capacity}
	b.mu = make(chan struct{}, 1)
	b.mu <- struct{}{}
	return b
}

// Append adds a Buffer in lock-free manner.
func (b *Batch) Append(buf api.Buffer) {
	<-b.mu
	b.buffers = append(b.buffers, buf)
	b.mu <- struct{}{}
}

// Swap atomically retrieves and resets the batch slice.
func (b *Batch) Swap() []api.Buffer {
	<-b.mu
	old := b.buffers
	b.buffers = make([]api.Buffer, 0, b.cap)
	b.mu <- struct{}{}
	return old
}

// Len returns current batch size.
func (b *Batch) Len() int {
	return len(b.buffers)
}
