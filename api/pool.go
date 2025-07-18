// File: api/pool.go
// Author: momentics <momentics@gmail.com>
//
// Defines abstract pooling APIs: zero-copy allocators for buffer and object reuse.

package api

// BytePool provides reusable []byte buffers for all high-intensity operations
type BytePool interface {
	// Acquire returns a slice of at least n bytes.
	Acquire(n int) []byte

	// Release returns a buffer to the pool
	Release(buf []byte)
}

// ObjectPool provides generic pooling of Go objects allocated transiently
type ObjectPool[T any] interface {
	// Get returns an available instance from pool
	Get() T

	// Put returns an instance for reuse
	Put(obj T)
}
