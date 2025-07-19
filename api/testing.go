// Package api
// Author: momentics@gmail.com
//
// Testing and mocking primitives for core abstractions.

package api

// MockTransport is a test-oriented replacement for Transport.
type MockTransport struct {
    SendFunc func([][]byte) error
    RecvFunc func() ([][]byte, error)
    CloseFunc func() error
    FeaturesFunc func() TransportFeatures
}

func (m *MockTransport) Send(b [][]byte) error                 { return m.SendFunc(b) }
func (m *MockTransport) Recv() ([][]byte, error)               { return m.RecvFunc() }
func (m *MockTransport) Close() error                          { return m.CloseFunc() }
func (m *MockTransport) Features() TransportFeatures           { return m.FeaturesFunc() }

// Add more test/mocking helpers as needed...
