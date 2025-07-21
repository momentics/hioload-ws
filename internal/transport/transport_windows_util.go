// File: internal/transport/transport_windows_util.go
//go:build windows
// +build windows

//
// Utility functions and types for overlapped I/O batching.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"golang.org/x/sys/windows"
)

// EnsureMaxProcessors checks, что максимальное число процессоров не превышает 320.
func EnsureMaxProcessors() int {
	n := windows.GetMaximumProcessorCount(0)
	if n > 320 {
		return 320
	}
	return int(n)
}

// ZeroBufferPool prepares буфер размера page для zero-copy операций.
func ZeroBufferPool(size int, node int) *windows.Overlapped {
	return new(windows.Overlapped)
}
