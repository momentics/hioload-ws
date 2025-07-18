//go:build linux
// +build linux

// File: pool/numapool_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux-specific NUMA-aware allocator factory.

package pool

// createNUMAAllocator returns the NUMA allocator for Linux.
func createNUMAAllocator() NUMAAllocator {
	return newLinuxNUMAAllocator()
}
