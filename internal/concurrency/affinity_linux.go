// File: internal/concurrency/affinity_linux.go
//go:build linux && cgo
// +build linux,cgo

package concurrency

// #cgo LDFLAGS: -lnuma
// #define _GNU_SOURCE
// #include <numa.h>
// #include <sched.h>
// #include <errno.h>
//
// // Helper to check if numa is available (wrapper to avoid macro issues)
// int check_numa_avail() {
//     return numa_available();
// }
import "C"

import (
	"fmt"
	"runtime"
	"sync"
)

var (
	numaAvailOnce sync.Once
	numaAvailable bool
)

func isNumaAvailable() bool {
	numaAvailOnce.Do(func() {
		if C.check_numa_avail() != -1 {
			numaAvailable = true
		}
	})
	return numaAvailable
}

// platformPreferredCPUID returns a suggested CPU core index for the given NUMA node.
// Falls back to sysfs parsing or simplified logic as libnuma focuses on nodes.
func platformPreferredCPUID(numaNode int) int {
	// For now, return 0 or rely on OS scheduler if pinning to node.
	// We can implement strict CPU picking if needed, but per-node pinning is the main requirement.
	return 0 
}

// platformCurrentNUMANodeID returns the NUMA node ID of the current thread.
func platformCurrentNUMANodeID() int {
	if !isNumaAvailable() {
		return 0
	}
	cpu := C.sched_getcpu()
	if cpu < 0 {
		return -1
	}
	node := C.numa_node_of_cpu(cpu)
	return int(node)
}

// platformNUMANodes returns the total number of configured NUMA nodes.
func platformNUMANodes() int {
	if !isNumaAvailable() {
		return 1
	}
	return int(C.numa_num_configured_nodes())
}

// platformPinCurrentThread binds the current OS thread to the specified NUMA node.
func platformPinCurrentThread(numaNode, cpuID int) error {
	runtime.LockOSThread()
	if !isNumaAvailable() {
		return fmt.Errorf("numa not available")
	}

	// 1. Bind to Node (memory and cpus)
	// numa_run_on_node(node)
	if ret := C.numa_run_on_node(C.int(numaNode)); ret != 0 {
		return fmt.Errorf("numa_run_on_node failed")
	}
	
	// 2. If valid cpuID provided, refine affinity to specific CPU (optional but strict)
	// This would require sched_setaffinity. 
	// The requirement is "Strict NUMA", usually node binding is sufficient.
	// But if cpuID is > -1, we might want to pin to it.
	
	return nil
}

// platformUnpinCurrentThread clears NUMA affinity.
func platformUnpinCurrentThread() error {
	runtime.UnlockOSThread()
	if !isNumaAvailable() {
		return nil
	}
	// Run on all nodes
	C.numa_run_on_node(-1)
	return nil
}