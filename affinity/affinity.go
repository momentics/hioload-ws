// File: affinity/affinity.go
// Author: momentics <momentics@gmail.com>
//
// Platform-neutral API for CPU affinity. Platform-specific implementations are located
// in separate files (affinity_linux.go, affinity_windows.go, etc.) guarded by build tags.

package affinity

// SetAffinity pins current OS thread to a given logical CPU/core on supported platforms.
// On unsupported platforms returns an error.
func SetAffinity(cpuID int) error {
	return setAffinityPlatform(cpuID)
}
