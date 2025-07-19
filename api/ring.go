// Package api
// Author: momentics
//
// Fast, lock-free ring buffer contract for cross-thread data transfer.

package api

// Ring contract for high-performance, concurrent FIFO.
type Ring[T any] interface {
    // Enqueue adds item, returns false if buffer full.
    Enqueue(item T) bool

    // Dequeue removes and returns the oldest item, false if buffer empty.
    Dequeue() (T, bool)

    // Len returns number of items currently in buffer.
    Len() int

    // Cap returns fixed buffer capacity.
    Cap() int
}
