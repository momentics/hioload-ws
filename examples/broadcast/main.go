// File: examples/broadcast/main.go
// Package main
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Native high-performance WebSocket broadcast (chat) server using hioload-ws only, no HTTP dependency.
// Demonstrates zero-copy broadcasting with NUMA-aware buffer pools, custom registry, and event-driven handler.
// All debug/trace messages are printed only by this example.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
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
	count   int
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
	r.count++
	log.Printf("[registry] Client added: %s (total: %d)", clientID, r.count)
}

// Remove unregisters a client connection.
func (r *BroadcastRegistry) Remove(conn *protocol.WSConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if clientID, exists := r.clients[conn]; exists {
		delete(r.clients, conn)
		r.count--
		log.Printf("[registry] Client removed: %s (total: %d)", clientID, r.count)
	}
}

// Broadcast sends the data to all active connections except the sender.
func (r *BroadcastRegistry) Broadcast(sender *protocol.WSConnection, data []byte, bufPool api.BufferPool) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sent := 0
	for client, clientID := range r.clients {
		if client == sender {
			continue // Skip sender
		}

		// Create zero-copy buffer for this client
		buf := bufPool.Get(len(data), -1)
		copy(buf.Bytes(), data)

		// Send as WebSocket frame
		frame := &protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: int64(len(data)),
			Payload:    buf.Bytes(),
		}

		if err := client.Send(frame); err != nil {
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
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// BroadcastHandler implements api.Handler for broadcast functionality.
type BroadcastHandler struct {
	registry *BroadcastRegistry
	bufPool  api.BufferPool
	sender   *protocol.WSConnection
}

// Handle processes incoming data and broadcasts it to all clients.
func (h *BroadcastHandler) Handle(data any) error {
	// Convert data to bytes
	var msg []byte
	switch d := data.(type) {
	case []byte:
		msg = d
	case string:
		msg = []byte(d)
	default:
		return nil // Skip unknown data types
	}

	// Broadcast to all clients
	count := h.registry.Broadcast(h.sender, msg, h.bufPool)
	log.Printf("[broadcast] Message broadcasted to %d clients (size: %d bytes)", count, len(msg))

	return nil
}

// main is the entry point for the broadcast server example.
func main() {
	// Parse CLI flags; see examples/echo for descriptions
	addr := flag.String("addr", ":9002", "WebSocket listen address")
	shards := flag.Int("shards", 16, "number of session shards")
	workers := flag.Int("workers", 4, "number of executor workers")
	batchSize := flag.Int("batch", 16, "poller batch size")
	ringCap := flag.Int("ring", 1024, "poller ring capacity")
	numaNode := flag.Int("numa", -1, "preferred NUMA node")
	flag.Parse()

	// Build HioloadWS config
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.SessionShards = *shards
	cfg.NumWorkers = *workers
	cfg.BatchSize = *batchSize
	cfg.RingCapacity = *ringCap
	cfg.NUMANode = *numaNode

	// Create HioloadWS facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Failed to create HioloadWS: %v", err)
		os.Exit(1)
	}

	// Start services (affinity, executor, poller, buffer pools)
	if err := hioload.Start(); err != nil {
		log.Fatalf("[FATAL] Failed to start HioloadWS: %v", err)
		os.Exit(1)
	}

	log.Printf("[info] Broadcast server listening on %s (NUMA=%d)", cfg.ListenAddr, cfg.NUMANode)

	// Create client registry for broadcast
	registry := NewBroadcastRegistry()

	// Debug probe for live stats
	hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
		return registry.Size()
	})

	// Get buffer pool for zero-copy operations
	bufPool := hioload.GetBufferPool()

	// Initialize native WebSocket listener (NO HTTP dependency)
	listener, err := transport.NewWebSocketListener(*addr, bufPool, cfg.ChannelSize)
	if err != nil {
		log.Fatalf("[FATAL] Failed to create WebSocket listener: %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	log.Printf("[info] Native WebSocket listener ready on %s", *addr)

	// Accept loop: each connection runs its own handler loop, multiplexed by hioload-ws
	go func() {
		connCounter := 0
		for {
			// Accept a new ws connection (native handshake, zero-copy transport)
			wsConn, err := listener.Accept()
			if err != nil {
				log.Printf("[error] Failed to accept WebSocket connection: %v", err)
				continue
			}

			connCounter++
			clientID := fmt.Sprintf("client-%d", connCounter)

			// Register connection into broadcast registry
			registry.Add(wsConn, clientID)

			// Create broadcast handler for this connection
			handler := &BroadcastHandler{
				registry: registry,
				bufPool:  bufPool,
				sender:   wsConn,
			}

			// Wrap with middleware chain
			middleware := adapters.NewMiddlewareHandler(handler).
				Use(adapters.LoggingMiddleware).
				Use(adapters.RecoveryMiddleware).
				Use(adapters.MetricsMiddleware(hioload.GetControl()))

			// Set handler on WSConnection
			wsConn.SetHandler(middleware)

			// Start connection processing in background
			go func(conn *protocol.WSConnection, id string) {
				defer func() {
					registry.Remove(conn)
					conn.Close()
				}()

				// Start connection loops
				conn.Start()

				// Wait for connection to finish
				<-conn.Done()
				log.Printf("[disconnect] Client %s disconnected", id)
			}(wsConn, clientID)

			log.Printf("[connect] Client %s connected (total: %d)", clientID, registry.Size())
		}
	}()

	// Graceful shutdown by OS signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("[info] Shutting down broadcast server...")

	// Close listener first to stop accepting new connections
	listener.Close()

	// Stop HioloadWS services
	if err := hioload.Stop(); err != nil {
		log.Printf("[error] Error stopping HioloadWS: %v", err)
	}

	log.Println("[info] Server stopped gracefully.")
}
