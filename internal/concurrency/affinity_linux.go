//go:build linux
// +build linux

//
// Linux-specific CPU and NUMA affinity implementation.
// Uses libnuma and pthread_setaffinity_np for precise control.

package concurrency

/*
#include <sched.h>
#include <pthread.h>
#include <numa.h>
*/
import "C"
import (
	"fmt"
	"runtime"
)

func platformPreferredCPUID(numaNode int) int {
	if numaNode < 0 || C.numa_available() < 0 {
		return 0
	}
	ncpus := runtime.NumCPU()
	for cpu := 0; cpu < ncpus; cpu++ {
		if int(C.numa_node_of_cpu(C.int(cpu))) == numaNode {
			return cpu
		}
	}
	return 0
}

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

func platformNUMANodes() int {
	if C.numa_available() < 0 {
		return 1
	}
	return int(C.numa_num_configured_nodes())
}

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

func platformUnpinCurrentThread() error {
	runtime.LockOSThread()
	var mask C.cpu_set_t
	C.CPU_ZERO(&mask)
	ncpus := runtime.NumCPU()
	for i := 0; i < ncpus; i++ {
		C.CPU_SET(C.int(i), &mask)
	}
	if rc := C.pthread_setaffinity_np(C.pthread_self(), C.sizeof_cpu_set_t, &mask); rc != 0 {
		return fmt.Errorf("pthread_unpin failed: %d", rc)
	}
	return nil
}
