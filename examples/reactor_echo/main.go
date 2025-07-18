// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Example: Cross-platform echo server using poll-mode reactor abstraction.
// Console output indicates activity for demonstration and debugging.

package main

import (
	"fmt"
	"github.com/momentics/hioload-ws/reactor"
	"net"
	"os"
)

// getFD extracts the raw file descriptor (Unix) or socket handle (Windows)
// from net.TCPConn. This allows passing the descriptor to the reactor.
func getFD(c *net.TCPConn) uintptr {
	raw, err := c.SyscallConn()
	var fd uintptr
	if err == nil {
		raw.Control(func(f uintptr) { fd = f })
	}
	return fd
}

// readFromSocket, writeToSocket, closeSocket are platform-specific and
// declared in separate files using build tags.

func main() {
	// Start TCP listener on port 9002
	ln, err := net.Listen("tcp", ":9002")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[reactor-echo] Listening on :9002 ...\n")

	// Create cross-platform reactor (epoll for Linux, IOCP for Windows)
	r, err := reactor.NewReactor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reactor error: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()
	fmt.Printf("[reactor-echo] Reactor initialized\n")

	for {
		// Accept new TCP connections
		conn, err := ln.(*net.TCPListener).AcceptTCP()
		if err != nil {
			fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
			continue
		}
		clientAddr := conn.RemoteAddr().String()
		fmt.Printf("[reactor-echo] Accepted connection from %s\n", clientAddr)

		fd := getFD(conn)
		r.Register(fd, reactor.EventRead, func(fd uintptr, events reactor.FDEventType) {
			buf := make([]byte, 4096)
			// Read data from socket using platform-specific function.
			n, err := readFromSocket(fd, buf)
			if err != nil {
				fmt.Printf("[reactor-echo] Read error on fd=%d: %v\n", fd, err)
				closeSocket(fd)
				r.Unregister(fd)
				return
			}
			if n == 0 {
				fmt.Printf("[reactor-echo] Connection closed by peer (fd=%d)\n", fd)
				closeSocket(fd)
				r.Unregister(fd)
				return
			}
			data := buf[:n]
			fmt.Printf("[reactor-echo] Received (%d bytes): %s\n", n, string(data))

			// Echo the received data back to the client.
			nw, err := writeToSocket(fd, data)
			if err != nil {
				fmt.Printf("[reactor-echo] Write error on fd=%d: %v\n", fd, err)
				closeSocket(fd)
				r.Unregister(fd)
				return
			}
			fmt.Printf("[reactor-echo] Sent (%d bytes) back to client\n", nw)
		})
	}
}
