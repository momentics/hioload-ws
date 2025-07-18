//go:build windows
// +build windows

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package reactor - Windows IOCP implementation.
package reactor

import (
    "fmt"
    "os"
    "sync"
    "sync/atomic"
    "syscall"
)

// fdCallbackEntry stores both the callback and original fd for key mapping.
type fdCallbackEntry struct {
    fd uintptr
    cb FDCallback
}

// iocpReactor implements Reactor using Windows IOCP.
type iocpReactor struct {
    iocp       syscall.Handle
    callbacks  sync.Map // map[uint32]*fdCallbackEntry
    keyCounter uint32   // atomic for completion key generation
    closed     chan struct{}
}

// newIOCPReactor creates and returns a new IOCP reactor for Windows.
func newIOCPReactor() (*iocpReactor, error) {
    iocp, err := syscall.CreateIoCompletionPort(syscall.InvalidHandle, 0, 0, 0)
    if err != nil {
        return nil, fmt.Errorf("iocp create: %w", err)
    }
    r := &iocpReactor{
        iocp:   iocp,
        closed: make(chan struct{}),
    }
    return r, nil
}

func (r *iocpReactor) Register(fd uintptr, events FDEventType, cb FDCallback) error {
    // Generate unique completion key as uint32 (Windows expects ULONG_PTR, but syscall interface wants uint32).
    key := atomic.AddUint32(&r.keyCounter, 1)
    handle := syscall.Handle(fd)
    ret, err := syscall.CreateIoCompletionPort(handle, r.iocp, uint32(key), 0)
    if err != nil || ret == 0 {
        return fmt.Errorf("iocp associate: %w", err)
    }
    r.callbacks.Store(key, &fdCallbackEntry{fd: fd, cb: cb})
    return nil
}

func (r *iocpReactor) Unregister(fd uintptr) error {
    // Remove only the mapping that matches fd. Inefficient linear scan, but fine for demo skeleton.
    var keyToDelete interface{}
    r.callbacks.Range(func(k, v interface{}) bool {
        entry, _ := v.(*fdCallbackEntry)
        if entry != nil && entry.fd == fd {
            keyToDelete = k
            return false
        }
        return true
    })
    if keyToDelete != nil {
        r.callbacks.Delete(keyToDelete)
    }
    return nil
}

func (r *iocpReactor) Poll(timeoutMs int) error {
    var bytes uint32
    var key uint32
    var overlapped *syscall.Overlapped
    timeout := uint32(syscall.INFINITE)
    if timeoutMs >= 0 {
        timeout = uint32(timeoutMs)
    }
    for {
        select {
        case <-r.closed:
            return nil
        default:
        }
        err := syscall.GetQueuedCompletionStatus(
            r.iocp,
            &bytes,
            &key,
            &overlapped,
            timeout,
        )
        if err != nil && err != syscall.Errno(0) {
            if err == syscall.Errno(syscall.WAIT_TIMEOUT) {
                return nil
            }
            fmt.Fprintf(os.Stderr, "iocp poll: %v\n", err)
            continue
        }
        val, ok := r.callbacks.Load(key)
        if !ok {
            continue
        }
        entry, _ := val.(*fdCallbackEntry)
        evt := EventRead // Actual event type determination requires more context (not shown here)
        func() {
            defer func() { _ = recover() }()
            entry.cb(entry.fd, evt)
        }()
    }
}

func (r *iocpReactor) Close() error {
    close(r.closed)
    return syscall.CloseHandle(r.iocp)
}
