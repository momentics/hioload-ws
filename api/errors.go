// Package api
// Author: momentics <momentics@gmail.com>
//
// Common error types and error handling utilities for hioload-ws library.

package api

import "fmt"

// Common errors used across the library.
var (
	ErrTransportClosed   = fmt.Errorf("transport is closed")
	ErrBufferPoolClosed  = fmt.Errorf("buffer pool is closed")
	ErrInvalidArgument   = fmt.Errorf("invalid argument")
	ErrResourceExhausted = fmt.Errorf("resource exhausted")
	ErrOperationTimeout  = fmt.Errorf("operation timeout")
	ErrNotSupported      = fmt.Errorf("operation not supported")
	ErrAlreadyExists     = fmt.Errorf("resource already exists")
	ErrNotFound          = fmt.Errorf("resource not found")
)

// ErrorCode represents specific error conditions in the library.
type ErrorCode int

const (
	ErrCodeOK ErrorCode = iota
	ErrCodeInvalidArgument
	ErrCodeResourceExhausted
	ErrCodeTimeout
	ErrCodeNotSupported
	ErrCodeAlreadyExists
	ErrCodeNotFound
	ErrCodeInternal
)

// Error represents a structured error with code and context.
type Error struct {
	Code    ErrorCode
	Message string
	Context map[string]any
}

// Error implements the error interface.
func (e *Error) Error() string {
	if len(e.Context) == 0 {
		return e.Message
	}
	return fmt.Sprintf("%s (context: %+v)", e.Message, e.Context)
}

// NewError creates a new structured error.
func NewError(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Context: make(map[string]any),
	}
}

// WithContext adds context information to the error.
func (e *Error) WithContext(key string, value any) *Error {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}
