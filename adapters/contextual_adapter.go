// File: adapters/contextual_adapter.go
// Package adapters provides glue between api and internal implementations.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package adapters

import (
	"context"

	"github.com/momentics/hioload-ws/api"
)

// ContextualHandler wraps a base Handler to inject a context containing the WSConnection.
func ContextualHandler(base api.Handler) api.Handler {
	return api.HandlerFunc(func(data any) error {
		// Identify OpenEvent and CloseEvent to extract connection & context.
		var ctx context.Context
		var conn any

		switch evt := data.(type) {
		case api.OpenEvent:
			conn = evt.Conn
			ctx = evt.Ctx
		case api.CloseEvent:
			conn = evt.Conn
			ctx = evt.Ctx
		default:
			// If data has a Ctx() method, use it.
			if withCtx, ok := data.(interface{ Ctx() context.Context }); ok {
				ctx = withCtx.Ctx()
				conn = api.FromContext(ctx)
			}
		}
		// If we have a connection, inject it into context.
		if conn != nil && ctx != nil {
			ctx = api.ContextWithConnection(ctx, conn)
			// Wrap data in a struct providing Ctx()
			data = eventWithCtx{data: data, ctx: ctx}
		}
		// Call next handler
		return base.Handle(data)
	})
}

// eventWithCtx wraps an event and provides its context.
type eventWithCtx struct {
	data any
	ctx  context.Context
}

func (e eventWithCtx) Ctx() context.Context { return e.ctx }
func (e eventWithCtx) Payload() any {
	// If original data has Payload(), forward; else return data.
	if withPayload, ok := e.data.(interface{ Payload() any }); ok {
		return withPayload.Payload()
	}
	return e.data
}

// Ensure eventWithCtx satisfies api.Handler input expectations.
var _ interface {
	Ctx() context.Context
	Payload() any
} = (*eventWithCtx)(nil)
