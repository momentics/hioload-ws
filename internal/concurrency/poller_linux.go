// +build linux

// internal/concurrency/poller_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux (epoll/io_uring) poller implementation for high-concurrency, high-throughput workloads.
// Designed for zero-copy, batch processing, and NUMA-local event demultiplexing.

package concurrency

import (
	"log"
	"syscall"
	"hioload-ws/api"
	"hioload-ws/pool"
)

// LinuxPoller is an edge-triggered event demultiplexer using epoll.
type LinuxPoller struct {
	epfd   int
	events []syscall.EpollEvent
	ring   *pool.RingBuffer[api.Buffer] // NUMA-local event ring
}

// NewLinuxPoller sets up a new epoll-based poller.
func NewLinuxPoller(maxEvents int, ring *pool.RingBuffer[api.Buffer]) (*LinuxPoller, error) {
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &LinuxPoller{
		epfd:   epfd,
		events: make([]syscall.EpollEvent, maxEvents),
		ring:   ring,
	}, nil
}

// RegisterFD adds a descriptor to epoll interest set.
func (p *LinuxPoller) RegisterFD(fd int) error {
	ev := syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLET,
		Fd:     int32(fd),
	}
	return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_ADD, fd, &ev)
}

// UnregisterFD removes a descriptor.
func (p *LinuxPoller) UnregisterFD(fd int) error {
	return syscall.EpollCtl(p.epfd, syscall.EPOLL_CTL_DEL, fd, nil)
}

// Poll blocks up to timeoutMs, fills buffer ring with ready events.
func (p *LinuxPoller) Poll(timeoutMs int) (n int, err error) {
	n, err = syscall.EpollWait(p.epfd, p.events, timeoutMs)
	for i := 0; i < n; i++ {
		fd := p.events[i].Fd
		// Assume api.Buffer abstraction for readiness, actual IO is handled elsewhere.
		buf := p.handleEvent(int(fd))
		p.ring.Enqueue(buf)
	}
	return n, err
}

// handleEvent performs application-specific zero-copy IO (stub for illustration).
func (p *LinuxPoller) handleEvent(fd int) api.Buffer {
	// This would normally call into zero-copy recv using pool/buffer, filling a buffer from the fd.
	log.Printf("[LinuxPoller] handleEvent for fd=%d (stub)", fd)
	return nil
}
