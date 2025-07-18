// File: reactor/reactor.go
// Author: momentics <momentics@gmail.com>
//
// Platform-neutral event reactor interface for cross-platform IO multiplexing.

package reactor

// EventReactor defines basic reactor operations across OS platforms.
type EventReactor interface {
	// Register an FD (epoll) or HANDLE (Windows) for IO notifications.
	Register(fd uintptr, userData uintptr) error

	// Wait blocks until events are available and writes into the output slice.
	// Returns number of events written or an error.
	Wait(events []Event) (n int, err error)

	// Close cleans up resources (handle/epfd).
	Close() error
}

// Event contains event information returned by Wait call.
type Event struct {
	Fd       uintptr // File descriptor or handle.
	UserData uintptr // User-provided data.
}
