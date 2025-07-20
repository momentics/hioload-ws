//go:build !linux && !windows
// +build !linux,!windows

// File: internal/concurrency/affinity_stub.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Stub implementation for unsupported platforms.

package concurrency

// platformPreferredCPUID stub implementation.
func platformPreferredCPUID(numaNode int) int {
	return 0
}

// platformCurrentNUMANodeID stub implementation.
func platformCurrentNUMANodeID() int {
	return 0
}

// platformPinCurrentThread stub implementation.
func platformPinCurrentThread(numaNode int, cpuID int) {
	// No-op on unsupported platforms
}

// platformUnpinCurrentThread stub implementation.
func platformUnpinCurrentThread() {
	// No-op on unsupported platforms
}

// platformNUMANodes stub implementation.
func platformNUMANodes() int {
	return 1
}
