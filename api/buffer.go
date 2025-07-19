// Package api
// Author: momentics
//
// Zero-copy memory buffer and NUMA-aware pooling for high-performance IO.
//
// Buffers may be hugepages, mmap, shared memory, or device-backed memory.
// All operations must be zero-copy unless Copy is explicitly called.

package api

// Buffer describes a resliceable, reference-counted memory region.
type Buffer interface {
    // Bytes returns an immutable view of the current buffer data.
    Bytes() []byte

    // Slice produces a sub-buffer in O(1), memory-safe fashion.
    Slice(from, to int) Buffer

    // Release returns the buffer (and underlying region) to its pool.
    // After Release, buffer must not be used.
    Release()

    // Copy returns a deep copy of buffer contents as a standalone []byte.
    Copy() []byte

    // NUMANode returns the NUMA node this buffer was allocated from.
    NUMANode() int
}

// BufferPool abstracts memory region management for buffers.
type BufferPool interface {
    // Get returns a buffer sized at least 'size' bytes.
    // Always NUMA-aware: should satisfy locality preference if possible.
    Get(size int, numaPreferred int) Buffer

    // Put returns buffer to pool; buffer must not be used afterwards.
    Put(b Buffer)

    // Stats exposes resource/accounting metrics for observability.
    Stats() BufferPoolStats
}

// BufferPoolStats aggregates buffer allocation/reuse stats.
type BufferPoolStats struct {
    TotalAlloc int64
    TotalFree  int64
    InUse      int64
    NUMAStats  map[int]int64
}
