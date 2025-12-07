// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
package highlevel

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/protocol"
)

// RouteParam represents a parameter in a route pattern
type RouteParam struct {
	Key   string
	Value string
}

// Conn represents a WebSocket connection with automatic resource management.
type Conn struct {
	// Low-level connection from hioload-ws (could be server or client connection)
	underlying *protocol.WSConnection
	pool       api.BufferPool

	// Connection state
	mutex      sync.RWMutex
	closed     bool
	closeOnce  sync.Once

	// Configuration
	readLimit  int64
	readTimeout time.Duration
	writeTimeout time.Duration

	// Automatic buffer management
	autoRelease bool

	// Callbacks
	onClose func()

	// Client-specific fields (may be nil for server connections)
	client *client.Client

	// URL parameters extracted from the route
	params []RouteParam
}

// newConn creates a new Conn wrapper around protocol.WSConnection
func newConn(underlying *protocol.WSConnection, pool api.BufferPool) *Conn {
	return &Conn{
		underlying:  underlying,
		pool:        pool,
		readLimit:   32 << 20, // 32MB default
		autoRelease: true,
		params:      make([]RouteParam, 0),
	}
}

// newConnWithParams creates a new Conn wrapper with URL parameters
func newConnWithParams(underlying *protocol.WSConnection, pool api.BufferPool, params []RouteParam) *Conn {
	return &Conn{
		underlying:  underlying,
		pool:        pool,
		params:      params,
		readLimit:   32 << 20, // 32MB default
		autoRelease: true,
	}
}

// newClientConn creates a new Conn wrapper for client connections
func newClientConn(underlying *protocol.WSConnection, pool api.BufferPool, client *client.Client) *Conn {
	return &Conn{
		underlying:  underlying,
		pool:        pool,
		client:      client,
		readLimit:   32 << 20, // 32MB default
		autoRelease: true,
		params:      make([]RouteParam, 0),
	}
}

// GetUnderlyingWSConnection returns the underlying protocol.WSConnection
// This can be used for direct access to low-level functionality
func (c *Conn) GetUnderlyingWSConnection() *protocol.WSConnection {
	if c.client != nil {
		// If we have a client, get the connection from the client
		return c.client.GetWSConnection()
	}
	return c.underlying
}

// ReadJSON unmarshals the next JSON message from the connection.
func (c *Conn) ReadJSON(v interface{}) error {
	_, payload, err := c.readMessage()
	if err != nil {
		return err
	}
	
	// Unmarshal JSON directly from the payload
	return json.Unmarshal(payload, v)
}

// WriteJSON marshals and sends a JSON message over the connection.
func (c *Conn) WriteJSON(v interface{}) error {
	// Marshal to JSON first
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	
	// Use WriteMessage to send as binary
	return c.WriteMessage(int(BinaryMessage), data)
}

// ReadMessage reads a message from the connection.
func (c *Conn) ReadMessage() (messageType int, p []byte, err error) {
	return c.readMessage()
}

// internal readMessage function that returns the raw message
func (c *Conn) readMessage() (messageType int, p []byte, err error) {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return 0, nil, errors.New("connection closed")
	}
	c.mutex.RUnlock()

	// Use zero-copy receive method with timeout
	var buffers []api.Buffer

	// Get the underlying connection properly (for clients, this comes from the client instance)
	wsConn := c.GetUnderlyingWSConnection()
	if wsConn == nil {
		return 0, nil, errors.New("no underlying connection available")
	}

	if c.readTimeout > 0 {
		// Use timeout channel for synchronization without blocking
		done := make(chan struct {
			buffers []api.Buffer
			err     error
		}, 1)

		go func() {
			buffs, recvErr := wsConn.RecvZeroCopy()
			done <- struct {
				buffers []api.Buffer
				err     error
			}{buffs, recvErr}
		}()

		select {
		case result := <-done:
			if result.err != nil {
				return 0, nil, result.err
			}
			buffers = result.buffers
		case <-time.After(c.readTimeout):
			return 0, nil, errors.New("read timeout")
		}
	} else {
		buffers, err = wsConn.RecvZeroCopy()
		if err != nil {
			return 0, nil, err
		}
	}

	if len(buffers) == 0 {
		return 0, nil, errors.New("no message received")
	}

	// Get the first buffer
	buf := buffers[0]
	data := buf.Bytes()

	// Check size limit
	if c.readLimit > 0 && int64(len(data)) > c.readLimit {
		if c.autoRelease {
			buf.Release()
		}
		return 0, nil, errors.New("message exceeds read limit")
	}

	// Preserve zero-copy semantics by returning a copy to maintain high-level API simplicity
	// while ensuring the original buffer can be released if autoRelease is enabled
	result := make([]byte, len(data))
	copy(result, data)

	// Release the buffer (critical for preventing memory leaks in zero-copy system)
	if c.autoRelease {
		buf.Release()
	}

	// Return binary message type for now - in real implementation we'd get this from frame
	return int(BinaryMessage), result, nil
}

// WriteMessage writes a message to the connection.
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("connection closed")
	}
	c.mutex.RUnlock()

	// Set write timeout if configured
	if c.writeTimeout > 0 {
		// In the real implementation, we'd implement timeout handling
		// This is a simplified version
	}

	// Get a buffer from the pool for zero-copy sending
	buf := c.pool.Get(len(data), -1)  // Use appropriate NUMA node

	// Copy data to the buffer
	dest := buf.Bytes()
	copy(dest, data)

	// Create a frame based on message type
	var opcode byte
	switch MessageType(messageType) {
	case TextMessage:
		opcode = protocol.OpcodeText
	case BinaryMessage:
		opcode = protocol.OpcodeBinary
	case CloseMessage:
		opcode = protocol.OpcodeClose
	case PingMessage:
		opcode = protocol.OpcodePing
	case PongMessage:
		opcode = protocol.OpcodePong
	default:
		opcode = protocol.OpcodeBinary // default to binary
	}

	// Create the frame to send
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     opcode,
		PayloadLen: int64(len(data)),
		Payload:    dest[:len(data)], // Use the buffer slice directly for zero-copy
	}

	// For client connections, make sure frames are masked per RFC 6455
	if c.client != nil {
		frame.Masked = true
	}

	// Send the frame using the appropriate connection method
	var sendErr error
	if c.client != nil {
		// Use client's WriteMessage method for client connections
		sendErr = c.client.WriteMessage(messageType, data)
	} else {
		// Use server connection's SendFrame method
		sendErr = c.underlying.SendFrame(frame)
	}

	// Release the buffer after we're done referencing it
	if c.autoRelease {
		buf.Release()
	}

	return sendErr
}

// Close closes the connection.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.mutex.Lock()
		c.closed = true
		c.mutex.Unlock()

		// Close the underlying connection
		wsConn := c.GetUnderlyingWSConnection()
		if wsConn != nil {
			err = wsConn.Close()
		}

		// Call close callback if set
		if c.onClose != nil {
			c.onClose()
		}
	})

	return err
}

// SetReadLimit sets the maximum size for incoming messages.
func (c *Conn) SetReadLimit(limit int64) {
	c.mutex.Lock()
	c.readLimit = limit
	c.mutex.Unlock()
}

// SetReadDeadline sets the read deadline.
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.mutex.Lock()
	c.readTimeout = time.Until(t)
	c.mutex.Unlock()

	// Apply deadline to the underlying transport if it supports it
	conn := c.GetUnderlyingWSConnection()
	if conn != nil {
		transport := conn.Transport()
		if deadlineSetter, ok := transport.(interface{ SetReadDeadline(time.Time) error }); ok {
			return deadlineSetter.SetReadDeadline(t)
		}
	}
	return nil
}

// SetWriteDeadline sets the write deadline.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.mutex.Lock()
	c.writeTimeout = time.Until(t)
	c.mutex.Unlock()

	// Apply deadline to the underlying transport if it supports it
	conn := c.GetUnderlyingWSConnection()
	if conn != nil {
		transport := conn.Transport()
		if deadlineSetter, ok := transport.(interface{ SetWriteDeadline(time.Time) error }); ok {
			return deadlineSetter.SetWriteDeadline(t)
		}
	}
	return nil
}

// GetClient returns the underlying client if this is a client connection
func (c *Conn) GetClient() *client.Client {
	return c.client
}

// writeMessage writes a message to the connection (internal implementation).
func (c *Conn) writeMessage(messageType int, data []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("connection closed")
	}
	c.mutex.RUnlock()

	// Get a buffer from the pool for zero-copy sending
	buf := c.pool.Get(len(data), -1)  // Use appropriate NUMA node

	// Copy data to the buffer
	dest := buf.Bytes()
	copy(dest, data)

	// Create a frame based on message type
	var opcode byte
	switch MessageType(messageType) {
	case TextMessage:
		opcode = protocol.OpcodeText
	case BinaryMessage:
		opcode = protocol.OpcodeBinary
	case CloseMessage:
		opcode = protocol.OpcodeClose
	case PingMessage:
		opcode = protocol.OpcodePing
	case PongMessage:
		opcode = protocol.OpcodePong
	default:
		opcode = protocol.OpcodeBinary // default to binary
	}

	// Create the frame to send
	frame := &protocol.WSFrame{
		IsFinal:    true,
		Opcode:     opcode,
		PayloadLen: int64(len(data)),
		Payload:    dest[:len(data)], // Use the buffer slice directly for zero-copy
	}

	// For client connections, make sure frames are masked per RFC 6455
	if c.client != nil {
		frame.Masked = true
	}

	// Send the frame using the appropriate connection method with timeout
	var sendErr error
	if c.client != nil {
		// Use client's WriteMessage method with timeout handling
		if c.writeTimeout > 0 {
			done := make(chan error, 1)
			go func() {
				done <- c.client.WriteMessage(messageType, data)
			}()

			select {
			case sendErr = <-done:
				// Message sent, continue
			case <-time.After(c.writeTimeout):
				sendErr = errors.New("write timeout")
			}
		} else {
			sendErr = c.client.WriteMessage(messageType, data)
		}
	} else {
		// Use server connection's SendFrame method with timeout handling
		if c.writeTimeout > 0 {
			done := make(chan error, 1)
			go func() {
				done <- c.underlying.SendFrame(frame)
			}()

			select {
			case sendErr = <-done:
				// Message sent, continue
			case <-time.After(c.writeTimeout):
				sendErr = errors.New("write timeout")
			}
		} else {
			sendErr = c.underlying.SendFrame(frame)
		}
	}

	// Release the buffer after we're done referencing it
	if c.autoRelease {
		buf.Release()
	}

	return sendErr
}

// SetCloseCallback sets a function to be called when the connection closes.
func (c *Conn) SetCloseCallback(callback func()) {
	c.mutex.Lock()
	c.onClose = callback
	c.mutex.Unlock()
}

// Param gets the value of a parameter by name.
func (c *Conn) Param(name string) string {
	for _, param := range c.params {
		if param.Key == name {
			return param.Value
		}
	}
	return ""
}

// AllParams returns all route parameters.
func (c *Conn) AllParams() map[string]string {
	result := make(map[string]string, len(c.params))
	for _, param := range c.params {
		result[param.Key] = param.Value
	}
	return result
}

// ReadString reads a UTF-8 string message from the connection.
func (c *Conn) ReadString() (string, error) {
	_, payload, err := c.readMessage()
	if err != nil {
		return "", err
	}

	// Convert the payload to a string
	return string(payload), nil
}

// WriteString sends a UTF-8 string message over the connection.
func (c *Conn) WriteString(s string) error {
	// Use WriteMessage to send as text
	return c.WriteMessage(int(TextMessage), []byte(s))
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() string {
	// Placeholder - would return actual local address
	return "localhost"
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() string {
	// Placeholder - would return actual remote address
	return "remote"
}