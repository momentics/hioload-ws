// File: examples/lowlevel/echo/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Echo WebSocket Server Example using hioload-ws `/server` facade.
// Demonstrates facade pattern, middleware chain, batch IO via Poller,
// and zero-copy buffer usage with NUMA-aware buffer pools.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/core/protocol"
	"github.com/momentics/hioload-ws/lowlevel/server"
)

func main() {
	// Parse flags
	addr := flag.String("addr", ":9001", "WebSocket listen address")
	batchSize := flag.Int("batch", 32, "Reactor batch size")
	ringCap := flag.Int("ring", 1024, "Reactor ring capacity")
	workers := flag.Int("workers", 0, "Executor worker count (0 = NumCPU)")
	numa := flag.Int("numa", -1, "Preferred NUMA node (-1 = auto)")
	flag.Parse()

	// Build server configuration
	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.BatchSize = *batchSize
	cfg.ReactorRing = *ringCap
	if *workers > 0 {
		cfg.ExecutorWorkers = *workers
	}
	cfg.NUMANode = *numa

	// Initialize server facade with middleware
	srv, err := server.NewServer(
		cfg,
		server.WithMiddleware(
			adapters.LoggingMiddleware,
			adapters.RecoveryMiddleware,
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
		os.Exit(1)
	}

	srv.UseMiddleware(
		adapters.HandlerMetricsMiddleware(srv.GetControl()), // increments handler.processed
	)

	fmt.Println("Starting WS Echo Server on ", *addr)

	// Register debug probes for console metrics
	var activeConns int64 = 0
	var totalMsgs int64 = 0

	ctrl := srv.GetControl()
	ctrl.RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt64(&activeConns)
	})
	ctrl.RegisterDebugProbe("messages_processed", func() any {
		return atomic.LoadInt64(&totalMsgs)
	})

	// Periodic stats output
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			stats := ctrl.Stats()
			fmt.Printf("[%s] Active: %v, Msgs: %v\n",
				time.Now().Format(time.Stamp),
				stats["active_connections"],
				stats["messages_processed"],
			)
		}
	}()

	echoHandler := adapters.HandlerFunc(func(data any) error {
		buf := data.(api.Buffer)
		atomic.AddInt64(&totalMsgs, 1)

		// Получаем WSConnection из контекста
		conn := api.FromContext(api.ContextFromData(data)).(*protocol.WSConnection)

		// Строим и отправляем фрейм через протокол
		frame := &protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: int64(len(buf.Bytes())),
			Payload:    buf.Bytes(),
		}
		err := conn.SendFrame(frame)

		buf.Release()
		return err
	})

	// Run server with echo handler
	go func() {
		if err := srv.Run(echoHandler); err != nil {
			fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
			os.Exit(1)
		}
	}()

	// Update active connections via Connection middleware
	track := func(next api.Handler) api.Handler {
		return adapters.HandlerFunc(func(data any) error {
			// OpenEvent increments, CloseEvent decrements
			switch evt := data.(type) {
			case api.OpenEvent:
				atomic.AddInt64(&activeConns, 1)
				defer func() { _ = next.Handle(evt) }()
			case api.CloseEvent:
				atomic.AddInt64(&activeConns, -1)
				defer func() { _ = next.Handle(evt) }()
			default:
				return next.Handle(data)
			}
			return nil
		})
	}
	srv.UseMiddleware(track) // attach tracking

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("Shutting down echo server...")
	srv.Shutdown()
	fmt.Println("Server stopped.")
}
