// File: internal/transport/dpdk_transport_stub.go
//go:build !dpdk
// +build !dpdk

// Package transport provides a stub fallback when DPDK is unavailable.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// newDPDKTransport in stub always returns error.

package transport

import (
	"errors"

	"github.com/momentics/hioload-ws/api"
)

func newDPDKTransport(int) (api.Transport, error) {
	return nil, errors.New("DPDK transport not available (build tag 'dpdk' not enabled)")
}
