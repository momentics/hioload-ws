// File: examples/broadcast/main.go
// Package main
// Author: momentics <momentics@gmail.com>
//
// Broadcast/Chat server demonstrating hioload-ws capabilities:
// - Cross-platform transport (TCP or optional DPDK)
// - NUMA-aware zero-copy buffer pools
// - Batched, poll-mode event loop
// - CPU/NUMA affinity pinning
// - Middleware: logging, recovery, metrics
// - Dynamic debug probes and runtime stats
// - Graceful shutdown on signals

package main

import (
	"flag"
	"fmt"
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

// ChatHandler broadcasts messages to all connected clients.
type ChatHandler struct {
	mu       sync.RWMutex
	sessions map[string]*protocol.WSConnection
	hioload  *facade.HioloadWS
}

func NewChatHandler(h *facade.HioloadWS) *ChatHandler {
	return &ChatHandler{
		sessions: make(map[string]*protocol.WSConnection),
		hioload:  h,
	}
}

// Handle registers or broadcasts based on message type.
func (h *ChatHandler) Handle(data any) error {
	buf, ok := data.([]byte)
	if !ok {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for id, conn := range h.sessions {
		// allocate zero-copy buffer
		out := h.hioload.GetBuffer(len(buf))
		copy(out.Bytes(), buf)
		if err := conn.Send(&protocol.WSFrame{
			IsFinal: true,
			Opcode:  protocol.OpcodeBinary,
			Payload: out.Bytes(),
		}); err != nil {
			log.Printf("[broadcast] send to %s error: %v", id, err)
		}
		out.Release()
	}
	log.Printf("[broadcast] message sent to %d clients", len(h.sessions))
	return nil
}

// AddClient tracks a new connection.
func (h *ChatHandler) AddClient(id string, conn *protocol.WSConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[id] = conn
	log.Printf("[chat] client connected: %s", id)
}

// RemoveClient removes a connection.
func (h *ChatHandler) RemoveClient(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, id)
	log.Printf("[chat] client disconnected: %s", id)
}

func main() {
	// CLI flags
	addr := flag.String("addr", ":8080", "HTTP listen address")
	useDPDK := flag.Bool("dpdk", false, "enable DPDK transport (build with -tags=dpdk)")
	shards := flag.Int("shards", 16, "session shards")
	workers := flag.Int("workers", 4, "executor workers")
	flag.Parse()

	// Configure facade
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.UseDPDK = *useDPDK
	cfg.SessionShards = *shards
	cfg.NumWorkers = *workers

	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create HioloadWS: %v", err)
	}

	// Register debug probe for active clients
	chatHandler := NewChatHandler(hioload)
	hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
		return fmt.Sprintf("%d", len(chatHandler.sessions))
	})

	// Start service
	if err := hioload.Start(); err != nil {
		log.Fatalf("HioloadWS start error: %v", err)
	}
	log.Printf("Broadcast server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)

	// Middleware chain
	mw := adapters.NewMiddlewareHandler(chatHandler).
		Use(adapters.LoggingMiddleware).
		Use(adapters.RecoveryMiddleware).
		Use(adapters.MetricsMiddleware(hioload.GetControl()))

	// Register handler
	if err := hioload.RegisterHandler(mw); err != nil {
		log.Fatalf("RegisterHandler error: %v", err)
	}

	// HTTP /ws endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		headers, err := protocol.UpgradeToWebSocket(r)
		if err != nil {
			http.Error(w, "Upgrade failed", http.StatusBadRequest)
			return
		}
		for k, v := range headers {
			w.Header()[k] = v
		}
		w.WriteHeader(http.StatusSwitchingProtocols)

		conn := hioload.CreateWebSocketConnection()
		conn.SetHandler(mw)
		chatHandler.AddClient(r.RemoteAddr, conn)
		go func(id string, c *protocol.WSConnection) {
			c.Start()
			<-c.Done()
			chatHandler.RemoveClient(id)
		}(r.RemoteAddr, conn)
	})

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down broadcast server...")
	if err := hioload.Stop(); err != nil {
		log.Printf("Error stopping HioloadWS: %v", err)
	}
	log.Println("Server stopped.")
}
