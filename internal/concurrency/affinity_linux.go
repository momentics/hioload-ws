// File: internal/concurrency/affinity_linux.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
// Description: Linux-specific CPU/NUMA affinity implementation.

package concurrency

/*
#cgo LDFLAGS: -lnuma
#include <numa.h>
#include <sched.h>
#include <pthread.h>
*/
import "C"

import (
	"fmt"
	"runtime"
)

// platformPreferredCPUID returns a recommended CPU core index for the given NUMA node.
// If numaNode < 0 or NUMA is unavailable, falls back to CPU 0.
func platformPreferredCPUID(numaNode int) int {
	if numaNode < 0 || C.numa_available() < 0 {
		return 0
	}
	totalCPUs := runtime.NumCPU()
	for cpu := 0; cpu < totalCPUs; cpu++ {
		if int(C.numa_node_of_cpu(C.int(cpu))) == numaNode {
			return cpu
		}
	}
	return 0
}

// platformCurrentNUMANodeID returns the NUMA node ID of the current thread.
// Returns -1 if NUMA is unavailable or an error occurs.
func platformCurrentNUMANodeID() int {
	if C.numa_available() < 0 {
		return -1
	}
	cpu := int(C.sched_getcpu())
	if cpu < 0 {
		return -1
	}
	return int(C.numa_node_of_cpu(C.int(cpu)))
}

// platformNUMANodes returns the total number of configured NUMA nodes on the system.
// Returns 1 if NUMA is unavailable.
func platformNUMANodes() int {
	if C.numa_available() < 0 {
		return 1
	}
	return int(C.numa_num_configured_nodes())
}

// platformPinCurrentThread binds the current OS thread to the specified CPU and NUMA node.
// - If cpuID >= 0: sets CPU affinity using pthread_setaffinity_np.
// - If numaNode >= 0 and NUMA available: binds the thread to the specified NUMA node.
// The thread is locked to the OS thread to maintain affinity.
func platformPinCurrentThread(numaNode, cpuID int) error {
	runtime.LockOSThread()

	if cpuID >= 0 {
		var mask C.cpu_set_t
		C.CPU_ZERO(&mask)
		C.CPU_SET(C.int(cpuID), &mask)
		if rc := C.pthread_setaffinity_np(C.pthread_self(), C.sizeof_cpu_set_t, &mask); rc != 0 {
			return fmt.Errorf("pthread_setaffinity_np failed: %d", rc)
		}
	}

	if numaNode >= 0 && C.numa_available() >= 0 {
		if rc := C.numa_run_on_node(C.int(numaNode)); rc != 0 {
			return fmt.Errorf("numa_run_on_node failed: %d", rc)
		}
	}

	return nil
}

// platformUnpinCurrentThread clears any CPU affinity, allowing the OS to schedule the thread anywhere.
func platformUnpinCurrentThread() error {
	runtime.LockOSThread()
	var mask C.cpu_set_t
	C.CPU_ZERO(&mask)
	total := runtime.NumCPU()
	for i := 0; i < total; i++ {
		C.CPU_SET(C.int(i), &mask)
	}
	if rc := C.pthread_setaffinity_np(C.pthread_self(), C.sizeof_cpu_set_t, &mask); rc != 0 {
		return fmt.Errorf("pthread_unpin failed: %d", rc)
	}
	return nil
}
