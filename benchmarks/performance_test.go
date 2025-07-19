// Package benchmarks
// Author: momentics <momentics@gmail.com>
//
// Performance benchmarks for hioload-ws components.

package benchmarks

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/fake"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// BenchmarkBufferPoolAllocation tests buffer pool allocation performance.
func BenchmarkBufferPoolAllocation(b *testing.B) {
	manager := pool.NewBufferPoolManager()
	bufferPool := manager.GetPool(0) // NUMA node 0

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buffer := bufferPool.Get(4096, 0)
			buffer.Release()
		}
	})
}

// BenchmarkRingBufferThroughput tests lock-free ring buffer performance.
func BenchmarkRingBufferThroughput(b *testing.B) {
	ring := pool.NewRingBuffer[int](1024)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if !ring.Enqueue(i) {
				ring.Dequeue()
				ring.Enqueue(i)
			}
			i++
		}
	})
}

// BenchmarkFacadeIntegration tests end-to-end facade performance.
func BenchmarkFacadeIntegration(b *testing.B) {
	config := facade.DefaultConfig()
	config.NumWorkers = 4
	config.BatchSize = 16
	hioload, err := facade.New(config)
	if err != nil {
		b.Fatal(err)
	}
	defer hioload.Stop()
	if err := hioload.Start(); err != nil {
		b.Fatal(err)
	}
	bufferPool := hioload.GetBufferPool()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer := bufferPool.Get(1024, 0)
		buffer.Release()
	}
}

// BenchmarkTransportSend benchmarks transport layer sending performance.
func BenchmarkTransportSend(b *testing.B) {
	transport := fake.NewTransport()
	data := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transport.Send([][]byte{data})
	}
}

// BenchmarkPollerProcessing tests event poller processing speed.
func BenchmarkPollerProcessing(b *testing.B) {
	config := facade.DefaultConfig()
	hioload, _ := facade.New(config)
	hioload.Start()
	defer hioload.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hioload.Poll(10)
	}
}

// BenchmarkWebSocketFrameEncoding tests WebSocket frame encoding performance.
func BenchmarkWebSocketFrameEncoding(b *testing.B) {
	payload := make([]byte, 1024)
	dst := make([]byte, 2048)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		protocol.EncodeFrame(dst, protocol.OpcodeText, payload, false)
	}
}

// BenchmarkConcurrentConnections tests performance under high connection load.
func BenchmarkConcurrentConnections(b *testing.B) {
	config := facade.DefaultConfig()
	config.NumWorkers = 8
	hioload, _ := facade.New(config)
	hioload.Start()
	defer hioload.Stop()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn := hioload.CreateWebSocketConnection()
			time.Sleep(time.Microsecond)
			conn.Close()
		}
	})
}
