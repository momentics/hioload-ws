// File: api/handler.go
// Package api defines Handler interface.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// Handler processes data payloads.
type Handler interface {
	Handle(data any) error
}
