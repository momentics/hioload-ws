// File: internal/transport/transport.go
// Package transport provides factory functions for platform-specific transports.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// This file is included on all platforms (no build tags).
// It dispatches to platform implementations in transport_linux.go, transport_windows.go.
// DPDK support is provided via build tag "dpdk" in dpdk_transport.go / dpdk_transport_stub.go.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// wrapper serializes concurrent access to underlying Transport.
type wrapper struct {
	impl api.Transport
	mu   sync.RWMutex
}

// NewTransport constructs the native OS transport with the given ioBufferSize.
// Delegates to newTransportInternal defined in platform-specific files.
func NewTransport(ioBufferSize int) (api.Transport, error) {
	impl, err := newTransportInternal(ioBufferSize)
	if err != nil {
		return nil, fmt.Errorf("native transport init: %w", err)
	}
	return &wrapper{impl: impl}, nil
}

// NewDPDKTransport constructs a DPDK-backed transport with the given ioBufferSize.
// Delegates to newDPDKTransport defined in dpdk_transport.go or dpdk_transport_stub.go.
func NewDPDKTransport(ioBufferSize int) (api.Transport, error) {
	impl, err := newDPDKTransport(ioBufferSize)
	if err != nil {
		return nil, fmt.Errorf("DPDK transport init: %w", err)
	}
	return &wrapper{impl: impl}, nil
}

func (w *wrapper) Send(buffers [][]byte) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Send(buffers)
}

func (w *wrapper) Recv() ([][]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Recv()
}

func (w *wrapper) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.impl.Close()
}

func (w *wrapper) Features() api.TransportFeatures {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Features()
}
