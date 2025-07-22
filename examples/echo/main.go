// File: examples/echo/main.go
// Package main
// Native WebSocket Echo server using hioload-ws without HTTP dependency.
// Demonstrates true zero-copy, NUMA-aware, high-performance WebSocket handling.
// This version uses sensible defaults from facade.DefaultConfig() and allows
// overriding only the address via flag. English comments explain key parts.

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/protocol"
)

func main() {
	// Allow overriding only the listen address; other parameters come from defaults.
	addr := flag.String("addr", "", "WebSocket listen address (default from config)")
	flag.Parse()

	// Build facade configuration with defaults.
	cfg := facade.DefaultConfig()
	if *addr != "" {
		cfg.ListenAddr = *addr
	}
	// Initialize facade.
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	defer hioload.Stop()

	// Register debug probe for active connections.
	var connectionCount int32
	hioload.GetControl().RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt32(&connectionCount)
	})

	log.Printf("Echo server starting on %s", cfg.ListenAddr)

	// Setup native WebSocket listener.
	bufPool := hioload.GetBufferPool()
	listener, err := transport.NewWebSocketListener(cfg.ListenAddr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Scheduler for heartbeat and shutdown timers.
	scheduler := hioload.GetScheduler()
	cfgMap := hioload.GetControl().GetConfig()
	hbInterval := cfgMap["heartbeat_interval"].(int64)
	shutdownTimeout := cfgMap["shutdown_timeout"].(int64)

	go func() {
		for {
			wsConn, err := listener.Accept()
			if err != nil {
				log.Printf("accept error: %v", err)
				continue
			}
			atomic.AddInt32(&connectionCount, 1)

			// Echo handler.
			echoHandler := adapters.HandlerFunc(func(data any) error {
				switch d := data.(type) {
				case []byte:
					return wsConn.Send(&protocol.WSFrame{
						IsFinal:    true,
						Opcode:     protocol.OpcodeBinary,
						PayloadLen: int64(len(d)),
						Payload:    d,
					})
				}
				return nil
			})
			mw := adapters.NewMiddlewareHandler(echoHandler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))
			wsConn.SetHandler(mw)
			wsConn.Start()

			// Heartbeat based on configured interval.
			scheduler.Schedule(hbInterval, func() {
				wsConn.Send(&protocol.WSFrame{
					IsFinal:    true,
					Opcode:     protocol.OpcodePing,
					PayloadLen: 0,
				})
			})

			// Decrement count on close.
			go func() {
				<-wsConn.Done()
				atomic.AddInt32(&connectionCount, -1)
			}()
		}
	}()

	// Handle shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutdown signal received, initiating graceful shutdown")

	// Schedule force exit after configured timeout.
	shutdownDone := make(chan struct{})
	scheduler.Schedule(shutdownTimeout, func() {
		log.Println("Graceful shutdown timed out, forcing exit")
		close(shutdownDone)
	})

	listener.Close()

	// Wait for either all connections to close or timeout.
	select {
	case <-shutdownDone:
	case <-time.After(time.Duration(shutdownTimeout)):
	}

	log.Println("Server exited")
}
