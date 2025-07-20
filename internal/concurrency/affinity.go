// File: internal/concurrency/affinity.go
// Package concurrency provides cross-platform affinity API.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Public functions delegate to platform-specific implementations via build tags.

package concurrency

// PreferredCPUID returns a recommended CPU core for numaNode; -1 for any.
func PreferredCPUID(numaNode int) int {
	return platformPreferredCPUID(numaNode)
}

// CurrentNUMANodeID returns the NUMA node ID of current thread or -1.
func CurrentNUMANodeID() int {
	return platformCurrentNUMANodeID()
}

// NUMANodes returns total NUMA nodes count (>=1).
func NUMANodes() int {
	return platformNUMANodes()
}

// PinCurrentThread binds the current OS thread to CPU and NUMA.
// cpuID<0 means any CPU, numaNode<0 means any node.
func PinCurrentThread(numaNode, cpuID int) error {
	return platformPinCurrentThread(numaNode, cpuID)
}

// UnpinCurrentThread clears affinity constraints.
func UnpinCurrentThread() error {
	return platformUnpinCurrentThread()
}
