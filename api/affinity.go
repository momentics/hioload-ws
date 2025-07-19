// Package api
// Author: momentics@gmail.com
//
// CPU/NUMA affinity, thread pinning and topology definitions.

package api

// Affinity controls execution on particular CPUs/NUMA nodes.
type Affinity interface {
    // Pin locks the current goroutine to a CPU or NUMA node.
    Pin(cpuID int, numaID int) error
    // Unpin removes affinity.
    Unpin() error
    // Get returns current CPU and NUMA node.
    Get() (cpuID int, numaID int, err error)
}
