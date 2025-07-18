//go:build linux
// +build linux

// File: reactor/reactor_linux.go
// Author: momentics <momentics@gmail.com>
//
// Linux epoll(7)-based reactor implementation and factory.

package reactor

import (
	"golang.org/x/sys/unix"
	"unsafe"
)

// linuxReactor is an epoll-based event reactor.
type linuxReactor struct {
	epfd int
}

// NewReactor constructs a new platform-specific EventReactor for Linux.
func NewReactor() (EventReactor, error) {
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	return &linuxReactor{epfd: epfd}, nil
}

// Register adds file descriptor to epoll.
func (r *linuxReactor) Register(fd uintptr, udata uintptr) error {
	event := &unix.EpollEvent{
		Events: unix.EPOLLIN | unix.EPOLLOUT | unix.EPOLLET,
		Fd:     int32(fd),
	}
	*(*uintptr)(unsafe.Pointer(&event.Pad)) = udata
	return unix.EpollCtl(r.epfd, unix.EPOLL_CTL_ADD, int(fd), event)
}

// Wait waits for epoll events and fills the result into events slice.
func (r *linuxReactor) Wait(events []Event) (int, error) {
	rawEvents := make([]unix.EpollEvent, len(events))
	n, err := unix.EpollWait(r.epfd, rawEvents, -1)
	if err != nil {
		return 0, err
	}
	for i := 0; i < n; i++ {
		events[i] = Event{
			Fd:       uintptr(rawEvents[i].Fd),
			UserData: *(*uintptr)(unsafe.Pointer(&rawEvents[i].Pad)),
		}
	}
	return n, nil
}

// Close closes the epoll instance.
func (r *linuxReactor) Close() error {
	return unix.Close(r.epfd)
}
