// File: api/buffer.go
// Package api defines Buffer and BufferPool.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// Buffer represents a zero-copy memory slice.
type Buffer interface {
	Bytes() []byte
	Slice(from, to int) Buffer
	Release()
	Copy() []byte
	NUMANode() int
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
