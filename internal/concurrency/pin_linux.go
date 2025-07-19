//go:build linux
// +build linux

// hioload-ws/internal/concurrency/pin_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux-specific implementation of runtime pinning (NUMA and CPU affinity).
// Uses sched_setaffinity and NUMA bindings via cgo for optimal locality.
//
// Note: For full NUMA awareness, ensure CGO is enabled and libnuma-dev is available.

package concurrency

/*
#include <sched.h>
#include <pthread.h>
#include <string.h>
#ifdef __linux__
#include <numa.h>
#endif
#include <errno.h>
*/
import "C"
import (
	"log"
	"runtime"
)

// PinCurrentThread pins the calling native thread to a NUMA node and CPU core.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread()
	mask := C.cpu_set_t{}
	C.CPU_ZERO(&mask)
	C.CPU_SET(C.int(cpuID), &mask)
	ret, err := C.pthread_setaffinity_np(C.pthread_self(), C.size_t(C.sizeof_cpu_set_t), &mask)
	if ret != 0 {
		log.Printf("pin: failed to set thread affinity: %v", err)
	}
	// Optionally, bind to the NUMA node as well.
	// #ifdef __linux__
	if numaNode >= 0 {
		C.numa_run_on_node(C.int(numaNode))
	}
	// #endif
}
