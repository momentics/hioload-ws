// Package fake provides mock implementations for testing hioload-ws components.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package fake

// FakeHandler implements api.Handler for testing.
type FakeHandler struct {
	HandleFunc   func(data any) error
	HandleCalls  []any
	HandleReturn error
}

// NewFakeHandler creates a new fake handler.
func NewFakeHandler() *FakeHandler {
	return &FakeHandler{
		HandleCalls: make([]any, 0),
	}
}

func (fh *FakeHandler) Handle(data any) error {
	fh.HandleCalls = append(fh.HandleCalls, data)
	if fh.HandleFunc != nil {
		return fh.HandleFunc(data)
	}
	return fh.HandleReturn
}

// Reset resets the call tracking.
func (fh *FakeHandler) Reset() {
	fh.HandleCalls = fh.HandleCalls[:0]
}

// GetLastCall returns the last handled data.
func (fh *FakeHandler) GetLastCall() any {
	if len(fh.HandleCalls) == 0 {
		return nil
	}
	return fh.HandleCalls[len(fh.HandleCalls)-1]
}

// GetCallCount returns the number of calls.
func (fh *FakeHandler) GetCallCount() int {
	return len(fh.HandleCalls)
}