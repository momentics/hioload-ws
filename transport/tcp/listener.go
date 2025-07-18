// Copyright (c) 2025
// Author: momentics <momentics@gmail.com>

// Package tcp provides a minimal TCP listener/acceptor and
// simple WebSocket handshake logic for hioload-ws.
// The design is extensible for zero-copy, CPU affinity, and future optimizations.

package tcp

import (
    "bufio"
    "crypto/sha1"
    "encoding/base64"
    "fmt"
    "net"
    "os"
    "strings"
    "time"
)

// wsGUID is the fixed GUID, per RFC 6455, used in WebSocket handshake computations.
const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// ListenerConfig holds configuration for the TCP listener.
type ListenerConfig struct {
    Addr        string             // TCP address to bind (e.g., ":9001")
    WorkerCPUs  []int              // List of CPUs for optional affinity pinning
    ConnHandler func(net.Conn)     // Handler for successfully upgraded WebSocket connections
}

// StartTCPListener opens the TCP listening socket, applies affinity if requested,
// and runs the accept loop with WebSocket handshake.
func StartTCPListener(cfg *ListenerConfig) error {
    ln, err := net.Listen("tcp", cfg.Addr)
    if err != nil {
        return fmt.Errorf("tcp listen failed: %v", err)
    }
    defer ln.Close()
    fmt.Printf("TCP listening on %s\n", cfg.Addr)

    // Apply affinity to main accept goroutine (if configured)
    if len(cfg.WorkerCPUs) > 0 {
        setCPUAffinity(cfg.WorkerCPUs[0])
    }

    for {
        conn, err := ln.Accept()
        if err != nil {
            fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
            continue
        }
        // Accept and upgrade connection in a separate goroutine,
        // hook for affinity can be applied inside handler (future extension)
        go handleConn(conn, cfg.ConnHandler)
    }
}

// handleConn performs a minimal RFC 6455 WebSocket handshake.
// If handshake is valid, hands off the connection to user handler.
// On error, connection is closed.
func handleConn(conn net.Conn, handler func(net.Conn)) {
    defer func() {
        if r := recover(); r != nil {
            fmt.Fprintf(os.Stderr, "panic in connection: %v\n", r)
        }
    }()
    conn.SetDeadline(time.Now().Add(5 * time.Second))
    br := bufio.NewReader(conn)

    reqLine, err := br.ReadString('\n')
    if err != nil {
        conn.Close()
        return
    }
    if !strings.HasPrefix(reqLine, "GET") {
        conn.Close()
        return
    }

    headers := map[string]string{}
    // Read HTTP headers until CRLF line.
    for {
        line, err := br.ReadString('\n')
        if err != nil || line == "\r\n" {
            break
        }
        if sep := strings.Index(line, ":"); sep > 0 {
            key := strings.TrimSpace(line[:sep])
            val := strings.TrimSpace(line[sep+1:])
            headers[strings.ToLower(key)] = val
        }
    }

    if !strings.EqualFold(headers["upgrade"], "websocket") {
        conn.Close()
        return
    }
    secKey, ok := headers["sec-websocket-key"]
    if !ok {
        conn.Close()
        return
    }
    acceptKey := handshakeAcceptKey(secKey)

    response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
        "Upgrade: websocket\r\n"+
        "Connection: Upgrade\r\n"+
        "Sec-WebSocket-Accept: %s\r\n\r\n", acceptKey)

    if _, err := conn.Write([]byte(response)); err != nil {
        conn.Close()
        return
    }
    conn.SetDeadline(time.Time{})
    handler(conn)
}

// handshakeAcceptKey computes the correct Sec-WebSocket-Accept response value.
func handshakeAcceptKey(secWebSocketKey string) string {
    h := sha1.New()
    h.Write([]byte(strings.TrimSpace(secWebSocketKey) + wsGUID))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
