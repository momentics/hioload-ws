// File: api/transport.go
// Package api defines Transport interface.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// TransportFeatures describes transport capabilities.
type TransportFeatures struct {
	ZeroCopy  bool
	Batch     bool
	NUMAAware bool
	TLS       bool
	OS        []string
}

// Transport is the core IO abstraction.
type Transport interface {
	// Send transmits a batch of buffers.
	Send(buffers [][]byte) error
	// Recv receives a batch of buffers.
	Recv() ([][]byte, error)
	// Close releases all resources.
	Close() error
	// Features reports transport capabilities.
	Features() TransportFeatures
}
