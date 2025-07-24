// File: examples/stest/client/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Test client for examples/stest/server:
// - Spawns up to N parallel WebSocket clients
// - Binds each outgoing socket to a unique local IP alias
// - Sends binary messages of configured length (e.g., 32 or 64 bytes) using protocol.WSFrame
// - Aggregates total connections and RPS, printing metrics every second
// - Uses functional option WithDialer to bind local IP addresses

package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/momentics/hioload-ws/client"
	"github.com/momentics/hioload-ws/protocol"
)

func main() {
	// Command-line flags for benchmark configuration
	addr := flag.String("addr", "localhost:9000", "server host:port")
	concurrency := flag.Int("concurrency", 1, "max parallel connections")
	payloadLen := flag.Int("payload", 32, "bytes per packet (e.g., 32 or 64)")
	aliasCount := flag.Int("aliases", 1, "number of source-IP aliases")
	pauseSec := flag.Int("pause", 1, "pause in seconds between send/recv cycles")
	flag.Parse()

	// Prepare local IP aliases for client binding (e.g., 127.0.0.2-127.0.0.129)
	base := net.ParseIP("127.0.0.1").To4()
	addrs := make([]net.IP, *aliasCount)
	for i := 0; i < *aliasCount; i++ {
		ip := make(net.IP, 4)
		copy(ip, base)
		ip[3] = byte(1 + i%254)
		addrs[i] = ip
	}

	var totalConns, totalCloses, rpsCount int64

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Metrics reporter goroutine
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for range t.C {
			conns := atomic.LoadInt64(&totalConns)
			closes := atomic.LoadInt64(&totalCloses)
			rps := atomic.SwapInt64(&rpsCount, 0)
			fmt.Printf("opened=%d closed=%d RPS=%d\n", conns, closes, rps)
		}
	}()

	// Spawn all client worker goroutines
	for i := 0; i < *concurrency; i++ {
		go worker(ctx, *addr, addrs, *payloadLen, time.Duration(*pauseSec)*time.Second,
			&totalConns, &totalCloses, &rpsCount)
	}

	<-ctx.Done()
	fmt.Println("Shutting down client...")
	time.Sleep(2 * time.Second) // Allow workers to exit gracefully
}

// worker establishes a WebSocketClient bound to a rotating local IP alias,
// sends/receives binary frames with pauses, and updates metrics.
func worker(
	ctx context.Context,
	serverAddr string,
	addrs []net.IP,
	payloadLen int,
	pause time.Duration,
	totalConns, totalCloses, rpsCount *int64,
) {
	// Select local IP alias in round-robin manner
	idx := atomic.AddUint64(new(uint64), 1)
	localIP := addrs[int(idx)%len(addrs)]

	// Create a custom net.Dialer with LocalAddr set
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: localIP},
		Timeout:   5 * time.Second,
	}

	// Prepare WebSocket client config (NUMANode=-1: auto, BatchSize=1 for echo)
	cfg := client.ClientConfig{
		Addr:              fmt.Sprintf("ws://%s", serverAddr),
		IOBufferSize:      payloadLen,
		BatchSize:         1,
		NUMANode:          -1,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		ReconnectMax:      1,
		HeartbeatInterval: 0,
	}

	// Pass the dialer as a functional option (requires WithDialer implementation)
	c, err := client.NewWebSocketClient(cfg, client.WithDialer(dialer))
	if err != nil {
		return
	}
	atomic.AddInt64(totalConns, 1)
	defer func() {
		c.Close()
		atomic.AddInt64(totalCloses, 1)
	}()

	// Prebuild fixed-size payload
	payload := make([]byte, payloadLen)
	binary.LittleEndian.PutUint32(payload, uint32(payloadLen))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Send binary frame
			err := c.SendFrame(&protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(payloadLen),
				Payload:    payload,
			})
			if err != nil {
				return
			}
			// Receive echo
			batch, err := c.RecvBatch()
			if err != nil {
				return
			}
			atomic.AddInt64(rpsCount, int64(len(batch)))
			time.Sleep(pause)
		}
	}
}
