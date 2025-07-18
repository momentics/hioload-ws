//go:build linux
// +build linux

// File: pool/numa_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux-specific NUMA allocator using libnuma via CGO.

package pool

/*
#cgo LDFLAGS: -lnuma
#include <numa.h>
#include <stdlib.h>
void* go_numa_alloc(int size, int node) {
	if (numa_available() == -1 || node < 0) {
		return malloc(size);
	}
	return numa_alloc_onnode(size, node);
}
void go_numa_free(void *mem, int size, int node) {
	if (numa_available() == -1 || node < 0) {
		free(mem);
		return;
	}
	numa_free(mem, size);
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// linuxNUMAAllocator is a NUMA allocator implementation for Linux.
type linuxNUMAAllocator struct{}

func newLinuxNUMAAllocator() NUMAAllocator {
	return &linuxNUMAAllocator{}
}

func (l *linuxNUMAAllocator) Alloc(size int, node int) ([]byte, error) {
	ptr := C.go_numa_alloc(C.int(size), C.int(node))
	if ptr == nil {
		return nil, fmt.Errorf("linux NUMA alloc failed")
	}
	return unsafe.Slice((*byte)(ptr), size), nil
}

func (l *linuxNUMAAllocator) Free(buf []byte) {
	if len(buf) == 0 {
		return
	}
	C.go_numa_free(unsafe.Pointer(&buf[0]), C.int(len(buf)), -1)
}

func (l *linuxNUMAAllocator) Nodes() (int, error) {
	nodes := C.numa_max_node()
	if nodes < 0 {
		return 1, fmt.Errorf("NUMA not available")
	}
	return int(nodes + 1), nil
}
