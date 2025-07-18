//go:build linux || darwin

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Platform-specific socket I/O for Unix-like systems (Linux, macOS).

package main

import "syscall"

// readFromSocket reads from a raw file descriptor (socket) on Unix-like systems.
func readFromSocket(fd uintptr, buf []byte) (int, error) {
	// syscall.Read operates on file descriptors (int)
	return syscall.Read(int(fd), buf)
}

// writeToSocket writes to a raw file descriptor (socket) on Unix-like systems.
func writeToSocket(fd uintptr, buf []byte) (int, error) {
	return syscall.Write(int(fd), buf)
}

// closeSocket closes a raw file descriptor (socket) on Unix-like systems.
func closeSocket(fd uintptr) error {
	return syscall.Close(int(fd))
}
