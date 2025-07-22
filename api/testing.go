// Package api
// Author: momentics
//
// Mock/testing utilities for all core contracts; extendable for new interfaces.

package api

// MockTransport is a test and mock-friendly implementation of Transport.
type MockTransport struct {
	SendFunc     func([][]byte) error
	RecvFunc     func() ([][]byte, error)
	CloseFunc    func() error
	FeaturesFunc func() TransportFeatures
}

func (m *MockTransport) Send(b [][]byte) error       { return m.SendFunc(b) }
func (m *MockTransport) Recv() ([][]byte, error)     { return m.RecvFunc() }
func (m *MockTransport) Close() error                { return m.CloseFunc() }
func (m *MockTransport) Features() TransportFeatures { return m.FeaturesFunc() }

// Extend with mocks for all additional core contracts as architecture evolves.
