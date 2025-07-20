// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// benchmark_echo_server_test.go — Basic benchmark for echo server data path.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/pool"
)

// BenchmarkBufferCopy measures zero-copy echo loop.
func BenchmarkBufferCopy(b *testing.B) {
	pool := pool.NewBufferPoolManager().GetPool(-1)
	msg := []byte("dummy message for echo")
	size := len(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(size, -1)
		copy(buf.Bytes(), msg)
		_ = buf.Bytes()
		buf.Release()
	}
}
