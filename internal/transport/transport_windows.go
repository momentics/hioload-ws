// File: internal/transport/transport_windows.go
//go:build windows
// +build windows

//
// Windows-specific transport implementation with true batched WSASend/WSARecv.
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

const maxBatch = 32 // maximum number of WSABUF in one call

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
	// Create a non-blocking TCP socket
	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)

	// Create IOCP and associate the socket
	iocp, err := windows.CreateIoCompletionPort(windows.Handle(sock), 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}

	// Obtain NUMA-aware buffer pool for node 0 by default
	bp := pool.NewBufferPoolManager().GetPool(0)

	return &windowsTransport{
		socket:       sock,
		iocp:         iocp,
		bufPool:      bp,
		ioBufferSize: ioBufferSize,
	}, nil
}

// Recv performs a batched WSARecv using IOCP and Overlapped I/O.
func (wt *windowsTransport) Recv() ([][]byte, error) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return nil, api.ErrTransportClosed
	}

	// Prepare arrays for WSABUF, overlapped, and data buffers
	wsabufs := make([]windows.WSABuf, maxBatch)
	overlapps := make([]*windows.Overlapped, maxBatch)
	bufs := make([][]byte, maxBatch)

	for i := 0; i < maxBatch; i++ {
		// Allocate buffer from the pool
		buf := wt.bufPool.Get(wt.ioBufferSize, 0)
		data := buf.Bytes()
		bufs[i] = data

		wsabufs[i].Len = uint32(len(data))
		wsabufs[i].Buf = &data[0]

		// Each receive uses its own Overlapped structure
		overlapps[i] = new(windows.Overlapped)
	}

	// Initiate batched receive
	var received uint32
	err := windows.WSARecv(wt.socket, &wsabufs[0], uint32(maxBatch), &received, nil, overlapps[0], nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return nil, fmt.Errorf("WSARecv batch: %w", err)
	}

	// Wait for completion on IOCP
	var ol *windows.Overlapped
	errGQCS := windows.GetQueuedCompletionStatus(wt.iocp, &received, nil, &ol, windows.INFINITE)
	if errGQCS != nil {
		return nil, fmt.Errorf("GetQueuedCompletionStatus recv: %w", errGQCS)
	}

	// Return the first 'received' buffers
	return bufs[:received], nil
}

// Send performs batched WSASend using IOCP and Overlapped I/O.
func (wt *windowsTransport) Send(buffers [][]byte) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return api.ErrTransportClosed
	}

	// Split into batches of maxBatch
	for offset := 0; offset < len(buffers); offset += maxBatch {
		end := offset + maxBatch
		if end > len(buffers) {
			end = len(buffers)
		}
		slice := buffers[offset:end]

		// Prepare WSABUF array and one Overlapped
		wsabufs := make([]windows.WSABuf, len(slice))
		overl := new(windows.Overlapped)
		for i, b := range slice {
			wsabufs[i].Len = uint32(len(b))
			wsabufs[i].Buf = &b[0]
		}

		// Initiate send
		var sent uint32
		err := windows.WSASend(wt.socket, &wsabufs[0], uint32(len(wsabufs)), &sent, 0, overl, nil)
		if err != nil && err != windows.ERROR_IO_PENDING {
			return fmt.Errorf("WSASend batch: %w", err)
		}

		// Wait for completion
		errGQCS := windows.GetQueuedCompletionStatus(wt.iocp, &sent, nil, &overl, windows.INFINITE)
		if errGQCS != nil {
			return fmt.Errorf("GetQueuedCompletionStatus send: %w", errGQCS)
		}
	}
	return nil
}

// Close closes the IOCP handle and the socket.
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

// Features reports transport capabilities.
func (wt *windowsTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{
		ZeroCopy:  true,
		Batch:     true,
		NUMAAware: true,
		OS:        []string{"windows"},
	}
}
