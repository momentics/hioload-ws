// File: examples/echo/main.go
// Package main
// Author: momentics <momentics@gmail.com>
//
// High-performance echo server demonstrating hioload-ws usage:
// - Cross-platform transport (TCP, optional DPDK)
// - NUMA-aware buffer pooling
// - Batched, zero-copy WebSocket framing
// - Middleware: logging, recovery, metrics
// - Hot-reloadable configuration and runtime metrics
// - CPU/NUMA affinity pinning
// - Runtime debug output

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/protocol"
)

func main() {
	// CLI flags
	addr := flag.String("addr", ":8080", "listen address")
	useDPDK := flag.Bool("dpdk", false, "enable DPDK transport (build with -tags=dpdk)")
	shardCount := flag.Int("shards", 16, "session shards for concurrency")
	numWorkers := flag.Int("workers", 4, "number of executor workers")
	flag.Parse()

	// Build config
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.UseDPDK = *useDPDK
	cfg.SessionShards = *shardCount
	cfg.NumWorkers = *numWorkers

	// Create facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("New hioload-ws: %v", err)
	}

	// Register debug probe with real session count
	hioload.GetControl().RegisterDebugProbe("active_sessions", func() any {
		return fmt.Sprintf("%d", hioload.GetSessionCount())
	})

	// Start service
	if err := hioload.Start(); err != nil {
		log.Fatalf("Start failed: %v", err)
	}
	log.Printf("Server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)

	// Build middleware chain
	baseHandler := &EchoHandler{hioload: hioload}
	mw := adapters.NewMiddlewareHandler(baseHandler).
		Use(adapters.LoggingMiddleware).
		Use(adapters.RecoveryMiddleware).
		Use(adapters.MetricsMiddleware(hioload.GetControl()))

	// Register poller handler
	if err := hioload.RegisterHandler(mw); err != nil {
		log.Fatalf("RegisterHandler failed: %v", err)
	}

	// HTTP upgrade endpoint
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
		log.Printf("Client connected: %s", r.RemoteAddr)
	})

	// Signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	if err := hioload.Stop(); err != nil {
		log.Printf("Stop error: %v", err)
	}
	log.Println("Server stopped.")
}

// EchoHandler implements api.Handler for echoing received buffers.
type EchoHandler struct {
	hioload *facade.HioloadWS
}

// Handle is called for each received message (zero-copy api.Buffer).
func (h *EchoHandler) Handle(data any) error {
	buf, ok := data.([]byte)
	if !ok {
		// non-byte data: ignore
		return nil
	}
	// Allocate a buffer for echo reply
	out := h.hioload.GetBuffer(len(buf))
	copy(out.Bytes(), buf)
	// Send back in batch
	if err := h.hioload.GetTransport().Send([][]byte{out.Bytes()}); err != nil {
		log.Printf("Send error: %v", err)
	}
	out.Release()
	return nil
}
