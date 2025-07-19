// go:build linux && !cgo
// +build linux,!cgo

// Package concurrency
// Author: momentics <momentics@gmail.com>
//
// Stub implementation of PinCurrentThread for Linux when CGO is disabled.
// The real CGO-based version (pin_linux.go) uses sched_setaffinity / libnuma,
// однако при сборке без CGO файл с import "C" исключается, что приводило
// к ошибке «undefined: PinCurrentThread». Данный no-op-вариант решает проблему
// и позволяет проекту компилироваться на pure-Go окружениях.
//
// В production-cреди с включённым CGO продолжит использоваться полноценная
// версия из pin_linux.go — Go build выберет её благодаря условиям сборки.

package concurrency

import "runtime"

// PinCurrentThread no-op stub for Linux without CGO.
func PinCurrentThread(numaNode int, cpuID int) {
	runtime.LockOSThread()
}