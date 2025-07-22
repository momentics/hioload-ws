//go:build windows
// +build windows

// File: pool/bufferpool_windows.go
// Package pool provides Windows-specific NUMA-aware buffer pool.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// windowsBuffer uses VirtualAllocExNuma for large pages on Windows.
// newBufferPool returns a baseBufferPool configured for windowsBuffer.

package pool

import (
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

var procVirtualAllocExNuma = windows.
	NewLazySystemDLL("kernel32.dll").
	NewProc("VirtualAllocExNuma")

// windowsBuffer implements api.Buffer for Windows large-page allocations.
type windowsBuffer struct {
	data   []byte
	pool   *baseBufferPool[*windowsBuffer]
	numaID int
}

// Bytes returns the underlying byte slice.
func (b *windowsBuffer) Bytes() []byte { return b.data }

// Slice returns a sub-buffer referencing the same data.
func (b *windowsBuffer) Slice(from, to int) api.Buffer {
	return &windowsBuffer{data: b.data[from:to], pool: b.pool, numaID: b.numaID}
}

// Release returns this buffer to its pool.
func (b *windowsBuffer) Release() { b.pool.recycle(b) }

// Copy makes a fresh copy of the data.
func (b *windowsBuffer) Copy() []byte {
	c := make([]byte, len(b.data))
	copy(c, b.data)
	return c
}

// NUMANode reports the NUMA node.
func (b *windowsBuffer) NUMANode() int { return b.numaID }

// newWindowsBuffer allocates via VirtualAllocExNuma or falls back.
func newWindowsBuffer(size, numaNode int) *windowsBuffer {
	node := uint32(numaNode)
	addr, _, err := procVirtualAllocExNuma.Call(
		uintptr(windows.CurrentProcess()), 0,
		uintptr(size),
		uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
		uintptr(windows.PAGE_READWRITE),
		uintptr(node),
	)
	var data []byte
	if addr == 0 || err != nil {
		data = make([]byte, size)
	} else {
		data = unsafe.Slice((*byte)(unsafe.Pointer(addr)), size)
	}
	return &windowsBuffer{data: data, numaID: numaNode}
}

// newBufferPool constructs the Windows buffer pool via baseBufferPool.
func newBufferPool(numaNode int) api.BufferPool {
	var base *baseBufferPool[*windowsBuffer]
	factory := func(size, pref int) *windowsBuffer {
		buf := newWindowsBuffer(size, pref)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPool(numaNode, factory)
	return base
}
