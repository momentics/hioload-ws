//go:build linux && !io_uring
// +build linux,!io_uring

// Package internal/transport implements epoll-based transport for Linux (fallback when io_uring unavailable).
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Epoll-based transport for Linux using SendmsgBuffers/RecvmsgBuffers for maximum performance.
// Full support for buffer pool NUMA pinning. Integrated with latest BufferPoolManager.
package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/pool"
	"golang.org/x/sys/unix"
)

// normalizeNUMANode ensures numaNode is valid within platform limits.
func normalizeNUMANode(numaNode int) int {
	maxNodes := concurrency.NUMANodes()
	if maxNodes <= 0 {
		return 0
	}
	if numaNode < 0 || numaNode >= maxNodes {
		return 0
	}
	return numaNode
}

// newTransportInternal creates a platform-native transport for Linux (epoll-based fallback).
func newTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	node := normalizeNUMANode(numaNode)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)

	// Create NUMA-aware buffer pool
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &epollTransport{
		fd:           fd,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

// epollTransport implements api.Transport using epoll and SendmsgBuffers for maximum performance.
type epollTransport struct {
	mu           sync.Mutex
	fd           int
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
}

func (et *epollTransport) Recv() ([][]byte, error) {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.closed {
		return nil, api.ErrTransportClosed
	}
	batch := 16
	bufs := make([][]byte, batch)
	for i := range bufs {
		buf := et.bufPool.Get(et.ioBufferSize, et.numaNode)
		bufs[i] = buf.Bytes()
	}
	n, _, _, _, err := unix.RecvmsgBuffers(et.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
}

func (et *epollTransport) Send(buffers [][]byte) error {
	et.mu.Lock()
	defer et.mu.Unlock()
	if et.closed {
		return api.ErrTransportClosed
	}
	const maxBatch = 16
	left := len(buffers)
	sent := 0
	for left > 0 {
		batch := buffers[sent:]
		if len(batch) > maxBatch {
			batch = batch[:maxBatch]
		}
		n, err := unix.SendmsgBuffers(et.fd, batch, nil, nil, 0)
		if err != nil {
			return fmt.Errorf("SendmsgBuffers: %w", err)
		}
		// n is the number of bytes sent, but it should be at least the size of our batch
		if n <= 0 {
			return fmt.Errorf("SendmsgBuffers: sent no data")
		}
		// All buffers in the batch were sent successfully
		sent += len(batch)
		left -= len(batch)
	}
	return nil
}

func (et *epollTransport) Close() error {
	et.mu.Lock()
	defer et.mu.Unlock()
	if !et.closed {
		unix.Close(et.fd)
		et.closed = true
	}
	return nil
}

func (et *epollTransport) Features() api.TransportFeatures {
	return DetectTransportFeatures()
}