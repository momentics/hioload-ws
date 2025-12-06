// File: internal/concurrency/affinity_other.go
//go:build !linux && !windows
// +build !linux,!windows

//
// Fallback implementation for platform-agnostic affinity operations.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

// platformPreferredCPUID returns a suggested CPU core index for the given NUMA node.
// On non-Linux/Windows platforms, returns 0 as fallback.
func platformPreferredCPUID(numaNode int) int {
	return 0
}

// platformCurrentNUMANodeID returns the NUMA node ID of the current thread.
// Returns -1 on non-Linux/Windows platforms (NUMA not available).
func platformCurrentNUMANodeID() int {
	return -1
}

// platformNUMANodes returns the total number of configured NUMA nodes.
// Returns 1 on non-Linux/Windows platforms (no NUMA).
func platformNUMANodes() int {
	return 1
}

// platformPinCurrentThread binds the current OS thread to the specified CPU and NUMA node.
// On non-Linux/Windows platforms, this is a no-op.
func platformPinCurrentThread(numaNode, cpuID int) error {
	// On other platforms, no pinning is performed
	return nil
}

// platformUnpinCurrentThread clears any CPU affinity (no-op on other platforms).
func platformUnpinCurrentThread() error {
	// On other platforms, no pinning was performed, so no unpinning needed
	return nil
}