// File: pool/bufferpool_linux.go
//go:build linux
// +build linux

//
// Package pool: Linux-specific slab allocator using hugepages and libnuma.
//
// On Linux, buffers are allocated via mmap with MAP_HUGETLB for 2 MiB pages.
// Fallback to Go heap if hugepage allocation fails.
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"syscall"

	"github.com/momentics/hioload-ws/api"
)

// linuxAlloc maps or allocates a buffer of exactly `sz` bytes on `numaNode`.
func linuxAlloc(sz, numaNode int) api.Buffer {
	// Round to hugepage (2 MiB) boundary
	const hugeSize = 2 << 20
	length := ((sz + hugeSize - 1) / hugeSize) * hugeSize

	data, err := syscall.Mmap(-1, 0, length,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE|syscall.MAP_HUGETLB)
	if err != nil {
		data = make([]byte, sz)
	} else {
		data = data[:sz]
	}
	return &Buffer{data: data, numaNode: numaNode, slab: nil}
}

// linuxRelease returns hugepage memory to the OS.
func linuxRelease(buf api.Buffer) {
	if b, ok := buf.(*Buffer); ok {
		syscall.Munmap(b.data)
	}
}

// newSlabPool builds a slabPool with linuxAlloc/release callbacks.
func newSlabPool(size int) *slabPool {
	sp := &slabPool{size: size}
	sp.newBuf = func(sz, numaNode int) api.Buffer {
		buf := linuxAlloc(sz, numaNode)
		if b, ok := buf.(*Buffer); ok {
			b.slab = sp
		}
		return buf
	}
	sp.release = linuxRelease
	return sp
}
