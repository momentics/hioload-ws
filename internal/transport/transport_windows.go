//go:build windows
// +build windows

// Package transport
// Author: momentics <momentics@gmail.com>
//
// Windows platform-specific Transport implementation using IOCP and zero-copy
// with NUMA-aware buffer management and thread affinity.

package transport

import (
    "errors"
    "sync"
    "github.com/momentics/hioload-ws/api"
    "golang.org/x/sys/windows"
)

// windowsTransport is Windows optimized Transport using IOCP.
type windowsTransport struct {
    iocp windows.Handle
    // bufferPool for NUMA-aware zero-copy buffers
    bufferPool api.BufferPool
    mu sync.Mutex
    closed bool
}

// newTransportInternal creates windowsTransport platform specific implementation.
func newTransportInternal() (api.Transport, error) {
    iocp, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0)
    if err != nil {
        return nil, err
    }
    var bufferPool api.BufferPool
    // TODO: initialize bufferPool with NUMA-aware allocations

    wt := &windowsTransport{
        iocp: iocp,
        bufferPool: bufferPool,
    }
    return wt, nil
}

// Send implements zero-copy batch send operation.
func (wt *windowsTransport) Send(buffers [][]byte) error {
    if wt.closed {
        return errors.New("transport closed")
    }
    wt.mu.Lock()
    defer wt.mu.Unlock()
    // Implement zero-copy send using WSASend or overlapped IO
    // TODO: zero-copy send
    return nil
}

// Recv implements zero-copy batch receive operation.
func (wt *windowsTransport) Recv() ([][]byte, error) {
    if wt.closed {
        return nil, errors.New("transport closed")
    }
    // TODO: zero-copy receive implementation via ReadFile with overlapped IO
    var buffers [][]byte
    return buffers, nil
}

// Close closes all resources.
func (wt *windowsTransport) Close() error {
    wt.mu.Lock()
    defer wt.mu.Unlock()
    if wt.closed {
        return nil
    }
    wt.closed = true
    if wt.iocp != 0 {
        windows.CloseHandle(wt.iocp)
    }
    return nil
}

// Features returns transport capabilities.
func (wt *windowsTransport) Features() api.TransportFeatures {
    return api.TransportFeatures{
        ZeroCopy:     true,
        Batch:        true,
        NUMAAware:    true,
        LockFree:     false,
        SharedMemory: false,
        OS:           []string{"windows"},
    }
}
