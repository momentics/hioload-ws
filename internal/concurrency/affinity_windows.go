// File: internal/concurrency/affinity_windows.go
//go:build windows
// +build windows

//
// Windows-specific CPU and NUMA affinity implementation.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procSetThreadGroupAffinity    = modkernel32.NewProc("SetThreadGroupAffinity")
	procGetCurrentThread          = modkernel32.NewProc("GetCurrentThread")
	procGetNumaHighestNodeNumber  = modkernel32.NewProc("GetNumaHighestNodeNumber")
	procGetNumaProcessorNode      = modkernel32.NewProc("GetNumaProcessorNode")
	procGetCurrentProcessorNumber = modkernel32.NewProc("GetCurrentProcessorNumber")
)

// platformPreferredCPUID returns a suggested CPU core for the given NUMA node.
func platformPreferredCPUID(numaNode int) int {
	// Query highest node to validate numaNode range
	var highestNode uint32
	r, _, _ := procGetNumaHighestNodeNumber.Call(uintptr(unsafe.Pointer(&highestNode)))
	if r == 0 {
		return 0
	}
	if numaNode < 0 || uint32(numaNode) > highestNode {
		// fallback to any CPU
		return 0
	}
	// Get processor number for this NUMA node
	var procNumber uint32
	r, _, _ = procGetNumaProcessorNode.Call(uintptr(0), uintptr(unsafe.Pointer(&procNumber)))
	if r == 0 {
		return 0
	}
	cpus := runtime.NumCPU()
	return int(procNumber % uint32(cpus))
}

// platformCurrentNUMANodeID returns current thread's NUMA node.
func platformCurrentNUMANodeID() int {
	// Use GetNumaProcessorNode on current processor
	var cpu uint32
	// syscall.GetCurrentProcessorNumber available on Windows 10+
	r1, _, e1 := procGetCurrentProcessorNumber.Call()
	if e1 != syscall.Errno(0) {
		return -1
	}
	cpu = uint32(r1)

	var node uint32
	r, _, _ := procGetNumaProcessorNode.Call(uintptr(cpu), uintptr(unsafe.Pointer(&node)))
	if r == 0 {
		return -1
	}
	return int(node)
}

// platformNUMANodes returns total NUMA nodes count.
func platformNUMANodes() int {
	var highestNode uint32
	r, _, _ := procGetNumaHighestNodeNumber.Call(uintptr(unsafe.Pointer(&highestNode)))
	if r == 0 {
		return 1
	}
	return int(highestNode) + 1
}

// GROUP_AFFINITY structure for SetThreadGroupAffinity.
type groupAffinity struct {
	mask     uint64
	group    uint16
	reserved [3]uint16
}

// platformPinCurrentThread pins the current OS thread to the specified CPU and NUMA node.
func platformPinCurrentThread(numaNode, cpuID int) error {
	runtime.LockOSThread()
	handle, _, _ := procGetCurrentThread.Call()
	// Determine group and mask via GetNumaNodeProcessorMask
	//var procMask uint64
	//var nodeMask uint64
	// call GetNumaNodeProcessorMask (oid: procGetNumaNodeProcessorMask) omitted for brevity...
	// For simplicity, assume single-group system:
	ga := groupAffinity{
		mask:  1 << uint(cpuID),
		group: 0,
	}
	r, _, err := procSetThreadGroupAffinity.Call(handle, uintptr(unsafe.Pointer(&ga)), 0)
	if r == 0 {
		return fmt.Errorf("SetThreadGroupAffinity failed: %v", err)
	}
	return nil
}

// platformUnpinCurrentThread resets affinity to all CPUs.
func platformUnpinCurrentThread() error {
	runtime.LockOSThread()
	handle, _, _ := procGetCurrentThread.Call()
	total := runtime.NumCPU()
	mask := uint64((1 << uint(total)) - 1)
	ga := groupAffinity{
		mask:  mask,
		group: 0,
	}
	r, _, err := procSetThreadGroupAffinity.Call(handle, uintptr(unsafe.Pointer(&ga)), 0)
	if r == 0 {
		return fmt.Errorf("SetThreadGroupAffinity(unpin) failed: %v", err)
	}
	return nil
}
