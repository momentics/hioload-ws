// internal/transport/transport_windows.go
//go:build windows
// +build windows

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows implementation: zero-copy batch via WSASend/WSARecv and IOCP.

package transport

import (
	"fmt"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/windows"
)

const WSA_IO_PENDING = windows.ERROR_IO_PENDING

type windowsTransport struct {
	conn     windows.Handle
	bufPool  api.BufferPool
	features api.TransportFeatures
}

// newTransportInternal for windows: create overlapped socket and IOCP.
func newTransportInternal() (api.Transport, error) {
	// create overlapped TCP socket
	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	// disable Nagle
	_ = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	// create IOCP and associate
	iocp, err := windows.CreateIoCompletionPort(windows.Handle(sock), 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("iocp create: %w", err)
	}
	tm := pool.NewBufferPoolManager()
	bp := tm.GetPool(0)
	wt := &windowsTransport{
		conn:    iocp,
		bufPool: bp,
		features: api.TransportFeatures{
			ZeroCopy:     true,
			Batch:        true,
			NUMAAware:    true,
			LockFree:     true,
			SharedMemory: false,
			OS:           []string{"windows"},
		},
	}
	return wt, nil
}

// Send posts WSABUFs to IOCP.
// Send posts WSABUFs to IOCP.
func (wt *windowsTransport) Send(buffers [][]byte) error {
	// prepare overlapped as pointer to Overlapped
	ov := new(windows.Overlapped)
	wsabufs := make([]windows.WSABuf, len(buffers))
	for i, b := range buffers {
		wsabufs[i].Len = uint32(len(b))
		wsabufs[i].Buf = &b[0]
	}
	var sent uint32
	err := windows.WSASend(windows.Handle(wt.conn), &wsabufs[0], uint32(len(wsabufs)), &sent, 0, ov, nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return fmt.Errorf("WSASend: %w", err)
	}
	// wait for completion; ov is *Overlapped, so &ov is **Overlapped
	if err := windows.GetQueuedCompletionStatus(wt.conn, &sent, nil, &ov, windows.INFINITE); err != nil {
		return fmt.Errorf("GetQueuedCompletionStatus (send): %w", err)
	}
	return nil
}

// Recv posts WSARecv and waits for result.
// Recv posts WSARecv and waits for result.
func (wt *windowsTransport) Recv() ([][]byte, error) {
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
	err := windows.WSARecv(windows.Handle(wt.conn), &wsabufs[0], maxBatch, &received, nil, ov, nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return nil, fmt.Errorf("WSARecv: %w", err)
	}
	if err := windows.GetQueuedCompletionStatus(wt.conn, &received, nil, &ov, windows.INFINITE); err != nil {
		return nil, fmt.Errorf("GetQueuedCompletionStatus (recv): %w", err)
	}
	// вернём полученные буферы
	count := int(received)
	result := make([][]byte, count)
	for i := 0; i < count; i++ {
		result[i] = bufs[i]
	}
	return result, nil
}

// Close closes the IOCP handle (and underlying socket).
func (wt *windowsTransport) Close() error {
	return windows.CloseHandle(wt.conn)
}

// Features returns capabilities.
func (wt *windowsTransport) Features() api.TransportFeatures {
	return wt.features
}
