// File: examples/numa_affinity/main.go
// Author: momentics <momentics@gmail.com>

package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/momentics/hioload-ws/affinity"
	"github.com/momentics/hioload-ws/pool"
)

func main() {
	// Pin this goroutine to OS thread for affinity control
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Set thread affinity to logical CPU 2 (change as needed)
	err := affinity.SetAffinity(2)
	if err != nil {
		log.Fatalf("Failed to set CPU affinity: %v", err)
	}
	fmt.Println("CPU affinity set to logical CPU 2")

	// Initialize BytePool with NUMA-aware allocation on NUMA node 0 (default config)
	const bufSize = 8192 // 8KB buffers
	bytePool := pool.NewBytePool(bufSize, 0, true)
	fmt.Println("NUMA-aware BytePool initialized on node 0, buffer size:", bufSize)

	// Acquire a buffer from the NUMA pool
	buffer := bytePool.GetBuffer()
	defer bytePool.PutBuffer(buffer)

	// Use buffer (for demonstration fill and print slice len/cap)
	for i := range buffer {
		buffer[i] = byte(i % 256)
	}
	fmt.Printf("Buffer acquired: len=%d cap=%d example[0]=%d\n", len(buffer), cap(buffer), buffer[0])

	// Stress-test: allocate, use and free multiple buffers
	const count = 10000
	buffers := make([][]byte, count)
	for i := range buffers {
		buffers[i] = bytePool.GetBuffer()
		buffers[i][0] = 0xAA
	}
	for i := range buffers {
		bytePool.PutBuffer(buffers[i])
	}

	fmt.Println("Stress-test completed: 10,000 NUMA-allocated buffers allocated and freed")
}
