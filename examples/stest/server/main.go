// File: examples/stest/server/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// resptest-server: High-performance echo server using hioload-ws.
// This example uses the facade.CreateWebSocketListener API to accept
// WebSocket connections and perform zero-copy echo with per-second metrics.

package main

import (
	"flag"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/server"
)

func main() {
	// Command-line flag: address to listen on
	addr := flag.String("addr", ":9000", "WebSocket listen address")
	flag.Parse()

	// Initialize server with default config and override listen address
	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr

	// Create the HioloadWS server instance
	hioload, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create HioloadWS: %v", err)
	}

	// Start server subsystems (affinity, metrics, etc.)
	if err := hioload.Start(); err != nil {
		log.Fatalf("Failed to start HioloadWS: %v", err)
	}
	defer hioload.Stop()

	// Metrics counters
	var connCount int64
	var rpsCount int64

	// Register debug probes for external metric scraping
	hioload.GetControl().RegisterDebugProbe("connections", func() any {
		return atomic.LoadInt64(&connCount)
	})
	hioload.GetControl().RegisterDebugProbe("rps", func() any {
		return atomic.LoadInt64(&rpsCount)
	})

	// Console reporter: prints metrics each second
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fmt.Printf("Connections=%d RPS=%d\n",
				atomic.LoadInt64(&connCount),
				atomic.SwapInt64(&rpsCount, 0),
			)
		}
	}()

	// Create native WebSocket listener via facade API
	listener, err := hioload.CreateWebSocketListener(cfg.ListenAddr)
	if err != nil {
		log.Fatalf("Failed to create WebSocket listener: %v", err)
	}
	defer listener.Close()

	// Accept loop: each connection is handled in a separate goroutine
	for {
		wsConn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		atomic.AddInt64(&connCount, 1)

		go func(conn *protocol.WSConnection) {
			defer conn.Close()
			defer atomic.AddInt64(&connCount, -1)

			// Process incoming frames zero-copy
			for {
				bufs, err := conn.RecvZeroCopy()
				if err != nil {
					return
				}

				// Echo each buffer back using zero-copy framing
				for _, buf := range bufs {
					frame := &protocol.WSFrame{
						IsFinal:    true,
						Opcode:     protocol.OpcodeBinary,
						PayloadLen: int64(len(buf.Bytes())),
						Payload:    buf.Bytes(),
					}
					if err := conn.SendFrame(frame); err != nil {
						buf.Release()
						return
					}
					// Count incoming frames as RPS
					atomic.AddInt64(&rpsCount, int64(len(bufs)))
					buf.Release()
				}
			}
		}(wsConn)
	}
}
