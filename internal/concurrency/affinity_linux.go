//go:build linux
// +build linux

// File: internal/concurrency/affinity_linux.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux-specific CPU affinity implementation.

package concurrency

/*
#include <sched.h>
#include <pthread.h>
#include <unistd.h>
#include <numa.h>

static int set_cpu_affinity(int cpu) {
    cpu_set_t mask;
    CPU_ZERO(&mask);
    CPU_SET(cpu, &mask);
    return pthread_setaffinity_np(pthread_self(), sizeof(cpu_set_t), &mask);
}

static int get_current_cpu() {
    return sched_getcpu();
}

static int numa_available_wrapper() {
    return numa_available();
}

static int numa_node_of_cpu_wrapper(int cpu) {
    return numa_node_of_cpu(cpu);
}

static int numa_num_configured_nodes_wrapper() {
    return numa_num_configured_nodes();
}
*/
import "C"

// platformPreferredCPUID returns the preferred CPU for a NUMA node on Linux.
func platformPreferredCPUID(numaNode int) int {
	if C.numa_available_wrapper() < 0 {
		return 0
	}

	// Find first CPU in the NUMA node
	for cpu := 0; cpu < NumCPUs(); cpu++ {
		if int(C.numa_node_of_cpu_wrapper(C.int(cpu))) == numaNode {
			return cpu
		}
	}

	return 0
}

// platformCurrentNUMANodeID returns current NUMA node on Linux.
func platformCurrentNUMANodeID() int {
	if C.numa_available_wrapper() < 0 {
		return -1
	}

	cpu := int(C.get_current_cpu())
	if cpu < 0 {
		return -1
	}

	return int(C.numa_node_of_cpu_wrapper(C.int(cpu)))
}

// platformPinCurrentThread pins thread to CPU and NUMA node on Linux.
func platformPinCurrentThread(numaNode int, cpuID int) {
	if cpuID >= 0 {
		C.set_cpu_affinity(C.int(cpuID))
	}

	if numaNode >= 0 && C.numa_available_wrapper() >= 0 {
		C.numa_run_on_node(C.int(numaNode))
	}
}

// platformUnpinCurrentThread removes affinity constraints on Linux.
func platformUnpinCurrentThread() {
	// Set affinity to all CPUs
	// Implementation would reset CPU mask to all available CPUs
}

// platformNUMANodes returns number of NUMA nodes on Linux.
func platformNUMANodes() int {
	if C.numa_available_wrapper() < 0 {
		return 1
	}
	return int(C.numa_num_configured_nodes_wrapper())
}
