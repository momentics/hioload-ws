// Package api
// Author: momentics@gmail.com
//
// Lock-free ring buffer for cross-thread producer/consumer.

package api

// Ring is a lock-free ring buffer contract.
type Ring[T any] interface {
    // Enqueue adds an item, returns false if full.
    Enqueue(item T) bool
    // Dequeue removes oldest item, returns false if empty.
    Dequeue() (T, bool)
    // Len returns current number of items.
    Len() int
    // Cap returns buffer capacity.
    Cap() int
}
