// File: internal/transport/transport_linux.go
//go:build linux
// +build linux
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux transport using zero-copy batch I/O via SendmsgBuffers.
// Uses Config.IOBufferSize instead of hardcoded 65536.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

type linuxTransport struct {
	mu       sync.Mutex
	fd       int
	bufPool  api.BufferPool
	features api.TransportFeatures
	closed   bool
}

var prevFd int = -1

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
	defer func() {
		if err != nil {
			unix.Close(fd)
		}
	}()

	if err = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1); err != nil {
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}

	prevFd = fd

	// Use IOBufferSize from DefaultConfig instead of 65536
	ioSize := facade.DefaultConfig().IOBufferSize
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

func (lt *linuxTransport) Recv() ([][]byte, error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
		return nil, api.ErrTransportClosed
	}
	// Use IOBufferSize instead of magic 65536
	bufferSize := facade.DefaultConfig().IOBufferSize
	const maxBuffers = 16
	bufs := make([][]byte, maxBuffers)
	for i := range bufs {
		buf := lt.bufPool.Get(bufferSize, 0)
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
