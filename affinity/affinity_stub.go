//go:build !linux && !windows
// +build !linux,!windows

// File: affinity/affinity_stub.go
// Author: momentics <momentics@gmail.com>
//
// Stub implementation for unsupported platforms.
// Returns error to indicate unavailability.

package affinity

import "errors"

// setAffinityPlatform is a stub for platforms where CPU affinity is not supported.
func setAffinityPlatform(cpuID int) error {
	return errors.New("affinity: not supported on this platform")
}
