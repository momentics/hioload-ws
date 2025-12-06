// File: server/handler_chain.go
// Package server implements middleware chain utilities.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import "github.com/momentics/hioload-ws/api"

// Middleware augments an api.Handler.
type Middleware func(api.Handler) api.Handler

// NewHandlerChain applies middleware in order: first in slice is outermost.
func NewHandlerChain(base api.Handler, mw ...Middleware) api.Handler {
    h := base
    for i := len(mw) - 1; i >= 0; i-- {
        h = mw[i](h)
    }
    return h
}
