//go:build linux
// +build linux

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Epoll + io_uring based poller for zero-copy.

package concurrency

import (
    "golang.org/x/sys/unix"
    "github.com/momentics/hioload-ws/pool"
)

type LinuxPoller struct {
    ring     *pool.RingBuffer[any]
    epfd     int
    maxEvents int
    events   []unix.EpollEvent
}

func NewLinuxPoller(maxEvents int, ring *pool.RingBuffer[any]) (*LinuxPoller, error) {
    epfd, err := unix.EpollCreate1(0)
    if err != nil {
        return nil, err
    }
    return &LinuxPoller{
        ring:      ring,
        epfd:      epfd,
        maxEvents: maxEvents,
        events:    make([]unix.EpollEvent, maxEvents),
    }, nil
}

func (p *LinuxPoller) Poll(timeoutMs int) (int, error) {
    n, err := unix.EpollWait(p.epfd, p.events, timeoutMs)
    for i := 0; i < n; i++ {
        // TODO: replace stub with io_uring + recvmmsg integration
        buf := p.ring // placeholder for actual buffer
        p.ring.Enqueue(buf)
    }
    return n, err
}

func (p *LinuxPoller) Close() error {
    return unix.Close(p.epfd)
}
