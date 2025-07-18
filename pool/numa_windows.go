//go:build windows
// +build windows

// File: pool/numa_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows-specific NUMA allocator using VirtualAllocExNuma.

package pool

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	MEM_COMMIT   = 0x00001000
	MEM_RESERVE  = 0x00002000
	PAGE_READWRITE = 0x04
)

// windowsNUMAAllocator is a NUMA allocator implementation for Windows.
type windowsNUMAAllocator struct{}

func newWindowsNUMAAllocator() NUMAAllocator {
	return &windowsNUMAAllocator{}
}

func (w *windowsNUMAAllocator) Alloc(size int, node int) ([]byte, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procVirtualAllocExNuma := kernel32.NewProc("VirtualAllocExNuma")
	procGetCurrentProcess := kernel32.NewProc("GetCurrentProcess")
	hProc, _, _ := procGetCurrentProcess.Call()
	ptr, _, err := procVirtualAllocExNuma.Call(
		hProc,
		0,
		uintptr(size),
		uintptr(MEM_RESERVE|MEM_COMMIT),
		uintptr(PAGE_READWRITE),
		uintptr(node),
	)
	if ptr == 0 {
		return nil, errors.New("windows NUMA VirtualAllocExNuma failed: " + err.Error())
	}
	bs := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), size)
	return bs, nil
}

func (w *windowsNUMAAllocator) Free(buf []byte) {
	if len(buf) == 0 {
		return
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procVirtualFree := kernel32.NewProc("VirtualFree")
	addr := uintptr(unsafe.Pointer(&buf[0]))
	const MEM_RELEASE = 0x8000
	procVirtualFree.Call(addr, 0, uintptr(MEM_RELEASE))
}

func (w *windowsNUMAAllocator) Nodes() (int, error) {
	return 1, nil
}
