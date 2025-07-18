//go:build windows
// +build windows

// File: affinity/affinity_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows-specific implementation for setting thread CPU affinity.

package affinity

import (
	"syscall"
)

// setAffinityPlatform sets thread affinity to a given CPU for Windows.
func setAffinityPlatform(cpuID int) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procSetThreadAffinityMask := kernel32.NewProc("SetThreadAffinityMask")
	procGetCurrentThread := kernel32.NewProc("GetCurrentThread")
	hThread, _, _ := procGetCurrentThread.Call()
	mask := uintptr(1) << cpuID
	ret, _, err := procSetThreadAffinityMask.Call(hThread, mask)
	if ret == 0 {
		return err
	}
	return nil
}
