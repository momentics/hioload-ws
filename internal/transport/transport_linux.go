// internal/transport/transport_linux.go
//go:build linux
// +build linux

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux transport using zero-copy batch I/O via SendmsgBuffers.

package transport

import (
	"fmt"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

type linuxTransport struct {
	fd       int
	bufPool  api.BufferPool
	features api.TransportFeatures
}

// newTransportInternal creates a non-blocking TCP socket and buffer pool.
func newTransportInternal() (api.Transport, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)
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
	// SendmsgBuffers signature: SendmsgBuffers(fd int, buffers [][]byte, oob []byte, to unix.Sockaddr, flags int) (n int, err error)
	// Since socket is connected, we pass to = nil and flags = 0 (blocking) or MSG_DONTWAIT as needed.
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
	const maxBuffers = 16
	bufs := make([][]byte, maxBuffers)
	for i := range bufs {
		buf := lt.bufPool.Get(65536, 0)
		bufs[i] = buf.Bytes()
	}
	// RecvmsgBuffers(fd int, buffers [][]byte, oob []byte, flags int) (n, oobn int, recvflags int, from unix.Sockaddr, err error)
	n, _, _, _, err := unix.RecvmsgBuffers(lt.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return nil, nil
		}
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
}

// Close closes the socket.
func (lt *linuxTransport) Close() error {
	return unix.Close(lt.fd)
}

// Features returns transport capabilities.
func (lt *linuxTransport) Features() api.TransportFeatures {
	return lt.features
}
