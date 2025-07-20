// File: internal/transport/dpdk_transport.go
//go:build dpdk
// +build dpdk

// Package transport implements DPDK-based transport for Linux.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// newDPDKTransport accepts ioBufferSize from facade.Config and does not call facade internally.

package transport

import (
	"github.com/momentics/hioload-ws/api"
)

type dpdkTransport struct {
	ioBufferSize int
	// DPDK internals...
}

func newDPDKTransport(ioBufferSize int) (api.Transport, error) {
	// Initialize DPDK EAL, ports, etc.
	// For demonstration, succeed without real DPDK binding.
	return &dpdkTransport{ioBufferSize: ioBufferSize}, nil
}

func (d *dpdkTransport) Recv() ([][]byte, error) {
	// Real DPDK Rx logic should use ioBufferSize for buffer allocation.
	return nil, nil
}

func (d *dpdkTransport) Send(buffers [][]byte) error {
	// Real DPDK Tx logic...
	return nil
}

func (d *dpdkTransport) Close() error {
	// Cleanup DPDK resources...
	return nil
}

func (d *dpdkTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{ZeroCopy: true, Batch: true, NUMAAware: true}
}
