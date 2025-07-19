//go:build linux
// +build linux

// control/platform_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux-specific platform metrics or debug probe integrations.

package control

import (
	"runtime"
)

// RegisterPlatformProbes sets Linux-specific debug metrics.
func RegisterPlatformProbes(dp *DebugProbes) {
	dp.RegisterProbe("platform.cpus", func() any {
		return runtime.NumCPU()
	})
}
