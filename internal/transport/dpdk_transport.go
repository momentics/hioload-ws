// internal/transport/dpdk_transport.go
// +build dpdk

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// DPDK-backed transport for hioload-ws.
// This file is only built and included if the `dpdk` build tag is provided. 
// If DPDK is not available, the stub (see dpdk_transport_stub.go) is used.

package transport

import (
	"errors"
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// dpdkTransport is a stub/sample class; full working integration with Go-DPDK bindings is needed for production use.
type dpdkTransport struct {
	inited   bool
	stopped  bool
	mu       sync.Mutex
	features api.TransportFeatures
}

// NewDPDKTransport attempts to initialize DPDK subsystem and returns a transport.
func NewDPDKTransport(mode string) (api.Transport, error) {
	// Here should be DPDK initialization via GoCGO or cgo bindings.
	// In the real world you would load the EAL, configure ports, queues, memory, etc.
	// For demonstration, we only simulate and succeed.
	tr := &dpdkTransport{
		inited: true,
		features: api.TransportFeatures{
			ZeroCopy:     true,
			Batch:        true,
			NUMAAware:    true,
			LockFree:     true,
			SharedMemory: false,
			OS:           []string{"linux"},
		},
	}
	return tr, nil
}

func (d *dpdkTransport) Send(buffers [][]byte) error {
	if !d.inited {
		return errors.New("DPDK not initialized")
	}
	// Implement real send with DPDK Go binding here
	return nil
}
func (d *dpdkTransport) Recv() ([][]byte, error) {
	if !d.inited {
		return nil, errors.New("DPDK not initialized")
	}
	// Implement real receive for DPDK
	return nil, nil
}
func (d *dpdkTransport) Close() error {
	d.stopped = true
	return nil
}
func (d *dpdkTransport) Features() api.TransportFeatures {
	return d.features
}
