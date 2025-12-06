// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakeTransport implements api.Transport for testing.
type FakeTransport struct {
	SendFunc      func(buffers [][]byte) error
	RecvFunc      func() ([][]byte, error)
	CloseFunc     func() error
	FeaturesFunc  func() api.TransportFeatures
	SendCalls     [][]byte // Track calls for verification
	RecvCalls     int      // Track number of recv calls
	CloseCalled   bool
}

// NewFakeTransport creates a new fake transport.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		SendCalls: make([][]byte, 0),
		RecvCalls: 0,
	}
}

func (ft *FakeTransport) Send(buffers [][]byte) error {
	ft.SendCalls = append(ft.SendCalls, buffers...)
	if ft.SendFunc != nil {
		return ft.SendFunc(buffers)
	}
	return nil
}

func (ft *FakeTransport) Recv() ([][]byte, error) {
	ft.RecvCalls++
	if ft.RecvFunc != nil {
		return ft.RecvFunc()
	}
	return [][]byte{}, nil
}

func (ft *FakeTransport) Close() error {
	ft.CloseCalled = true
	if ft.CloseFunc != nil {
		return ft.CloseFunc()
	}
	return nil
}

func (ft *FakeTransport) Features() api.TransportFeatures {
	if ft.FeaturesFunc != nil {
		return ft.FeaturesFunc()
	}
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: false,
		OS:        []string{"fake"},
	}
}

// Reset resets the call tracking.
func (ft *FakeTransport) Reset() {
	ft.SendCalls = ft.SendCalls[:0]
	ft.RecvCalls = 0
	ft.CloseCalled = false
}