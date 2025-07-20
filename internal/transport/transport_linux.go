// File: internal/transport/transport_linux.go
//go:build linux
// +build linux

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux transport using zero-copy batch I/O via SendmsgBuffers.
// Ensures socket descriptor is properly closed on errors and when replacing implementation.

package transport

import (
	"fmt"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

// linuxTransport implements api.Transport for Linux.
type linuxTransport struct {
	fd       int
	bufPool  api.BufferPool
	features api.TransportFeatures
	closed   bool
}

// newTransportInternal creates a non-blocking TCP socket and buffer pool.
func newTransportInternal() (api.Transport, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	// Ensure fd is closed on any early error
	defer func() {
		if err != nil {
			unix.Close(fd)
		}
	}()

	if err = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}

	bp := pool.NewBufferPoolManager().GetPool(0)
	return &linuxTransport{
		fd:      fd,
		bufPool: bp,
		features: api.TransportFeatures{
			ZeroCopy:     true,
			Batch:        true,
			NUMAAware:    false,
			LockFree:     true,
			SharedMemory: false,
			OS:           []string{"linux"},
		},
	}, nil
}

// Send sends all buffers in one atomic batch via SendmsgBuffers.
func (lt *linuxTransport) Send(buffers [][]byte) error {
	if lt.closed {
		return api.ErrTransportClosed
	}
	sent, err := unix.SendmsgBuffers(lt.fd, buffers, nil, nil, 0)
	if err != nil {
		return fmt.Errorf("SendmsgBuffers: %w", err)
	}
	if sent != len(buffers) {
		return fmt.Errorf("partial send: %d/%d buffers", sent, len(buffers))
	}
	return nil
}

// Recv reads up to maxBuffers via RecvmsgBuffers and returns slices trimmed to lengths.
func (lt *linuxTransport) Recv() ([][]byte, error) {
	if lt.closed {
		return nil, api.ErrTransportClosed
	}
	const maxBuffers = 16
	bufs := make([][]byte, maxBuffers)
	for i := range bufs {
		buf := lt.bufPool.Get(65536, 0)
		bufs[i] = buf.Bytes()
	}
	n, _, _, _, err := unix.RecvmsgBuffers(lt.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil, nil
		}
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
}

// Close closes the socket and prevents further operations.
func (lt *linuxTransport) Close() error {
	if lt.closed {
		return nil
	}
	lt.closed = true
	return unix.Close(lt.fd)
}

// Features returns transport capabilities.
func (lt *linuxTransport) Features() api.TransportFeatures {
	return lt.features
}
