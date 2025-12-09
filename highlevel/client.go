// File: highlevel/client.go
// Package highlevel provides a user-friendly API for WebSocket clients and servers.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package highlevel

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	lowlevel_client "github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/pool"
)

// Options configuration for high-level client.
type Options struct {
	IOBufferSize int
	NUMANode     int
	TLSConfig    *tls.Config
}

// DefaultOptions returns default client configuration.
func DefaultOptions() Options {
	return Options{
		IOBufferSize: 64 * 1024,
		NUMANode:     -1,
	}
}

func logToFileHelper(msg string) {
	f, err := os.OpenFile("c:\\hioload-ws\\debug_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(f, "[%s] %s\n", ts, msg)
}

// Dial connects to a WebSocket server using default options.
func Dial(url string) (*Conn, error) {
	return DialWithOptions(url, DefaultOptions())
}

// DialWithOptions connects to a WebSocket server with custom options.
func DialWithOptions(urlStr string, opts Options) (*Conn, error) {
	logToFileHelper("DialWithOptions called")

	// Construct configuration for lowlevel client
	cfg := &lowlevel_client.Config{
		Addr:         urlStr,
		IOBufferSize: opts.IOBufferSize,
		NUMANode:     opts.NUMANode,
		ReadTimeout:  5 * time.Second, // Default timeouts
		WriteTimeout: 5 * time.Second,
		BatchSize:    16,
	}

	client, err := lowlevel_client.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("client creation failed: %w", err)
	}

	wsConn := client.GetWSConnection()

	// Reuse the process-wide NUMA-aware buffer pools to avoid fragmentation
	bufPool := pool.DefaultManager().GetPool(opts.IOBufferSize, opts.NUMANode)

	// Use newClientConn to link the client instance (for WriteMessage delegating)
	// highlevel/conn.go: func newClientConn(underlying *protocol.WSConnection, pool api.BufferPool, client *client.Client) *Conn
	return newClientConn(wsConn, bufPool, client), nil
}
