// File: internal/concurrency/affinity_linux_pure.go
//go:build linux && !cgo
// +build linux,!cgo

//
// Pure-Go fallback implementation for Linux affinity operations when CGO is disabled.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

// platformPreferredCPUID returns a suggested CPU core index for the given NUMA node.
// On Linux without CGO, returns 0 as fallback.
func platformPreferredCPUID(numaNode int) int {
	return 0
}

// platformCurrentNUMANodeID returns the NUMA node ID of the current thread.
// Returns -1 on Linux without CGO (NUMA not available via CGO).
func platformCurrentNUMANodeID() int {
	return -1
}

// platformNUMANodes returns the total number of configured NUMA nodes.
// Returns 1 on Linux without CGO (no NUMA information available).
func platformNUMANodes() int {
	return 1
}

// platformPinCurrentThread binds the current OS thread to the specified CPU and NUMA node.
// On Linux without CGO, this is a no-op.
func platformPinCurrentThread(numaNode, cpuID int) error {
	// Without CGO, no NUMA/CPU pinning is performed
	return nil
}

// platformUnpinCurrentThread clears any CPU affinity (no-op without CGO).
func platformUnpinCurrentThread() error {
	// Without CGO, no pinning was performed, so no unpinning needed
	return nil
}