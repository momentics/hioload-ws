// File: api/buffer.go
// Package api defines Buffer and BufferPool.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// Buffer represents a zero-copy memory slice.
// Converted to struct to avoid interface boxing.
type Buffer struct {
	Data     []byte
	NUMA     int
	Pool     Releaser
	Class    int
}

// Releaser interface to decouple pool dependency.
type Releaser interface {
	Put(Buffer)
}

// Bytes returns the full byte slice backing this Buffer.
func (b Buffer) Bytes() []byte { return b.Data }

// NUMANode returns the NUMA node where this buffer was allocated.
func (b Buffer) NUMANode() int { return b.NUMA }

// Copy returns a copy of the buffer data.
func (b Buffer) Copy() []byte {
	dup := make([]byte, len(b.Data))
	copy(dup, b.Data)
	return dup
}

// Slice returns a new Buffer view sharing the same underlying memory.
func (b Buffer) Slice(from, to int) Buffer {
	if from < 0 || to > len(b.Data) || from > to {
		return Buffer{NUMA: b.NUMA, Class: b.Class, Pool: b.Pool}
	}
	return Buffer{
		Data:  b.Data[from:to],
		NUMA:  b.NUMA,
		Pool:  b.Pool,
		Class: b.Class,
	}
}

// Release returns the buffer to its pool.
func (b Buffer) Release() {
	if b.Pool != nil {
		b.Pool.Put(b)
	}
}

// Capacity returns the capacity of the underlying slice.
func (b Buffer) Capacity() int {
	return cap(b.Data)
}

// BufferPool provides NUMA-aware buffer allocation.
type BufferPool interface {
	Get(size int, numaPreferred int) Buffer
	Put(b Buffer)
	Stats() BufferPoolStats
}

// BufferPoolStats summarizes pool usage.
type BufferPoolStats struct {
	TotalAlloc int64
	TotalFree  int64
	InUse      int64
	NUMAStats  map[int]int64
}
