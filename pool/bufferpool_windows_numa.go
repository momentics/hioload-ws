// File: pool/bufferpool_windows_numa.go
//go:build windows
// +build windows

//
// Package pool provides Windows NUMA helper functions.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file exposes virtualAllocOnNode for explicit NUMA-aware allocation.
// We remove the duplicate proc declaration present in bufferpool_windows.go.

package pool

import (
	"golang.org/x/sys/windows"
)

// virtualAllocOnNode allocates 'size' bytes on the specified NUMA node in 'process'.
// It wraps Windows VirtualAllocExNuma and returns the base address or error.
func virtualAllocOnNode(process windows.Handle, size int, node uint32) (uintptr, error) {
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
