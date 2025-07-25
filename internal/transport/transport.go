// File: internal/transport/transport.go
// Package transport
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Factory and contract for creation of NUMA-aware, zero-copy, batch transports,
// abstracting platform implementation behind unified methods.
//
// Compatible with the latest /pool and /internal/concurrency contracts.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// TransportFactory produces blanket api.Transport instances using
// the new NUMA-aware BufferPoolManager and all required parameters.
type TransportFactory struct {
	IOBufferSize int
	NUMANode     int
}

// NewTransportFactory creates a factory for the preferred NUMA node and buffer size.
func NewTransportFactory(ioBufferSize, numaNode int) *TransportFactory {
	return &TransportFactory{
		IOBufferSize: ioBufferSize,
		NUMANode:     numaNode,
	}
}

// Create builds a transport using the correct platform implementation and NUMA node.
func (f *TransportFactory) Create() (api.Transport, error) {
	impl, err := newTransportInternal(f.IOBufferSize, f.NUMANode)
	if err != nil {
		return nil, fmt.Errorf("transport init: %w", err)
	}
	return &safeWrapper{impl: impl}, nil
}

// safeWrapper synchronizes all external api.Transport calls, making transport thread-safe.
// This does not serialize I/O inside the transport but only API visibility.
type safeWrapper struct {
	impl api.Transport
	mu   sync.RWMutex
}

func (w *safeWrapper) Send(bufs [][]byte) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Send(bufs)
}
func (w *safeWrapper) Recv() ([][]byte, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Recv()
}
func (w *safeWrapper) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.impl.Close()
}
func (w *safeWrapper) Features() api.TransportFeatures {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.impl.Features()
}
