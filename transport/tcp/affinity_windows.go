//go:build windows
// +build windows

// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package tcp - Windows stub for CPU affinity.

package tcp

// setCPUAffinity is a no-op on Windows (future stub).
func setCPUAffinity(cpu int) {
    // Not implemented: see SetThreadAffinityMask for Windows in future versions.
}
