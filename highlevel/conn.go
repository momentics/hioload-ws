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
	mutex     sync.RWMutex
	closed    bool
	closeOnce sync.Once

	// Configuration
	readLimit    int64
	readTimeout  time.Duration
	writeTimeout time.Duration

	// Automatic buffer management
	autoRelease bool

	// Callbacks
	onClose func()
	// Inbound queue for server-side connections fed by the event loop
	incoming    chan api.Buffer
	handlerOnce sync.Once

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
		incoming:    make(chan api.Buffer, 128),
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
		incoming:    make(chan api.Buffer, 128),
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
	_, payload, err := c.ReadMessage()
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

// ReadMessage reads a message from the connection and returns a safe copy.
// For zero-copy callers, use ReadBuffer and release the buffer when done.
func (c *Conn) ReadMessage() (messageType int, p []byte, err error) {
	mt, buf, err := c.readBuffer()
	if err != nil {
		return 0, nil, err
	}
	defer buf.Release()

	payload := buf.Bytes()
	if c.readLimit > 0 && int64(len(payload)) > c.readLimit {
		return 0, nil, errors.New("message exceeds read limit")
	}

	out := make([]byte, len(payload))
	copy(out, payload)
	return mt, out, nil
}

// ReadBuffer returns the next message without copying.
// Caller must call buf.Release() when finished.
func (c *Conn) ReadBuffer() (int, api.Buffer, error) {
	return c.readBuffer()
}

// internal readBuffer function that returns the raw buffer
func (c *Conn) readBuffer() (messageType int, buf api.Buffer, err error) {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return 0, api.Buffer{}, errors.New("connection closed")
	}
	c.mutex.RUnlock()

	// Server-side connections consume data pushed by the reactor into the queue
	if c.client == nil && c.incoming != nil {
		return c.readBufferFromIncoming()
	}

	// Use zero-copy receive method with timeout
	var buffers []api.Buffer

	// Get the underlying connection properly (for clients, this comes from the client instance)
	wsConn := c.GetUnderlyingWSConnection()
	if wsConn == nil {
		return 0, api.Buffer{}, errors.New("no underlying connection available")
	}

	if c.readTimeout > 0 {
		if rd, ok := wsConn.Transport().(interface{ SetReadDeadline(time.Time) error }); ok {
			rd.SetReadDeadline(time.Now().Add(c.readTimeout))
		}
	}

	buffers, err = wsConn.RecvZeroCopy()
	if err != nil {
		return 0, api.Buffer{}, err
	}

	if len(buffers) == 0 {
		return 0, api.Buffer{}, errors.New("no message received")
	}

	buf = buffers[0]
	if c.readLimit > 0 && int64(len(buf.Bytes())) > c.readLimit {
		buf.Release()
		return 0, api.Buffer{}, errors.New("message exceeds read limit")
	}

	return int(BinaryMessage), buf, nil
}

// WriteMessage writes a message to the connection.
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	c.mutex.RLock()
	if c.closed {
		c.mutex.RUnlock()
		return errors.New("connection closed")
	}
	c.mutex.RUnlock()

	// Client connections delegate directly to the low-level client which handles framing/masking.
	if c.client != nil {
		return c.client.WriteMessage(messageType, data)
	}

	// Get a buffer from the pool for zero-copy sending
	buf := c.pool.Get(len(data), -1) // Use appropriate NUMA node
	dest := buf.Bytes()
	usePool := len(dest) >= len(data)

	if usePool {
		copy(dest, data)
	} else {
		// Pool buffer is smaller than payload; fall back to an owned slice.
		buf.Release()
		dest = append([]byte(nil), data...)
	}

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
		Payload:    dest[:len(data)], // Use the buffer slice directly for zero-copy when possible
	}

	// Send the frame using the server connection's SendFrame method
	sendErr := c.underlying.SendFrame(frame)

	// Release the buffer after we're done referencing it
	if usePool && c.autoRelease {
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

		// Drain any queued buffers to avoid leaks
		if c.incoming != nil {
			for {
				select {
				case buf := <-c.incoming:
					if buf.Data != nil {
						buf.Release()
					}
				default:
					goto drained
				}
			}
		}
	drained:

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

	// Client connections can delegate directly to the low-level client to avoid buffer size mismatches.
	if c.client != nil {
		if c.writeTimeout > 0 {
			done := make(chan error, 1)
			go func() {
				done <- c.client.WriteMessage(messageType, data)
			}()

			select {
			case err := <-done:
				return err
			case <-time.After(c.writeTimeout):
				return errors.New("write timeout")
			}
		}
		return c.client.WriteMessage(messageType, data)
	}

	// Get a buffer from the pool for zero-copy sending
	buf := c.pool.Get(len(data), -1) // Use appropriate NUMA node
	dest := buf.Bytes()
	usePool := len(dest) >= len(data)

	if usePool {
		copy(dest, data)
	} else {
		buf.Release()
		dest = append([]byte(nil), data...)
	}

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

	// Send the frame using the appropriate connection method with timeout
	var sendErr error
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

	// Release the buffer after we're done referencing it
	if usePool && c.autoRelease {
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

// enqueueIncoming adds an inbound buffer to the queue for server-side reads.
func (c *Conn) enqueueIncoming(buf api.Buffer) {
	if c.incoming == nil {
		buf.Release()
		return
	}

	var done <-chan struct{}
	if ws := c.GetUnderlyingWSConnection(); ws != nil {
		done = ws.Done()
	}

	// Fast path: try non-blocking enqueue first.
	select {
	case c.incoming <- buf:
		return
	default:
	}

	// Offload blocking enqueue to a goroutine so the poller isn't stalled.
	go func() {
		if done == nil {
			select {
			case c.incoming <- buf:
			default:
				// If still blocked, fall back to a blocking send.
				c.incoming <- buf
			}
			return
		}

		select {
		case c.incoming <- buf:
		case <-done:
			buf.Release()
		}
	}()
}

// runHandlerOnce ensures the provided handler is started only once per connection.
func (c *Conn) runHandlerOnce(handler func(*Conn)) {
	c.handlerOnce.Do(func() {
		go handler(c)
	})
}

// readBufferFromIncoming pulls a buffer from the inbound queue respecting deadlines.
func (c *Conn) readBufferFromIncoming() (int, api.Buffer, error) {
	var timer *time.Timer
	if c.readTimeout > 0 {
		timer = time.NewTimer(c.readTimeout)
		defer timer.Stop()
	}

	var done <-chan struct{}
	if ws := c.GetUnderlyingWSConnection(); ws != nil {
		done = ws.Done()
	}

	if timer != nil {
		select {
		case buf := <-c.incoming:
			if buf.Data == nil {
				return 0, api.Buffer{}, errors.New("connection closed")
			}
			return int(BinaryMessage), buf, nil
		case <-timer.C:
			return 0, api.Buffer{}, errors.New("read timeout")
		case <-done:
			return 0, api.Buffer{}, errors.New("connection closed")
		}
	}

	select {
	case buf := <-c.incoming:
		if buf.Data == nil {
			return 0, api.Buffer{}, errors.New("connection closed")
		}
		return int(BinaryMessage), buf, nil
	case <-done:
		return 0, api.Buffer{}, errors.New("connection closed")
	}
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
	_, payload, err := c.ReadMessage()
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
