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
