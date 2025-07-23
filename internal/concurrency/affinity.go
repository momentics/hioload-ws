// File: internal/concurrency/affinity.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Public functions delegate to platform-specific implementations via build tags.

package concurrency

// PreferredCPUID returns a recommended CPU core for the given NUMA node.
func PreferredCPUID(numaNode int) int {
	return platformPreferredCPUID(numaNode)
}

// CurrentNUMANodeID returns the NUMA node ID of the current thread.
func CurrentNUMANodeID() int {
	return platformCurrentNUMANodeID()
}

// NUMANodes returns the total number of NUMA nodes (>=1).
func NUMANodes() int {
	return platformNUMANodes()
}

// PinCurrentThread binds the current OS thread to CPU and NUMA.
// cpuID<0 means any CPU; numaNode<0 means any NUMA node.
func PinCurrentThread(numaNode, cpuID int) error {
	return platformPinCurrentThread(numaNode, cpuID)
}

// UnpinCurrentThread clears any affinity constraints.
func UnpinCurrentThread() error {
	return platformUnpinCurrentThread()
}
