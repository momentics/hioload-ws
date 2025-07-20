// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// transport_mock.go — Minimal mock Transport for unit/integration tests.
package mocks

import "github.com/momentics/hioload-ws/api"

type MockTransport struct {
	Sent     [][]byte
	RecvSeq  [][]byte
	Closed   bool
	OnSend   func([][]byte) error
	OnRecv   func() ([][]byte, error)
	OnClose  func() error
	Features api.TransportFeatures
}

func (m *MockTransport) Send(b [][]byte) error {
	m.Sent = append(m.Sent, b...)
	if m.OnSend != nil {
		return m.OnSend(b)
	}
	return nil
}

func (m *MockTransport) Recv() ([][]byte, error) {
	if m.OnRecv != nil {
		return m.OnRecv()
	}
	return m.RecvSeq, nil
}

func (m *MockTransport) Close() error {
	m.Closed = true
	if m.OnClose != nil {
		return m.OnClose()
	}
	return nil
}

func (m *MockTransport) Features() api.TransportFeatures {
	return m.Features
}
