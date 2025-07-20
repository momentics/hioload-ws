// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// handler_middleware_test.go — Middleware chain, logging, recovery coverage.
package tests

import (
	"errors"
	"testing"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/api"
)

type failHandler struct{}

func (f failHandler) Handle(data any) error {
	return errors.New("expected error")
}

func TestMiddlewareHandler_RecoveryAndLogging(t *testing.T) {
	base := failHandler{}
	control := &mockControl{}
	mw := adapters.NewMiddlewareHandler(base).
		Use(adapters.LoggingMiddleware).
		Use(adapters.RecoveryMiddleware).
		Use(adapters.MetricsMiddleware(control))

	testData := "test"
	// Recovery middleware should handle panic, Logging should log
	err := mw.Handle(testData)
	if err == nil {
		t.Error("Expected error from failHandler, got nil")
	}
	// Metrics check
	if control.cfg["handler.processed"] != int64(1) {
		t.Error("MetricsMiddleware did not increment counter")
	}
}

// mockControl is a minimal Control API mock.
type mockControl struct {
	cfg map[string]any
}

func (m *mockControl) GetConfig() map[string]any { return m.cfg }
func (m *mockControl) SetConfig(cfg map[string]any) error {
	if m.cfg == nil { m.cfg = map[string]any{} }
	for k, v := range cfg { m.cfg[k] = v }
	return nil
}
func (m *mockControl) Stats() map[string]any    { return m.cfg }
func (m *mockControl) OnReload(fn func())       {}
func (m *mockControl) RegisterDebugProbe(string, func() any) {}
