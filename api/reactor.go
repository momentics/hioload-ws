// File: api/reactor.go
// Author: momentics <momentics@gmail.com>
//
// Defines the abstract interface for event-driven IO Reactors
// used to multiplex connections across poll-mode backends (epoll, IOCP, io_uring, etc.)

package api

// Event encapsulates the result of an OS-level readiness notification
type Event struct {
	Fd       uintptr // file descriptor or system handle
	UserData uintptr // opaque application value, usually a pointer-to-connection/context
}

// Reactor defines the common interface for an event-loop that dispatches I/O events
// regardless of specific polling mechanism used.
type Reactor interface {
	// Register must associate a socket/file handle with the event loop
	Register(fd uintptr, userData uintptr) error

	// Wait must block and fill events into output buffer when IO is ready
	Wait(events []Event) (int, error)

	// Close must cleanup the internal poller backend
	Close() error
}
