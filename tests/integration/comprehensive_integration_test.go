package integration

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/tests/fake"
)

// TestBufferPoolProtocolIntegration tests integration between buffer pool and protocol
func TestBufferPoolProtocolIntegration(t *testing.T) {
	// Verify that buffer pools and protocol components work together
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)

	// Create a fake transport for testing
	fakeTransport := fake.NewFakeTransport()

	// Create a WSConnection with the buffer pool
	conn := protocol.NewWSConnection(fakeTransport, bufPool, 16)
	conn.Start()
	defer conn.Close()

	// Test that the connection can handle frames properly with pooled buffers
	payload := []byte("integration test message")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}

	// Send a frame and track successful operations
	sendErrors := 0
	successfulSends := 0

	err := conn.SendFrame(frame)
	if err != nil {
		sendErrors++
		t.Errorf("Failed to send frame: %v", err)
	} else {
		successfulSends++
		t.Logf("Successfully sent frame with %d bytes payload", len(payload))
	}

	// Verify success rate
	if successfulSends > 0 && sendErrors == 0 {
		successRate := float64(successfulSends) / float64(successfulSends+sendErrors) * 100
		if successRate != 100.0 {
			t.Errorf("Expected 100%% success rate, got %.2f%%", successRate)
		} else {
			t.Logf("Buffer pool protocol integration: 100%% success rate (%d/%d operations)", successfulSends, successfulSends)
		}
	} else if sendErrors > 0 {
		successRate := float64(successfulSends) / float64(successfulSends+sendErrors) * 100
		t.Errorf("Buffer pool protocol integration: low success rate of %.2f%% (%d successful, %d failed)", successRate, successfulSends, sendErrors)
	}
}

// TestProtocolTransportIntegration tests protocol working with transport
func TestProtocolTransportIntegration(t *testing.T) {
	// Create fake transport that simulates real transport behavior
	fakeTransport := fake.NewFakeTransport()

	// Use a real buffer pool
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)

	// Create connection
	conn := protocol.NewWSConnection(fakeTransport, bufPool, 16)
	conn.Start()
	defer conn.Close()

	// Create test frame
	payload := []byte("integration test")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeText,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}

	// Test send operation and measure success
	startTime := time.Now()

	sendErr := conn.SendFrame(frame)
	elapsed := time.Since(startTime)

	if sendErr != nil {
		t.Errorf("Protocol-transport integration failed to send frame: %v", sendErr)
	} else {
		t.Logf("Protocol-transport integration successful: sent %d bytes in %v", len(payload), elapsed)
	}

	// Add success rate tracking
	totalOps := 1
	errors := 0
	if sendErr != nil {
		errors = 1
	}

	successRate := float64(totalOps-errors) / float64(totalOps) * 100
	if successRate != 100 {
		t.Errorf("Protocol-transport integration success rate: %.2f%% (wanted 100%%)", successRate)
	} else {
		t.Logf("Protocol-transport integration: 100%% success rate")
	}
}

// TestBufferPoolSlabIntegration tests buffer pool manager with slab allocation
func TestBufferPoolSlabIntegration(t *testing.T) {
	nodeCount := 2
	manager := pool.NewBufferPoolManager(nodeCount)

	// Test getting pools for different NUMA nodes
	pool0 := manager.GetPool(2048, 0)
	pool1 := manager.GetPool(4096, 1)

	if pool0 == nil || pool1 == nil {
		t.Fatal("Failed to get buffer pools from manager")
	}

	// Test buffer operations and track results
	totalAllocations := 0
	successfulAllocations := 0
	errorCount := 0

	// Test allocations for different sizes
	sizes := []int{100, 512, 1024, 2000}
	for _, size := range sizes {
		totalAllocations++

		// Get buffer from pool 0
		buf0 := pool0.Get(size, 0)
		if buf0.Data == nil {
			errorCount++
			t.Errorf("Failed to get buffer of size %d from pool 0", size)
		} else {
			successfulAllocations++

			// Verify buffer properties
			bufBytes := buf0.Bytes()
			if len(bufBytes) < size {
				t.Errorf("Buffer of requested size %d only has %d bytes", size, len(bufBytes))
			} else {
				t.Logf("Successfully allocated buffer of size %d with %d available bytes", size, len(bufBytes))
			}

			// Test NUMA preservation
			if buf0.NUMANode() != 0 {
				t.Errorf("Expected NUMA node 0, got %d", buf0.NUMANode())
			}

			buf0.Release()
		}

		// Get buffer from pool 1
		buf1 := pool1.Get(size, 1)
		if buf1.Data == nil {
			errorCount++
			t.Errorf("Failed to get buffer of size %d from pool 1", size)
		} else {
			successfulAllocations++

			// Test NUMA preservation
			if buf1.NUMANode() != 1 {
				t.Errorf("Expected NUMA node 1, got %d", buf1.NUMANode())
			}

			buf1.Release()
		}
	}

	// Calculate success rate - note that we have 2 pools (pool0 and pool1) so total successful should be 2x the number of sizes
	expectedSuccessful := len(sizes) * 2 // 2 pools per size
	successRate := float64(successfulAllocations) / float64(expectedSuccessful) * 100
	if successRate != 100.0 {
		t.Errorf("Buffer pool slab integration success rate: %.2f%% (failed %d out of %d allocations)", successRate, errorCount, expectedSuccessful)
	} else {
		t.Logf("Buffer pool slab integration: 100%% success rate (%d/%d allocations)", successfulAllocations, expectedSuccessful)
	}
}

// TestRealWorldScenario tests realistic usage patterns
func TestRealWorldScenario(t *testing.T) {
	// Test a realistic scenario with multiple operations
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(4096, 0)

	// Create multiple connections to simulate concurrent usage
	const numConnections = 3
	var connections []*protocol.WSConnection

	for i := 0; i < numConnections; i++ {
		fakeTransport := fake.NewFakeTransport()
		conn := protocol.NewWSConnection(fakeTransport, bufPool, 16)
		conn.Start()
		connections = append(connections, conn)
	}

	// Perform operations on each connection
	totalOperations := 0
	successfulOperations := 0
	errorOperations := 0

	for i, conn := range connections {
		// Send multiple frames
		for j := 0; j < 5; j++ {
			totalOperations++

			frame := &protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(64),
				Payload:    make([]byte, 64),
			}

			// Fill payload with pattern
			for k := range frame.Payload {
				frame.Payload[k] = byte((i*100 + j*10 + k) % 256)
			}

			err := conn.SendFrame(frame)
			if err != nil {
				errorOperations++
				t.Errorf("Connection %d, operation %d failed: %v", i, j, err)
			} else {
				successfulOperations++
			}
		}
	}

	// Calculate and verify success rate
	successRate := float64(successfulOperations) / float64(totalOperations) * 100
	t.Logf("Real world scenario: %d operations total, %d successful, %d errors", totalOperations, successfulOperations, errorOperations)
	t.Logf("Success rate: %.2f%%", successRate)

	if errorOperations > 0 {
		t.Errorf("Expected 0 errors, got %d errors", errorOperations)
	}

	if successRate != 100.0 {
		t.Errorf("Expected 100%% success rate, got %.2f%%", successRate)
	} else {
		t.Log("Real world scenario test passed with 100% success rate")
	}

	// Cleanup
	for _, conn := range connections {
		conn.Close()
	}
}

// TestConcurrentAccess tests concurrent buffer pool access
func TestConcurrentAccess(t *testing.T) {
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)

	// Create workers that use the same pool concurrently
	const numWorkers = 5
	const opsPerWorker = 10
	results := make(chan bool, numWorkers*opsPerWorker)

	for w := 0; w < numWorkers; w++ {
		go func(workerID int) {
			for op := 0; op < opsPerWorker; op++ {
				buf := bufPool.Get(256, 0)
				if buf.Data == nil {
					results <- false // Operation failed
				} else {
					// Do some work with buffer
					bufData := buf.Bytes()
					if len(bufData) >= 10 {
						copy(bufData[:10], []byte("test data"))
					}
					buf.Release()   // Important: release buffer back to pool
					results <- true // Operation succeeded
				}
			}
		}(w)
	}

	// Collect results
	totalOps := 0
	successfulOps := 0
	for i := 0; i < numWorkers*opsPerWorker; i++ {
		totalOps++
		if <-results {
			successfulOps++
		}
	}

	// Calculate success rate
	successRate := float64(successfulOps) / float64(totalOps) * 100
	errors := totalOps - successfulOps

	t.Logf("Concurrent access test: %d operations, %d successful, %d errors", totalOps, successfulOps, errors)
	t.Logf("Success rate: %.2f%%", successRate)

	if errors > 0 {
		t.Errorf("Concurrent access test had %d errors, expected 0", errors)
	}

	if successRate != 100.0 {
		t.Errorf("Expected 100%% success rate in concurrent access, got %.2f%%", successRate)
	} else {
		t.Log("Concurrent access test passed with 100% success rate")
	}
}
