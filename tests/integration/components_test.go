// Package integration tests the interaction between multiple components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package integration

import (
	"testing"
	"time"

	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/tests/fake"
)

// TestBufferPoolProtocolIntegration tests integration between buffer pool and protocol.
func TestBufferPoolProtocolIntegration(t *testing.T) {
	nodeCount := 1
	manager := pool.NewBufferPoolManager(nodeCount)
	bufPool := manager.GetPool(1024, 0)
	
	// Create a fake transport for testing
	fakeTransport := fake.NewFakeTransport()
	
	// Create a WSConnection with the buffer pool
	conn := protocol.NewWSConnection(fakeTransport, bufPool, 16)

	// Start the connection loops
	conn.Start()

	// Test that the connection can handle frames properly with pooled buffers
	payload := []byte("test message")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeBinary,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}

	// Send a frame and verify it gets to transport
	err := conn.SendFrame(frame)
	if err != nil {
		t.Fatalf("Failed to send frame: %v", err)
	}

	// Give time for async processing
	time.Sleep(10 * time.Millisecond)

	// Verify the transport received the encoded frame
	if len(fakeTransport.SendCalls) == 0 {
		t.Error("Expected transport to receive encoded frame")
	}

	conn.Close()
}

// TestProtocolTransportIntegration tests the protocol working with transport.
func TestProtocolTransportIntegration(t *testing.T) {
	// Create fake transport that returns a specific frame
	fakeTransport := fake.NewFakeTransport()
	
	// Use a real buffer pool
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)
	
	// Create connection
	conn := protocol.NewWSConnection(fakeTransport, bufPool, 16)
	
	// Create a frame and encode it
	payload := []byte("integration test")
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     protocol.OpcodeText,
		PayloadLen: int64(len(payload)),
		Payload:    payload,
		Masked:     false,
	}
	
	encoded, err := protocol.EncodeFrameToBytes(frame)
	if err != nil {
		t.Fatalf("Failed to encode frame: %v", err)
	}
	
	// Set up the fake transport to return our encoded frame
	fakeTransport.RecvFunc = func() ([][]byte, error) {
		return [][]byte{encoded}, nil
	}
	
	// Test receiving with zero-copy
	buffers, err := conn.RecvZeroCopy()
	if err != nil {
		t.Fatalf("Failed to receive with zero copy: %v", err)
	}
	
	if len(buffers) == 0 {
		t.Fatal("Expected at least one buffer from RecvZeroCopy")
	}
	
	recvBuf := buffers[0]
	if string(recvBuf.Bytes()[:len(payload)]) != string(payload) {
		t.Errorf("Expected received buffer to contain '%s', got '%s'", 
			string(payload), string(recvBuf.Bytes()[:len(payload)]))
	}
	
	conn.Close()
	// Release the received buffer
	for _, buf := range buffers {
		buf.Release()
	}
}

// TestBufferPoolSlabIntegration tests integration between buffer pool manager and slab pools.
func TestBufferPoolSlabIntegration(t *testing.T) {
	nodeCount := 2
	manager := pool.NewBufferPoolManager(nodeCount)
	
	// Get pools for different NUMA nodes
	pool0 := manager.GetPool(2048, 0) // 2KB
	pool1 := manager.GetPool(4096, 1) // 4KB
	
	if pool0 == nil || pool1 == nil {
		t.Fatal("Failed to get pools from manager")
	}
	
	// Get buffers from different pools
	buf0 := pool0.Get(2000, 0) // Should fit in 2KB class
	buf1 := pool1.Get(4000, 1) // Should fit in 4KB class
	
	if buf0 == nil || buf1 == nil {
		t.Fatal("Failed to get buffers from pools")
	}
	
	// Verify buffer sizes are appropriate
	if len(buf0.Bytes()) < 2000 {
		t.Errorf("Buffer 0 size %d is less than requested 2000", len(buf0.Bytes()))
	}
	
	if len(buf1.Bytes()) < 4000 {
		t.Errorf("Buffer 1 size %d is less than requested 4000", len(buf1.Bytes()))
	}
	
	// Verify NUMA nodes are preserved
	if buf0.NUMANode() != 0 {
		t.Errorf("Expected buffer 0 NUMA node 0, got %d", buf0.NUMANode())
	}
	
	if buf1.NUMANode() != 1 {
		t.Errorf("Expected buffer 1 NUMA node 1, got %d", buf1.NUMANode())
	}
	
	// Test releasing back to correct pools
	buf0.Release()
	buf1.Release()
	
	// Verify stats after allocations and releases
	stats0 := pool0.Stats()
	stats1 := pool1.Stats()
	
	if stats0.TotalAlloc <= 0 {
		t.Error("Expected pool 0 to have allocations")
	}
	
	if stats1.TotalAlloc <= 0 {
		t.Error("Expected pool 1 to have allocations")
	}
}

// TestHandlerBufferIntegration tests integration between handlers and buffers.
func TestHandlerBufferIntegration(t *testing.T) {
	manager := pool.NewBufferPoolManager(1)
	bufPool := manager.GetPool(1024, 0)
	
	// Create a fake handler to receive buffers
	receivedBuffer := make(chan []byte, 1)
	testHandler := fake.NewFakeHandler()
	testHandler.HandleFunc = func(data any) error {
		if buf, ok := data.(interface{ Bytes() []byte }); ok {
			receivedBuffer <- buf.Bytes()
		}
		return nil
	}
	
	// Create some test data
	testData := []byte("handler integration test")
	
	// Create a buffer with this data
	buf := bufPool.Get(len(testData), 0)
	copy(buf.Bytes(), testData)
	
	// Process the buffer through the handler
	testHandler.Handle(buf)
	
	// Verify the handler received the correct data
	select {
	case received := <-receivedBuffer:
		if string(received[:len(testData)]) != string(testData) {
			t.Errorf("Expected handler to receive '%s', got '%s'", 
				string(testData), string(received[:len(testData)]))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Handler did not receive buffer within timeout")
	}
	
	// Clean up
	buf.Release()
}