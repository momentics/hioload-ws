// go:build windows
// +build windows

// Package concurrency
// Author: momentics <momentics@gmail.com>
//
// Windows-specific implementation of thread and CPU/NUMA affinity control.
// Used for pinning runtime goroutines to specific OS threads on designated CPU cores.
//
// This module uses SetThreadAffinityMask from the Windows API to bind the current thread
// to a logical processor. Basic NUMA policies can also be introduced at a later stage.
//
// Reference: https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-setthreadaffinitymask

package concurrency

import (
	"log"
	"runtime"
	"syscall"
)

// PinCurrentThread attempts to bind the current thread to a logical CPU core.
//
// cpuID:    target logical processor index (0-based)
// numaNode: reserved for future use (NUMA node support not implemented)
//
// Note: The goroutine must be locked beforehand using runtime.LockOSThread().
// If SetThreadAffinityMask fails, the call degrades gracefully without termination.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread() // Ensure system thread match

	procSetAffinity := syscall.NewLazyDLL("kernel32.dll").NewProc("SetThreadAffinityMask")

	currentThread := syscall.Handle(^uintptr(1)) // Pseudo-handle for GetCurrentThread()

	// Prepare affinity mask: set a single bit that corresponds to the desired CPU.
	if cpuID < 0 || cpuID >= 64 {
		log.Printf("[pin_windows] Invalid CPU index: %d (valid: 0..63)", cpuID)
		return
	}
	var mask uintptr = 1 << uint(cpuID)

	oldMask, _, callErr := procSetAffinity.Call(uintptr(currentThread), mask)
	if oldMask == 0 {
		log.Printf("[pin_windows] Failed to set thread affinity: %v", callErr)
		return
	}

	log.Printf("[pin_windows] Thread pinned to CPU #%d (mask=0x%X)", cpuID, mask)
}

