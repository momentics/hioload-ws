// File: internal/concurrency/numa_windows.go
//go:build windows
// +build windows

//
// Windows NUMA helper functions.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

import (
	"golang.org/x/sys/windows"
)

// VirtualAllocExNuma prototype
var (
	procVirtualAllocExNuma = windows.NewLazySystemDLL("kernel32.dll").NewProc("VirtualAllocExNuma")
)

// AllocOnNode reserves and commits memory on specified NUMA node.
func AllocOnNode(size int, node uint32) (uintptr, error) {
	// HANDLE hProcess = GetCurrentProcess()
	hProc := windows.CurrentProcess()
	addr, _, err := procVirtualAllocExNuma.Call(
		uintptr(hProc),
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
