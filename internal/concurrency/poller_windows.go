// +build windows

// internal/concurrency/poller_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows IOCP (I/O Completion Port) poller implementation for scalable NUMA-aware event demultiplexing.
// Supports zero-copy buffer hand-offs to the event loop.

package concurrency

import (
	"log"
	"golang.org/x/sys/windows"
	"hioload-ws/api"
	"hioload-ws/pool"
)

// WindowsPoller is the event loop using IOCP.
type WindowsPoller struct {
	iocp windows.Handle
	ring *pool.RingBuffer[api.Buffer]
}

// NewWindowsPoller creates a new IOCP poller.
func NewWindowsPoller(ring *pool.RingBuffer[api.Buffer]) (*WindowsPoller, error) {
	iocp, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	return &WindowsPoller{iocp: iocp, ring: ring}, nil
}

// Register associates a socket with the IOCP.
func (p *WindowsPoller) Register(handle windows.Handle) error {
	_, err := windows.CreateIoCompletionPort(handle, p.iocp, 0, 0)
	return err
}

// Poll retrieves completed asynchronous IO and fills the buffer ring.
func (p *WindowsPoller) Poll(timeoutMs int) (int, error) {
	var n uint32
	var key uintptr
	var overlapped *windows.Overlapped

	err := windows.GetQueuedCompletionStatus(p.iocp, &n, &key, &overlapped, uint32(timeoutMs))
	if err != nil {
		return 0, err
	}
	// Create buffer and enqueue (stub for illustration)
	log.Printf("[WindowsPoller] IOCP event handled (stub)")
	return 1, nil
}
