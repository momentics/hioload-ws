//go:build !windows
// +build !windows

// hioload-ws/internal/concurrency/pin.go
// Author: momentics <momentics@gmail.com>
//
// Platform-generic symbol for CPU/NUMA pinning dispatcher.
// Always overridden by a matching platform file via build tag.

package concurrency

// PinCurrentThread pins the current OS thread to a given NUMA node and CPU core.
// This function is implemented per platform (Linux/Windows). On unsupported systems it is a no-op.
func PinCurrentThread(numaNode int, cpuID int) {}
