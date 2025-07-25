// File: api/handler.go
// Package api defines Handler interface.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// Handler processes data payloads.
type Handler interface {
	Handle(data any) error
}

// HandlerFunc — это функция с сигнатурой обработки, преобразующая
// func(data any) error в полноценный Handler.
type HandlerFunc func(data any) error

// Чтобы HandlerFunc соответствовал интерфейсу Handler, ему даётся метод Handle:
func (fn HandlerFunc) Handle(data any) error {
	return fn(data)
}
