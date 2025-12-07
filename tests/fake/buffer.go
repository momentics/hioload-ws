package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakePool implements api.Releaser and api.BufferPool for testing.
type FakePool struct {
	Size     int
	Returned []api.Buffer
	Node     int
}

// NewFakePool creates a new fake buffer pool.
func NewFakePool(size int) *FakePool {
	return &FakePool{
		Size:     size,
		Returned: make([]api.Buffer, 0),
	}
}

// Put implements api.Releaser and api.BufferPool.
func (fp *FakePool) Put(b api.Buffer) {
	fp.Returned = append(fp.Returned, b)
}

// Get implements api.BufferPool.
func (fp *FakePool) Get(size int, numaPreferred int) api.Buffer {
	return api.Buffer{
		Data:  make([]byte, size),
		NUMA:  numaPreferred,
		Pool:  fp,
		Class: size,
	}
}

func (fp *FakePool) Stats() api.BufferPoolStats {
	return api.BufferPoolStats{}
}

// Helpers for backward compatibility with tests (to some extent).
// Note: IsReleased check requires access to the pool now.

// NewFakeBuffer creates a standalone buffer with a dummy pool or nil pool.
func NewFakeBuffer(size int, node int) api.Buffer {
	return api.Buffer{
		Data: make([]byte, size),
		NUMA: node,
		Pool: nil,
	}
}

// IsReleased checks if the buffer was returned to the given pool.
// This is a helper function to replace b.IsReleased() in tests.
// Usage: fake.IsReleased(pool, buf)
func IsReleased(pool *FakePool, buf api.Buffer) bool {
	for _, b := range pool.Returned {
		// Compare underlying data pointer?
		if &b.Data[0] == &buf.Data[0] {
			return true
		}
	}
	return false
}
