// Package transport
// Author: momentics <momentics@gmail.com>
//
// Platform-independent facade and factory for Transport implementations.
// Provides a unified interface for sending and receiving zero-copy data buffers
// across heterogeneous operating systems with NUMA-aware optimizations.

package transport

import (
	"sync"

	"github.com/momentics/hioload-ws/api"
)

// TransportWrapper implements api.Transport interface and wraps platform-specific transport implementation.
type TransportWrapper struct {
	impl api.Transport
	mu   sync.RWMutex
}

// NewTransport creates a new Transport instance suitable to the host platform.
func NewTransport() (api.Transport, error) {
	return newTransportInternal()
}

// Send implements api.Transport.Send using platform-specific implementation.
func (t *TransportWrapper) Send(buffers [][]byte) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.impl.Send(buffers)
}

// Recv implements api.Transport.Recv using platform-specific implementation.
func (t *TransportWrapper) Recv() ([][]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.impl.Recv()
}

// Close implements api.Transport.Close using platform-specific implementation.
func (t *TransportWrapper) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.impl.Close()
}

// Features implements api.Transport.Features using platform-specific implementation.
func (t *TransportWrapper) Features() api.TransportFeatures {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.impl.Features()
}

// SetImplementation allows hot-swapping the underlying transport implementation.
func (t *TransportWrapper) SetImplementation(newImpl api.Transport) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.impl != nil {
		_ = t.impl.Close()
	}
	t.impl = newImpl
}
