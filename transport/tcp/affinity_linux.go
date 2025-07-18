//go:build linux
// +build linux

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package tcp - Linux-specific CPU affinity implementation.

package tcp

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// setCPUAffinity pins the current OS thread to a specific CPU (Linux only).
func setCPUAffinity(cpu int) {
	runtime.LockOSThread()
	pid := syscall.Getpid()
	var mask [1024 / 64]uint64
	mask[cpu/64] |= 1 << uint(cpu%64)
	_, _, e := syscall.RawSyscall(
		syscall.SYS_SCHED_SETAFFINITY,
		uintptr(pid),
		uintptr(unsafe.Sizeof(mask)),
		uintptr(unsafe.Pointer(&mask[0])),
	)
	if e != 0 {
		fmt.Fprintf(os.Stderr, "failed to set CPU affinity: %v\n", e)
	}
}
