// File: examples/lowlevel/broadcast/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Broadcast WebSocket Server Example using the hioload-ws `/server` facade.
// Demonstrates zero-copy, NUMA-aware, lock-free batch-IO, and CPU/NUMA affinity.
// Outputs live metrics (connections, messages per second) to console.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/core/protocol"
	"github.com/momentics/hioload-ws/lowlevel/server"
)

func main() {
	// 1. Parse flags
	addr := flag.String("addr", ":9002", "WebSocket listen address")
	batch := flag.Int("batch", 64, "Reactor batch size")
	ring := flag.Int("ring", 2048, "Reactor ring capacity")
	workers := flag.Int("workers", 0, "Executor worker count (0 = num CPUs)")
	numa := flag.Int("numa", -1, "Preferred NUMA node (-1 = auto)")
	flag.Parse()

	// 2. Configure server
	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.BatchSize = *batch
	cfg.ReactorRing = *ring
	if *workers > 0 {
		cfg.ExecutorWorkers = *workers
	}
	cfg.NUMANode = *numa

	// 3. New Server with logging & recovery middleware
	srv, err := server.NewServer(cfg,
		server.WithMiddleware(adapters.LoggingMiddleware),
		server.WithMiddleware(adapters.RecoveryMiddleware),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Starting WS Broadcast Server on ", *addr)

	// 4. Global broadcast pool: thread-safe set of connections
	var (
		conns     = make(map[*protocol.WSConnection]struct{})
		connsLock sync.RWMutex
		totalMsgs int64
	)

	// 5. Register debug probes
	ctrl := srv.GetControl()
	ctrl.RegisterDebugProbe("connections", func() any {
		connsLock.RLock()
		n := len(conns)
		connsLock.RUnlock()
		return n
	})
	ctrl.RegisterDebugProbe("total_messages", func() any {
		return atomic.LoadInt64(&totalMsgs)
	})

	// 6. Stats ticker
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := ctrl.Stats()
			fmt.Printf("[%s] Conns: %v, Msgs: %v\n",
				time.Now().Format(time.Stamp),
				stats["connections"],
				stats["total_messages"],
			)
		}
	}()

	// 7. Broadcast handler: on message, echo to all connections except sender
	handler := adapters.HandlerFunc(func(data any) error {
		// Extract buffer and connection from context
		buf := data.(api.Buffer)
		defer buf.Release() // return buffer to pool

		payload := buf.Bytes()
		atomic.AddInt64(&totalMsgs, 1)

		// Get sender
		connAny := api.ContextFromData(data).Value("connection")
		sender := connAny.(*protocol.WSConnection)

		// Copy payload once for broadcast
		bcast := make([]byte, len(payload))
		copy(bcast, payload)

		// Broadcast to all connections
		connsLock.RLock()
		for conn := range conns {
			if conn == sender {
				continue
			}
			// Zero-copy send: wrap in WSFrame
			frame := &protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(len(bcast)),
				Payload:    bcast,
			}
			conn.SendFrame(frame)
		}
		connsLock.RUnlock()
		return nil
	})

	// 8. Connection lifecycle middleware to track conns
	track := func(next api.Handler) api.Handler {
		return adapters.HandlerFunc(func(data any) error {
			// On new conn Open event: add to pool
			if evt, ok := data.(api.OpenEvent); ok {
				ws := evt.Conn.(*protocol.WSConnection)
				connsLock.Lock()
				conns[ws] = struct{}{}
				connsLock.Unlock()
			}
			// On CloseEvent: remove from pool
			if evt, ok := data.(api.CloseEvent); ok {
				ws := evt.Conn.(*protocol.WSConnection)
				connsLock.Lock()
				delete(conns, ws)
				connsLock.Unlock()
			}
			return next.Handle(data)
		})
	}

	// 9. Run server with chained handler: tracking → broadcast
	go func() {
		if err := srv.Run(track(handler)); err != nil {
			fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
			os.Exit(1)
		}
	}()

	// 10. Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("Shutting down broadcast server...")
	srv.Shutdown()
	fmt.Println("Server stopped.")
}
