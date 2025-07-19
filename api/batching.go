// Package api
// Author: momentics@gmail.com
//
// Batching primitives for highload data processing.

package api

// Batch defines a generic batch of any objects.
type Batch[T any] interface {
    // Len returns the number of items in the batch.
    Len() int
    // Get retrieves item at index.
    Get(index int) T
    // Slice returns the underlying array.
    Slice() []T
}
