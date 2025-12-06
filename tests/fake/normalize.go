// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

// FakeNormalize provides a mock implementation of normalization functions for testing.
type FakeNormalize struct {
	NodeCount int
	CPUsCount int
}

// NUMANode validates and normalizes a NUMA node index.
func (fn *FakeNormalize) NUMANode(requested int, maxNodes int) int {
	if maxNodes < 1 {
		return 0
	}
	if requested < 0 || requested >= maxNodes {
		return 0
	}
	return requested
}

// NUMANodeAuto chooses the correct index using current topology.
func (fn *FakeNormalize) NUMANodeAuto(requested int) int {
	if requested < 0 {
		// Simulate auto-detection
		return 0
	}
	return fn.NUMANode(requested, fn.NodeCount)
}

// CPUIndex validates and normalizes a CPU index.
func (fn *FakeNormalize) CPUIndex(requested int, maxCPUs int) int {
	if maxCPUs < 1 {
		return 0
	}
	if requested < 0 || requested >= maxCPUs {
		return 0
	}
	return requested
}

// CPUIndexAuto tries to use preferred index.
func (fn *FakeNormalize) CPUIndexAuto(requested int) int {
	return fn.CPUIndex(requested, fn.CPUsCount)
}