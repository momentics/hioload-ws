// internal/transport/dpdk_transport_stub.go
//go:build !dpdk
// +build !dpdk

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Stub fallback for DPDK transport: used automatically when the dpdk build tag is not set.
// Always returns not implemented, signals fallback to standard transport.

package transport

import (
	"errors"

	"github.com/momentics/hioload-ws/api"
)

// NewDPDKTransport always returns error: no DPDK at build.
func NewDPDKTransport(mode string) (api.Transport, error) {
	return nil, errors.New("DPDK transport not available (build tag not enabled)")
}
