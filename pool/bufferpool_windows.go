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
	"golang.org/x/sys/windows"
)

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
	var buf *Buffer
	if ret == 0 {
		buf = &Buffer{data: make([]byte, sz), numaNode: numaNode, slab: nil}
	} else {
		data := unsafe.Slice((*byte)(unsafe.Pointer(ret)), sz)
		buf = &Buffer{data: data, numaNode: numaNode, slab: nil}
	}
	return buf
}

// windowsRelease frees memory via VirtualFree.
func windowsRelease(buf api.Buffer) {
	if b, ok := buf.(*Buffer); ok {
		proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualFree")
		proc.Call(
			uintptr(unsafe.Pointer(&b.data[0])),
			uintptr(len(b.data)),
			uintptr(windows.MEM_RELEASE),
		)
	}
}

// newSlabPool builds a slabPool with windowsAlloc/release callbacks.
func newSlabPool(size int) *slabPool {
	sp := &slabPool{size: size}
	sp.newBuf = func(sz, numaNode int) api.Buffer {
		buf := windowsAlloc(sz, numaNode)
		if b, ok := buf.(*Buffer); ok {
			b.slab = sp
		}
		return buf
	}
	sp.release = windowsRelease
	return sp
}
