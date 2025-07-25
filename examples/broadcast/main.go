// File: examples/broadcast/main.go
// High-performance zero-copy WebSocket broadcast server using hioload-ws server façade.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package main

import (
	"flag"
	"log"
	"sync"
	"sync/atomic"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/server"
)

func main() {
	// Command-line flag for listen address
	addr := flag.String("addr", ":9002", "WebSocket listen address")
	flag.Parse()

	// Create server configuration with default NUMA-aware buffer pooling
	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr

	// Initialize server facade
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("server.NewServer error: %v", err)
	}
	defer srv.Shutdown()

	// Registry of active connections
	var (
		mu      sync.RWMutex
		clients = make(map[*protocol.WSConnection]struct{})
		count   int32
	)

	// Expose active client count via Control API
	srv.GetControl().RegisterDebugProbe("active_clients", func() any {
		return atomic.LoadInt32(&count)
	})

	// Broadcast handler: logs message and broadcasts payload to all other clients
	handler := adapters.HandlerFunc(func(data any) error {
		buf := data.(api.Buffer)
		payload := buf.Bytes()
		log.Printf("Received message: %s", string(payload)) // print to console
		buf.Release()

		mu.RLock()
		defer mu.RUnlock()
		for conn := range clients {
			// For each other client, send a copy
			b2 := srv.GetBufferPool().Get(len(payload), -1)
			copy(b2.Bytes(), payload)
			frame := &protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(len(payload)),
				Payload:    b2.Bytes(),
			}
			conn.SendFrame(frame)
			b2.Release()
		}
		return nil
	})

	// Serve connections with our broadcast handler
	go func() {
		srv.Serve(handler)
	}()

	// Create low-level WebSocket listener
	listener, err := srv.CreateWebSocketListener()
	if err != nil {
		log.Fatalf("CreateWebSocketListener error: %v", err)
	}
	defer listener.Close()

	log.Printf("Broadcast server listening on %s", cfg.ListenAddr)

	// Accept loop
	for {
		wsConn, err := listener.Accept()
		if err != nil {
			if err.Error() == "listener closed" {
				break
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		// Register new connection
		mu.Lock()
		clients[wsConn] = struct{}{}
		atomic.AddInt32(&count, 1)
		mu.Unlock()
		log.Printf("Client connected, total=%d", atomic.LoadInt32(&count))

		// Start per-connection receive loop
		go func(c *protocol.WSConnection) {
			defer func() {
				c.Close()
				mu.Lock()
				delete(clients, c)
				atomic.AddInt32(&count, -1)
				mu.Unlock()
				log.Printf("Client disconnected, total=%d", atomic.LoadInt32(&count))
			}()

			for {
				bufs, err := c.RecvZeroCopy()
				if err != nil {
					return
				}
				for _, b := range bufs {
					handler.Handle(b)
				}
			}
		}(wsConn)
	}

	log.Println("Broadcast server shutdown complete")
}
