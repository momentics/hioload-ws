//go:build windows

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Platform-specific socket I/O for Windows systems.

package main

import "syscall"

// readFromSocket reads from a socket handle on Windows using syscall.Read.
// On modern Go/Windows, syscall.Read works for TCP sockets.
func readFromSocket(fd uintptr, buf []byte) (int, error) {
	return syscall.Read(syscall.Handle(fd), buf)
}

// writeToSocket writes to a socket handle on Windows using syscall.Write.
func writeToSocket(fd uintptr, buf []byte) (int, error) {
	return syscall.Write(syscall.Handle(fd), buf)
}

// closeSocket closes a socket handle on Windows using syscall.Closesocket.
func closeSocket(fd uintptr) error {
	return syscall.Closesocket(syscall.Handle(fd))
}
