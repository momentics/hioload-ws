// File: examples/stest/server/main.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Hioload-ws ultra-high-performance echo server.
// Implements explicit, cross-platform shutdown logic:
// - On shutdown signal, all active connections are forcibly closed
// - Waits only until connections are closed or timeout, then completes shutdown
// - No shutdown timeout or hang possible even with stuck handlers

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/server"
)

func main() {
	// CLI flag for listen address.
	addr := flag.String("addr", ":9000", "WebSocket listen address")
	flag.Parse()

	cfg := server.DefaultConfig()
	cfg.ListenAddr = *addr

	hioload, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create HioloadWS: %v", err)
	}
	if err := hioload.Start(); err != nil {
		log.Fatalf("Failed to start HioloadWS: %v", err)
	}

	var connCount int64
	var rpsCount int64
	var bytesCount int64
	var frameCount int64

	hioload.GetControl().RegisterDebugProbe("connections", func() any { return atomic.LoadInt64(&connCount) })
	hioload.GetControl().RegisterDebugProbe("rps", func() any { return atomic.LoadInt64(&rpsCount) })
	hioload.GetControl().RegisterDebugProbe("throughput_bps", func() any { return atomic.LoadInt64(&bytesCount) })
	hioload.GetControl().RegisterDebugProbe("avg_packet_size", func() any {
		fc := atomic.LoadInt64(&frameCount)
		if fc == 0 {
			return 0.0
		}
		return float64(atomic.LoadInt64(&bytesCount)) / float64(fc)
	})

	shutdownCh := make(chan struct{})
	var wg sync.WaitGroup        // Waits for metrics reporter and accept loop
	var handlerWg sync.WaitGroup // Waits for all per-connection handler goroutines

	// Map to track all open connections for forced closure.
	var connsMu sync.Mutex
	conns := make(map[*protocol.WSConnection]struct{})

	// Metrics reporter goroutine—terminates immediately at shutdown.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-shutdownCh:
				return
			case <-ticker.C:
				rps := atomic.SwapInt64(&rpsCount, 0)
				bytes := atomic.SwapInt64(&bytesCount, 0)
				frames := atomic.SwapInt64(&frameCount, 0)
				avg := 0.0
				if frames > 0 {
					avg = float64(bytes) / float64(frames)
				}
				fmt.Printf(
					"Connections=%d RPS=%d Throughput=%d B/s AvgPacketSize=%.2f B\n",
					atomic.LoadInt64(&connCount), rps, bytes, avg,
				)
			}
		}
	}()

	listener, err := hioload.CreateWebSocketListener(cfg.ListenAddr)
	if err != nil {
		log.Fatalf("Failed to create WebSocket listener: %v", err)
	}

	// Accept loop, create handler goroutine for each connection, track all conns in map.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-shutdownCh:
				return
			default:
			}
			wsConn, err := listener.Accept()
			if err != nil {
				if isListenerClosedError(err.Error()) {
					return
				}
				log.Printf("Accept error: %v", err)
				continue
			}
			connsMu.Lock()
			conns[wsConn] = struct{}{}
			connsMu.Unlock()
			atomic.AddInt64(&connCount, 1)
			handlerWg.Add(1)
			go func(conn *protocol.WSConnection) {
				defer func() {
					conn.Close()
					connsMu.Lock()
					delete(conns, conn)
					connsMu.Unlock()
					atomic.AddInt64(&connCount, -1)
					handlerWg.Done()
				}()
				for {
					select {
					case <-shutdownCh:
						// Send close frame and exit on shutdown
						_ = conn.SendFrame(&protocol.WSFrame{
							IsFinal:    true,
							Opcode:     protocol.OpcodeClose,
							PayloadLen: 0,
							Payload:    nil,
						})
						return
					default:
					}
					bufs, err := conn.RecvZeroCopy()
					if err != nil {
						return
					}
					atomic.AddInt64(&rpsCount, int64(len(bufs)))
					for _, buf := range bufs {
						size := int64(len(buf.Bytes()))
						atomic.AddInt64(&bytesCount, size)
						atomic.AddInt64(&frameCount, 1)
						frame := &protocol.WSFrame{
							IsFinal:    true,
							Opcode:     protocol.OpcodeBinary,
							PayloadLen: size,
							Payload:    buf.Bytes(),
						}
						if err := conn.SendFrame(frame); err != nil {
							buf.Release()
							return
						}
						buf.Release()
					}
				}
			}(wsConn)
		}
	}()

	// Cross-platform signal handling.
	signalCh := make(chan os.Signal, 2)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	<-signalCh
	log.Println("Shutdown signal received, closing listener...")

	close(shutdownCh) // Signal shutdown to all goroutines
	listener.Close()

	log.Println("Closing all active connections...")

	// Close all connections gracefully.
	connsMu.Lock()
	for conn := range conns {
		_ = conn.SendFrame(&protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeClose,
			PayloadLen: 0,
			Payload:    nil,
		})
		conn.Close()
	}
	connsMu.Unlock()

	log.Println("Waiting for active connections to close...")
	// Wait for handler goroutines, but force quit after timeout.
	const shutdownTimeout = 15 * time.Second
	done := make(chan struct{})
	go func() {
		handlerWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		// All connections closed in time.
	case <-time.After(shutdownTimeout):
		// Force exit if handlers hang.
		log.Printf("Shutdown warning: forced exit after %v", shutdownTimeout)
	}

	// Now cleanly wait for metrics reporter and accept loop.
	wg.Wait()

	// Shutdown facade after all user goroutines.
	if err := hioload.Shutdown(); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Server shutdown complete.")
}

// isListenerClosedError allows Accept() error handling on all major OSes.
func isListenerClosedError(errMsg string) bool {
	return errMsg == "listener closed" ||
		errMsg == "use of closed network connection" ||
		(runtime.GOOS == "windows" && (errMsg == "accept tcp: use of closed network connection" ||
			errMsg == "AcceptEx: An operation was attempted on something that is not a socket." ||
			errMsg == "read tcp: use of closed network connection"))
}
