// File: pool/bufferpool_windows_numa.go
//go:build windows
// +build windows

//
// Helper for NUMA memory allocation via VirtualAllocExNuma.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package pool

import (
	"golang.org/x/sys/windows"
)

// VirtualAllocExNuma wrapper
func virtualAllocExNuma(process windows.Handle, size int, node uint32) (uintptr, error) {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualAllocExNuma")
	addr, _, err := proc.Call(
		uintptr(process),
		0,
		uintptr(size),
		uintptr(windows.MEM_RESERVE|windows.MEM_COMMIT|windows.MEM_LARGE_PAGES),
		uintptr(windows.PAGE_READWRITE),
		uintptr(node),
	)
	if addr == 0 {
		return 0, err
	}
	return addr, nil
}
