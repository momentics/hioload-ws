// File: internal/concurrency/affinity.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Public API surface for affinity operations, delegating to platform-specific implementations.

package concurrency

// PreferredCPUID returns a recommended CPU ID for the specified NUMA node.
func PreferredCPUID(numaNode int) int {
	return platformPreferredCPUID(numaNode)
}

// CurrentNUMANodeID returns the NUMA node of the calling thread.
func CurrentNUMANodeID() int {
	return platformCurrentNUMANodeID()
}

// NUMANodes returns the total number of NUMA nodes available.
func NUMANodes() int {
	return platformNUMANodes()
}

// PinCurrentThread binds the current OS thread to the given NUMA node and CPU.
// Passing -1 for either parameter indicates "no preference".
func PinCurrentThread(numaNode, cpuID int) error {
	return platformPinCurrentThread(numaNode, cpuID)
}

// UnpinCurrentThread clears any CPU or NUMA binding on this OS thread.
func UnpinCurrentThread() error {
	return platformUnpinCurrentThread()
}
