//go:build !linux && !windows
// +build !linux,!windows

// File: reactor/reactor_stub.go
// Author: momentics <momentics@gmail.com>
//
// Stub implementation for unsupported platforms.

package reactor

import "errors"

// NewReactor returns an error for unsupported platforms.
func NewReactor() (EventReactor, error) {
	return nil, errors.New("reactor: this platform is not supported")
}
