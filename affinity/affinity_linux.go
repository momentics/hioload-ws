//go:build linux
// +build linux

// File: affinity/affinity_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux-specific implementation for setting thread CPU affinity.

package affinity

/*
#define _GNU_SOURCE
#include <sched.h>
#include <pthread.h>
#include <errno.h>

// Set calling thread's affinity to the provided CPU core.
int go_setaffinity(int cpu) {
	cpu_set_t set;
	CPU_ZERO(&set);
	CPU_SET(cpu, &set);
	return pthread_setaffinity_np(pthread_self(), sizeof(set), &set);
}
*/
import "C"
import "fmt"

// setAffinityPlatform sets thread affinity to a given CPU for Linux.
func setAffinityPlatform(cpuID int) error {
	ret := C.go_setaffinity(C.int(cpuID))
	if ret != 0 {
		return fmt.Errorf("affinity: pthread_setaffinity_np failed, code %d", ret)
	}
	return nil
}
