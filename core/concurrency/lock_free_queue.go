// File: internal/concurrency/lock_free_queue.go
// Package concurrency provides a lock-free queue for executors.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Single-producer/single-consumer ring buffer with minimal atomics to reduce contention.

package concurrency

import "sync/atomic"

// LockFreeQueue is a specific MPMC bounded queue using sequence numbers to fix race conditions.
// Based on the pattern by Dmitry Vyukov for MPMC queues.
type LockFreeQueue[T any] struct {
	head  uint64
	_     [cacheLinePad]byte
	tail  uint64
	_     [cacheLinePad]byte
	mask  uint64
	cells []cell[T]
}

const cacheLinePad = 64

type cell[T any] struct {
	sequence atomic.Uint64
	data     T
}

// NewLockFreeQueue creates a new queue with capacity rounded to power of two.
func NewLockFreeQueue[T any](capacity int) *LockFreeQueue[T] {
	if capacity < 2 {
		capacity = 2
	}
	// Round up to power of 2
	size := 1
	for size < capacity {
		size <<= 1
	}

	q := &LockFreeQueue[T]{
		mask:  uint64(size - 1),
		cells: make([]cell[T], size),
	}

	for i := range q.cells {
		q.cells[i].sequence.Store(uint64(i))
	}
	return q
}

// Enqueue adds val; returns false if full.
func (q *LockFreeQueue[T]) Enqueue(val T) bool {
	cell, _ := q.enqueueCell(val)
	return cell != nil
}

// enqueueCell is a helper to allow reusing logic if needed,
// though for this specific struct we just return bool.
func (q *LockFreeQueue[T]) enqueueCell(val T) (*cell[T], bool) {
	for {
		tail := atomic.LoadUint64(&q.tail)
		index := tail & q.mask
		c := &q.cells[index]
		seq := c.sequence.Load()
		dif := int64(seq) - int64(tail)

		if dif == 0 {
			if atomic.CompareAndSwapUint64(&q.tail, tail, tail+1) {
				c.data = val
				c.sequence.Store(tail + 1)
				return c, true
			}
		} else if dif < 0 {
			return nil, false // full
		} else {
			// tail moved, retry
		}
	}
}

// Dequeue removes and returns an item; ok false if empty.
func (q *LockFreeQueue[T]) Dequeue() (item T, ok bool) {
	for {
		head := atomic.LoadUint64(&q.head)
		index := head & q.mask
		c := &q.cells[index]
		seq := c.sequence.Load()
		dif := int64(seq) - int64(head+1)

		if dif == 0 {
			if atomic.CompareAndSwapUint64(&q.head, head, head+1) {
				item = c.data
				c.sequence.Store(head + q.mask + 1)
				return item, true
			}
		} else if dif < 0 {
			var zero T
			return zero, false // empty
		} else {
			// head moved, retry
		}
	}
}
