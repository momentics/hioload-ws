//go:build windows
// +build windows

// File: internal/concurrency/affinity_windows.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows-specific CPU affinity implementation.

package concurrency

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procSetThreadAffinityMask    = kernel32.NewProc("SetThreadAffinityMask")
	procGetCurrentThread         = kernel32.NewProc("GetCurrentThread")
	procGetNumaHighestNodeNumber = kernel32.NewProc("GetNumaHighestNodeNumber")
	procGetNumaProcessorNodeEx   = kernel32.NewProc("GetNumaProcessorNodeEx")
)

// platformPreferredCPUID returns preferred CPU for NUMA node on Windows.
func platformPreferredCPUID(numaNode int) int {
	// Simple mapping: return first CPU of the NUMA node
	return numaNode * (NumCPUs() / NUMANodes())
}

// platformCurrentNUMANodeID returns current NUMA node on Windows.
func platformCurrentNUMANodeID() int {
	// Windows implementation would query current processor's NUMA node
	return 0 // Simplified for now
}

// platformPinCurrentThread pins thread to CPU on Windows.
func platformPinCurrentThread(numaNode int, cpuID int) {
	if cpuID < 0 {
		return
	}

	handle, _, _ := procGetCurrentThread.Call()
	mask := uintptr(1) << uint(cpuID)
	procSetThreadAffinityMask.Call(handle, mask)
}

// platformUnpinCurrentThread removes affinity on Windows.
func platformUnpinCurrentThread() {
	handle, _, _ := procGetCurrentThread.Call()
	// Set affinity to all CPUs
	allCPUsMask := uintptr((1 << uint(NumCPUs())) - 1)
	procSetThreadAffinityMask.Call(handle, allCPUsMask)
}

// platformNUMANodes returns number of NUMA nodes on Windows.
func platformNUMANodes() int {
	var highestNodeNumber uint32
	ret, _, _ := procGetNumaHighestNodeNumber.Call(uintptr(unsafe.Pointer(&highestNodeNumber)))
	if ret != 0 {
		return int(highestNodeNumber) + 1
	}
	return 1
}
