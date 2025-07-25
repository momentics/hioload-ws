// File: examples/echo/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Zero-copy, NUMA-aware WebSocket server using hioload-ws `/server` facade.
// Listens for connections and echoes each binary frame back to the sender.

package main

import (
	"errors"
	"flag"
	"log"
	"sync/atomic"

	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/server"
)

func main() {
	// Command-line flag for listen address
	addr := flag.String("addr", ":9001", "WebSocket listen address")
	flag.Parse()

	// 1. Configure server facade
	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr
	cfg.IOBufferSize = 64 * 1024
	cfg.ChannelCapacity = 64
	cfg.NUMANode = -1

	// 2. Create Server facade
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("NewServer error: %v", err)
	}
	defer srv.Shutdown()

	// 3. Register debug probe for active connections count
	var active int64
	srv.GetControl().RegisterDebugProbe("active_connections", func() any {
		return atomic.LoadInt64(&active)
	})

	log.Printf("Echo server listening on %s", cfg.ListenAddr)

	// 4. Create WebSocket listener via facade
	listener, err := srv.CreateWebSocketListener()
	if err != nil {
		log.Fatalf("CreateWebSocketListener error: %v", err)
	}
	defer listener.Close()

	// 5. Accept loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, transport.ErrListenerClosed) {
				break
			}
			log.Printf("accept error: %v", err)
			continue
		}
		atomic.AddInt64(&active, 1)
		go handleConnection(conn, &active)
	}

	log.Println("Echo server shutdown complete")
}

func handleConnection(conn *protocol.WSConnection, active *int64) {
	defer func() {
		conn.Close()
		atomic.AddInt64(active, -1)
	}()

	for {
		bufs, err := conn.RecvZeroCopy()
		if err != nil {
			return
		}
		for _, buf := range bufs {
			// Echo back using zero-copy WSFrame
			frame := &protocol.WSFrame{
				IsFinal:    true,
				Opcode:     protocol.OpcodeBinary,
				PayloadLen: int64(len(buf.Bytes())),
				Payload:    buf.Bytes(),
			}
			conn.SendFrame(frame)
			buf.Release()
		}
	}
}
