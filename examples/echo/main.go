// Author: momentics <momentics@gmail.com>
// SPDX-License-Identifier: MIT

package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "os/signal"
    "github.com/momentics/hioload-ws/fake"
    "github.com/momentics/hioload-ws/transport"
    "github.com/momentics/hioload-ws/protocol"
)

func main() {
    listener, err := net.Listen("tcp", ":9001")
    if err != nil {
        panic(err)
    }
    fmt.Println("Echo WebSocket server started on :9001...")

    pool := &fake.FakeBytePool{}

    // Handle SIGINT for graceful shutdown.
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    go func() {
        for {
            conn, err := listener.Accept()
            if err != nil {
                continue
            }
            go handleConn(conn, pool)
        }
    }()

    <-ctx.Done()
    fmt.Println("Server shutting down.")
}

func handleConn(conn net.Conn, pool *fake.FakeBytePool) {
    netc := transport.NewNetConn(conn, pool)
    wsc := protocol.NewWebSocketConn(netc, pool)
    defer wsc.Close()

    for {
        frame, err := wsc.ReadFrame()
        if err != nil || frame == nil {
            break
        }
        // Echo back
        _ = wsc.WriteFrame(frame)
    }
}
