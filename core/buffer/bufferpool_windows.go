// File: pool/bufferpool_windows.go
//go:build windows
// +build windows

//
// Package pool: Windows-specific slab allocator using VirtualAllocExNuma.
//
// Buffers are allocated with MEM_LARGE_PAGES on the specified NUMA node.
// Fallback to Go heap on failure.
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"unsafe"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/core/concurrency"
	"golang.org/x/sys/windows"
)

// windowsAlloc reserves `sz` bytes on `numaNode` via VirtualAllocExNuma.
// windowsAlloc reserves `sz` bytes on `numaNode` via VirtualAllocExNuma.
func windowsAlloc(sz, numaNode int) api.Buffer {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualAllocExNuma")
	ret, _, _ := proc.Call(
		uintptr(windows.CurrentProcess()),
		0,
		uintptr(sz),
		uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
		uintptr(windows.PAGE_READWRITE),
		uintptr(uint32(numaNode)),
	)
	var buf api.Buffer
	if ret == 0 {
		buf = api.Buffer{Data: make([]byte, sz), NUMA: numaNode}
	} else {
		data := unsafe.Slice((*byte)(unsafe.Pointer(ret)), sz)
		buf = api.Buffer{Data: data, NUMA: numaNode}
	}
	return buf
}

// windowsRelease frees memory via VirtualFree.
func windowsRelease(buf api.Buffer) {
	if len(buf.Data) > 0 {
		// We need pointer to first byte of data
		addr := &buf.Data[0]
		proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualFree")
		proc.Call(
			uintptr(unsafe.Pointer(addr)),
			uintptr(len(buf.Data)),
			uintptr(windows.MEM_RELEASE),
		)
	}
}

// newSlabPool builds a slabPool with windowsAlloc/release callbacks.
func newSlabPool(size int) *slabPool {
	sp := &slabPool{
		size:  size,
		queue: concurrency.NewLockFreeQueue[api.Buffer](defaultPoolCapacity),
	}
	sp.newBuf = windowsAlloc
	sp.release = windowsRelease
	return sp
}
