// File: examples/echo/main.go
// Package main
// Native WebSocket Echo server using hioload-ws without HTTP dependency.
// Demonstrates true zero-copy, NUMA-aware, high-performance WebSocket handling.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

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
	addr := flag.String("addr", ":9001", "WebSocket listen address")
	flag.Parse()

	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr

	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	defer hioload.Stop()

	var connCount int32
	hioload.GetControl().RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt32(&connCount)
	})

	log.Printf("Echo server listening on %s", cfg.ListenAddr)

	listener, err := transport.NewWebSocketListener(cfg.ListenAddr, hioload.GetBufferPool(), cfg.ChannelSize)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan struct{})

	go func() {
		defer close(acceptDone)
		for {
			wsConn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, transport.ErrListenerClosed) {
					log.Println("Listener closed, exiting accept loop")
					return
				}
				log.Printf("accept error: %v", err)
				continue
			}

			id := fmt.Sprintf("conn-%d", atomic.AddInt32(&connCount, 1))
			log.Printf("Client connected: %s", id)

			// Prepare handler
			echoHandler := adapters.HandlerFunc(func(data any) error {
				buf, ok := data.([]byte)
				if !ok {
					return nil
				}
				return wsConn.Send(&protocol.WSFrame{
					IsFinal:    true,
					Opcode:     protocol.OpcodeBinary,
					PayloadLen: int64(len(buf)),
					Payload:    buf,
				})
			})
			mw := adapters.NewMiddlewareHandler(echoHandler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			wsConn.SetHandler(mw)
			wsConn.Start()

			go func(connID string) {
				<-wsConn.Done()
				log.Printf("Client disconnected: %s", connID)
				atomic.AddInt32(&connCount, -1)
			}(id)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutdown signal received, closing listener")

	listener.Close()
	<-acceptDone

	log.Println("Server shutdown complete")
}
