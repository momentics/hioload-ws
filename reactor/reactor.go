// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package reactor provides an abstraction for the poll-mode event reactor,
// implementing scalable, low-latency event loops via epoll (Linux) and IOCP (Windows).
package reactor

import (
    "errors"
    "runtime"
)

// FDEventType represents the type of event on a file descriptor or handle.
type FDEventType int

const (
    EventRead FDEventType = 1 << iota
    EventWrite
    EventError
)

// FDCallback defines the type for file/event handlers.
type FDCallback func(fd uintptr, events FDEventType)

// Reactor defines the platform-neutral interface for a poll-mode event reactor.
type Reactor interface {
    Register(fd uintptr, events FDEventType, cb FDCallback) error
    Unregister(fd uintptr) error
    Poll(timeoutMs int) error // timeoutMs < 0 = infinite
    Close() error
}

// NewReactor creates a new platform-specific poll-mode reactor instance.
func NewReactor() (Reactor, error) {
    switch runtime.GOOS {
    case "linux":
        return newEpollReactor()
    case "windows":
        return newIOCPReactor()
    default:
        return nil, errors.New("poll-mode reactor not implemented for this OS")
    }
}

// newEpollReactor is a stub for non-Linux OS.
func newEpollReactor() (Reactor, error) {
    return nil, errors.New("epoll reactor not implemented on this OS")
}
