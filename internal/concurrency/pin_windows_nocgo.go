// go:build windows && !cgo
// +build windows,!cgo

// Package concurrency
// Author: momentics <momentics@gmail.com>
//
// Stub implementation of PinCurrentThread for Windows when CGO is disabled
// (или при сборке в средах, где системные вызовы через syscall уже недоступны
// — например tinygo, wasm и пр.).  Реальная версия pin_windows.go использует
// вызовы Kernel32, но при некоторых вариантах CGO/syscall она может быть
// исключена.  Данная реализация гарантирует наличие символа и единое API.

package concurrency

import "runtime"

// PinCurrentThread no-op stub for Windows without CGO.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread()
}