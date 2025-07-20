//go:build windows
// +build windows

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows-specific implementation of thread and CPU/NUMA affinity control.

package concurrency

import (
	"log"
	"runtime"
	"syscall"
)

// PinCurrentThread pins the current thread to CPU core and attempts NUMA binding.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread()
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadAffinityMask")
	mask := uintptr(1) << uint(cpuID)
	oldMask, _, err := proc.Call(uintptr(syscall.Handle(^uintptr(1))), mask)
	if oldMask == 0 {
		log.Printf("[pin_windows] Failed to set thread affinity: %v", err)
	}
}
