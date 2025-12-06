// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// FakePoller implements api.Poller for testing.
type FakePoller struct {
	PollFunc      func(maxEvents int) (handled int, err error)
	RegisterFunc  func(h api.Handler) error
	UnregisterFunc func(h api.Handler) error
	StopFunc      func()
	PushFunc      func(ev concurrency.Event) bool
	PollCalls     []int // Track poll calls with their maxEvents parameter
	RegisterCalls []api.Handler
	StopCalled    bool
	PushCalled    int
}

// NewFakePoller creates a new fake poller.
func NewFakePoller() *FakePoller {
	return &FakePoller{
		PollCalls:     make([]int, 0),
		RegisterCalls: make([]api.Handler, 0),
	}
}

func (fp *FakePoller) Poll(maxEvents int) (handled int, err error) {
	fp.PollCalls = append(fp.PollCalls, maxEvents)
	if fp.PollFunc != nil {
		return fp.PollFunc(maxEvents)
	}
	return 0, nil
}

func (fp *FakePoller) Register(h api.Handler) error {
	fp.RegisterCalls = append(fp.RegisterCalls, h)
	if fp.RegisterFunc != nil {
		return fp.RegisterFunc(h)
	}
	return nil
}

func (fp *FakePoller) Unregister(h api.Handler) error {
	if fp.UnregisterFunc != nil {
		return fp.UnregisterFunc(h)
	}
	return nil
}

func (fp *FakePoller) Stop() {
	fp.StopCalled = true
	if fp.StopFunc != nil {
		fp.StopFunc()
	}
}

func (fp *FakePoller) Push(ev concurrency.Event) bool {
	fp.PushCalled++
	if fp.PushFunc != nil {
		return fp.PushFunc(ev)
	}
	return true
}

// Reset resets the call tracking.
func (fp *FakePoller) Reset() {
	fp.PollCalls = fp.PollCalls[:0]
	fp.RegisterCalls = fp.RegisterCalls[:0]
	fp.StopCalled = false
	fp.PushCalled = 0
}

// FakeExecutor implements api.Executor for testing.
type FakeExecutor struct {
	SubmitFunc   func(task func()) error
	StopFunc     func()
	SubmitCalls  []func()
	StopCalled   bool
}

// NewFakeExecutor creates a new fake executor.
func NewFakeExecutor() *FakeExecutor {
	return &FakeExecutor{
		SubmitCalls: make([]func(), 0),
	}
}

func (fe *FakeExecutor) Submit(task func()) error {
	fe.SubmitCalls = append(fe.SubmitCalls, task)
	if fe.SubmitFunc != nil {
		return fe.SubmitFunc(task)
	}
	return nil
}

func (fe *FakeExecutor) Stop() {
	fe.StopCalled = true
	if fe.StopFunc != nil {
		fe.StopFunc()
	}
}

// Reset resets the call tracking.
func (fe *FakeExecutor) Reset() {
	fe.SubmitCalls = fe.SubmitCalls[:0]
	fe.StopCalled = false
}