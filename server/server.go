package server

import (
	"errors"
	"sync"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
	"github.com/momentics/hioload-ws/internal/transport"
	"github.com/momentics/hioload-ws/pool"
	"github.com/momentics/hioload-ws/protocol"
)

var ErrAlreadyRunning = errors.New("server already running")

// CreateWebSocketListener returns a NUMA-aware TCP→WebSocket listener
// performing HTTP upgrade handshakes.
func (s *Server) CreateWebSocketListener() (*transport.WebSocketListener, error) {
	return transport.NewWebSocketListener(s.cfg.ListenAddr, s.pool, s.cfg.ChannelCapacity,
		transport.WithListenerNUMANode(s.cfg.NUMANode))
}

// NewServer builds the Server facade.
func NewServer(cfg *Config, opts ...ServerOption) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Создаём адаптер контроля
	ctrl := adapters.NewControlAdapter()

	// Получаем число доступных NUMA-узлов
	nodeCnt := concurrency.NUMANodes()

	// Инициализируем менеджер пулов с указанием числа узлов
	mgr := pool.NewBufferPoolManager(nodeCnt)

	// Берём пул для заданного размера и узла
	bufPool := mgr.GetPool(cfg.IOBufferSize, cfg.NUMANode)

	listener, err := NewListener(cfg.ListenAddr, bufPool, cfg.ChannelCapacity, cfg.NUMANode)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:      cfg,
		pool:     bufPool,
		control:  ctrl,
		listener: listener,
		shutdown: make(chan struct{}),
	}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Serve starts accepting and handling connections with the given Handler.
func (s *Server) Serve(handler api.Handler, opts ...HandlerOption) error {
	var once sync.Once
	settings := &handlerSettings{}
	for _, o := range opts {
		o(settings)
	}

	go func() {
		for {
			select {
			case <-s.shutdown:
				return
			default:
			}
			ws, err := s.listener.Accept()
			if err != nil {
				continue
			}
			// wrap with middleware chain
			h := handler
			for i := len(settings.middleware) - 1; i >= 0; i-- {
				h = settings.middleware[i](h)
			}
			go func(cConn *protocol.WSConnection) {
				defer cConn.Close()
				for {
					bufs, err := cConn.RecvZeroCopy()
					if err != nil {
						return
					}
					for _, buf := range bufs {
						h.Handle(buf)
						buf.Release()
					}
				}
			}(ws)
		}
	}()

	once.Do(func() {}) // placeholder

	<-s.shutdown
	return nil
}

// Shutdown signals Serve to stop accepting new connections.
func (s *Server) Shutdown() error {
	close(s.shutdown)
	return s.listener.Close()
}

// GetControl exposes runtime metrics and debug control.
func (s *Server) GetControl() api.Control {
	return s.control
}

// GetBufferPool returns the server's NUMA-aware buffer pool.
func (s *Server) GetBufferPool() api.BufferPool {
	return s.pool
}
