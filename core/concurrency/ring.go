// File: internal/concurrency/ring.go
// Package concurrency implements lock-free ring buffers.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// RingBuffer is a bounded circular buffer with atomic head/tail,
// padded to prevent false sharing.
// Implements api.Ring for cross-package consistency.

package concurrency

import (
	"sync/atomic"

	"github.com/momentics/hioload-ws/api"
)

// Ensure compile-time interface compliance.
var _ api.Ring[any] = (*RingBuffer[any])(nil)

// RingBuffer is a lock-free ring buffer (MPMC API).
type RingBuffer[T any] struct {
	head  uint64
	_     [64]byte // Padding for hot/cold separation
	tail  uint64
	_     [64]byte // Padding
	mask  uint64
	cells []cell[T] // Reuse cell struct from lock_free_queue if possible or redefine
}

// NewRingBuffer allocates a ring buffer of power-of-two size.
func NewRingBuffer[T any](size uint64) *RingBuffer[T] {
	if size < 2 {
		size = 2
	}
	if size&(size-1) != 0 {
		// Round up to next power of two
		n := size - 1
		n |= n >> 1
		n |= n >> 2
		n |= n >> 4
		n |= n >> 8
		n |= n >> 16
		n |= n >> 32
		size = n + 1
	}
	r := &RingBuffer[T]{
		mask:  size - 1,
		cells: make([]cell[T], size),
	}
	for i := range r.cells {
		r.cells[i].sequence.Store(uint64(i))
	}
	return r
}

// Enqueue adds item; returns false if full.
func (r *RingBuffer[T]) Enqueue(item T) bool {
	for {
		tail := atomic.LoadUint64(&r.tail)
		index := tail & r.mask
		c := &r.cells[index]
		seq := c.sequence.Load()
		dif := int64(seq) - int64(tail)

		if dif == 0 {
			if atomic.CompareAndSwapUint64(&r.tail, tail, tail+1) {
				c.data = item
				c.sequence.Store(tail + 1)
				return true
			}
		} else if dif < 0 {
			return false // full
		} else {
			// tail moved
		}
	}
}

// Dequeue removes and returns item; ok false if empty.
func (r *RingBuffer[T]) Dequeue() (T, bool) {
	for {
		head := atomic.LoadUint64(&r.head)
		index := head & r.mask
		c := &r.cells[index]
		seq := c.sequence.Load()
		dif := int64(seq) - int64(head + 1)

		if dif == 0 {
			if atomic.CompareAndSwapUint64(&r.head, head, head+1) {
				item := c.data
				c.sequence.Store(head + r.mask + 1)
				return item, true
			}
		} else if dif < 0 {
			var zero T
			return zero, false // empty
		} else {
			// head moved
		}
	}
}

// Len returns number of items currently in buffer.
func (r *RingBuffer[T]) Len() int {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	return int(tail - head)
}

// Cap returns fixed buffer capacity.
func (r *RingBuffer[T]) Cap() int {
	return len(r.cells)
}
