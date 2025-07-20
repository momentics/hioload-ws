// File: internal/transport/transport_windows.go
//go:build windows
// +build windows

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows implementation: zero-copy batch via WSASend/WSARecv and IOCP.
// Ensures both IOCP handle and underlying socket are closed on Close().

package transport

import (
	"fmt"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/windows"
)

const WSA_IO_PENDING = windows.ERROR_IO_PENDING

type windowsTransport struct {
	connHandle windows.Handle // IOCP handle
	socket     windows.Handle // Underlying socket handle
	bufPool    api.BufferPool
	features   api.TransportFeatures
	closed     bool
}

// newTransportInternal for windows: create overlapped socket and IOCP.
func newTransportInternal() (api.Transport, error) {
	// create overlapped TCP socket
	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	// ensure socket is closed on early errors
	defer func() {
		if err != nil {
			windows.Closesocket(sock)
		}
	}()
	// disable Nagle
	if err = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1); err != nil {
		return nil, fmt.Errorf("setsockopt TCP_NODELAY: %w", err)
	}
	// create IOCP and associate
	iocp, err := windows.CreateIoCompletionPort(windows.Handle(sock), 0, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("iocp create: %w", err)
	}

	bp := pool.NewBufferPoolManager().GetPool(0)
	return &windowsTransport{
		connHandle: iocp,
		socket:     windows.Handle(sock),
		bufPool:    bp,
		features: api.TransportFeatures{
			ZeroCopy:     true,
			Batch:        true,
			NUMAAware:    true,
			LockFree:     true,
			SharedMemory: false,
			OS:           []string{"windows"},
		},
	}, nil
}

// Send posts WSABUFs to IOCP.
func (wt *windowsTransport) Send(buffers [][]byte) error {
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
	if err != nil && err != WSA_IO_PENDING {
		return fmt.Errorf("WSASend: %w", err)
	}
	if err := windows.GetQueuedCompletionStatus(wt.connHandle, &sent, nil, &ov, windows.INFINITE); err != nil {
		return fmt.Errorf("GetQueuedCompletionStatus (send): %w", err)
	}
	return nil
}

// Recv posts WSARecv and waits for result.
func (wt *windowsTransport) Recv() ([][]byte, error) {
	if wt.closed {
		return nil, api.ErrTransportClosed
	}
	const maxBatch = 16
	bufs := make([][]byte, maxBatch)
	wsabufs := make([]windows.WSABuf, maxBatch)
	ov := new(windows.Overlapped)
	for i := 0; i < maxBatch; i++ {
		buf := wt.bufPool.Get(65536, 0)
		data := buf.Bytes()
		bufs[i] = data
		wsabufs[i].Len = uint32(len(data))
		wsabufs[i].Buf = &data[0]
	}
	var received uint32
	err := windows.WSARecv(wt.socket, &wsabufs[0], maxBatch, &received, nil, ov, nil)
	if err != nil && err != WSA_IO_PENDING {
		return nil, fmt.Errorf("WSARecv: %w", err)
	}
	if err := windows.GetQueuedCompletionStatus(wt.connHandle, &received, nil, &ov, windows.INFINITE); err != nil {
		return nil, fmt.Errorf("GetQueuedCompletionStatus (recv): %w", err)
	}
	count := int(received)
	result := make([][]byte, count)
	for i := 0; i < count; i++ {
		result[i] = bufs[i]
	}
	return result, nil
}

// Close closes the IOCP handle and the underlying socket.
func (wt *windowsTransport) Close() error {
	if wt.closed {
		return nil
	}
	wt.closed = true
	// Close IOCP handle
	if err := windows.CloseHandle(wt.connHandle); err != nil {
		return fmt.Errorf("close IOCP handle: %w", err)
	}
	// Close underlying socket
	if err := windows.Closesocket(wt.socket); err != nil {
		return fmt.Errorf("close socket: %w", err)
	}
	return nil
}

// Features returns capabilities.
func (wt *windowsTransport) Features() api.TransportFeatures {
	return wt.features
}
