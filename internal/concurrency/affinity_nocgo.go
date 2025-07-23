// File: internal/concurrency/affinity_nocgo.go
//go:build linux && !cgo
// +build linux,!cgo

//
// Stubbed Linux affinity implementation for builds with CGO disabled.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

// Return CPU 0 when CGO is disabled.
func platformPreferredCPUID(numaNode int) int { return 0 }

// No NUMA information when CGO is disabled.
func platformCurrentNUMANodeID() int { return -1 }
func platformNUMANodes() int         { return 1 }

// No-op pin/unpin functions when CGO is disabled.
func platformPinCurrentThread(numaNode, cpuID int) error { return nil }
func platformUnpinCurrentThread() error                  { return nil }
