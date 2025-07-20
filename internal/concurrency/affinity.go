// File: internal/concurrency/affinity.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Cross-platform CPU and NUMA affinity management with runtime detection.

package concurrency

import (
	"runtime"
)

// affinitySupported indicates if CPU affinity is supported on this platform.
var affinitySupported = checkAffinitySupport()

// checkAffinitySupport detects if CPU affinity is available.
func checkAffinitySupport() bool {
	// This would be implemented per platform
	return true
}

// PreferredCPUID returns the preferred CPU for the given NUMA node.
func PreferredCPUID(numaNode int) int {
	if numaNode < 0 {
		return 0
	}
	return platformPreferredCPUID(numaNode)
}

// CurrentNUMANodeID returns the NUMA node of the current thread.
func CurrentNUMANodeID() int {
	return platformCurrentNUMANodeID()
}

// UnpinCurrentThread removes CPU affinity constraints from current thread.
func UnpinCurrentThread() {
	if !affinitySupported {
		return
	}

	platformUnpinCurrentThread()
}

// NumCPUs returns the number of logical CPUs.
func NumCPUs() int {
	return runtime.NumCPU()
}

// NUMANodes returns the number of NUMA nodes.
func NUMANodes() int {
	return platformNUMANodes()
}
