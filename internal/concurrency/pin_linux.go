//go:build linux
// +build linux

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux-specific implementation of runtime pinning (NUMA and CPU affinity).

package concurrency

/*
#include <sched.h>
#include <pthread.h>
#include <numa.h>
*/
import "C"
import "runtime"

// PinCurrentThread pins the thread to specified NUMA node and CPU core.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread()
	var mask C.cpu_set_t
	C.CPU_ZERO(&mask)
	C.CPU_SET(C.int(cpuID), &mask)
	C.pthread_setaffinity_np(C.pthread_self(), C.sizeof_cpu_set_t, &mask)
	if numaNode >= 0 {
		C.numa_run_on_node(C.int(numaNode))
	}
}
