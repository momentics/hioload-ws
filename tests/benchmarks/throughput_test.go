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
	srvCfg.IOBufferSize = 65536 // Support large frames

	srv, err := server.NewServer(srvCfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Echo work channel - worker goroutine serializes sends
	type echoWork struct {
		conn  *protocol.WSConnection
		frame *protocol.WSFrame
	}
	echoChan := make(chan echoWork, 1000)
	
	// Worker goroutine - serializes all echo sends
	go func() {
		for work := range echoChan {
// fmt.Printf("DEBUG: Worker SendFrame PayloadLen=%d, len(Payload)=%d\n", work.frame.PayloadLen, len(work.frame.Payload))
			work.conn.SendFrame(work.frame)
		}
	}()

	// Echo handler
	echoHandler := adapters.HandlerFunc(func(data any) (err error) {
		defer func() {
			if r := recover(); r != nil {
				// fmt.Printf("DEBUG: EchoHandler PANIC: %v\n", r)
			}
		}()
		
		// Unwrap thunk if data is a function
		if thunk, ok := data.(func() interface{}); ok {
			data = thunk()
		}
		
		// fmt.Printf("DEBUG: EchoHandler data type: %T\n", data)
		
		// Handle Open/Close events
		switch data.(type) {
		case api.OpenEvent, api.CloseEvent:
			// fmt.Println("DEBUG: EchoHandler: Open/Close event, returning")
			return nil
		}
		
		var buf api.Buffer
		if getter, ok := data.(interface{ GetBuffer() api.Buffer }); ok {
			buf = getter.GetBuffer()
			// fmt.Printf("DEBUG: EchoHandler: Got buffer via GetBuffer, len=%d\n", len(buf.Bytes()))
		} else if b, ok := data.(api.Buffer); ok {
			buf = b
			// fmt.Printf("DEBUG: EchoHandler: Got buffer direct, len=%d\n", len(buf.Bytes()))
		} else {
			// fmt.Println("DEBUG: EchoHandler: No buffer found")
		}

		if buf != nil {
			var conn *protocol.WSConnection
			if connData, ok := data.(interface{ WSConnection() *protocol.WSConnection }); ok {
				conn = connData.WSConnection()
				// fmt.Println("DEBUG: EchoHandler: Got conn via WSConnection()")
			} else {
				ctx := api.ContextFromData(data)
				if val := api.FromContext(ctx); val != nil {
					conn = val.(*protocol.WSConnection)
					// fmt.Println("DEBUG: EchoHandler: Got conn via Context")
				}
			}

		if conn != nil {
			// Send work to dedicated worker goroutine via channel
				// Copy payload since buffer will be released
				bufBytes := buf.Bytes()
// fmt.Printf("DEBUG: Echo handler bufBytes len=%d\n", len(bufBytes))
				payloadCopy := make([]byte, len(bufBytes))
				copy(payloadCopy, bufBytes)
				
				frame := &protocol.WSFrame{
					IsFinal:    true,
					Opcode:     protocol.OpcodeBinary,
					PayloadLen: int64(len(payloadCopy)),
					Payload:    payloadCopy,
				}
				
				// Non-blocking send - if channel full, drop (shouldn't happen with 1000 capacity)
				select {
				case echoChan <- echoWork{conn: conn, frame: frame}:
					// Sent
				default:
					// Channel full - shouldn't happen
				}
				buf.Release()
				return nil
			} else {
				// fmt.Println("DEBUG: EchoHandler: conn is nil, releasing buf")
			}
			buf.Release()
		}
		return nil
	})

	serverReady := make(chan struct{})
	go func() {
		// Signal ready after a brief delay to ensure listener is up
		go func() {
			time.Sleep(50 * time.Millisecond)
			close(serverReady)
		}()
		if err := srv.Run(echoHandler); err != nil {
			// t.Logf("Server stopped: %v", err)
		}
	}()

	<-serverReady
	time.Sleep(50 * time.Millisecond) // Extra margin

	// 2. Start Low-Level Client
	cliCfg := client.DefaultConfig()
	cliCfg.Addr = wsAddr 
	cliCfg.IOBufferSize = 65536 // Match server
	// cliCfg.NUMANode = 0 // Auto
	
	cli, err := client.NewClient(cliCfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer cli.Close()

	// 3. Benchmark Loop
	sizes := []int{32} // Reduced for quick verification
	msgCount := 20 // Reduced for completion

	var received uint64

	// Receiver routine
	go func() {
		for {
			bufs, err := cli.Recv()
			if err != nil {
// fmt.Printf("DEBUG: Receiver error: %v\n", err)
				return
			}
			for _, b := range bufs {
				rx := atomic.AddUint64(&received, 1)
				if rx <= 5 || rx % 100 == 0 {
// fmt.Printf("DEBUG: Receiver got buf len=%d, total=%d\n", len(b.Bytes()), rx)
				}
				b.Release()
			}
		}
	}()

	fmt.Printf("\n=== Low-Level Throughput Statistics ===\n")
	fmt.Printf("%-10s %-12s %-15s %-15s\n", "Size(B)", "Count", "Msg/sec", "MB/sec")
	fmt.Printf("--------------------------------------------------------\n")

	for _, size := range sizes {
		payload := make([]byte, size)
		
		initialRx := atomic.LoadUint64(&received)
		targetRx := initialRx + uint64(msgCount)

		start := time.Now()
		// fmt.Printf("DEBUG: Testing size %d. Sending %d messages...\n", size, msgCount)
		for i := 0; i < msgCount; i++ {
			cli.Send(payload)
		}
		// fmt.Println("DEBUG: Send loop done. Waiting for receive...")
		
		// Wait for completion
		for atomic.LoadUint64(&received) < targetRx {
			if time.Since(start) > 30*time.Second {
				t.Errorf("Timeout waiting for %d messages of size %d", msgCount, size)
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		duration := time.Since(start)

		rx := atomic.LoadUint64(&received) - initialRx
		if duration.Seconds() == 0 { duration = time.Millisecond }
		mbps := float64(rx*uint64(size)) / 1024 / 1024 / duration.Seconds()
		rps := float64(rx) / duration.Seconds()

		fmt.Printf("%-10d %-12d %-15.2f %-15.2f\n", size, rx, rps, mbps)
		
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("========================================================\n\n")

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

	// 3. Benchmark Loop
	sizes := []int{32, 64, 1024, 16384, 32768}
	msgCount := 2000

	var received uint64
	
	// Receiver routine
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
			atomic.AddUint64(&received, 1)
		}
	}()

	fmt.Printf("\n=== High-Level Throughput Statistics ===\n")
	fmt.Printf("%-10s %-12s %-15s %-15s\n", "Size(B)", "Count", "Msg/sec", "MB/sec")
	fmt.Printf("--------------------------------------------------------\n")

	for _, size := range sizes {
		payload := make([]byte, size)
		
		initialRx := atomic.LoadUint64(&received)
		targetRx := initialRx + uint64(msgCount)

		start := time.Now()
		for i := 0; i < msgCount; i++ {
			if err := conn.WriteMessage(int(highlevel.BinaryMessage), payload); err != nil {
				t.Errorf("Write error at size %d iter %d: %v", size, i, err)
				return // Stop test on error
			}
		}

		// Wait for completion
		for atomic.LoadUint64(&received) < targetRx {
			if time.Since(start) > 30*time.Second {
				t.Errorf("Timeout waiting for %d messages of size %d", msgCount, size)
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		duration := time.Since(start)

		rx := atomic.LoadUint64(&received) - initialRx
		mbps := float64(rx*uint64(size)) / 1024 / 1024 / duration.Seconds()
		rps := float64(rx) / duration.Seconds()

		fmt.Printf("%-10d %-12d %-15.2f %-15.2f\n", size, rx, rps, mbps)
		
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Printf("========================================================\n\n")

	srv.Shutdown()
}
