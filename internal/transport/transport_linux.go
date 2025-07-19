//go:build linux
// +build linux

// Package transport
// Author: momentics <momentics@gmail.com>
//
// Linux platform-specific Transport implementation using epoll/io_uring and zero-copy
// with NUMA-aware buffer and thread affinity management.

package transport

import (
    "errors"
    "sync"
    "github.com/momentics/hioload-ws/api"
    "golang.org/x/sys/unix"
)

// linuxTransport is a Linux-optimized Transport implementation using epoll and io_uring.
type linuxTransport struct {
    fd int
    epfd int
    // bufferPool manages NUMA-aware buffers
    bufferPool api.BufferPool
    // concurrency control primitives
    mu sync.Mutex
    closed bool
    // eventFd for event notifications
    eventFd int
}

// newTransportInternal creates linuxTransport platform specific implementation.
func newTransportInternal() (api.Transport, error) {
    epfd, err := unix.EpollCreate1(0)
    if err != nil {
        return nil, err
    }
    // Setup eventFd for notifications (could be created here)
    eventFd, err := unix.Eventfd(0, 0)
    if err != nil {
        unix.Close(epfd)
        return nil, err
    }

    // Initialize bufferPool here or obtain from elsewhere
    var bufferPool api.BufferPool
    // TODO: initialize bufferPool with NUMA-aware allocator

    lt := &linuxTransport{
        fd: -1, // placeholder for underlying socket
        epfd: epfd,
        bufferPool: bufferPool,
        eventFd: eventFd,
    }
    return lt, nil
}

// Send implements zero-copy batch send operation.
func (lt *linuxTransport) Send(buffers [][]byte) error {
    if lt.closed {
        return errors.New("transport closed")
    }
    lt.mu.Lock()
    defer lt.mu.Unlock()
    // Implement zero-copy send logic here, e.g., using sendmsg with iovecs or io_uring
    // TODO: zero-copy send
    return nil
}

// Recv implements zero-copy batch receive operation.
func (lt *linuxTransport) Recv() ([][]byte, error) {
    if lt.closed {
        return nil, errors.New("transport closed")
    }
    // Implementation of zero-copy receive using recvmsg or io_uring
    var buffers [][]byte
    // TODO: fill buffers with zero-copy data slices
    return buffers, nil
}

// Close closes all resources.
func (lt *linuxTransport) Close() error {
    lt.mu.Lock()
    defer lt.mu.Unlock()
    if lt.closed {
        return nil
    }
    lt.closed = true
    if lt.fd >= 0 {
        unix.Close(lt.fd)
    }
    if lt.epfd >= 0 {
        unix.Close(lt.epfd)
    }
    if lt.eventFd >= 0 {
        unix.Close(lt.eventFd)
    }
    return nil
}

// Features returns transport capabilities.
func (lt *linuxTransport) Features() api.TransportFeatures {
    return api.TransportFeatures{
        ZeroCopy:     true,
        Batch:        true,
        NUMAAware:    true,
        LockFree:     false, // depends on implementation
        SharedMemory: false,
        OS:           []string{"linux"},
    }
}
