// File: pool/bufferpool_linux.go
//go:build linux
// +build linux

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Provides a Linux-specific NUMA-aware BufferPool implementation with hugepage
// support. Exports two constructors:
//   - newBufferPool           (uses default channel capacity, 1024)
//   - newBufferPoolWithCap    (user-supplied channel capacity for NUMA node)
// Used by BufferPoolManager for scalable, memory-localized pooling.

package pool

import (
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

// linuxBuffer implements api.Buffer, carrying its slice, pool reference and NUMA node id.
// The pool pointer allows buffers to be safely recycled to the correct pool.
type linuxBuffer struct {
	data   []byte
	pool   *baseBufferPool
	numaID int
}

// Bytes exposes the buffer's underlying byte slice for zero-copy reads or writes.
func (b *linuxBuffer) Bytes() []byte {
	return b.data
}

// Slice creates a sub-buffer referencing a subslice of the original memory.
// This is always zero-copy and does not allocate.
func (b *linuxBuffer) Slice(from, to int) api.Buffer {
	return &linuxBuffer{
		data:   b.data[from:to],
		pool:   b.pool,
		numaID: b.numaID,
	}
}

// Release returns the buffer to its originating pool for reuse.
func (b *linuxBuffer) Release() {
	b.pool.recycle(b)
}

// Copy produces a newly allocated []byte snapshot of the buffer's contents.
func (b *linuxBuffer) Copy() []byte {
	dup := make([]byte, len(b.data))
	copy(dup, b.data)
	return dup
}

// NUMANode reports where this buffer was allocated (for NUMA-aware use).
func (b *linuxBuffer) NUMANode() int {
	return b.numaID
}

// newLinuxBuffer tries to allocate a byte slice of given size on the requested NUMA node.
// Main attempt is hugepage mmap (MAP_HUGETLB); fallback is classic make([]byte) as a degraded path.
func newLinuxBuffer(size, numaNode int) *linuxBuffer {
	const hugePage = 2 << 20 // 2 MiB
	length := ((size + hugePage - 1) / hugePage) * hugePage

	data, err := syscall.Mmap(
		-1, 0, length,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_HUGETLB,
	)
	if err != nil {
		data = make([]byte, size)
	} else {
		data = data[:size]
	}
	return &linuxBuffer{
		data:   data,
		numaID: numaNode,
	}
}

// newBufferPool constructs the Linux-specific BufferPool using default channel capacity (1024).
// Used for legacy code or when explicit channel cap is not required.
func newBufferPool(numaNode int) api.BufferPool {
	return newBufferPoolWithCap(numaNode, 1024)
}

// newBufferPoolWithCap constructs the Linux BufferPool with a user-specified per-NUMA channel capacity.
// Supports NUMA optimizations and high-load scenarios with tight memory bounds.
//   - numaNode: target NUMA node id (-1 for default)
//   - cap: channel capacity (buffers retained in this pool instance)
func newBufferPoolWithCap(numaNode int, cap int) api.BufferPool {
	var base *baseBufferPool
	factory := func(size, node int) api.Buffer {
		buf := newLinuxBuffer(size, node)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPoolWithCap(numaNode, factory, cap)
	return base
}

// newBaseBufferPoolWithCap creates a baseBufferPool using a provided capacity for the internal channel.
// This enables dynamic tuning of buffer pool size on a per-NUMA basis.
func newBaseBufferPoolWithCap(numaNode int, factory func(int, int) api.Buffer, cap int) *baseBufferPool {
	return &baseBufferPool{
		pools:   map[int]chan api.Buffer{numaNode: make(chan api.Buffer, cap)},
		factory: factory,
	}
}
