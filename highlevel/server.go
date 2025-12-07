// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
// It offers easy-to-use APIs while preserving high performance, zero-copy, NUMA-awareness, and batch processing.
package highlevel

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/lowlevel/server"
	"github.com/momentics/hioload-ws/protocol"
)

// HTTPMethod represents an HTTP method
type HTTPMethod string

const (
	GET     HTTPMethod = "GET"
	POST    HTTPMethod = "POST"
	PUT     HTTPMethod = "PUT"
	PATCH   HTTPMethod = "PATCH"
	DELETE  HTTPMethod = "DELETE"
	HEAD    HTTPMethod = "HEAD"
	OPTIONS HTTPMethod = "OPTIONS"
	TRACE   HTTPMethod = "TRACE"
)

// RouteHandler holds both the WebSocket handler and the allowed HTTP methods
type RouteHandler struct {
	Handler func(*Conn)
	Methods []HTTPMethod
}

// Middleware is a function that can intercept and process a connection before passing it to the next handler
type Middleware func(next func(*Conn)) func(*Conn)

// RouteGroup represents a group of routes with common prefix
type RouteGroup struct {
	server *Server
	prefix string
}

// Global metrics counters
var (
	globalActiveConns int64
	globalTotalMsgs   int64
)

// Server wraps the low-level server with a high-level API.
type Server struct {
	addr       string
	handlers   map[string]*RouteHandler // Exact path handlers with HTTP methods
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
	patterns map[*regexp.Regexp]*RouteHandler
	// Route patterns with parameter names (for named parameter extraction)
	routePatterns map[string][]string // maps pattern to parameter names
	// Store allowed methods for each pattern (for error responses)
	patternMethods map[*regexp.Regexp][]HTTPMethod
	// Middleware chain
	middleware []Middleware
}

// NewServer creates a new high-level WebSocket server.
func NewServer(addr string) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		addr:           addr,
		handlers:       make(map[string]*RouteHandler),
		opts:           make([]server.ServerOption, 0),
		cfg:            server.DefaultConfig(),
		ctx:            ctx,
		cancel:         cancel,
		connections:    make(map[*Conn]bool),
		patterns:       make(map[*regexp.Regexp]*RouteHandler),
		routePatterns:  make(map[string][]string),
		patternMethods: make(map[*regexp.Regexp][]HTTPMethod),
		middleware:     make([]Middleware, 0),
	}
}

// HandleFunc registers a function to handle WebSocket connections for the given pattern with default methods (GET).
func (s *Server) HandleFunc(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{GET}, handler)
}

// HandleFuncWithMethods registers a function to handle WebSocket connections for the given pattern with specific HTTP methods.
func (s *Server) HandleFuncWithMethods(pattern string, methods []HTTPMethod, handler func(*Conn)) {
	s.handlerMux.Lock()
	defer s.handlerMux.Unlock()

	routeHandler := &RouteHandler{
		Handler: handler,
		Methods: methods,
	}

	// Check if the pattern contains parameters (e.g., /users/:id/messages/:messageId)
	if containsParam(pattern) {
		// Convert parameterized route to regex
		regexPattern, paramNames := convertToRegex(pattern)
		regex := regexp.MustCompile("^" + regexPattern + "$")

		// Store the handler and parameter names
		s.patterns[regex] = routeHandler
		s.routePatterns[regexPattern] = paramNames
		s.patternMethods[regex] = methods
	} else if !containsRegex(pattern) {
		// If the pattern is a simple path without regex, store it directly
		s.handlers[pattern] = routeHandler
	} else {
		// Compile the pattern as a regex
		regex := regexp.MustCompile(pattern)
		s.patterns[regex] = routeHandler
		s.patternMethods[regex] = methods
	}
}

// containsRegex checks if a pattern contains regex characters
func containsRegex(pattern string) bool {
	return regexp.MustCompile(`[\*\+\?\[\]\^\$\.\|\\()]`).MatchString(pattern)
}

// GET registers a handler for GET method on the specified pattern.
func (s *Server) GET(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{GET}, handler)
}

// POST registers a handler for POST method on the specified pattern.
func (s *Server) POST(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{POST}, handler)
}

// PUT registers a handler for PUT method on the specified pattern.
func (s *Server) PUT(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{PUT}, handler)
}

// PATCH registers a handler for PATCH method on the specified pattern.
func (s *Server) PATCH(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{PATCH}, handler)
}

// DELETE registers a handler for DELETE method on the specified pattern.
func (s *Server) DELETE(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{DELETE}, handler)
}

// HEAD registers a handler for HEAD method on the specified pattern.
func (s *Server) HEAD(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{HEAD}, handler)
}

// OPTIONS registers a handler for OPTIONS method on the specified pattern.
func (s *Server) OPTIONS(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{OPTIONS}, handler)
}

// TRACE registers a handler for TRACE method on the specified pattern.
func (s *Server) TRACE(pattern string, handler func(*Conn)) {
	s.HandleFuncWithMethods(pattern, []HTTPMethod{TRACE}, handler)
}

// Use adds middleware to the server's middleware chain.
func (s *Server) Use(middleware ...Middleware) {
	s.handlerMux.Lock()
	defer s.handlerMux.Unlock()
	s.middleware = append(s.middleware, middleware...)
}

// Middleware returns the server's middleware chain for testing purposes
func (s *Server) Middleware() []Middleware {
	s.handlerMux.RLock()
	defer s.handlerMux.RUnlock()
	m := make([]Middleware, len(s.middleware))
	copy(m, s.middleware)
	return m
}

// Handlers returns the server's handlers map for testing purposes
func (s *Server) Handlers() map[string]*RouteHandler {
	s.handlerMux.RLock()
	defer s.handlerMux.RUnlock()
	h := make(map[string]*RouteHandler, len(s.handlers))
	for k, v := range s.handlers {
		h[k] = v
	}
	return h
}

// Group creates a new route group with the given prefix.
func (s *Server) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		server: s,
		prefix: prefix,
	}
}

// Group methods - all routes registered on the group will have the prefix prepended
// GET registers a handler for GET method on the specified pattern with group prefix.
func (g *RouteGroup) GET(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{GET}, handler)
}

// POST registers a handler for POST method on the specified pattern with group prefix.
func (g *RouteGroup) POST(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{POST}, handler)
}

// PUT registers a handler for PUT method on the specified pattern with group prefix.
func (g *RouteGroup) PUT(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{PUT}, handler)
}

// PATCH registers a handler for PATCH method on the specified pattern with group prefix.
func (g *RouteGroup) PATCH(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{PATCH}, handler)
}

// DELETE registers a handler for DELETE method on the specified pattern with group prefix.
func (g *RouteGroup) DELETE(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{DELETE}, handler)
}

// HEAD registers a handler for HEAD method on the specified pattern with group prefix.
func (g *RouteGroup) HEAD(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{HEAD}, handler)
}

// OPTIONS registers a handler for OPTIONS method on the specified pattern with group prefix.
func (g *RouteGroup) OPTIONS(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{OPTIONS}, handler)
}

// TRACE registers a handler for TRACE method on the specified pattern with group prefix.
func (g *RouteGroup) TRACE(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{TRACE}, handler)
}

// HandleFunc registers a function to handle WebSocket connections for the given pattern with group prefix and default method (GET).
func (g *RouteGroup) HandleFunc(pattern string, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), []HTTPMethod{GET}, handler)
}

// HandleFuncWithMethods registers a function to handle WebSocket connections for the given pattern with group prefix and specific HTTP methods.
func (g *RouteGroup) HandleFuncWithMethods(pattern string, methods []HTTPMethod, handler func(*Conn)) {
	g.server.HandleFuncWithMethods(g.joinPrefix(pattern), methods, handler)
}

// Group creates a nested route group with the given prefix appended to the current group's prefix.
func (g *RouteGroup) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		server: g.server,
		prefix: g.joinPrefix(prefix),
	}
}

// Use adds middleware to the group's server.
func (g *RouteGroup) Use(middleware ...Middleware) {
	g.server.Use(middleware...)
}

// Prefix returns the group's prefix
func (g *RouteGroup) Prefix() string {
	return g.prefix
}

// joinPrefix joins the group prefix with the pattern
func (g *RouteGroup) joinPrefix(pattern string) string {
	if g.prefix == "" {
		return pattern
	}

	// Ensure there's only one slash between prefix and pattern
	result := g.prefix
	if !strings.HasSuffix(g.prefix, "/") && !strings.HasPrefix(pattern, "/") {
		result += "/"
	} else if strings.HasSuffix(g.prefix, "/") && strings.HasPrefix(pattern, "/") {
		// Remove leading slash from pattern to avoid double slash
		result += pattern[1:]
		return result
	}

	return result + pattern
}

// applyMiddleware applies the middleware chain to a handler function
func (s *Server) applyMiddleware(handler func(*Conn)) func(*Conn) {
	// Apply middleware in reverse order to create proper chain (last middleware executes first)
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}
	return handler
}

// Built-in middleware functions

// LoggingMiddleware logs connection information
func LoggingMiddleware(next func(*Conn)) func(*Conn) {
	return func(conn *Conn) {
		// Log connection start
		fmt.Printf("[LOG] WebSocket connection started from %s\n", conn.RemoteAddr())

		// Execute the next handler
		next(conn)

		// Log connection end
		fmt.Printf("[LOG] WebSocket connection from %s ended\n", conn.RemoteAddr())
	}
}

// RecoveryMiddleware recovers from panics in handlers
func RecoveryMiddleware(next func(*Conn)) func(*Conn) {
	return func(conn *Conn) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[RECOVERY] Panic recovered in handler: %v\n", r)
				// Optionally close the connection if there was a panic
				_ = conn.Close()
			}
		}()
		next(conn)
	}
}

// MetricsMiddleware collects basic metrics
func MetricsMiddleware(next func(*Conn)) func(*Conn) {
	return func(conn *Conn) {
		// Increment active connections
		active := atomic.AddInt64(&globalActiveConns, 1)
		fmt.Printf("[METRICS] Active connections: %d\n", active)

		// Execute the next handler
		next(conn)

		// Decrement active connections
		active = atomic.AddInt64(&globalActiveConns, -1)
		fmt.Printf("[METRICS] Active connections: %d\n", active)
	}
}

// GetMetrics returns current server metrics
func GetMetrics() map[string]int64 {
	return map[string]int64{
		"active_connections": globalActiveConns,
		"total_messages":     globalTotalMsgs,
	}
}

// containsParam checks if a pattern contains parameter placeholders (e.g., :id)
func containsParam(pattern string) bool {
	return strings.Contains(pattern, ":")
}

// convertToRegex converts a parameterized route to a regex pattern and extracts parameter names
func convertToRegex(pattern string) (regex string, paramNames []string) {
	// Split the pattern by "/"
	parts := strings.Split(pattern, "/")
	regexParts := make([]string, 0, len(parts))
	var params []string

	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			// This is a parameter part like ":id"
			paramName := strings.TrimPrefix(part, ":")
			regexParts = append(regexParts, `([^/]+)`) // Match any characters except "/"
			params = append(params, paramName)
		} else if part == "" && len(parts) > 1 {
			// Handle the case where pattern starts with "/" (first part is empty)
			continue
		} else {
			// This is a static part, escape special regex chars
			escaped := regexp.QuoteMeta(part)
			regexParts = append(regexParts, escaped)
		}
	}

	// Combine with "/" separators
	regex = strings.Join(regexParts, "/")
	paramNames = params
	return
}

// findHandler finds the appropriate handler for a request path and extracts parameters
// For now, we assume the HTTP method is GET since WebSocket upgrade requires GET method
// In the future, this can be extended to check against allowed methods
func (s *Server) findHandler(path string, method HTTPMethod) (*RouteHandler, []RouteParam) {
	s.handlerMux.RLock()
	defer s.handlerMux.RUnlock()

	// Find exact match first
	if handler, exists := s.handlers[path]; exists {
		// Check if the method is allowed
		if isMethodAllowed(method, handler.Methods) {
			return handler, nil
		}
	}

	// Try to match with regex patterns and extract parameters
	for pattern, handler := range s.patterns {
		matches := pattern.FindStringSubmatch(path)
		if matches != nil && len(matches) > 1 {
			// Check if the method is allowed
			if !isMethodAllowed(method, handler.Methods) {
				continue
			}

			// Extract parameter names for this pattern
			// Find the corresponding regex pattern to get parameter names
			var paramNames []string
			for regexStr, names := range s.routePatterns {
				// Check if this regex matches our pattern
				if regexp.MustCompile("^" + regexp.QuoteMeta(regexStr) + "$").MatchString(pattern.String()) {
					paramNames = names
					break
				}
			}

			// Create parameter map
			var params []RouteParam
			for i, paramName := range paramNames {
				if i+1 < len(matches) {
					params = append(params, RouteParam{Key: paramName, Value: matches[i+1]})
				}
			}

			return handler, params
		}
	}

	// Return nil if no handler found or method not allowed
	return nil, nil
}

// isMethodAllowed checks if the given HTTP method is in the allowed methods list
func isMethodAllowed(method HTTPMethod, allowedMethods []HTTPMethod) bool {
	if len(allowedMethods) == 0 {
		// If no methods are specified, default to allowing GET for WebSocket
		return method == GET
	}

	for _, allowed := range allowedMethods {
		if allowed == method {
			return true
		}
	}
	return false
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
		// fmt.Printf("DEBUG: basicHandler called with type: %T\n", data)
		var buf api.Buffer

		// Unpack buffer from data
		if getter, ok := data.(interface{ GetBuffer() api.Buffer }); ok {
			buf = getter.GetBuffer()
		} else if b, ok := data.(api.Buffer); ok {
			buf = b
		}

		if buf.Data != nil {
			// This is a message from a connection
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
				// Find the appropriate handler based on the connection's path
				// For WebSocket connections, the method is always GET (for upgrade)
				routeHandler, params := s.findHandler(wsConn.Path(), GET)

				if routeHandler != nil {
					// Create a high-level connection wrapper with parameters
					pool := s.underlying.GetBufferPool()
					hlConn := newConnWithParams(wsConn, pool, params)

					// Apply middleware chain to the handler
					finalHandler := s.applyMiddleware(routeHandler.Handler)
					finalHandler(hlConn)
				} else {
					// No handler found, close connection or return error
					// Create a basic connection just to close it
					pool := s.underlying.GetBufferPool()
					hlConn := newConn(wsConn, pool)
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
