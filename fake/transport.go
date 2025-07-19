// Package fake
// Author: momentics <momentics@gmail.com>
//
// Fake implementations for testing and development.
// Provides predictable, controllable behavior for all core interfaces.

package fake

import (
	"fmt"
	"github.com/momentics/hioload-ws/api"
	"sync"
)

// Transport is a fake implementation of api.Transport for testing.
type Transport struct {
	mu          sync.Mutex
	sendBuffer  [][]byte
	recvBuffer  [][]byte
	closed      bool
	sendError   error
	recvError   error
	closeError  error
	features    api.TransportFeatures
}

// NewTransport creates a new fake transport with default settings.
func NewTransport() *Transport {
	return &Transport{
		sendBuffer: make([][]byte, 0),
		recvBuffer: make([][]byte, 0),
		features: api.TransportFeatures{
			ZeroCopy:     true,
			Batch:        true,
			NUMAAware:    true,
			LockFree:     true,
			SharedMemory: false,
			OS:           []string{"fake"},
		},
	}
}

// Send implements api.Transport.Send.
func (t *Transport) Send(buffers [][]byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return api.ErrTransportClosed
	}
	
	if t.sendError != nil {
		return t.sendError
	}
	
	for _, buf := range buffers {
		bufCopy := make([]byte, len(buf))
		copy(bufCopy, buf)
		t.sendBuffer = append(t.sendBuffer, bufCopy)
	}
	
	return nil
}

// Recv implements api.Transport.Recv.
func (t *Transport) Recv() ([][]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closed {
		return nil, api.ErrTransportClosed
	}
	
	if t.recvError != nil {
		return nil, t.recvError
	}
	
	buffers := make([][]byte, len(t.recvBuffer))
	copy(buffers, t.recvBuffer)
	t.recvBuffer = t.recvBuffer[:0] // Clear after reading
	
	return buffers, nil
}

// Close implements api.Transport.Close.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.closeError != nil {
		return t.closeError
	}
	
	t.closed = true
	return nil
}

// Features implements api.Transport.Features.
func (t *Transport) Features() api.TransportFeatures {
	return t.features
}

// SetSendError configures the transport to return an error on Send.
func (t *Transport) SetSendError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendError = err
}

// SetRecvError configures the transport to return an error on Recv.
func (t *Transport) SetRecvError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recvError = err
}

// SetCloseError configures the transport to return an error on Close.
func (t *Transport) SetCloseError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeError = err
}

// AddRecvData adds data to be returned by the next Recv call.
func (t *Transport) AddRecvData(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	t.recvBuffer = append(t.recvBuffer, dataCopy)
}

// GetSentData returns all data that has been sent via Send.
func (t *Transport) GetSentData() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	sent := make([][]byte, len(t.sendBuffer))
	copy(sent, t.sendBuffer)
	return sent
}

// ClearSentData clears the internal send buffer.
func (t *Transport) ClearSentData() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendBuffer = t.sendBuffer[:0]
}

// Error types for fake transport.
var (
	ErrTransportClosed = fmt.Errorf("transport is closed")
)
