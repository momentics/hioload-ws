//go:build windows
// +build windows

// control/platform_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows-specific metrics/debug introspection points.

package control

import (
	"runtime"
)

// RegisterPlatformProbes sets Windows-specific debug probes.
func RegisterPlatformProbes(dp *DebugProbes) {
	dp.RegisterProbe("platform.cpus", func() any {
		return runtime.NumCPU()
	})
}
