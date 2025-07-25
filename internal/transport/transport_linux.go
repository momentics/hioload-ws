// File: internal/transport/transport_linux.go
//go:build linux
// +build linux

//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Linux-specific, NUMA-aware, batch-IO transport using SendmsgBuffers/RecvmsgBuffers.
// Fully integrated with latest BufferPoolManager and NUMA-detection logic.

package transport

import (
	"fmt"
	"sync"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/normalize"
	"github.com/momentics/hioload-ws/pool"

	"golang.org/x/sys/unix"
)

type linuxTransport struct {
	mu           sync.Mutex
	fd           int
	bufPool      api.BufferPool
	ioBufferSize int
	numaNode     int
	closed       bool
}

// normalizeNUMANode ensures numaNode is valid within platform limits.
// Returns fallback node=0 if out of range or negative.
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

// newTransportInternal creates a platform-native transport for Linux.
// NUMA node is used for buffer allocation; if negative or invalid, fallback to 0.
func newTransportInternal(ioBufferSize, numaNode int) (api.Transport, error) {
	node := normalize.NUMANodeAuto(numaNode)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM|unix.SOCK_NONBLOCK, unix.IPPROTO_TCP)
	if err != nil {
		return nil, fmt.Errorf("socket create: %w", err)
	}
	_ = unix.SetsockoptInt(fd, unix.IPPROTO_TCP, unix.TCP_NODELAY, 1)

	// Create NUMA-aware buffer pool using normalized node
	bufPool := pool.NewBufferPoolManager(concurrency.NUMANodes()).GetPool(ioBufferSize, node)

	return &linuxTransport{
		fd:           fd,
		bufPool:      bufPool,
		ioBufferSize: ioBufferSize,
		numaNode:     node,
	}, nil
}

func (lt *linuxTransport) Recv() ([][]byte, error) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
		return nil, api.ErrTransportClosed
	}
	batch := 16
	bufs := make([][]byte, batch)
	for i := range bufs {
		buf := lt.bufPool.Get(lt.ioBufferSize, lt.numaNode)
		bufs[i] = buf.Bytes()
	}
	n, _, _, _, err := unix.RecvmsgBuffers(lt.fd, bufs, nil, unix.MSG_DONTWAIT)
	if err != nil {
		return nil, fmt.Errorf("RecvmsgBuffers: %w", err)
	}
	return bufs[:n], nil
}

func (lt *linuxTransport) Send(buffers [][]byte) error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if lt.closed {
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
		n, err := unix.SendmsgBuffers(lt.fd, batch, nil, nil, 0)
		if err != nil {
			return fmt.Errorf("SendmsgBuffers: %w", err)
		}
		if n <= 0 {
			return fmt.Errorf("SendmsgBuffers: sent no buffers")
		}
		sent += n
		left -= n
	}
	return nil
}

func (lt *linuxTransport) Close() error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if !lt.closed {
		unix.Close(lt.fd)
		lt.closed = true
	}
	return nil
}

func (lt *linuxTransport) Features() api.TransportFeatures {
	return DetectTransportFeatures()
}
