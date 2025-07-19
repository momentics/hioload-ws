//go:build windows
// +build windows

// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Windows IOCP-based poller for zero-copy and batching.

package concurrency

import (
    "github.com/momentics/hioload-ws/pool"
    "golang.org/x/sys/windows"
)

type WindowsPoller struct {
    iocp windows.Handle
    ring *pool.RingBuffer[any]
}

func NewWindowsPoller(maxEvents int, ring *pool.RingBuffer[any]) (*WindowsPoller, error) {
    iocp, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0)
    if err != nil {
        return nil, err
    }
    return &WindowsPoller{iocp: iocp, ring: ring}, nil
}

func (p *WindowsPoller) Poll(timeoutMs int) (int, error) {
    var n uint32
    _, err := windows.GetQueuedCompletionStatus(p.iocp, &n, nil, nil, uint32(timeoutMs))
    if err != nil {
        return 0, err
    }
    return int(n), nil
}

func (p *WindowsPoller) Close() error {
    return windows.CloseHandle(p.iocp)
}
