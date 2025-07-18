//go:build linux
// +build linux

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package reactor - Linux epoll implementation.

package reactor

import (
    "fmt"
    "os"
    "sync"
    "syscall"
)

// epollReactor implements Reactor interface using Linux epoll.
type epollReactor struct {
    epfd      int           // epoll file descriptor
    callbacks sync.Map      // map[uintptr]FDCallback
}

// newEpollReactor creates a new instance of epollReactor.
func newEpollReactor() (Reactor, error) {
    epfd, err := syscall.EpollCreate1(0)
    if err != nil {
        return nil, fmt.Errorf("epoll create: %w", err)
    }

    return &epollReactor{
        epfd:      epfd,
        callbacks: sync.Map{},
    }, nil
}

// Register adds a file descriptor to the epoll watch list.
func (r *epollReactor) Register(fd uintptr, events FDEventType, cb FDCallback) error {
    var ev syscall.EpollEvent
    ev.Events = 0
    if events&EventRead != 0 {
        ev.Events |= syscall.EPOLLIN
    }
    if events&EventWrite != 0 {
        ev.Events |= syscall.EPOLLOUT
    }
    ev.Fd = int32(fd)

    if err := syscall.EpollCtl(r.epfd, syscall.EPOLL_CTL_ADD, int(fd), &ev); err != nil {
        return fmt.Errorf("epoll ctl add: %w", err)
    }

    r.callbacks.Store(fd, cb)
    return nil
}

// Unregister removes a file descriptor from the epoll watch list.
func (r *epollReactor) Unregister(fd uintptr) error {
    if err := syscall.EpollCtl(r.epfd, syscall.EPOLL_CTL_DEL, int(fd), nil); err != nil {
        return fmt.Errorf("epoll ctl del: %w", err)
    }
    r.callbacks.Delete(fd)
    return nil
}

// Poll blocks and waits for events on registered file descriptors.
// timeoutMs < 0 means block infinitely.
func (r *epollReactor) Poll(timeoutMs int) error {
    const maxEvents = 128
    var events [maxEvents]syscall.EpollEvent
    timeout := timeoutMs
    if timeout < 0 {
        timeout = -1
    }

    n, err := syscall.EpollWait(r.epfd, events[:], timeout)
    if err != nil {
        if err == syscall.EINTR {
            return nil // interrupted by signal — normal
        }
        return fmt.Errorf("epoll wait: %w", err)
    }

    for i := 0; i < n; i++ {
        ev := events[i]
        fd := uintptr(ev.Fd)

        val, ok := r.callbacks.Load(fd)
        if !ok {
            continue
        }

        var eventType FDEventType
        if ev.Events&syscall.EPOLLIN != 0 {
            eventType |= EventRead
        }
        if ev.Events&syscall.EPOLLOUT != 0 {
            eventType |= EventWrite
        }
        if ev.Events&(syscall.EPOLLERR|syscall.EPOLLHUP) != 0 {
            eventType |= EventError
        }

        cb, _ := val.(FDCallback)
        // Use deferred recover to ensure reactor continuity on panics.
        func() {
            defer func() { _ = recover() }()
            cb(fd, eventType)
        }()
    }

    return nil
}

// Close releases the epoll file descriptor.
func (r *epollReactor) Close() error {
    return syscall.Close(r.epfd)
}
