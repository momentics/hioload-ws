// File: internal/normalize/normalizer.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Unified index normalization routines for NUMA nodes and CPU indices.
// Ensures all allocations, affinity actions, and pool lookups validate their indices
// against actual hardware topology to prevent out-of-bounds and silent fallbacks.
// Should be used by ALL call sites working with NUMA or CPU parameters.
//
// Example usage:
//
//   node := normalize.NUMANode(requested, concurrency.NUMANodes())
//   cpu  := normalize.CPUIndex(requested, runtime.NumCPU())
//
// If input is invalid, default index (0) is used and warning may be logged.

package normalize

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/momentics/hioload-ws/internal/concurrency"
)

var (
	// For logging normalization events (can be replaced with structured logger).
	logNormalize = func(msg string, args ...any) {
		fmt.Printf("[normalize] "+msg+"\n", args...)
	}
	mu sync.Mutex
)

// NUMANode validates and normalizes a NUMA node index against current hardware topology.
//   - If requested < 0, or >= maxNodes, returns fallback value 0.
//   - If maxNodes < 1, always returns 0.
func NUMANode(requested int, maxNodes int) int {
	if maxNodes < 1 {
		logNormalize("NUMA nodes topology reported zero, fallback to node 0")
		return 0
	}
	if requested < 0 || requested >= maxNodes {
		logNormalize("NUMA node index %d out of range [0, %d), fallback to node 0", requested, maxNodes)
		return 0
	}
	return requested
}

// NUMANodeAuto chooses the correct index using current topology.
// If requested < 0, tries to use concurrency.CurrentNUMANodeID(), falls back to 0.
func NUMANodeAuto(requested int) int {
	cnt := concurrency.NUMANodes()
	if requested < 0 {
		node := concurrency.CurrentNUMANodeID()
		if node >= 0 && node < cnt {
			return node
		}
		logNormalize("CurrentNUMANodeID() returned invalid (%d), fallback to 0", node)
		return 0
	}
	return NUMANode(requested, cnt)
}

// CPUIndex validates and normalizes a CPU index against runtime.NumCPU().
//   - If requested < 0, or >= maxCPUs, returns 0.
//   - If maxCPUs < 1, returns 0.
func CPUIndex(requested int, maxCPUs int) int {
	if maxCPUs < 1 {
		logNormalize("CPU topology returned <1 cores, fallback to 0")
		return 0
	}
	if requested < 0 || requested >= maxCPUs {
		logNormalize("CPU index %d out of range [0, %d), fallback to 0", requested, maxCPUs)
		return 0
	}
	return requested
}

// CPUIndexAuto tries to use preferred index, else picks 0.
func CPUIndexAuto(requested int) int {
	cnt := runtime.NumCPU()
	if requested < 0 {
		return 0 // or could do smarter affinity detection
	}
	return CPUIndex(requested, cnt)
}
