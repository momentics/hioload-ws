// File: examples/broadcast/main.go
// Package main
// Native high-performance WebSocket broadcast (chat) server using hioload-ws only, no HTTP dependency.
// Demonstrates zero-copy broadcasting with NUMA-aware buffer pools, custom registry, and event-driven handler.
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
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/protocol"
)

// BroadcastRegistry manages a thread-safe set of active WebSocket clients.
type BroadcastRegistry struct {
	mu      sync.RWMutex
	clients map[*protocol.WSConnection]string
	count   int32
}

// NewBroadcastRegistry initializes an empty registry.
func NewBroadcastRegistry() *BroadcastRegistry {
	return &BroadcastRegistry{
		clients: make(map[*protocol.WSConnection]string),
	}
}

// Add registers a client connection with a unique ID.
func (r *BroadcastRegistry) Add(conn *protocol.WSConnection, clientID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[conn] = clientID
	total := atomic.AddInt32(&r.count, 1)
	log.Printf("[registry] Client added: %s (total: %d)", clientID, total)
}

// Remove unregisters a client connection.
func (r *BroadcastRegistry) Remove(conn *protocol.WSConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if clientID, exists := r.clients[conn]; exists {
		delete(r.clients, conn)
		total := atomic.AddInt32(&r.count, -1)
		log.Printf("[registry] Client removed: %s (total: %d)", clientID, total)
	}
}

// Broadcast sends the data to all active connections except the sender.
func (r *BroadcastRegistry) Broadcast(sender *protocol.WSConnection, data []byte, pool api.BufferPool) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sent := 0
	for client, clientID := range r.clients {
		if client == sender {
			continue // Skip sender
		}

		// Create zero-copy buffer for this client
		buf := pool.Get(len(data), -1)
		copy(buf.Bytes(), data)

		// Send as WebSocket frame
		frame := &protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: int64(len(data)),
			Payload:    buf.Bytes(),
		}

		if err := client.SendFrame(frame); err != nil {
			log.Printf("[broadcast] Failed to send to %s: %v", clientID, err)
		} else {
			sent++
		}

		buf.Release()
	}

	return sent
}

// Size returns the number of active clients.
func (r *BroadcastRegistry) Size() int {
	return int(atomic.LoadInt32(&r.count))
}

func main() {
	// Parse CLI flags
	addr := flag.String("addr", ":9002", "WebSocket listen address")
	flag.Parse()

	// Build HioloadWS config
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr

	// Create HioloadWS facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create HioloadWS: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("Failed to start HioloadWS: %v", err)
	}
	defer hioload.Stop()

	log.Printf("Broadcast server listening on %s", cfg.ListenAddr)

	// Create client registry for broadcast
	registry := NewBroadcastRegistry()
	hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
		return registry.Size()
	})

	// Get buffer pool for zero-copy operations
	bufPool := hioload.GetBufferPool()

	// Initialize native WebSocket listener (no HTTP dependency)
	listener, err := transport.NewWebSocketListener(cfg.ListenAddr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept loop with done notification
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		var connCounter int32

		for {
			wsConn, err := listener.Accept()
			if err != nil {
				if errors.Is(err, transport.ErrListenerClosed) {
					log.Println("Listener closed, exiting broadcast accept loop")
					return
				}
				log.Printf("accept error: %v", err)
				continue
			}

			id := fmt.Sprintf("client-%d", atomic.AddInt32(&connCounter, 1))
			registry.Add(wsConn, id)

			// Prepare broadcast handler
			broadcastHandler := adapters.HandlerFunc(func(data any) error {
				bytes, ok := data.([]byte)
				if !ok {
					return nil
				}
				count := registry.Broadcast(wsConn, bytes, bufPool)
				log.Printf("[broadcast] Message from %s to %d clients", id, count)
				return nil
			})

			mw := adapters.NewMiddlewareHandler(broadcastHandler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			wsConn.SetHandler(mw)
			wsConn.Start()

			go func(conn *protocol.WSConnection, clientID string) {
				<-conn.Done()
				registry.Remove(conn)
				log.Printf("[disconnect] Client %s disconnected", clientID)
			}(wsConn, id)
		}
	}()

	// Handle shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutdown signal received, closing broadcast listener")
	listener.Close()
	<-acceptDone
	log.Println("Broadcast server shutdown complete")
}
