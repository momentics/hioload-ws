// File: internal/transport/transport_linux.go
//go:build linux
// +build linux

//
// Package transport implements Linux-specific transport.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

// linuxTransport holds state for Linux transport operations.
type linuxTransport struct {
	mu           sync.Mutex
	fd           int
	bufPool      api.BufferPool
	ioBufferSize int
	closed       bool
}

// newTransportInternal constructs a Linux transport using the provided buffer size.
func newTransportInternal(ioBufferSize int) (api.Transport, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}
	// Obtain NUMA-aware buffer pool for node 0 by default
	bp := pool.NewBufferPoolManager().GetPool(0)
	return &linuxTransport{
		fd:           fd,
		bufPool:      bp,
		ioBufferSize: ioBufferSize,
	}, nil
}

func (lt *linuxTransport) Recv() ([][]byte, error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
		return nil, api.ErrTransportClosed
	}
	size := lt.ioBufferSize
	const maxBufs = 16
	bufs := make([][]byte, maxBufs)
	for i := 0; i < maxBufs; i++ {
		buf := lt.bufPool.Get(size, 0)
		bufs[i] = buf.Bytes()
	}
	n, _, _, _, err := unix.RecvmsgBuffers(lt.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
}

func (lt *linuxTransport) Send(buffers [][]byte) error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
		return api.ErrTransportClosed
	}
	const maxBufs = 16
	total := len(buffers)
	sent := 0
	for sent < total {
		end := sent + maxBufs
		if end > total {
			end = total
		}
		bufs := buffers[sent:end]
		n, err := unix.SendmsgBuffers(lt.fd, bufs, nil, nil, 0)
		if err != nil {
			return fmt.Errorf("SendmsgBuffers: %w", err)
		}
		if n <= 0 {
			return fmt.Errorf("SendmsgBuffers sent no buffers")
		}
		sent += n
	}
	return nil
}

func (lt *linuxTransport) Close() error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if !lt.closed {
		unix.Close(lt.fd)
		lt.closed = true
	}
	return nil
}

func (lt *linuxTransport) Features() api.TransportFeatures {
	// Indicate NUMA-awareness for Linux transport
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
	}
}
