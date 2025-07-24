// File: examples/broadcast/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-performance WebSocket broadcast server using hioload-ws event-driven architecture.
// Demonstrates NUMA-aware broadcast to all connected clients with zero-copy frames and handler chaining.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/server"
)

// BroadcastRegistry stores all currently connected clients for broadcasting.
type BroadcastRegistry struct {
	mu      sync.RWMutex
	clients map[*protocol.WSConnection]string
	count   int32
}

func NewBroadcastRegistry() *BroadcastRegistry {
	return &BroadcastRegistry{
		clients: make(map[*protocol.WSConnection]string),
	}
}

func (r *BroadcastRegistry) Add(conn *protocol.WSConnection, clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[conn] = clientID
	atomic.AddInt32(&r.count, 1)
	fmt.Printf("[registry] Client connected: %s\n", clientID)
}

func (r *BroadcastRegistry) Remove(conn *protocol.WSConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if clientID, ok := r.clients[conn]; ok {
		delete(r.clients, conn)
		atomic.AddInt32(&r.count, -1)
		fmt.Printf("[registry] Client disconnected: %s\n", clientID)
	}
}

func (r *BroadcastRegistry) Broadcast(sender *protocol.WSConnection, data []byte, pool api.BufferPool) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sent := 0
	for client, name := range r.clients {
		if client == sender {
			continue
		}

		buf := pool.Get(len(data), -1)
		copy(buf.Bytes(), data)

		frame := &protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: int64(len(data)),
			Payload:    buf.Bytes(),
		}

		if err := client.SendFrame(frame); err != nil {
			log.Printf("[broadcast] send error to %s: %v", name, err)
		} else {
			sent++
		}

		buf.Release()
	}
	return sent
}

func (r *BroadcastRegistry) Size() int {
	return int(atomic.LoadInt32(&r.count))
}

func main() {
	addr := flag.String("addr", ":9002", "WebSocket listen address")
	flag.Parse()

	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr

	hioload, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize hioload-ws: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("start failed: %v", err)
	}
	defer hioload.Stop()

	log.Printf("[broadcast] Server running on %s", cfg.ListenAddr)

	registry := NewBroadcastRegistry()
	hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
		return registry.Size()
	})

	bufPool := hioload.GetBufferPool() // Updated accessor

	listener, err := transport.NewWebSocketListener(cfg.ListenAddr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan struct{})

	// Handle new connections and assign handler to broadcast
	go func() {
		defer close(acceptDone)
		var connCounter int32

		for {
			wsConn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, transport.ErrListenerClosed) {
					log.Println("[broadcast] Listener closed, exit loop")
					return
				}
				log.Printf("[broadcast] Accept error: %v", err)
				continue
			}

			id := fmt.Sprintf("client-%d", atomic.AddInt32(&connCounter, 1))
			registry.Add(wsConn, id)

			// Define the broadcast logic: forward message to others
			handler := adapters.HandlerFunc(func(data any) error {
				msg, ok := data.([]byte)
				if !ok {
					return nil
				}
				count := registry.Broadcast(wsConn, msg, bufPool)
				log.Printf("[broadcast] %s → %d recipients", id, count)
				return nil
			})

			mw := adapters.NewMiddlewareHandler(handler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			wsConn.SetHandler(mw)
			wsConn.Start()

			go func(conn *protocol.WSConnection, cid string) {
				<-conn.Done()
				registry.Remove(conn)
				log.Printf("[disconnect] %s disconnected", cid)
			}(wsConn, id)
		}
	}()

	// Graceful shutdown handler
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[broadcast] Shutting down...")
	listener.Close()
	<-acceptDone
	log.Println("[broadcast] Cleanup complete.")
}
