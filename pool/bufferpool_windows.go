// File: pool/bufferpool_windows.go
//go:build windows
// +build windows

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows-specific NUMA-aware buffer pool supporting virtual allocation with large pages.
// Exports two constructors for BufferPool:
//   - newBufferPool
//   - newBufferPoolWithCap
// These enable high-performance buffer reuse, memory-localized allocations, and large channel pools.

package pool

import (
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"golang.org/x/sys/windows"
)

// windowsBuffer implements api.Buffer for memory allocated via VirtualAllocExNuma.
// Buffers reference their pool and originating NUMA node for correct recycling.
type windowsBuffer struct {
	data   []byte
	pool   *baseBufferPool
	numaID int
}

// Bytes returns the raw byte data for zero-copy I/O.
func (b *windowsBuffer) Bytes() []byte {
	return b.data
}

// Slice returns a zero-copy view of the buffer based on the specified range.
func (b *windowsBuffer) Slice(from, to int) api.Buffer {
	return &windowsBuffer{
		data:   b.data[from:to],
		pool:   b.pool,
		numaID: b.numaID,
	}
}

// Release recycles this buffer into its pool for future Get().
func (b *windowsBuffer) Release() {
	b.pool.recycle(b)
}

// Copy creates a new []byte with copied contents of this buffer.
func (b *windowsBuffer) Copy() []byte {
	dup := make([]byte, len(b.data))
	copy(dup, b.data)
	return dup
}

// NUMANode returns the NUMA node ID where the buffer was allocated.
func (b *windowsBuffer) NUMANode() int {
	return b.numaID
}

// newWindowsBuffer allocates a buffer using VirtualAllocExNuma if possible (large pages).
// If allocation fails or is unavailable, falls back to standard heap allocation.
func newWindowsBuffer(size, numaNode int) *windowsBuffer {
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

// newBufferPool creates a Windows BufferPool with default channel capacity (1024).
func newBufferPool(numaNode int) api.BufferPool {
	return newBufferPoolWithCap(numaNode, 1024)
}

// newBufferPoolWithCap creates a NUMA-aware BufferPool with a given channel capacity.
// This is the primary path for BufferPoolManagerWithCap to control in-pool buffer count.
func newBufferPoolWithCap(numaNode int, cap int) api.BufferPool {
	var base *baseBufferPool
	factory := func(size, node int) api.Buffer {
		buf := newWindowsBuffer(size, node)
		buf.pool = base
		return buf
	}
	base = newBaseBufferPoolWithCap(numaNode, factory, cap)
	return base
}

// newBaseBufferPoolWithCap is a helper constructing the baseBufferPool with user channel cap.
// It is platform-neutral (used for both Linux and Windows).
func newBaseBufferPoolWithCap(numaNode int, factory func(int, int) api.Buffer, cap int) *baseBufferPool {
	return &baseBufferPool{
		pools:   map[int]chan api.Buffer{numaNode: make(chan api.Buffer, cap)},
		factory: factory,
	}
}
