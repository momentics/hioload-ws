// File: pool/bufferpool_linux.go
//go:build linux
// +build linux

// Package pool provides Linux-specific NUMA-aware buffer pool implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file defines linuxBuffer and constructs a BufferPool for Linux.
// It uses hugepage-backed mmap when available and falls back to standard allocation.
// The newBufferPool function wires linuxBuffer into a generic baseBufferPool.

package pool

import (
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

// linuxBuffer implements api.Buffer for Linux.
// It holds a byte slice, a reference to its originating pool, and the NUMA node ID.
type linuxBuffer struct {
	data   []byte
	pool   *baseBufferPool
	numaID int
}

// Bytes returns the raw byte slice for I/O operations.
func (b *linuxBuffer) Bytes() []byte {
	return b.data
}

// Slice returns a zero-copy sub-buffer referencing the same underlying memory.
func (b *linuxBuffer) Slice(from, to int) api.Buffer {
	return &linuxBuffer{
		data:   b.data[from:to],
		pool:   b.pool,
		numaID: b.numaID,
	}
}

// Release returns this buffer to its pool for reuse.
func (b *linuxBuffer) Release() {
	b.pool.recycle(b)
}

// Copy makes an independent copy of the buffer's contents.
func (b *linuxBuffer) Copy() []byte {
	dup := make([]byte, len(b.data))
	copy(dup, b.data)
	return dup
}

// NUMANode reports which NUMA node this buffer was allocated on.
func (b *linuxBuffer) NUMANode() int {
	return b.numaID
}

// newLinuxBuffer allocates a buffer of at least 'size' bytes on the specified NUMA node.
// It attempts to mmap with MAP_HUGETLB for hugepages; on failure, it falls back to make([]byte).
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

// newBufferPool constructs and returns the Linux-specific BufferPool.
// We declare 'var base' first so that the closure can correctly capture it.
func newBufferPool(numaNode int) api.BufferPool {
	// Declare base pointer before initializing, so closure can assign to it.
	var base *baseBufferPool
	factory := func(size, node int) api.Buffer {
		buf := newLinuxBuffer(size, node)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPool(numaNode, factory)
	return base
}
