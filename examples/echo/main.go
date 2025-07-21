// File: examples/echo/main.go
// Package main
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Native WebSocket Echo server using hioload-ws without HTTP dependency.
// This demonstrates true zero-copy, NUMA-aware, high-performance WebSocket handling.

package main

import (
	"flag"
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
	// CLI flags
	addr := flag.String("addr", ":8080", "WebSocket listen address")
	useDPDK := flag.Bool("dpdk", false, "enable DPDK transport")
	shards := flag.Int("shards", 16, "number of session shards")
	workers := flag.Int("workers", 4, "number of executor workers")
	batchSize := flag.Int("batch", 16, "poller batch size")
	ringCap := flag.Int("ring", 1024, "poller ring capacity")
	numaNode := flag.Int("numa", -1, "preferred NUMA node")
	flag.Parse()

	// Build facade configuration
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.UseDPDK = *useDPDK
	cfg.SessionShards = *shards
	cfg.NumWorkers = *workers
	cfg.BatchSize = *batchSize
	cfg.RingCapacity = *ringCap
	cfg.NUMANode = *numaNode

	// Create HioloadWS facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}

	// Register debug probe for active connections
	var connectionCount int32
	hioload.GetControl().RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt32(&connectionCount)
	})

	// Start core services
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	log.Printf("Echo server starting on %s (DPDK=%v, NUMA=%d)",
		cfg.ListenAddr, cfg.UseDPDK, cfg.NUMANode)

	// Get buffer pool and channel size for WS listener
	bufPool := hioload.GetBufferPool()
	listener, err := transport.NewWebSocketListener(*addr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("failed to create WebSocket listener: %v", err)
	}
	defer listener.Close()

	log.Printf("Native WebSocket server listening on %s", *addr)

	// Connection handler loop
	go func() {
		for {
			wsConn, err := listener.Accept()
			if err != nil {
				log.Printf("failed to accept WebSocket connection: %v", err)
				continue
			}

			atomic.AddInt32(&connectionCount, 1)

			// Echo logic via handler
			echoHandler := adapters.HandlerFunc(func(data any) error {
				switch d := data.(type) {
				case []byte:
					return wsConn.Send(&protocol.WSFrame{
						IsFinal:    true,
						Opcode:     protocol.OpcodeBinary,
						PayloadLen: int64(len(d)),
						Payload:    d,
					})
				case string:
					b := []byte(d)
					return wsConn.Send(&protocol.WSFrame{
						IsFinal:    true,
						Opcode:     protocol.OpcodeText,
						PayloadLen: int64(len(b)),
						Payload:    b,
					})
				}
				return nil
			})

			// Middleware wrapping
			middleware := adapters.NewMiddlewareHandler(echoHandler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			wsConn.SetHandler(middleware)

			// Launch handler goroutine
			go func(conn *protocol.WSConnection) {
				defer func() {
					atomic.AddInt32(&connectionCount, -1)
					conn.Close()
				}()
				conn.Start()
				<-conn.Done()
			}(wsConn)
		}
	}()

	// Shutdown on signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down native WebSocket echo server...")
	listener.Close()
	if err := hioload.Stop(); err != nil {
		log.Printf("error stopping HioloadWS: %v", err)
	}
	log.Println("Server stopped gracefully.")
}
