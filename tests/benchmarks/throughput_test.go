// File: tests/benchmarks/throughput_test.go
package benchmarks

import (
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/highlevel"
	"github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/lowlevel/server"
	"github.com/momentics/hioload-ws/protocol"
)

// getFreePort returns a free port for valid binding
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// TestLowLevelThroughput benchmarks the low-level API throughput using real transports.
func TestLowLevelThroughput(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	addr := fmt.Sprintf(":%d", port)
	wsAddr := fmt.Sprintf("ws://localhost:%d/bench", port)

	// 1. Start Low-Level Server
	srvCfg := server.DefaultConfig()
	srvCfg.ListenAddr = addr
	srvCfg.IOBufferSize = 4096

	srv, err := server.NewServer(srvCfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Echo handler matching the pattern in examples/lowlevel/echo
	echoHandler := adapters.HandlerFunc(func(data any) error {
		// Handle Open/Close events if necessary to avoid panics or weird behavior
		switch data.(type) {
		case api.OpenEvent:
			return nil
		case api.CloseEvent:
			return nil
		}

		buf, ok := data.(api.Buffer)
		if !ok {
			return nil // Ignore unknown events
		}

		// Magic to get connection from data context
		ctx := api.ContextFromData(data)
		conn := api.FromContext(ctx).(*protocol.WSConnection)
		
		// Create frame to echo back
		frame := &protocol.WSFrame{
			IsFinal:    true,
			Opcode:     protocol.OpcodeBinary,
			PayloadLen: int64(len(buf.Bytes())),
			Payload:    buf.Bytes(), // Zero-copy usage might require buffer management, but for test echo implies copy usually unless we reuse buf. 
			// We should release input buffer?
		}
		
		err := conn.SendFrame(frame)
		buf.Release() // Release input buffer provided by poller
		return err
	})

	serverReady := make(chan struct{})
	go func() {
		close(serverReady) 
		if err := srv.Run(echoHandler); err != nil {
			// t.Logf("Server stopped: %v", err)
		}
	}()

	<-serverReady
	time.Sleep(200 * time.Millisecond) // Give it a moment to bind

	// 2. Start Low-Level Client
	cliCfg := client.DefaultConfig()
	cliCfg.Addr = wsAddr 
	cliCfg.IOBufferSize = 4096
	
	cli, err := client.NewClient(cliCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	// Client automatically connects in NewClient or starts loops.
	// But check logic: NewClient initializes. Does it connect?
	// Based on facade.go outline: NewClient initializes, handshakes, and starts I/O loops.
	defer cli.Close()

	// 3. Benchmark Loop
	packetSize := 1024
	payload := make([]byte, packetSize)
	duration := 5 * time.Second

	var sent, received uint64
	
	start := time.Now()
	end := start.Add(duration)

	go func() {
		// Receiver loop using Client Recv API
		for {
			bufs, err := cli.Recv()
			if err != nil {
				return
			}
			for _, b := range bufs {
				atomic.AddUint64(&received, 1) // Count messages? Or bytes?
				b.Release()
			}
		}
	}()

	for time.Now().Before(end) {
		// Use Client Send API
		cli.Send(payload)
		atomic.AddUint64(&sent, 1)
		
		// If we send too fast without flow control, we might overflow buffers.
		// Real throughput tests often just blast.
		// However, Client.Send enqueues to batch.
	}
	
	elapsed := time.Since(start)
	
	// Report
	rx := atomic.LoadUint64(&received)
	
	mbps := float64(rx*uint64(packetSize)) / 1024 / 1024 / elapsed.Seconds()
	rps := float64(rx) / elapsed.Seconds()

	t.Logf("=== Low-Level Throughput ===")
	t.Logf("Duration: %v", elapsed)
	t.Logf("Packet Size: %d bytes", packetSize)
	t.Logf("Messages Received: %d", rx)
	t.Logf("Throughput: %.2f Msg/sec", rps)
	t.Logf("Throughput: %.2f MB/sec", mbps)
	t.Logf("============================")

	srv.Shutdown()
}

// TestHighLevelThroughput benchmarks the high-level API throughput.
func TestHighLevelThroughput(t *testing.T) {
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	addr := fmt.Sprintf(":%d", port)
	url := fmt.Sprintf("ws://localhost:%d/echo", port)

	// 1. Server
	srv := highlevel.NewServer(addr)
	srv.HandleFunc("/echo", func(c *highlevel.Conn) {
		for {
			mt, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if err := c.WriteMessage(mt, data); err != nil {
				return
			}
		}
	})
	
	go srv.ListenAndServe()
	time.Sleep(200 * time.Millisecond)

	// 2. Client
	conn, err := highlevel.Dial(url)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// 3. Benchmark
	packetSize := 1024
	payload := make([]byte, packetSize)
	duration := 5 * time.Second

	var received uint64
	start := time.Now()
	end := start.Add(duration)

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			atomic.AddUint64(&received, 1)
		}
	}()

	for time.Now().Before(end) {
		if err := conn.WriteMessage(int(highlevel.BinaryMessage), payload); err != nil {
			t.Errorf("Write error: %v", err)
			break
		}
	}

	elapsed := time.Since(start)
	rx := atomic.LoadUint64(&received)
	
	mbps := float64(rx*uint64(packetSize)) / 1024 / 1024 / elapsed.Seconds()
	rps := float64(rx) / elapsed.Seconds()

	t.Logf("=== High-Level Throughput ===")
	t.Logf("Duration: %v", elapsed)
	t.Logf("Packet Size: %d bytes", packetSize)
	t.Logf("Messages Received: %d", rx)
	t.Logf("Throughput: %.2f Msg/sec", rps)
	t.Logf("Throughput: %.2f MB/sec", mbps)
	t.Logf("=============================")

	srv.Shutdown()
}
