// File: pool/bufferpool_windows.go
//go:build windows
// +build windows

//
// Package pool provides Windows-specific NUMA-aware buffer pool implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file defines windowsBuffer and constructs a BufferPool for Windows.
// It allocates using VirtualAllocExNuma for large pages when possible, else falls back.

package pool

import (
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

// windowsBuffer implements api.Buffer for Windows large-page allocations.
type windowsBuffer struct {
	data   []byte
	pool   *baseBufferPool
	numaID int
}

// Bytes returns the underlying byte slice.
func (b *windowsBuffer) Bytes() []byte {
	return b.data
}

// Slice returns a zero-copy sub-buffer of the same memory.
func (b *windowsBuffer) Slice(from, to int) api.Buffer {
	return &windowsBuffer{
		data:   b.data[from:to],
		pool:   b.pool,
		numaID: b.numaID,
	}
}

// Release returns the buffer to its originating pool.
func (b *windowsBuffer) Release() {
	b.pool.recycle(b)
}

// Copy makes an independent copy of the buffer's contents.
func (b *windowsBuffer) Copy() []byte {
	dup := make([]byte, len(b.data))
	copy(dup, b.data)
	return dup
}

// NUMANode reports the NUMA node where the buffer was allocated.
func (b *windowsBuffer) NUMANode() int {
	return b.numaID
}

// newWindowsBuffer allocates a buffer of 'size' bytes on the given NUMA node using VirtualAllocExNuma.
// If allocation fails, it falls back to make([]byte).
func newWindowsBuffer(size, numaNode int) *windowsBuffer {
	// Declare proc inside function to avoid duplicate at package scope.
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualAllocExNuma")

	addr, _, err := proc.Call(
		uintptr(windows.CurrentProcess()),
		0,
		uintptr(size),
		uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
		uintptr(windows.PAGE_READWRITE),
		uintptr(uint32(numaNode)),
	)

	var data []byte
	if addr == 0 || err != nil {
		data = make([]byte, size)
	} else {
		data = unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	}

	return &windowsBuffer{
		data:   data,
		numaID: numaNode,
	}
}

// newBufferPool constructs and returns the Windows-specific BufferPool.
// We declare 'var base' first so that the closure can correctly capture it.
func newBufferPool(numaNode int) api.BufferPool {
	var base *baseBufferPool
	factory := func(size, node int) api.Buffer {
		buf := newWindowsBuffer(size, node)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPool(numaNode, factory)
	return base
}
