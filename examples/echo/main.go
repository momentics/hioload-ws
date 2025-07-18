// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Example echo server using the TCP/WebSocket skeleton.

package main

import (
    "bufio"
    "fmt"
    "github.com/momentics/hioload-ws/transport/tcp"
    "io"
    "net"
)

// echoHandler echoes received lines back to the client.
func echoHandler(conn net.Conn) {
    defer conn.Close()
    rdr := bufio.NewReader(conn)
    for {
        msg, err := rdr.ReadString('\n')
        if err != nil {
            if err != io.EOF {
                fmt.Printf("recv error: %v\n", err)
            }
            return
        }
        fmt.Printf("Received: %s", msg)
        _, err = conn.Write([]byte(msg))
        if err != nil {
            fmt.Printf("send error: %v\n", err)
            return
        }
    }
}

func main() {
    cfg := &tcp.ListenerConfig{
        Addr:       ":9001",
        WorkerCPUs: []int{0}, // Pins acceptor to CPU 0 on Linux if possible.
        ConnHandler: echoHandler,
    }
    if err := tcp.StartTCPListener(cfg); err != nil {
        fmt.Printf("Listener error: %v\n", err)
    }
}
