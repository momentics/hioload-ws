// Package hioload provides a high-level WebSocket library built on top of hioload-ws primitives.
package highlevel

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/lowlevel/client"
	"github.com/momentics/hioload-ws/core/protocol"
)

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
}

// newConn creates a new Conn wrapper around protocol.WSConnection
func newConn(underlying *protocol.WSConnection, pool api.BufferPool) *Conn {
	return &Conn{
		underlying:  underlying,
		pool:        pool,
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
	}
}

// GetUnderlyingWSConnection returns the underlying protocol.WSConnection
// This can be used for direct access to low-level functionality
func (c *Conn) GetUnderlyingWSConnection() *protocol.WSConnection {
	if c.client != nil {
		// If we have a client, get the connection from there
		// Note: This requires the client to have a public method to access the connection
		// In a real implementation, we'd need to modify the client package
		return nil // For now, we can't access the unexported connection
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
	
	// Use zero-copy receive method
	buffers, err := c.underlying.RecvZeroCopy()
	if err != nil {
		return 0, nil, err
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
	
	// Create a copy of the data to return
	// For true zero-copy semantics, we could return a reference to the buffer
	// and manage its lifecycle, but this is simpler for the high-level API
	result := make([]byte, len(data))
	copy(result, data)
	
	// Release the buffer (important for zero-copy semantics)
	if c.autoRelease {
		buf.Release()
	}
	
	// Return binary message type for now - in real implementation we'd extract from frame
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
		err = c.underlying.Close()
		
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
	// This functionality would be implemented based on underlying transport
	// For now, we'll return nil as a placeholder
	return nil
}

// SetWriteDeadline sets the write deadline.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	// This functionality would be implemented based on underlying transport
	// For now, we'll return nil as a placeholder
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

	// Send the frame using the appropriate connection method
	var sendErr error
	if c.client != nil {
		// Use client's WriteMessage method
		sendErr = c.client.WriteMessage(messageType, data)  // Use our enhanced client method
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

// SetCloseCallback sets a function to be called when the connection closes.
func (c *Conn) SetCloseCallback(callback func()) {
	c.mutex.Lock()
	c.onClose = callback
	c.mutex.Unlock()
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