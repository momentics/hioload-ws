// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

import (
	"context"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// TestContextWithTimeout creates a context with timeout for tests.
// Returns context and cancel function - caller should handle cancellation appropriately.
func TestContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// FakeEvent implements api.Event for testing.
type FakeEvent struct {
	data any
}

// NewFakeEvent creates a new fake event.
func NewFakeEvent(data any) *FakeEvent {
	return &FakeEvent{data: data}
}

func (fe *FakeEvent) Data() any {
	return fe.data
}

// FakeControl implements api.Control for testing.
type FakeControl struct {
	GetConfigFunc    func() map[string]any
	SetConfigFunc    func(cfg map[string]any) error
	StatsFunc        func() map[string]any
	OnReloadFunc     func(fn func())
	RegisterDebugFunc func(name string, fn func() any)
	config           map[string]any
}

// NewFakeControl creates a new fake control.
func NewFakeControl() *FakeControl {
	return &FakeControl{
		config: make(map[string]any),
	}
}

func (fc *FakeControl) GetConfig() map[string]any {
	if fc.GetConfigFunc != nil {
		return fc.GetConfigFunc()
	}
	return fc.config
}

func (fc *FakeControl) SetConfig(cfg map[string]any) error {
	if fc.SetConfigFunc != nil {
		return fc.SetConfigFunc(cfg)
	}
	fc.config = cfg
	return nil
}

func (fc *FakeControl) Stats() map[string]any {
	if fc.StatsFunc != nil {
		return fc.StatsFunc()
	}
	return fc.config
}

func (fc *FakeControl) OnReload(fn func()) {
	if fc.OnReloadFunc != nil {
		fc.OnReloadFunc(fn)
	}
}

func (fc *FakeControl) RegisterDebugProbe(name string, fn func() any) {
	if fc.RegisterDebugFunc != nil {
		fc.RegisterDebugFunc(name, fn)
	}
}

func (fc *FakeControl) GetDebug() api.Debug {
	// Return nil for testing purposes
	return nil
}