// File: internal/transport/transport_linux.go
//go:build linux
// +build linux
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux transport using zero-copy batch I/O via SendmsgBuffers.
// Ensures socket descriptor is properly closed on errors and when replacing implementation.
// Added safe management of old file descriptor to prevent leaks on reinitialization.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

// linuxTransport implements api.Transport for Linux.
type linuxTransport struct {
	mu       sync.Mutex
	fd       int
	bufPool  api.BufferPool
	features api.TransportFeatures
	closed   bool
}

// global variable to hold prior fd to close on reinit
var prevFd int = -1

// newTransportInternal creates a non-blocking TCP socket and buffer pool.
// It now closes any previous fd before creating a new one to avoid leaks.
func newTransportInternal() (api.Transport, error) {
	// Close previous fd if exists
	if prevFd >= 0 {
		unix.Close(prevFd)
		prevFd = -1
	}

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	// On early error, close this fd
	defer func() {
		if err != nil {
			unix.Close(fd)
		}
	}()

	if err = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}

	// Save fd for potential future closure
	prevFd = fd

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

func (lt *linuxTransport) Send(buffers [][]byte) error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
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

func (lt *linuxTransport) Recv() ([][]byte, error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
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

func (lt *linuxTransport) Close() error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
		return nil
	}
	lt.closed = true
	err := unix.Close(lt.fd)
	lt.fd = -1
	if prevFd == lt.fd {
		prevFd = -1
	}
	return err
}

func (lt *linuxTransport) Features() api.TransportFeatures {
	return lt.features
}
