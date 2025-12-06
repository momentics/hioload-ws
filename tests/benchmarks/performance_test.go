// Package benchmarks provides performance benchmarks for hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package benchmarks

import (
	"fmt"
	"testing"

	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

// BenchmarkBufferPool_GetPut benchmarks the buffer pool's Get/Put operations.
func BenchmarkBufferPool_GetPut(b *testing.B) {
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufPool.Get(1000, 0)
		buf.Release()
	}
}

// BenchmarkBufferPool_GetPutParallel benchmarks the buffer pool's Get/Put operations in parallel.
func BenchmarkBufferPool_GetPutParallel(b *testing.B) {
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bufPool.Get(1000, 0)
			buf.Release()
		}
	})
}

// BenchmarkBufferPool_GetPut_SizeClasses benchmarks different size classes.
func BenchmarkBufferPool_GetPut_SizeClasses(b *testing.B) {
	manager := pool.NewBufferPoolManager(1)
	
	sizes := []int{256, 1024, 4096, 16384}
	
	for _, size := range sizes {
		b.Run(sizeToString(size), func(b *testing.B) {
			bufPool := manager.GetPool(size, 0)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf := bufPool.Get(size, 0)
				buf.Release()
			}
		})
	}
}

// BenchmarkEncodeFrameToBytes benchmarks frame encoding performance.
func BenchmarkEncodeFrameToBytes(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := protocol.EncodeFrameToBytes(frame)
		if err != nil {
			b.Fatalf("Encode failed: %v", err)
		}
	}
}

// BenchmarkDecodeFrameFromBytes benchmarks frame decoding performance.
func BenchmarkDecodeFrameFromBytes(b *testing.B) {
	payload := make([]byte, 1024)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	encoded, err := protocol.EncodeFrameToBytes(frame)
	if err != nil {
		b.Fatalf("Failed to encode frame for benchmark: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := protocol.DecodeFrameFromBytes(encoded)
		if err != nil {
			b.Fatalf("Decode failed: %v", err)
		}
	}
}

// BenchmarkEncodeDecodeRoundTrip benchmarks encode/decode round trip.
func BenchmarkEncodeDecodeRoundTrip(b *testing.B) {
	payload := make([]byte, 512)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded, err := protocol.EncodeFrameToBytes(frame)
		if err != nil {
			b.Fatalf("Encode failed: %v", err)
		}
		
		_, err = protocol.DecodeFrameFromBytes(encoded)
		if err != nil {
			b.Fatalf("Decode failed: %v", err)
		}
	}
}

// BenchmarkBufferSlice benchmarks the slice operation on buffers.
func BenchmarkBufferSlice(b *testing.B) {
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(2048, 0)
	
	buf := bufPool.Get(2000, 0)
	data := buf.Bytes()
	for i := range data {
		data[i] = byte(i % 256)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sliced := buf.Slice(100, 500)
		if sliced == nil {
			b.Fatal("Slice returned nil")
		}
	}
	
	buf.Release()
}

// Helper function to convert size to string for benchmark names.
func sizeToString(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%dKB", size/1024)
	}
	return fmt.Sprintf("%dMB", size/(1024*1024))
}

// BenchmarkRingBuffer_EnqueueDequeue benchmarks ring buffer operations.
func BenchmarkRingBuffer_EnqueueDequeue(b *testing.B) {
	rb := pool.NewRingBuffer[int](1024)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !rb.Enqueue(i) {
			b.Fatal("Failed to enqueue")
		}
		
		_, ok := rb.Dequeue()
		if !ok {
			b.Fatal("Failed to dequeue")
		}
	}
}

// BenchmarkRingBuffer_EnqueueDequeueParallel benchmarks ring buffer operations in parallel.
func BenchmarkRingBuffer_EnqueueDequeueParallel(b *testing.B) {
	rb := pool.NewRingBuffer[int](1024)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.Enqueue(42)
			rb.Dequeue()
		}
	})
}