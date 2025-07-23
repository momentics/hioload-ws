// File: examples/echo/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-throughput WebSocket echo server built on hioload-ws.
// Demonstrates zero-copy buffer usage, poll-mode event loop, middleware chain, and graceful shutdown.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/protocol"
)

func main() {
	// CLI flag to pass WS listen address
	addr := flag.String("addr", ":9001", "WebSocket listen address")
	flag.Parse()

	// Build and customize facade config
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr

	// Create high-performance WebSocket server facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	defer hioload.Stop()

	// Track number of active connections and register debug probe
	var connCount int32
	hioload.GetControl().RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt32(&connCount)
	})

	log.Printf("[echo] Server listening on %s", cfg.ListenAddr)

	// Use updated facade method to get NUMA-aware buffer pool
	bufPool := hioload.GetBufferPool()

	// Construct zero-copy WebSocket listener on specified address
	listener, err := transport.NewWebSocketListener(cfg.ListenAddr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan struct{})

	// Launch accept loop to handle new incoming WebSocket connections
	go func() {
		defer close(acceptDone)
		for {
			wsConn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, transport.ErrListenerClosed) {
					log.Println("[echo] Listener closed, shutting down connection accept loop")
					return
				}
				log.Printf("[echo] accept error: %v", err)
				continue
			}

			// Generate a connection ID and increment connection counter
			id := fmt.Sprintf("conn-%d", atomic.AddInt32(&connCount, 1))
			log.Printf("[echo] Client connected: %s", id)

			// Echo handler sends back whatever it receives
			echoHandler := adapters.HandlerFunc(func(data any) error {
				buf, ok := data.([]byte)
				if !ok {
					return nil
				}
				return wsConn.SendFrame(&protocol.WSFrame{
					IsFinal:    true,
					Opcode:     protocol.OpcodeBinary,
					PayloadLen: int64(len(buf)),
					Payload:    buf,
				})
			})

			// Apply middleware chain: logging, panic recovery, metrics
			mw := adapters.NewMiddlewareHandler(echoHandler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			// Register handler to connection and start processing
			wsConn.SetHandler(mw)
			wsConn.Start()

			// Monitor disconnection
			go func(connID string) {
				<-wsConn.Done()
				log.Printf("[echo] Client disconnected: %s", connID)
				atomic.AddInt32(&connCount, -1)
			}(id)
		}
	}()

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[echo] Shutdown signal received. Closing listener…")
	listener.Close()
	<-acceptDone
	log.Println("[echo] Shutdown complete.")
}
