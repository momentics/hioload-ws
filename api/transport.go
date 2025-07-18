// File: api/transport.go
// Author: momentics <momentics@gmail.com>
//
// Defines transport socket abstraction (NetConn) for compatibility
// with custom event loops, memory pools, and zero-copy pipelines.

package api


// NetConn abstracts a full-duplex network connection object
// that may or may not be backed by Go's net.Conn
type NetConn interface {
	// Read reads into a preallocated buffer
	Read(p []byte) (n int, err error)

	// Write writes buffer contents into the connection
	Write(p []byte) (n int, err error)

	// Close shuts down the connection and notifies upstream layers
	Close() error

	// RawFD returns the underlying OS-level file descriptor
	RawFD() uintptr
}
