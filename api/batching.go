// Package api
// Author: momentics
//
// Generic batching interface with non-allocating batch operations.
// Batch implementations must not allocate when splitting or sub-batching.

package api

// Batch abstracts a zero-alloc, sliceable batch of objects.
type Batch[T any] interface {
    // Len returns item count in this batch instance.
    Len() int

    // Get retrieves an item by index—never panics, always bounds-checked.
    Get(index int) T

    // Slice returns a zero-copy span of the batch (start inclusive, end exclusive).
    Slice(start, end int) Batch[T]

    // Underlying returns the native storage as a slice.
    Underlying() []T

    // Split divides the batch into two zero-alloc sub-batches at position idx.
    Split(idx int) (first, second Batch[T])

    // Reset clears the batch; underlying memory is retained and reused.
    Reset()
}
