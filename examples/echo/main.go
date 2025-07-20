// File: examples/echo/main.go
// Package main
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Echo WebSocket server example using hioload-ws facade.
// Corrected to use the 'buf' variable and compile successfully.

package main

import (
	"flag"
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
	addr := flag.String("addr", ":8080", "HTTP listen address")
	useDPDK := flag.Bool("dpdk", false, "enable DPDK transport")
	shards := flag.Int("shards", 16, "number of session shards")
	workers := flag.Int("workers", 4, "number of executor workers")
	flag.Parse()

	// Build facade configuration
	cfg := facade.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.UseDPDK = *useDPDK
	cfg.SessionShards = *shards
	cfg.NumWorkers = *workers

	// Create HioloadWS facade
	hioload, err := facade.New(cfg)
	if err != nil {
		log.Fatalf("failed to create HioloadWS: %v", err)
	}

	// Start facade
	if err := hioload.Start(); err != nil {
		log.Fatalf("failed to start HioloadWS: %v", err)
	}
	log.Printf("Echo server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)

	// HTTP /ws endpoint for WebSocket upgrade and echo logic
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Perform WebSocket handshake
		headers, err := protocol.UpgradeToWebSocket(r)
		if err != nil {
			http.Error(w, "upgrade failed", http.StatusBadRequest)
			return
		}
		for k, v := range headers {
			w.Header()[k] = v
		}
		w.WriteHeader(http.StatusSwitchingProtocols)

		// Create and start WSConnection
		conn := hioload.CreateWebSocketConnection()
		// Set per-connection handler to echo messages back
		conn.SetHandler(adapters.HandlerFunc(func(data any) error {
			buf, ok := data.([]byte)
			if !ok {
				return nil
			}
			// Prepare WebSocket frame with binary opcode
			frame := &protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(len(buf)),
				Payload:    buf,
			}
			// Send frame back to the client
			return conn.Send(frame)
		}))
		conn.Start()

		// Clean up on connection close
		go func() {
			<-conn.Done()
		}()
	})

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down echo server...")
	if err := hioload.Stop(); err != nil {
		log.Printf("error stopping HioloadWS: %v", err)
	}
	log.Println("Server stopped.")
}
