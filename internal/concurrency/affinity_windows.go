// File: internal/concurrency/affinity_windows.go
//go:build windows
// +build windows

//
// Package concurrency implements Windows-specific CPU affinity.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This implementation provides CPU pinning for the current OS thread.
// NUMA-awareness is not supported on Windows in this build.

package concurrency

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

var (
	modkernel32               = windows.NewLazySystemDLL("kernel32.dll")
	procSetThreadAffinityMask = modkernel32.NewProc("SetThreadAffinityMask")
	procGetCurrentThread      = modkernel32.NewProc("GetCurrentThread")
)

// PreferredCPUID returns a suggested CPU core for the given NUMA node.
// On Windows, NUMA-awareness is not supported here, so we ignore numaNode
// and return 0 if numaNode<0 or modulo number of CPUs otherwise.
func platformPreferredCPUID(numaNode int) int {
	total := runtime.NumCPU()
	if total <= 0 || numaNode < 0 {
		return 0
	}
	// spread across CPUs by NUMA node index, fallback
	return numaNode % total
}

// platformCurrentNUMANodeID returns -1 on Windows to indicate unsupported.
func platformCurrentNUMANodeID() int {
	return -1
}

// platformNUMANodes returns 1 to indicate no NUMA diversity.
func platformNUMANodes() int {
	return 1
}

// platformPinCurrentThread pins the current OS thread to the specified CPU.
// cpuID<0 means “any CPU” (no-op). NUMA node is ignored.
func platformPinCurrentThread(_, cpuID int) error {
	runtime.LockOSThread()
	if cpuID < 0 {
		return nil
	}
	handle, _, _ := procGetCurrentThread.Call()
	// construct affinity mask for one CPU
	mask := uintptr(1) << uint(cpuID)
	old, _, err := procSetThreadAffinityMask.Call(handle, mask)
	if old == 0 {
		return fmt.Errorf("SetThreadAffinityMask failed: %v", err)
	}
	return nil
}

// platformUnpinCurrentThread resets affinity to all CPUs.
func platformUnpinCurrentThread() error {
	runtime.LockOSThread()
	handle, _, _ := procGetCurrentThread.Call()
	total := runtime.NumCPU()
	if total <= 0 {
		total = 1
	}
	// mask with lower 'total' bits set
	mask := (uintptr(1) << uint(total)) - 1
	old, _, err := procSetThreadAffinityMask.Call(handle, mask)
	if old == 0 {
		return fmt.Errorf("SetThreadAffinityMask(unpin) failed: %v", err)
	}
	return nil
}
