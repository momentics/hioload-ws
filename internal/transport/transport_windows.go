// File: internal/transport/transport_windows.go
//go:build windows
// +build windows

//
// Package transport implements Windows-specific transport.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/windows"
)

// windowsTransport holds state for Windows IOCP transport.
type windowsTransport struct {
	mu           sync.Mutex
	socket       windows.Handle
	iocp         windows.Handle
	bufPool      api.BufferPool
	ioBufferSize int
	closed       bool
}

// newTransportInternal constructs a Windows transport with provided buffer size.
func newTransportInternal(ioBufferSize int) (api.Transport, error) {
	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	if err := windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1); err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}
	iocp, err := windows.CreateIoCompletionPort(windows.Handle(sock), 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}
	bp := pool.NewBufferPoolManager().GetPool(0)
	return &windowsTransport{
		socket:       sock,
		iocp:         iocp,
		bufPool:      bp,
		ioBufferSize: ioBufferSize,
	}, nil
}

func (wt *windowsTransport) Recv() ([][]byte, error) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return nil, api.ErrTransportClosed
	}
	size := wt.ioBufferSize
	const maxBatch = 16
	bufs := make([][]byte, maxBatch)
	wsabufs := make([]windows.WSABuf, maxBatch)
	ov := new(windows.Overlapped)
	for i := 0; i < maxBatch; i++ {
		buf := wt.bufPool.Get(size, 0)
		data := buf.Bytes()
		bufs[i] = data
		wsabufs[i].Len = uint32(len(data))
		wsabufs[i].Buf = &data[0]
	}
	var received uint32
	err := windows.WSARecv(wt.socket, &wsabufs[0], maxBatch, &received, nil, ov, nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return nil, fmt.Errorf("WSARecv: %w", err)
	}
	err = windows.GetQueuedCompletionStatus(wt.iocp, &received, nil, &ov, windows.INFINITE)
	if err != nil {
		return nil, fmt.Errorf("GetQueuedCompletionStatus recv: %w", err)
	}
	return bufs[:received], nil
}

func (wt *windowsTransport) Send(buffers [][]byte) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return api.ErrTransportClosed
	}
	ov := new(windows.Overlapped)
	wsabufs := make([]windows.WSABuf, len(buffers))
	for i, b := range buffers {
		wsabufs[i].Len = uint32(len(b))
		wsabufs[i].Buf = &b[0]
	}
	var sent uint32
	err := windows.WSASend(wt.socket, &wsabufs[0], uint32(len(wsabufs)), &sent, 0, ov, nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return fmt.Errorf("WSASend: %w", err)
	}
	err = windows.GetQueuedCompletionStatus(wt.iocp, &sent, nil, &ov, windows.INFINITE)
	if err != nil {
		return fmt.Errorf("GetQueuedCompletionStatus send: %w", err)
	}
	return nil
}

func (wt *windowsTransport) Close() error {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if !wt.closed {
		windows.CloseHandle(wt.iocp)
		windows.Closesocket(wt.socket)
		wt.closed = true
	}
	return nil
}

func (wt *windowsTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
	}
}
