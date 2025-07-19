// hioload-ws/internal/concurrency/affinity.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform affinity runtime helpers for advanced NUMA/CPU binding policy.
// Abstracts NUMA/CPU queries for scheduling-aware logic.

package concurrency

import (
	"runtime"
)

// PreferredCPUID returns the ID for the "best" local CPU given a NUMA node.
func PreferredCPUID(numaNode int) int {
	// This is a placeholder — on real servers use OS/hardware detection.
	numCPU := runtime.NumCPU()
	if numaNode < 0 {
		return 0
	}
	return (numaNode * 2) % numCPU
}

// CurrentNUMANodeID returns the NUMA node for the currently running thread (if known).
func CurrentNUMANodeID() int {
	// Cross-platform stub: can be replaced with syscall/cgo or platform API if required.
	return -1 // Unsupported by default
}
