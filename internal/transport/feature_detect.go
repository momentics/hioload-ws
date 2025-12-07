// File: internal/transport/feature_detect.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Advertises the current detected capabilities of transport for platform and runtime logic.

package transport

import (
	"runtime"

	"github.com/momentics/hioload-ws/api"
)

// DetectTransportFeatures returns the set of available transport features
// for this OS/platform. In the future, auto-detects io_uring, DPDK, RDMA, etc.
func DetectTransportFeatures() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
		OS:        []string{runtime.GOOS},
	}
}

// RuntimeTransportSelector returns the best available transport for the current platform
func RuntimeTransportSelector() string {
	if runtime.GOOS == "linux" && HasIoUringSupport() {
		return "io_uring"
	}
	return "epoll"
}

// HasIoUringSupport checks if the kernel supports io_uring (stub for non-Linux)
// The actual implementation is in Linux-specific file
var HasIoUringSupport = func() bool {
	return false
}
