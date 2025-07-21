// internal/transport/websocket_listener.go
// Direct WebSocket listener without HTTP overhead.
// Author: momentics <momentics@gmail.com>

package transport

import (
    "bufio"
    "fmt"
    "net"
    "strings"
    "sync"

    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/protocol"
)

// WebSocketListener provides direct native WebSocket connections.
type WebSocketListener struct {
    listener    net.Listener
    bufferPool  api.BufferPool
    channelSize int
    mu          sync.RWMutex
    closed      bool
}

// Конструктор теперь принимает channelSize.
func NewWebSocketListener(addr string, bufPool api.BufferPool, channelSize int) (*WebSocketListener, error) {
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        return nil, fmt.Errorf("listen on %s: %w", addr, err)
    }
    return &WebSocketListener{
        listener:    listener,
        bufferPool:  bufPool,
        channelSize: channelSize,
    }, nil
}

// Accept возвращает подключение WebSocket с нативным handshake.
func (wsl *WebSocketListener) Accept() (*protocol.WSConnection, error) {
    wsl.mu.RLock()
    if wsl.closed {
        wsl.mu.RUnlock()
        return nil, fmt.Errorf("listener closed")
    }
    wsl.mu.RUnlock()

    conn, err := wsl.listener.Accept()
    if err != nil {
        return nil, fmt.Errorf("accept connection: %w", err)
    }

    wsConn, err := wsl.performHandshake(conn)
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("websocket handshake: %w", err)
    }

    return wsConn, nil
}

// performHandshake делает парсинг заголовков и upgrade без HTTP.
func (wsl *WebSocketListener) performHandshake(conn net.Conn) (*protocol.WSConnection, error) {
    reader := bufio.NewReader(conn)

    // Request line
    requestLine, err := reader.ReadString('\n')
    if err != nil {
        return nil, fmt.Errorf("read request line: %w", err)
    }
    requestLine = strings.TrimRight(requestLine, "\r\n")
    if !strings.HasPrefix(requestLine, "GET ") {
        return nil, fmt.Errorf("invalid request method: %s", requestLine)
    }

    // Headers
    headers := make(map[string]string)
    for {
        line, err := reader.ReadString('\n')
        if err != nil {
            return nil, fmt.Errorf("read header: %w", err)
        }
        line = strings.TrimRight(line, "\r\n")
        if line == "" {
            break
        }
        parts := strings.SplitN(line, ":", 2)
        if len(parts) == 2 {
            key := strings.TrimSpace(parts[0])
            value := strings.TrimSpace(parts[1])
            headers[strings.ToLower(key)] = value
        }
    }

    // Проверка заголовков на допустимость апгрейда
    if err := protocol.ValidateUpgradeHeaders(headers); err != nil {
        return nil, err
    }

    wsKey := headers[protocol.HeaderSecWebSocketKey]
    acceptKey := protocol.ComputeAcceptKey(wsKey)

    // Ответ клиента WebSocket
    response := fmt.Sprintf(
        "HTTP/1.1 101 Switching Protocols\r\n"+
            "Upgrade: websocket\r\n"+
            "Connection: Upgrade\r\n"+
            "Sec-WebSocket-Accept: %s\r\n\r\n",
        acceptKey,
    )
    if _, err := conn.Write([]byte(response)); err != nil {
        return nil, fmt.Errorf("write upgrade response: %w", err)
    }

    // Нативный zero-copy Transport для отдельного подключения
    transport := &connTransport{
        conn:       conn,
        bufferPool: wsl.bufferPool,
    }

    // Критически важно: используем корректный channelSize
    return protocol.NewWSConnection(transport, wsl.bufferPool, wsl.channelSize), nil
}

func (wsl *WebSocketListener) Close() error {
    wsl.mu.Lock()
    defer wsl.mu.Unlock()
    if wsl.closed {
        return nil
    }
    wsl.closed = true
    return wsl.listener.Close()
}

func (wsl *WebSocketListener) Addr() net.Addr {
    return wsl.listener.Addr()
}

// connTransport — zero-copy обёртка для api.Transport.
type connTransport struct {
    conn       net.Conn
    bufferPool api.BufferPool
    mu         sync.RWMutex
    closed     bool
}

func (ct *connTransport) Send(buffers [][]byte) error {
    ct.mu.RLock()
    defer ct.mu.RUnlock()
    if ct.closed {
        return api.ErrTransportClosed
    }
    for _, buf := range buffers {
        if _, err := ct.conn.Write(buf); err != nil {
            return fmt.Errorf("write to connection: %w", err)
        }
    }
    return nil
}

func (ct *connTransport) Recv() ([][]byte, error) {
    ct.mu.RLock()
    defer ct.mu.RUnlock()
    if ct.closed {
        return nil, api.ErrTransportClosed
    }
    buffer := ct.bufferPool.Get(4096, -1)
    n, err := ct.conn.Read(buffer.Bytes())
    if err != nil {
        buffer.Release()
        return nil, fmt.Errorf("read from connection: %w", err)
    }
    actualData := make([]byte, n)
    copy(actualData, buffer.Bytes()[:n])
    buffer.Release()
    return [][]byte{actualData}, nil
}

func (ct *connTransport) Close() error {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    if ct.closed {
        return nil
    }
    ct.closed = true
    return ct.conn.Close()
}

func (ct *connTransport) Features() api.TransportFeatures {
    return api.TransportFeatures{
        ZeroCopy:  true,
        Batch:     true,
        NUMAAware: false,
        OS:        []string{"linux", "windows"},
    }
}
