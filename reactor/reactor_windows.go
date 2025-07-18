//go:build windows
// +build windows

// File: reactor/reactor_windows.go
// Author: momentics <momentics@gmail.com>
//
// Windows IOCP (I/O Completion Port) reactor implementation and factory.

package reactor

import (
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

// windowsReactor is an IOCP-based event reactor.
type windowsReactor struct {
	iocp windows.Handle
}

// NewReactor constructs a new platform-specific EventReactor for Windows.
func NewReactor() (EventReactor, error) {
	port, err := windows.CreateIoCompletionPort(
		windows.InvalidHandle,
		0,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}
	return &windowsReactor{
		iocp: port,
	}, nil
}

// Register associates a handle with IOCP.
func (r *windowsReactor) Register(handle uintptr, userData uintptr) error {
	h := windows.Handle(handle)
	_, err := windows.CreateIoCompletionPort(
		h,
		r.iocp,
		userData,
		0,
	)
	return err
}

// Wait blocks for IO events and fills output slice.
func (r *windowsReactor) Wait(events []Event) (int, error) {
	if len(events) == 0 {
		return 0, errors.New("reactor: empty event buffer")
	}

	var key uintptr
	var overlapped *windows.Overlapped

	err := windows.GetQueuedCompletionStatus(r.iocp, nil, &key, &overlapped, windows.INFINITE)
	if err != nil {
		return 0, err
	}
	events[0] = Event{
		Fd:       uintptr(unsafe.Pointer(overlapped)), // Overlapped pointer (often used as handle context)
		UserData: key,
	}
	return 1, nil
}

// Close closes the IOCP handle.
func (r *windowsReactor) Close() error {
	return windows.CloseHandle(r.iocp)
}
