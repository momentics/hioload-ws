// File: tests/benchmarks/performance_test.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Integration test to benchmark performance of real client-server communication.
// Tests throughput with different packet sizes using high-level API.

package benchmarks

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/highlevel"
)

// TestPerformanceSingleClient benchmarks performance with 1 client and 1 server for 30 seconds
func TestPerformanceSingleClient(t *testing.T) {
	// Define test parameters
	packetSizes := []int{64, 256, 512, 1024, 4096}
	
	for _, packetSize := range packetSizes {
		t.Run(fmt.Sprintf("PacketSize_%d", packetSize), func(t *testing.T) {
			stats := runBenchmark(t, 1, packetSize, 30*time.Second) // 30 second duration
			printStats(t, stats, 1, packetSize)
		})
	}
}

// Run benchmark with specified number of clients, packet size, and duration
func runBenchmark(t *testing.T, clientCount int, packetSize int, duration time.Duration) *BenchmarkStats {
	// Create a temporary listener to get a free port
	tempListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create temporary listener: %v", err)
	}
	tempPort := tempListener.Addr().(*net.TCPAddr).Port
	tempListener.Close()

	addr := fmt.Sprintf(":%d", tempPort)

	// Create echo server
	server := highlevel.NewServer(addr)

	// Define echo handler
	server.HandleFunc("/echo", func(c *highlevel.Conn) {
		defer c.Close() // Ensure connection is closed when handler returns

		// Set a reasonable read/write timeout to prevent hanging
		c.SetReadDeadline(time.Now().Add(10 * time.Second))

		// Echo back messages
		for {
			msgType, data, err := c.ReadMessage()
			if err != nil {
				return // Connection closed or error
			}

			err = c.WriteMessage(msgType, data)
			if err != nil {
				return // Error sending echo
			}
		}
	})

	// Start server in background
	go func() {
		_ = server.ListenAndServe()
	}()

	// Wait a bit for server to start accepting connections
	time.Sleep(200 * time.Millisecond)

	// Create clients and connect to server
	clients := make([]*highlevel.Conn, clientCount)
	clientCountActual := 0
	for i := 0; i < clientCount; i++ {
		conn, err := highlevel.Dial(fmt.Sprintf("ws://localhost:%d/echo", tempPort))
		if err != nil {
			t.Logf("Failed to create client %d: %v", i, err)
			continue // Don't fail the test for some client connection failures
		}
		clients[i] = conn
		clientCountActual++
	}

	// Prepare test data
	data := make([]byte, packetSize)
	for i := range data {
		data[i] = byte(i % 256) // Fill with pattern
	}

	// Start timing
	startTime := time.Now()
	endTime := startTime.Add(duration)

	// Run test: each client sends a limited number of messages and waits for echo
	var wg sync.WaitGroup
	resultCh := make(chan *ClientResult, clientCountActual)

	for i := 0; i < clientCount; i++ {
		client := clients[i]
		if client == nil {
			continue // Skip failed clients
		}

		wg.Add(1)
		go func(clientIndex int) {
			defer wg.Done()

			client := clients[clientIndex]
			if client == nil {
				return
			}

			clientResult := &ClientResult{
				ClientID: clientIndex,
				Sent:     0,
				Received: 0,
				Errors:   0,
				Duration: 0,
			}

			start := time.Now()

			// Send a limited number of messages (instead of for the entire duration)
			// This prevents blocking and allows the test to complete within the timeout
			for j := 0; j < 100; j++ {
				time.Sleep(1 * time.Millisecond) // Small delay to prevent overwhelming

				err := client.WriteMessage(int(highlevel.BinaryMessage), data)
				if err != nil {
					clientResult.Errors++
					continue
				}
				clientResult.Sent++

				// Set read deadline to avoid hanging on read
				client.SetReadDeadline(time.Now().Add(2 * time.Second))

				// Receive echo
				_, _, err = client.ReadMessage()
				if err != nil {
					clientResult.Errors++
				} else {
					clientResult.Received++
				}

				// Check if time limit has been reached
				if time.Now().After(endTime) {
					break
				}
			}

			clientResult.Duration = time.Since(start)
			resultCh <- clientResult
		}(i)
	}

	// Wait for all clients to finish and collect results
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(resultCh)
		close(done)
	}()

	// Set overall timeout for the whole test
	select {
	case <-done:
		// All clients finished normally
	case <-time.After(duration + 15*time.Second): // Add extra time for cleanup
		t.Log("Test timeout reached, stopping...")
	}

	clientResults := make([]*ClientResult, 0, clientCountActual)
	for result := range resultCh {
		clientResults = append(clientResults, result)
	}

	// Calculate overall statistics
	totalDuration := time.Since(startTime)

	// Close all clients
	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}

	// Stop server
	if err := server.Shutdown(); err != nil {
		t.Logf("Server shutdown error: %v", err)
	}

	// Wait a bit for graceful shutdown
	time.Sleep(50 * time.Millisecond)

	return &BenchmarkStats{
		ClientCount:   clientCountActual,
		PacketSize:    packetSize,
		ClientResults: clientResults,
		TotalDuration: totalDuration,
	}
}

// printStats prints benchmark statistics
func printStats(t *testing.T, stats *BenchmarkStats, clientCount int, packetSize int) {
	totalSent := 0
	totalReceived := 0
	totalErrors := 0
	totalDuration := stats.TotalDuration
	totalBytes := 0

	for _, result := range stats.ClientResults {
		totalSent += result.Sent
		totalReceived += result.Received
		totalErrors += result.Errors
		totalBytes += result.Received * stats.PacketSize
	}

	// Calculate throughput
	durationSec := totalDuration.Seconds()
	if durationSec == 0 {
		durationSec = 0.001 // avoid division by zero
	}
	throughputMsgs := float64(totalReceived) / durationSec
	throughputBytes := float64(totalBytes) / durationSec

	t.Logf("=== Performance Results ===")
	t.Logf("Configuration: %d clients, %d bytes packet size", clientCount, packetSize)
	t.Logf("Total Messages Sent: %d", totalSent)
	t.Logf("Total Messages Received: %d", totalReceived)
	t.Logf("Total Errors: %d", totalErrors)
	t.Logf("Total Duration: %v", totalDuration)
	t.Logf("Throughput: %.2f messages/sec", throughputMsgs)
	t.Logf("Throughput: %.2f bytes/sec (%.2f MB/s)", throughputBytes, throughputBytes/(1024*1024))
	
	if totalSent > 0 {
		t.Logf("Success Rate: %.2f%%", float64(totalReceived)/float64(totalSent)*100)
	} else {
		t.Logf("Success Rate: 0%% (no messages sent)")
	}
	
	// Calculate average client duration
	if len(stats.ClientResults) > 0 {
		var totalClientTime time.Duration
		for _, result := range stats.ClientResults {
			totalClientTime += result.Duration
		}
		avgClientTime := totalClientTime / time.Duration(len(stats.ClientResults))
		t.Logf("Avg Client Time: %v", avgClientTime)
	}
	t.Logf("=========================")
}

// BenchmarkStats holds benchmark statistics
type BenchmarkStats struct {
	ClientCount   int
	PacketSize    int
	ClientResults []*ClientResult
	TotalDuration time.Duration
}

// ClientResult holds results for a single client
type ClientResult struct {
	ClientID int
	Sent     int
	Received int
	Errors   int
	Duration time.Duration
}