package server

import (
	"github.com/momentics/hioload-ws/api"
)

// HandlerOption customizes per-connection handling.
type HandlerOption func(*handlerSettings)

type handlerSettings struct {
	middleware []Middleware
}

// Middleware processes each incoming Buffer before the user Handler.
type Middleware func(api.Handler) api.Handler

// WithMiddleware adds middleware in FIFO order.
func WithMiddleware(mw ...Middleware) HandlerOption {
	return func(h *handlerSettings) {
		h.middleware = append(h.middleware, mw...)
	}
}

// ServerOption customizes Server behavior on creation.
type ServerOption func(*Server)
