//go:build linux
// +build linux

// File: pool/bufferpool_linux.go
// Package pool provides Linux-specific NUMA-aware buffer pool.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// linuxBuffer uses mmap(HUGETLB) for zero-copy hugepage-backed allocations.
// newBufferPool returns a baseBufferPool configured for linuxBuffer.

package pool

import (
    "syscall"

    "github.com/momentics/hioload-ws/api"
)

// linuxBuffer implements api.Buffer for Linux NUMA-aware allocations.
type linuxBuffer struct {
    data   []byte
    pool   *baseBufferPool[*linuxBuffer]
    numaID int
}

// Bytes returns the underlying byte slice.
func (b *linuxBuffer) Bytes() []byte { return b.data }

// Slice returns a sub-buffer referencing the same underlying data.
func (b *linuxBuffer) Slice(from, to int) api.Buffer {
    return &linuxBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}

// Release returns this buffer to its pool.
func (b *linuxBuffer) Release() { b.pool.recycle(b) }

// Copy makes a fresh copy of the payload.
func (b *linuxBuffer) Copy() []byte {
    c := make([]byte, len(b.data))
    copy(c, b.data)
    return c
}

// NUMANode reports the originating NUMA node.
func (b *linuxBuffer) NUMANode() int { return b.numaID }

// newLinuxBuffer allocates a buffer aligned to hugepage boundary if possible.
func newLinuxBuffer(size, numaNode int) *linuxBuffer {
    // round up to nearest 2MiB (HUGETLB) multiple
    length := ((size + (2<<20) - 1) / (2 << 20)) * (2 << 20)
    data, err := syscall.Mmap(-1, 0, length,
        syscall.PROT_READ|syscall.PROT_WRITE,
        syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_HUGETLB)
    if err != nil {
        // fallback to regular allocation
        data = make([]byte, size)
    } else {
        data = data[:size]
    }
    return &linuxBuffer{data: data, numaID: numaNode}
}

// newBufferPool constructs the Linux buffer pool via baseBufferPool.
func newBufferPool(numaNode int) api.BufferPool {
    var base *baseBufferPool[*linuxBuffer]
    factory := func(size, pref int) *linuxBuffer {
        buf := newLinuxBuffer(size, pref)
        buf.pool = base
        return buf
    }
    base = newBaseBufferPool(numaNode, factory)
    return base
}
