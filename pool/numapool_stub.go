//go:build !linux && !windows
// +build !linux,!windows

// File: pool/numapool_stub.go
// Author: momentics <momentics@gmail.com>
//
// Stub NUMA allocator for unsupported platforms.

package pool

// createNUMAAllocator returns nil for unsupported platforms.
func createNUMAAllocator() NUMAAllocator {
	return nil
}
