// File: internal/transport/transport_windows.go
//go:build windows
// +build windows

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows-native NUMA-aware, batch-enabled transport using IOCP, WSASend/WSARecv.
// Full support for buffer pool NUMA pinning. Integrated with latest BufferPoolManager.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/windows"
)

const maxBatch = 32

type windowsTransport struct {
	mu           sync.Mutex
	socket       windows.Handle
	iocp         windows.Handle
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
}

// newTransportInternal creates a NUMA-aware batch transport for Windows.
func newTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	nodeCnt := concurrency.NUMANodes()
	node := numaNode
	if node < 0 || node >= nodeCnt {
		node = 0 // Fallback: use NUMA node 0
	}

	sock, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = windows.SetsockoptInt(sock, windows.IPPROTO_TCP, windows.TCP_NODELAY, 1)
	iocp, err := windows.CreateIoCompletionPort(sock, 0, 0, 0)
	if err != nil {
		windows.Closesocket(sock)
		return nil, fmt.Errorf("CreateIoCompletionPort: %w", err)
	}
	// Explicitly supply number of NUMA nodes to BufferPoolManager.
	bufPool := pool.NewBufferPoolManager(nodeCnt).GetPool(ioBufferSize, node)
	return &windowsTransport{
		socket:       sock,
		iocp:         iocp,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

func (wt *windowsTransport) Recv() ([][]byte, error) {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return nil, api.ErrTransportClosed
	}
	wsabufs := make([]windows.WSABuf, maxBatch)
	overlapps := make([]*windows.Overlapped, maxBatch)
	bufs := make([][]byte, maxBatch)
	for i := 0; i < maxBatch; i++ {
		buf := wt.bufPool.Get(wt.ioBufferSize, wt.numaNode)
		data := buf.Bytes()
		bufs[i] = data
		wsabufs[i].Len = uint32(len(data))
		wsabufs[i].Buf = &data[0]
		overlapps[i] = new(windows.Overlapped)
	}
	var received uint32
	err := windows.WSARecv(wt.socket, &wsabufs[0], uint32(maxBatch), &received, nil, overlapps[0], nil)
	if err != nil && err != windows.ERROR_IO_PENDING {
		return nil, fmt.Errorf("WSARecv batch: %w", err)
	}
	var ol *windows.Overlapped
	errGQCS := windows.GetQueuedCompletionStatus(wt.iocp, &received, nil, &ol, windows.INFINITE)
	if errGQCS != nil {
		return nil, fmt.Errorf("GetQueuedCompletionStatus recv: %w", errGQCS)
	}
	return bufs[:received], nil
}

func (wt *windowsTransport) Send(buffers [][]byte) error {
	wt.mu.Lock()
	defer wt.mu.Unlock()
	if wt.closed {
		return api.ErrTransportClosed
	}
	for offset := 0; offset < len(buffers); offset += maxBatch {
		end := offset + maxBatch
		if end > len(buffers) {
			end = len(buffers)
		}
		slice := buffers[offset:end]
		wsabufs := make([]windows.WSABuf, len(slice))
		overl := new(windows.Overlapped)
		for i, b := range slice {
			wsabufs[i].Len = uint32(len(b))
			wsabufs[i].Buf = &b[0]
		}
		var sent uint32
		err := windows.WSASend(wt.socket, &wsabufs[0], uint32(len(wsabufs)), &sent, 0, overl, nil)
		if err != nil && err != windows.ERROR_IO_PENDING {
			return fmt.Errorf("WSASend batch: %w", err)
		}
		errGQCS := windows.GetQueuedCompletionStatus(wt.iocp, &sent, nil, &overl, windows.INFINITE)
		if errGQCS != nil {
			return fmt.Errorf("GetQueuedCompletionStatus send: %w", errGQCS)
		}
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
		TLS:       false,
		OS:        []string{"windows"},
	}
}
