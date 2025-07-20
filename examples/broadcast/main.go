// File: examples/broadcast/main.go
// Package main
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Broadcast server example using hioload-ws facade.
// Updated to use current API: GetControl, RegisterHandler, SetHandler, Done.

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/protocol"
)

type BroadcastServer struct {
	mu      sync.RWMutex
	clients map[string]*protocol.WSConnection
}

func (s *BroadcastServer) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, conn := range s.clients {
		frame := &protocol.WSFrame{Opcode: protocol.OpcodeBinary, Payload: data}
		conn.Send(frame)
	}
}

func main() {
	// CLI flags
	addr := flag.String("addr", ":8080", "HTTP listen address")
	useDPDK := flag.Bool("dpdk", false, "enable DPDK transport")
	shards := flag.Int("shards", 16, "number of session shards")
	workers := flag.Int("workers", 4, "number of executor workers")
	flag.Parse()

	// Facade configuration
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.UseDPDK = *useDPDK
	cfg.SessionShards = *shards
	cfg.NumWorkers = *workers

	// Create facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}

	// Start facade
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	log.Printf("Broadcast server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)

	// Server instance
	server := &BroadcastServer{clients: make(map[string]*protocol.WSConnection)}

	// Build middleware chain
	broadcastHandler := adapters.HandlerFunc(func(data any) error {
		buf, ok := data.([]byte)
		if !ok {
			return nil
		}
		server.Broadcast(buf)
		return nil
	})
	mw := adapters.NewMiddlewareHandler(broadcastHandler).
		Use(adapters.LoggingMiddleware).
		Use(adapters.RecoveryMiddleware).
		Use(adapters.MetricsMiddleware(hioload.GetControl()))

	// Register global poller handler
	if err := hioload.RegisterHandler(mw); err != nil {
		log.Fatalf("failed to register handler: %v", err)
	}

	// HTTP /ws endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		headers, err := protocol.UpgradeToWebSocket(r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		for k, v := range headers {
			w.Header()[k] = v
		}
		w.WriteHeader(http.StatusSwitchingProtocols)

		conn := hioload.CreateWebSocketConnection()
		conn.SetHandler(mw)
		conn.Start()

		clientID := r.RemoteAddr
		server.mu.Lock()
		server.clients[clientID] = conn
		server.mu.Unlock()

		// Cleanup on disconnect
		go func(id string, c *protocol.WSConnection) {
			<-c.Done()
			server.mu.Lock()
			delete(server.clients, id)
			server.mu.Unlock()
		}(clientID, conn)
	})

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down broadcast server...")
	hioload.Stop()
	log.Println("Server stopped.")
}
