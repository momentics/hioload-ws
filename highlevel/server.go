// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
// It offers easy-to-use APIs while preserving high performance, zero-copy, NUMA-awareness, and batch processing.
package highlevel

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/protocol"
	"github.com/momentics/hioload-ws/lowlevel/server"
)

// Server wraps the low-level server with a high-level API.
type Server struct {
	addr       string
	handlers   map[string]func(*Conn)
	handlerMux sync.RWMutex
	opts       []server.ServerOption
	// Reference to the underlying server
	underlying *server.Server
	// Store server configuration
	cfg *server.Config
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	// Connection tracking
	connections   map[*Conn]bool
	connectionsMu sync.RWMutex
	// Path patterns for route matching
	patterns map[*regexp.Regexp]func(*Conn)
}

// NewServer creates a new high-level WebSocket server.
func NewServer(addr string) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		addr:        addr,
		handlers:    make(map[string]func(*Conn)),
		opts:        make([]server.ServerOption, 0),
		cfg:         server.DefaultConfig(),
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[*Conn]bool),
		patterns:    make(map[*regexp.Regexp]func(*Conn)),
	}
}

// HandleFunc registers a function to handle WebSocket connections for the given pattern.
func (s *Server) HandleFunc(pattern string, handler func(*Conn)) {
	s.handlerMux.Lock()
	defer s.handlerMux.Unlock()
	
	// If the pattern is a simple path without regex, store it directly
	if !containsRegex(pattern) {
		s.handlers[pattern] = handler
	} else {
		// Compile the pattern as a regex
		regex := regexp.MustCompile(pattern)
		s.patterns[regex] = handler
	}
}

// containsRegex checks if a pattern contains regex characters
func containsRegex(pattern string) bool {
	return regexp.MustCompile(`[\*\+\?\[\]\^\$\.\|\\()]`).MatchString(pattern)
}

// findHandler finds the appropriate handler for a request path
func (s *Server) findHandler(path string) func(*Conn) {
	s.handlerMux.RLock()
	defer s.handlerMux.RUnlock()
	
	// Find exact match first
	if handler, exists := s.handlers[path]; exists {
		return handler
	}
	
	// Try to match with regex patterns
	for pattern, handler := range s.patterns {
		if pattern.MatchString(path) {
			return handler
		}
	}
	
	// Return nil if no handler found
	return nil
}

// addConnection adds a connection to the tracking list
func (s *Server) addConnection(conn *Conn) {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	s.connections[conn] = true
}

// removeConnection removes a connection from the tracking list
func (s *Server) removeConnection(conn *Conn) {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()
	delete(s.connections, conn)
}

// ListenAndServe starts the server and serves requests until an error occurs or the server is stopped.
func (s *Server) ListenAndServe() error {
	// Set configuration
	s.cfg.ListenAddr = s.addr

	// Create the underlying server
	var err error
	s.underlying, err = server.NewServer(s.cfg, s.opts...)
	if err != nil {
		return fmt.Errorf("failed to create underlying server: %w", err)
	}

	// Create a combined handler that uses our routing
	basicHandler := adapters.HandlerFunc(func(data any) error {
		// This function handles incoming messages from connections
		// In the hioload-ws architecture, data should be a Buffer from the connection
		if buf, ok := data.(api.Buffer); ok {
			// This is a message from a connection
			// We need to determine which connection this is for and route appropriately
			// However, the buffer doesn't directly give us the connection info
			// We need to use context or other mechanisms to track this
			
			// For now, we'll implement a basic handler that can receive the connection info
			// This requires access to the WSConnection through ContextFromData
			// For events that contain the connection directly (bufEventWithConn), we can extract the path
			var wsConn *protocol.WSConnection

			// Check if the data contains a connection (in case of custom event with connection)
			if connData, ok := data.(interface{ WSConnection() *protocol.WSConnection }); ok {
				wsConn = connData.WSConnection()
			} else {
				connInterface := api.FromContext(api.ContextFromData(data))
				if wsConnInterface, ok := connInterface.(*protocol.WSConnection); ok {
					wsConn = wsConnInterface
				}
			}

			if wsConn != nil {
				// Create a high-level connection wrapper
				pool := s.underlying.GetBufferPool()
				hlConn := newConn(wsConn, pool)

				// Find the appropriate handler based on the connection's path
				handler := s.findHandler(wsConn.Path())

				if handler != nil {
					handler(hlConn)
				} else {
					// No handler found, close connection or return error
					hlConn.Close()
				}
			}
			
			// Release the buffer since we're done with it
			buf.Release()
		}
		return nil
	})

	// Start the underlying server
	return s.underlying.Run(basicHandler)
}

// ServerOption wraps server.ServerOption for high-level configuration
type ServerOption func(*Server)

// WithReadLimit sets the maximum size for incoming messages.
func WithReadLimit(limit int64) ServerOption {
	return func(s *Server) {
		// We'll store this for later use in connection creation
		// This will be applied to individual connections
	}
}

// WithWriteTimeout sets the write timeout for connections.
func WithWriteTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		// Store for later application to connections
	}
}

// WithReadTimeout sets the read timeout for connections.
func WithReadTimeout(d time.Duration) ServerOption {
	return func(s *Server) {
		// Store for later application to connections
	}
}

// WithMaxConnections sets the maximum number of concurrent connections.
func WithMaxConnections(max int) ServerOption {
	return func(s *Server) {
		s.cfg.MaxConnections = max
	}
}

// WithBatchSize sets the batch size for processing incoming messages.
func WithBatchSize(size int) ServerOption {
	return func(s *Server) {
		s.cfg.BatchSize = size
	}
}

// WithNUMANode sets the preferred NUMA node for this server.
func WithNUMANode(node int) ServerOption {
	return func(s *Server) {
		s.cfg.NUMANode = node
	}
}

// WithChannelCapacity sets the capacity of channels used for internal communication.
func WithChannelCapacity(cap int) ServerOption {
	return func(s *Server) {
		s.cfg.ChannelCapacity = cap
	}
}

// Shutdown stops the server gracefully.
func (s *Server) Shutdown() error {
	if s.underlying != nil {
		s.underlying.Shutdown()
	}
	if s.cancel != nil {
		s.cancel()
	}
	
	// Close all tracked connections
	s.connectionsMu.Lock()
	for conn := range s.connections {
		conn.Close()
	}
	s.connectionsMu.Unlock()
	
	return nil
}

// GetActiveConnections returns the number of currently active connections.
func (s *Server) GetActiveConnections() int64 {
	s.connectionsMu.RLock()
	defer s.connectionsMu.RUnlock()
	return int64(len(s.connections))
}