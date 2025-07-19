// Package api
// Author: momentics@gmail.com
//
// Zero-copy buffer, segment and allocator abstractions.

package api

// Buffer is a zero-copy, sliceable memory region.
type Buffer interface {
    // Bytes returns a view of buffer contents.
    Bytes() []byte
    // Slice returns a sub-buffer.
    Slice(from, to int) Buffer
    // Release deallocates the buffer.
    Release()
    // Copy returns deep copy as byte slice.
    Copy() []byte
}

// BufferPool manages reusable buffer regions.
type BufferPool interface {
    // Get allocates a buffer of at least size bytes.
    Get(size int) Buffer
    // Put releases buffer back to pool.
    Put(b Buffer)
}
