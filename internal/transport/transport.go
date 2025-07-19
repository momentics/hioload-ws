// internal/transport/transport.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Unified Transport factory and thread-safe wrapper.

package transport

import (
    "fmt"
    "sync"

    "github.com/momentics/hioload-ws/api"
)

// TransportWrapper wraps an api.Transport with mutex for safe concurrent reconfiguration.
type TransportWrapper struct {
    impl api.Transport
    mu   sync.RWMutex
}

// NewTransport constructs the underlying platform-specific transport.
// It calls newTransportInternal (implemented in linux/windows) and wraps it.
func NewTransport() (api.Transport, error) {
    impl, err := newTransportInternal()
    if err != nil {
        return nil, fmt.Errorf("transport init: %w", err)
    }
    return &TransportWrapper{impl: impl}, nil
}

// Send forwards to underlying transport.
func (w *TransportWrapper) Send(buffers [][]byte) error {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.impl.Send(buffers)
}

// Recv forwards to underlying transport.
func (w *TransportWrapper) Recv() ([][]byte, error) {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.impl.Recv()
}

// Close forwards to underlying transport.
func (w *TransportWrapper) Close() error {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.impl.Close()
}

// Features returns capabilities of underlying transport.
func (w *TransportWrapper) Features() api.TransportFeatures {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return w.impl.Features()
}

// SetImplementation replaces the underlying transport (for DPDK fallback).
func (w *TransportWrapper) SetImplementation(newImpl api.Transport) {
    w.mu.Lock()
    defer w.mu.Unlock()
    if w.impl != nil {
        _ = w.impl.Close()
    }
    w.impl = newImpl
}
