// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// benchmark_transport_parallel_test.go — Benchmark parallel batched transport send/recv.
package tests

import (
	"sync"
	"testing"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/pool"
)

type fakeTransport struct{}

func (f *fakeTransport) Send(batch [][]byte) error { return nil }
func (f *fakeTransport) Recv() ([][]byte, error)  { return nil, nil }
func (f *fakeTransport) Close() error             { return nil }
func (f *fakeTransport) Features() api.TransportFeatures {
	return api.TransportFeatures{ZeroCopy: true, Batch: true}
}

func Benchmark_ParallelSend(b *testing.B) {
	tr := &fakeTransport{}
	pool := pool.NewBufferPoolManager().GetPool(-1)
	data := make([]byte, 1024)
	batch := [][]byte{data, data, data, data}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for i := 0; i < 50; i++ {
				tr.Send(batch)
			}
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(4096, -1)
			buf.Release()
		}
	})
}
