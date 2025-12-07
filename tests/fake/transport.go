package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakeTransport implements api.Transport for testing.
type FakeTransport struct {
	SendCalls [][][]byte  // Track what was sent
	RecvFunc  func() ([][]byte, error)
	RecvData  [][]byte    // Data to return on Recv
	closed    bool
	features  api.TransportFeatures
}

// NewFakeTransport creates a new fake transport.
func NewFakeTransport() *FakeTransport {
	return &FakeTransport{
		SendCalls: make([][][]byte, 0),
		RecvData:  make([][]byte, 0),
		features: api.TransportFeatures{
			ZeroCopy:  true,
			Batch:     true,
			NUMAAware: false,
			TLS:       false,
			OS:        []string{"test"},
		},
	}
}

func (ft *FakeTransport) Send(buffers [][]byte) error {
	ft.SendCalls = append(ft.SendCalls, buffers)
	return nil
}

func (ft *FakeTransport) Recv() ([][]byte, error) {
	if ft.RecvFunc != nil {
		return ft.RecvFunc()
	}
	if len(ft.RecvData) > 0 {
		data := ft.RecvData
		ft.RecvData = [][]byte{} // Clear after use
		return data, nil
	}
	return [][]byte{}, nil
}

func (ft *FakeTransport) Close() error {
	ft.closed = true
	return nil
}

func (ft *FakeTransport) Features() api.TransportFeatures {
	return ft.features
}

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