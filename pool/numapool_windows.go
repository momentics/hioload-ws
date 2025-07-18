//go:build windows
// +build windows

// File: pool/numapool_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows-specific NUMA-aware allocator factory.

package pool

// createNUMAAllocator returns the NUMA allocator for Windows.
func createNUMAAllocator() NUMAAllocator {
	return newWindowsNUMAAllocator()
}
